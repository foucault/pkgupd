package main

import "net"
import "bufio"

import "os"
import "fmt"
import "time"
import "sync"
import "pkgupd/alpm"
import "encoding/json"
import "pkgupd/log"
import "strings"
import "errors"
import fsnotify "github.com/go-fsnotify/fsnotify"

// Length of the maximum incoming request in bytes
const MaxRequestLength = 16384

// Server is the basic structure that listens for client requests
// and processes them. It also holds a list of enabled services
type Server struct {
	services    map[string]DataService
	closeMsg    chan bool
	waitGroup   *sync.WaitGroup
	serverError chan bool
	fswatch     *FSWatchService
}

// Response struct is used when marshaling json responses
// to the clients
type Response struct {
	ResponseType string      `json:"ResponseType"`
	Data         []*alpm.Pkg `json:"Data"`
}

// Request struct is used to unmarshal json requests from
// the clients
type Request struct {
	RequestType string `json:"RequestType"`
}

type deadliningListener interface {
	SetDeadline(time.Time) error
	Accept() (net.Conn, error)
	Close() error
}

// NewServer creates a new server instance. Set the argument to true to also
// enable filesystem notifications
func NewServer(notifyEnable bool) *Server {
	var watch *FSWatchService
	if !notifyEnable {
		watch = nil
	} else {
		w, err := NewFSWatchService([]string{"/var/lib/pacman",
			"/var/lib/pacman/local"}, fsnotify.Create|fsnotify.Remove)
		watch = w
		if err != nil {
			log.Errorf("Could not start filesystem watcher: %s; disabling\n", err)
			watch = nil
		} else {
			log.Infoln("Enabling filesystem watcher")
		}
	}
	s := &Server{make(map[string]DataService),
		make(chan bool), &sync.WaitGroup{}, make(chan bool), watch}
	return s
}

// AddService adds a new service to the server with the specified key.
// The service must implement the DataService interface
func (s *Server) AddService(key string, service DataService) {
	s.services[key] = service
}

// RemoveService removes a service from the server with the specified key
func (s *Server) RemoveService(key string) {
	if _, ok := s.services[key]; ok {
		s.services[key].Stop()
		delete(s.services, key)
	}
}

// Start starts the server. This only starts the slave services. To enable
// the tcp/unix interface use Server.Serve
func (s *Server) Start() {
	for _, service := range s.services {
		go service.Start()
		if s.fswatch != nil {
			s.fswatch.AddListener(service)
		}
	}
	if s.fswatch != nil {
		go s.fswatch.Start()
	}
}

// Stop stops the server. This stops the server immediately!
// Use Server.Wait() before issuing this command
func (s *Server) Stop() {
	for _, service := range s.services {
		service.Stop()
	}
	close(s.closeMsg)
	if s.fswatch != nil {
		s.fswatch.Stop()
	}
}

func (s *Server) createListener(proto string, addr string) (deadliningListener, error) {
	proto = strings.ToLower(proto)
	switch proto {
	case "tcp":
		taddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			log.Errorln("Failed to resolve tcp address", addr)
			return nil, err
		}
		log.Debugln("TCP address", addr, "created successfully")
		return net.ListenTCP("tcp", taddr)
	case "unix":
		_, err := os.Stat(addr)
		if err == nil {
			log.Warnln("Found old socket, will try to delete")
			er := os.Remove(addr)
			if er != nil {
				log.ErrorFatal("Failed to remove old socket, bailing out:", er)
			}
		}
		uaddr, err := net.ResolveUnixAddr("unix", addr)
		if err != nil {
			log.Errorln("Failed to resolve unix address", addr)
			return nil, err
		}
		log.Debugln("UNIX address", addr, "created successfully")
		listen, err := net.ListenUnix("unix", uaddr)
		if err == nil {
			defer func() {
				fi, er := os.Stat(addr)
				if (er == nil) && (fi.Mode()&os.ModeSocket != 0) {
					os.Chmod(addr, 0666)
					log.Infoln("Changing permissions for socket")
				} else {
					log.Infoln("Socket", addr, "created but is not a file")
				}
			}()
		}
		return listen, err
	}
	return nil, errors.New("Invalid protocol specified")
}

// Serve starts serving clients to the configured address
func (s *Server) Serve(proto string, addr string) {
	s.waitGroup.Add(1)
	defer s.waitGroup.Done()
	listener, err := s.createListener(proto, addr)
	if err != nil {
		log.Errorln("Failed to create listener:", err)
		s.serverError <- true
		return
	}
	s.serve(listener)
}

// Wait blocks until all connections are flushed. Use this before
// stopping the server
func (s *Server) Wait() {
	s.waitGroup.Wait()
}

func (s *Server) serve(listener deadliningListener) {
	defer listener.Close()
	for {
		listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := listener.Accept()
		select {
		case <-s.closeMsg:
			return
		default:
			//nothing
		}
		if err != nil {
			continue
		}
		go s.handleRequest(conn)
	}
}

func (s *Server) errorResponse(conn net.Conn, msg string) {
	conn.Write(append([]byte(fmt.Sprintf(`{"ResponseType": "error","Data": "%s"}`, msg)), '\n'))
}

func (s *Server) handleRequest(conn net.Conn) {
	defer conn.Close()
	log.Debugf("Handling request from %s\n", conn.RemoteAddr())
	bin := bufio.NewScanner(conn)
	var line []byte
	totalRead := 0
	for bin.Scan() {
		line = bin.Bytes()
		totalRead += len(line)
		// Stop reading if request length is too big
		if totalRead >= MaxRequestLength {
			s.errorResponse(conn, "request length exceeded")
			log.Debugf("Request from %s exceeded length %d\n",
				conn.RemoteAddr(), MaxRequestLength)
			break
		}
		if err := bin.Err(); err == nil {
			var req Request
			err = json.Unmarshal(line, &req)
			var data []*alpm.Pkg
			switch req.RequestType {
			case "repo":
				if v, ok := s.services["repo"]; ok {
					data = v.GetData().([]*alpm.Pkg)
				} else {
					s.errorResponse(conn, "invalid request")
					break
				}
			case "aur":
				if v, ok := s.services["aur"]; ok {
					data = v.GetData().([]*alpm.Pkg)
				} else {
					s.errorResponse(conn, "invalid request")
					break
				}
			default:
				s.errorResponse(conn, "invalid request")
				break
			}
			resp := &Response{"ok", data}
			respString, err := json.Marshal(resp)
			if err != nil {
				s.errorResponse(conn, "could not marshal json")
			} else {
				respString = append(respString, '\n')
				conn.Write(respString)
			}
		} else {
			log.Warnln(err)
			break
		}
	}
	log.Debugf("Connection from %s handled successfully\n", conn.RemoteAddr())
}
