// Package server provides the Unix domain socket server for key-rest-daemon.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/koteitan/key-rest/internal/proxy"
)

const maxRequestSize = 10 * 1024 * 1024 // 10 MB
const maxConcurrentConns = 64

// ReloadFunc is called when a "reload" request is received.
type ReloadFunc func() error

// Server listens on a Unix domain socket and handles proxy requests.
type Server struct {
	socketPath    string
	proxy         *proxy.Proxy
	listener      net.Listener
	wg            sync.WaitGroup
	quit          chan struct{}
	connSem       chan struct{} // semaphore for limiting concurrent connections
	ReloadHandler ReloadFunc
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

		if req.Type == "reload" {
			s.handleReload(conn)
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

func (s *Server) writeResponse(conn net.Conn, resp *proxy.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	conn.Write(data)
}
