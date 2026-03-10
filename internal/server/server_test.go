package server

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/proxy"
)

func setupServer(t *testing.T, ts *httptest.Server) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	store, _ := keystore.New(dir)
	pass := []byte("pass")
	store.Add("user1/test/key", ts.URL+"/", false, false, []byte("real-key"), pass)
	store.DecryptAll(pass)

	p := proxy.NewWithClient(store, ts.Client())
	srv := New(socketPath, p)
	return srv, socketPath
}

func TestServerStartStop(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	srv, socketPath := setupServer(t, ts)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify socket exists
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatalf("failed to connect to socket: %v", err)
	}
	conn.Close()

	srv.Stop()
}

func TestServerHandleRequest(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer real-key" {
			t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer ts.Close()

	srv, socketPath := setupServer(t, ts)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send request
	req := proxy.Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/data",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/test/key",
		},
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	conn.Write(data)

	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}

	var resp proxy.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Status != 200 {
		t.Fatalf("expected 200, got %d", resp.Status)
	}
	if resp.Body != `{"result":"ok"}` {
		t.Fatalf("unexpected body: %s", resp.Body)
	}
}

func TestServerInvalidJSON(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	srv, socketPath := setupServer(t, ts)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	conn.Write([]byte("not json\n"))

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}

	var resp proxy.Response
	json.Unmarshal(scanner.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", resp.Error.Code)
	}
}
