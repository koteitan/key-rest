package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

func testTLSConfig(ts *httptest.Server) (*tls.Config, string) {
	certPool := x509.NewCertPool()
	certPool.AddCert(ts.Certificate())
	return &tls.Config{RootCAs: certPool}, ts.Listener.Addr().String()
}

func setupProxy(t *testing.T) (*Proxy, *keystore.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := keystore.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	pass := []byte("test-pass")
	store.Add("user1/test/api-key", "https://", false, false, []byte("real-api-key"), pass)
	store.Add("user1/test/url-key", "https://", true, false, []byte("url-key-val"), pass)
	store.Add("user1/test/body-key", "https://", false, true, []byte("body-key-val"), pass)
	store.DecryptAll(pass)
	return New(store), store
}

func TestHandleBasicRequest(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer real-api-key" {
			t.Errorf("unexpected auth header: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	store.Add("user1/ts/key", ts.URL+"/", false, false, []byte("real-api-key"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/data",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/ts/key",
		},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Status != 200 {
		t.Fatalf("expected 200, got %d", resp.Status)
	}

	var body map[string]interface{}
	json.Unmarshal([]byte(resp.Body), &body)
	if body["ok"] != true {
		t.Fatalf("unexpected body: %s", resp.Body)
	}
}

func TestHandleKeyNotFound(t *testing.T) {
	p, _ := setupProxy(t)
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "https://example.com/",
		Headers: map[string]string{
			"Authorization": "key-rest://nonexistent/key",
		},
	})

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != "KEY_NOT_FOUND" {
		t.Fatalf("expected KEY_NOT_FOUND, got %s", resp.Error.Code)
	}
}

func TestHandleFieldRestrictionURL(t *testing.T) {
	p, _ := setupProxy(t)

	// user1/test/api-key has allow_url=false, so using it in URL should fail
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "https://example.com/?key=key-rest://user1/test/api-key",
	})

	if resp.Error == nil {
		t.Fatal("expected field restriction error")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}
}

func TestHandleFieldRestrictionBody(t *testing.T) {
	p, _ := setupProxy(t)

	body := `{"api_key": "key-rest://user1/test/api-key"}`
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    "https://example.com/",
		Body:   &body,
	})

	if resp.Error == nil {
		t.Fatal("expected field restriction error")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}
}

func TestHandleAllowURL(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "url-key-val" {
			t.Errorf("unexpected query param: %s", r.URL.Query().Get("key"))
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/url-key", ts.URL+"/", true, false, []byte("url-key-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/?key=key-rest://user1/url-key",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
}

func TestHandleAllowBody(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)
		expected := `{"api_key": "body-key-val"}`
		if bodyStr != expected {
			t.Errorf("unexpected body: got %q, want %q", bodyStr, expected)
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/body-key", ts.URL+"/", false, true, []byte("body-key-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	body := `{"api_key": "key-rest://user1/body-key"}`
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/",
		Body:   &body,
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
}

func TestHandleHTTPRejected(t *testing.T) {
	p, _ := setupProxy(t)
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "http://example.com/",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/test/api-key",
		},
	})

	if resp.Error == nil {
		t.Fatal("expected INSECURE_REQUEST error")
	}
	if resp.Error.Code != "INSECURE_REQUEST" {
		t.Fatalf("expected INSECURE_REQUEST, got %s", resp.Error.Code)
	}
}

func TestHandleURLPrefixMismatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/brave/key", "https://api.search.brave.com/", false, false, []byte("brave-key"), pass)
	store.DecryptAll(pass)
	p := New(store)

	// Request URL does not match the key's url_prefix
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "https://evil.com/steal",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/brave/key",
		},
	})

	if resp.Error == nil {
		t.Fatal("expected URL_PREFIX_MISMATCH error")
	}
	if resp.Error.Code != "URL_PREFIX_MISMATCH" {
		t.Fatalf("expected URL_PREFIX_MISMATCH, got %s", resp.Error.Code)
	}
}

func TestHasURLPrefix(t *testing.T) {
	tests := []struct {
		name       string
		requestURL string
		prefix     string
		want       bool
	}{
		{"trailing slash match", "https://api.openai.com/v1/chat", "https://api.openai.com/", true},
		{"trailing slash no match", "https://api.openai.com.evil.com/", "https://api.openai.com/", false},
		{"no trailing slash with /", "https://api.openai.com/v1/chat", "https://api.openai.com", true},
		{"subdomain attack blocked", "https://api.openai.com.evil.com/steal", "https://api.openai.com", false},
		{"no trailing slash with ?", "https://api.openai.com?foo=bar", "https://api.openai.com", true},
		{"no trailing slash with #", "https://api.openai.com#section", "https://api.openai.com", true},
		{"exact match", "https://api.openai.com", "https://api.openai.com", true},
		{"path boundary blocked", "https://api.example.com/v1/chatgpt", "https://api.example.com/v1/chat", false},
		{"path boundary match", "https://api.example.com/v1/chat/completions", "https://api.example.com/v1/chat", true},
		{"completely different", "https://evil.com/", "https://api.openai.com/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasURLPrefix(tt.requestURL, tt.prefix)
			if got != tt.want {
				t.Errorf("hasURLPrefix(%q, %q) = %v, want %v", tt.requestURL, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestHandleResponseMasking(t *testing.T) {
	// Upstream echoes back the Authorization header (like httpbin.org/anything)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Echo-Auth", auth)
		w.WriteHeader(200)
		w.Write([]byte(`{"headers":{"Authorization":"` + auth + `"}}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/echo/key", ts.URL+"/", false, false, []byte("SECRET-VALUE-123"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/anything",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/echo/key",
		},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	// Verify credential is NOT in response body
	if strings.Contains(resp.Body, "SECRET-VALUE-123") {
		t.Fatal("credential leaked in response body")
	}
	// Verify credential is replaced with key-rest:// URI
	if !strings.Contains(resp.Body, "key-rest://user1/echo/key") {
		t.Fatal("credential was not reverse-substituted in response body")
	}

	// Verify credential is NOT in response headers
	if echoAuth, ok := resp.Headers["X-Echo-Auth"]; ok {
		if strings.Contains(echoAuth, "SECRET-VALUE-123") {
			t.Fatal("credential leaked in response header")
		}
	}
}

func TestHandleUserinfoRejected(t *testing.T) {
	p, _ := setupProxy(t)
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "https://api.example.com@evil.com/steal",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/test/api-key",
		},
	})

	if resp.Error == nil {
		t.Fatal("expected error for URL with userinfo")
	}
	if resp.Error.Code != "INSECURE_REQUEST" {
		t.Fatalf("expected INSECURE_REQUEST, got %s", resp.Error.Code)
	}
}

func TestHandleInvalidType(t *testing.T) {
	p, _ := setupProxy(t)
	resp := p.Handle(&Request{
		Type:   "websocket",
		Method: "GET",
		URL:    "https://example.com/",
	})

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", resp.Error.Code)
	}
}
