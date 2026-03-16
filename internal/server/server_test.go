package server

import (
	"bufio"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"crypto/tls"

	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/proxy"
)

func setupServer(t *testing.T, ts *httptest.Server) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	store, _ := keystore.New(dir)
	pass := []byte("pass")
	store.Add("user1/test/key", ts.URL+"/", false, false, nil, []byte("real-key"), pass)
	store.DecryptAll(pass)

	certPool := x509.NewCertPool()
	certPool.AddCert(ts.Certificate())
	tlsConfig := &tls.Config{RootCAs: certPool}

	p := proxy.NewForTest(store, tlsConfig, ts.Listener.Addr().String())
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

func TestServerDisableEnable(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	srv, socketPath := setupServer(t, ts)
	srv.DisableHandler = func(uriPrefix string) int { return 1 }
	srv.EnableHandler = func(uriPrefix string) (int, error) { return 1, nil }
	srv.ListHandler = func() []keystore.KeyStatus {
		return []keystore.KeyStatus{
			{URI: "user1/test/key", URLPrefix: ts.URL + "/", Disabled: false},
		}
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	sendAndCheck := func(reqJSON string) proxy.Response {
		t.Helper()
		conn, err := net.DialTimeout("unix", socketPath, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.Write([]byte(reqJSON + "\n"))
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		scanner := bufio.NewScanner(conn)
		if !scanner.Scan() {
			t.Fatal("no response")
		}
		var resp proxy.Response
		json.Unmarshal(scanner.Bytes(), &resp)
		return resp
	}

	// Test disable
	resp := sendAndCheck(`{"type":"disable","uri_prefix":"user1/test"}`)
	if resp.Error != nil {
		t.Fatalf("disable failed: %s", resp.Error.Message)
	}
	if resp.Body != "1" {
		t.Fatalf("expected body '1', got %q", resp.Body)
	}

	// Test enable
	resp = sendAndCheck(`{"type":"enable","uri_prefix":"user1/test"}`)
	if resp.Error != nil {
		t.Fatalf("enable failed: %s", resp.Error.Message)
	}
	if resp.Body != "1" {
		t.Fatalf("expected body '1', got %q", resp.Body)
	}

	// Test list
	resp = sendAndCheck(`{"type":"list"}`)
	if resp.Error != nil {
		t.Fatalf("list failed: %s", resp.Error.Message)
	}
	if resp.Body == "" || resp.Body == "null" {
		t.Fatal("expected non-empty list body")
	}

	// Test disable without uri_prefix
	resp = sendAndCheck(`{"type":"disable"}`)
	if resp.Error == nil {
		t.Fatal("expected error for missing uri_prefix")
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
