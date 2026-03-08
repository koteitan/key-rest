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

// Server listens on a Unix domain socket and handles proxy requests.
type Server struct {
	socketPath string
	proxy      *proxy.Proxy
	listener   net.Listener
	wg         sync.WaitGroup
	quit       chan struct{}
}

// New creates a new Server.
func New(socketPath string, p *proxy.Proxy) *Server {
	return &Server{
		socketPath: socketPath,
		proxy:      p,
		quit:       make(chan struct{}),
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

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
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

		resp := s.proxy.Handle(req)
		s.writeResponse(conn, resp)
	}
}

func (s *Server) writeResponse(conn net.Conn, resp *proxy.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	conn.Write(data)
}
