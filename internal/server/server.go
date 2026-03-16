// Package server provides the Unix domain socket server for key-rest-daemon.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/proxy"
)

const maxRequestSize = 10 * 1024 * 1024 // 10 MB
const maxConcurrentConns = 64

// ReloadFunc is called when a "reload" request is received.
type ReloadFunc func() error

// EnableFunc is called when an "enable" request is received.
type EnableFunc func(uriPrefix string) (int, error)

// DisableFunc is called when a "disable" request is received.
type DisableFunc func(uriPrefix string) int

// ListFunc is called when a "list" request is received.
type ListFunc func() []keystore.KeyStatus

// Server listens on a Unix domain socket and handles proxy requests.
type Server struct {
	socketPath     string
	proxy          *proxy.Proxy
	listener       net.Listener
	wg             sync.WaitGroup
	quit           chan struct{}
	connSem        chan struct{} // semaphore for limiting concurrent connections
	ReloadHandler  ReloadFunc
	EnableHandler  EnableFunc
	DisableHandler DisableFunc
	ListHandler    ListFunc
	Version        string
}

// New creates a new Server.
func New(socketPath string, p *proxy.Proxy) *Server {
	return &Server{
		socketPath: socketPath,
		proxy:      p,
		quit:       make(chan struct{}),
		connSem:    make(chan struct{}, maxConcurrentConns),
	}
}

// Start begins listening on the Unix socket.
func (s *Server) Start() error {
	// Remove stale socket file
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.socketPath, err)
	}

	// Set socket permissions to owner-only
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		ln.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.listener = ln

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	close(s.quit)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	os.Remove(s.socketPath)
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				continue
			}
		}

		// Limit concurrent connections
		select {
		case s.connSem <- struct{}{}:
		default:
			// At capacity — reject connection
			conn.Close()
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.connSem }()
			s.handleConnection(conn)
		}()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), maxRequestSize)

	for scanner.Scan() {
		select {
		case <-s.quit:
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		req, err := proxy.ParseRequest(line)
		if err != nil {
			resp := &proxy.Response{
				Error: &proxy.ErrorInfo{
					Code:    "INVALID_REQUEST",
					Message: "failed to parse request: " + err.Error(),
				},
			}
			s.writeResponse(conn, resp)
			continue
		}

		switch req.Type {
		case "reload":
			s.handleReload(conn)
			continue
		case "enable":
			s.handleEnableDisable(conn, line, true)
			continue
		case "disable":
			s.handleEnableDisable(conn, line, false)
			continue
		case "list":
			s.handleList(conn)
			continue
		case "version":
			s.writeResponse(conn, &proxy.Response{Status: 200, Body: s.Version})
			continue
		}

		resp := s.proxy.Handle(req)
		s.writeResponse(conn, resp)
	}
}

func (s *Server) handleReload(conn net.Conn) {
	if s.ReloadHandler == nil {
		s.writeResponse(conn, &proxy.Response{
			Error: &proxy.ErrorInfo{Code: "INTERNAL_ERROR", Message: "reload not supported"},
		})
		return
	}
	if err := s.ReloadHandler(); err != nil {
		s.writeResponse(conn, &proxy.Response{
			Error: &proxy.ErrorInfo{Code: "RELOAD_FAILED", Message: err.Error()},
		})
		return
	}
	s.writeResponse(conn, &proxy.Response{Status: 200, Body: "reloaded"})
}

func (s *Server) handleEnableDisable(conn net.Conn, line []byte, enable bool) {
	var parsed struct {
		URIPrefix string `json:"uri_prefix"`
	}
	if err := json.Unmarshal(line, &parsed); err != nil || parsed.URIPrefix == "" {
		s.writeResponse(conn, &proxy.Response{
			Error: &proxy.ErrorInfo{Code: "INVALID_REQUEST", Message: "uri_prefix is required"},
		})
		return
	}

	if enable {
		if s.EnableHandler == nil {
			s.writeResponse(conn, &proxy.Response{
				Error: &proxy.ErrorInfo{Code: "INTERNAL_ERROR", Message: "enable not supported"},
			})
			return
		}
		count, err := s.EnableHandler(parsed.URIPrefix)
		if err != nil {
			s.writeResponse(conn, &proxy.Response{
				Error: &proxy.ErrorInfo{Code: "ENABLE_FAILED", Message: err.Error()},
			})
			return
		}
		s.writeResponse(conn, &proxy.Response{Status: 200, Body: fmt.Sprintf("%d", count)})
	} else {
		if s.DisableHandler == nil {
			s.writeResponse(conn, &proxy.Response{
				Error: &proxy.ErrorInfo{Code: "INTERNAL_ERROR", Message: "disable not supported"},
			})
			return
		}
		count := s.DisableHandler(parsed.URIPrefix)
		s.writeResponse(conn, &proxy.Response{Status: 200, Body: fmt.Sprintf("%d", count)})
	}
}

func (s *Server) handleList(conn net.Conn) {
	if s.ListHandler == nil {
		s.writeResponse(conn, &proxy.Response{
			Error: &proxy.ErrorInfo{Code: "INTERNAL_ERROR", Message: "list not supported"},
		})
		return
	}
	statuses := s.ListHandler()
	body, err := json.Marshal(statuses)
	if err != nil {
		s.writeResponse(conn, &proxy.Response{
			Error: &proxy.ErrorInfo{Code: "INTERNAL_ERROR", Message: err.Error()},
		})
		return
	}
	s.writeResponse(conn, &proxy.Response{Status: 200, Body: string(body)})
}

func (s *Server) writeResponse(conn net.Conn, resp *proxy.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	conn.Write(data)
}
