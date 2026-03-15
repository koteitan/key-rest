package proxy

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

func TestAttack_GzipMaskingBypass(t *testing.T) {
	const credential = "sk-test-GZIP-secret-key-999"

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			gz := gzip.NewWriter(w)
			gz.Write([]byte(fmt.Sprintf(`{"echoed":"%s"}`, token)))
			gz.Close()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf(`{"echoed":"%s"}`, token)))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/gzip/key", ts.URL+"/", false, false, nil, []byte(credential), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api/data",
		Headers: map[string]string{
			"Authorization":   "Bearer key-rest://user1/gzip/key",
			"Accept-Encoding": "gzip",
		},
	})

	if resp.Error != nil {
		t.Fatalf("request failed: %s", resp.Error.Message)
	}

	// Credential must not appear in the response body
	if strings.Contains(resp.Body, credential) {
		t.Fatal("EXFILTRATED: credential recovered from gzip response")
	}

	// Credential should be masked to key-rest:// URI
	if !strings.Contains(resp.Body, "key-rest://user1/gzip/key") {
		t.Fatal("credential was not masked to key-rest:// URI in decompressed response")
	}

	// Content-Encoding should be removed (body is decompressed)
	if resp.Headers["Content-Encoding"] == "gzip" {
		t.Fatal("Content-Encoding: gzip should be removed after decompression")
	}

	t.Log("Defense held: gzip response was decompressed and credential was masked")
}
