package keyrest

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientDo(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	// Start a mock daemon
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			var req daemonRequest
			json.Unmarshal(scanner.Bytes(), &req)

			resp := daemonResponse{
				Status:     200,
				StatusText: "200 OK",
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"ok":true}`,
			}
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			conn.Write(data)
		}
	}()

	client := &Client{SocketPath: socketPath}
	req, _ := NewRequest("GET", "https://api.example.com/data", nil)
	req.Header.Set("Authorization", "Bearer key-rest://user1/test/key")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestClientError(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	ln, _ := net.Listen("unix", socketPath)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			resp := daemonResponse{
				Error: &struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    "KEY_NOT_FOUND",
					Message: "key 'user1/test/key' not found",
				},
			}
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			conn.Write(data)
		}
	}()

	client := &Client{SocketPath: socketPath}
	req, _ := NewRequest("GET", "https://api.example.com/data", nil)

	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "KEY_NOT_FOUND") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientConnectionError(t *testing.T) {
	client := &Client{SocketPath: "/nonexistent/socket.sock"}
	req, _ := NewRequest("GET", "https://example.com/", nil)

	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".key-rest", "key-rest.sock")
	if client.SocketPath != expected {
		t.Fatalf("expected %s, got %s", expected, client.SocketPath)
	}
}

func TestClientPost(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	ln, _ := net.Listen("unix", socketPath)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			var req daemonRequest
			json.Unmarshal(scanner.Bytes(), &req)

			if req.Method != "POST" {
				return
			}
			if req.Body == nil || *req.Body != `{"query":"test"}` {
				return
			}

			resp := daemonResponse{
				Status:     200,
				StatusText: "200 OK",
				Body:       `{"result":"ok"}`,
			}
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			conn.Write(data)
		}
	}()

	client := &Client{SocketPath: socketPath}
	resp, err := client.Post("https://api.example.com/search", "application/json", strings.NewReader(`{"query":"test"}`))
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
