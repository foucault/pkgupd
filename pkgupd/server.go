package main

import "net"
import "bufio"

import "fmt"
import "time"
import "sync"
import "pkgupd/alpm"
import "encoding/json"

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

func (s *Server) Serve() {
	s.waitGroup.Add(1)
	defer s.waitGroup.Done()
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":7356")
	if err != nil {
		s.ServerError <- true
		return
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
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
	bin := bufio.NewReader(conn)
	line, err := bin.ReadBytes('\n')
	if err == nil {
		var req Request
		err = json.Unmarshal(line, &req)
		var data []*alpm.Pkg
		switch req.RequestType {
		case "repo":
			if v, ok := s.services["repo"]; ok {
				data = v.GetData().([]*alpm.Pkg)
			} else {
				s.errorResponse(conn, "invalid request")
			}
		case "aur":
			if v, ok := s.services["aur"]; ok {
				data = v.GetData().([]*alpm.Pkg)
			} else {
				s.errorResponse(conn, "invalid request")
			}
		default:
			s.errorResponse(conn, "invalid request")
			return
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
		fmt.Println(err)
		conn.Write(append([]byte(`{"ResponseType": "error","Data": "invalid request"}`), '\n'))
	}
}
