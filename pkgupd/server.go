package main

import "net"
import "bufio"

import "fmt"
import "time"
import "sync"
import "pkgupd/alpm"
import "encoding/json"
import "pkgupd/log"
import "strings"
import "errors"

const MAX_REQUEST_LENGTH = 16384

type Server struct {
	services    map[string]ServiceRunner
	closeMsg    chan bool
	waitGroup   *sync.WaitGroup
	ServerError chan bool
}

type Response struct {
	ResponseType string      `json:"ResponseType"`
	Data         []*alpm.Pkg `json:"Data"`
}

type Request struct {
	RequestType string `json:"RequestType"`
}

type deadliningListener interface {
	SetDeadline(time.Time) error
	Accept() (net.Conn, error)
	Close() error
}

func NewServer() *Server {
	s := &Server{make(map[string]ServiceRunner),
		make(chan bool), &sync.WaitGroup{}, make(chan bool)}
	return s
}

func (s *Server) AddService(key string, service ServiceRunner) {
	s.services[key] = service
}

func (s *Server) RemoveService(key string) {
	if _, ok := s.services[key]; ok {
		s.services[key].Stop()
		delete(s.services, key)
	}
}

func (s *Server) Start() {
	for _, service := range s.services {
		go service.Start()
	}
}

func (s *Server) Stop() {
	for _, service := range s.services {
		service.Stop()
	}
	close(s.closeMsg)
}

func (s *Server) createListener(proto string, addr string) (deadliningListener, error) {
	proto = strings.ToLower(proto)
	switch proto {
	case "tcp":
		taddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			log.Errorln("Failed to resolve tcp address", addr)
			return nil, err
		} else {
			log.Debugln("TCP address", addr, "created successfully")
			return net.ListenTCP("tcp", taddr)
		}
	case "unix":
		uaddr, err := net.ResolveUnixAddr("unix", addr)
		if err != nil {
			log.Errorln("Failed to resolve unix address", addr)
			return nil, err
		} else {
			log.Debugln("UNIX address", addr, "created successfully")
			return net.ListenUnix("unix", uaddr)
		}
	}
	return nil, errors.New("Invalid protocol specified")
}

func (s *Server) Serve(proto string, addr string) {
	s.waitGroup.Add(1)
	defer s.waitGroup.Done()
	listener, err := s.createListener(proto, addr)
	if err != nil {
		log.Errorln("Failed to create listener:", err)
		s.ServerError <- true
		return
	}
	s.serve(listener)
}

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
		if totalRead >= MAX_REQUEST_LENGTH {
			s.errorResponse(conn, "request length exceeded")
			log.Debugf("Request from %s exceeded length %d\n",
				conn.RemoteAddr(), MAX_REQUEST_LENGTH)
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
