package proxy

// Penetration tests: attempt to exfiltrate credentials through masking bypasses
// and protocol-level attacks.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

// ==========================================================================
// Attack 1: URL-encoded credential bypass
//
// If the upstream server returns the credential in URL-encoded form,
// maskCredentials (which only checks raw and JSON-escaped forms) will miss it.
// The agent can URL-decode the response to recover the credential.
// ==========================================================================

func TestAttack_URLEncodedMaskingBypass(t *testing.T) {
	const credential = "sk-test-SECRET-key-123"

	// Mock server: receives credential in header, returns it URL-encoded in response body
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		// Server URL-encodes the credential (e.g., in a redirect URL or debug info)
		encoded := url.QueryEscape(token)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf(`{"debug_url":"https://example.com/callback?token=%s"}`, encoded)))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/victim/key", ts.URL+"/", false, false, []byte(credential), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api/data",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/victim/key",
		},
	})

	if resp.Error != nil {
		t.Fatalf("request failed: %s", resp.Error.Message)
	}

	// Attack: extract URL-encoded credential from response and decode it
	urlEncoded := url.QueryEscape(credential)
	if strings.Contains(resp.Body, urlEncoded) {
		// URL-encoded credential found in response — masking was bypassed!
		decoded, _ := url.QueryUnescape(urlEncoded)
		t.Logf("EXFILTRATED: credential recovered via URL-encoding bypass: %s", decoded)
		t.Fatal("VULNERABILITY: URL-encoded credential leaked through response masking")
	}

	// If we get here, masking caught the URL-encoded form (defense worked)
	t.Log("Defense held: URL-encoded credential was masked")
}

// ==========================================================================
// Attack 2: Double-JSON-encoded credential bypass
//
// If a credential contains JSON-special characters (", \) and the server
// returns a JSON string containing a JSON-encoded value (double encoding),
// the double-escaped form is not caught by maskCredentials.
//
// maskCredentials handles:  raw form and single-JSON-escaped form
// maskCredentials misses:   double-JSON-escaped form (\\\" for \")
// ==========================================================================

func TestAttack_DoubleJSONEncodingBypass(t *testing.T) {
	// Credential with JSON-special characters
	const credential = `secret"key\with"quotes`

	// Mock server: receives credential, embeds it in a JSON-within-JSON response
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")

		// Server creates a JSON string, then embeds that string in another JSON field
		// This creates double-escaped characters
		innerJSON, _ := json.Marshal(map[string]string{"token": token})
		outerJSON, _ := json.Marshal(map[string]string{"nested_data": string(innerJSON)})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(outerJSON)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/victim/key", ts.URL+"/", false, false, []byte(credential), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api/data",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/victim/key",
		},
	})

	if resp.Error != nil {
		t.Fatalf("request failed: %s", resp.Error.Message)
	}

	// The double-escaped form: \" becomes \\\", \\ becomes \\\\
	// Check if any form of the credential remains in the response
	doubleEscaped := `secret\\\"key\\with\\\"quotes`
	if strings.Contains(resp.Body, doubleEscaped) {
		t.Logf("EXFILTRATED: double-JSON-escaped credential found in response")
		t.Fatal("VULNERABILITY: double-JSON-encoded credential leaked through response masking")
	}

	// Also check: is the raw credential absent?
	if strings.Contains(resp.Body, credential) {
		t.Fatal("VULNERABILITY: raw credential leaked in response body")
	}

	t.Log("Defense held: double-JSON-encoded credential was masked")
}

// ==========================================================================
// Attack 3: CRLF injection in URL
//
// The proxy checks for CRLF in resolved HEADER values (sectransport.go)
// but does NOT check the URL. The raw URL is used directly in the HTTP
// request buffer, allowing HTTP request smuggling via CRLF in the URL.
// ==========================================================================

func TestAttack_CRLFInjectionInURL(t *testing.T) {
	const credential = "sk-test-CRLF-attack"

	// Mock server that records what it receives
	var receivedRequests []string
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequests = append(receivedRequests, r.Method+" "+r.URL.String())
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/victim/key", ts.URL+"/", false, false, []byte(credential), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Inject CRLF into the URL to smuggle additional headers
	// The " HTTP/1.1\r\n" terminates the request line, then we inject headers
	maliciousURL := ts.URL + "/api/v1 HTTP/1.1\r\nX-Injected: pwned\r\n\r\nGET /smuggled"

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    maliciousURL,
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/victim/key",
		},
	})

	// If the proxy doesn't reject CRLF in URLs, it's a vulnerability
	if resp.Error == nil {
		t.Log("CRLF injection in URL was NOT rejected by the proxy")
		t.Log("The server received these requests:")
		for i, req := range receivedRequests {
			t.Logf("  Request %d: %s", i+1, req)
		}
		// Check if multiple requests were received (request smuggling)
		if len(receivedRequests) > 1 {
			t.Fatal("VULNERABILITY: HTTP request smuggling via CRLF injection in URL")
		}
		t.Fatal("VULNERABILITY: CRLF in URL not rejected (potential HTTP header injection)")
	}

	t.Logf("Defense held: CRLF in URL was rejected: %s", resp.Error.Message)
}

// ==========================================================================
// Attack 4: Path traversal URL prefix bypass
//
// The URL prefix check is a simple string prefix match on the raw URL
// (before path normalization). If the server normalizes paths (e.g., Go's
// http.Server resolves ".."), a request can pass the prefix check for
// one service but be routed to a different handler by the server.
// ==========================================================================

func TestAttack_PathTraversalPrefixBypass(t *testing.T) {
	const credential = "sk-test-TRAVERSAL"

	// Mock server with two handlers: /legitimate/ (auth check) and /echo/ (reflects headers)
	mux := http.NewServeMux()
	mux.HandleFunc("/legitimate/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf(`{"echoed_auth":"%s"}`, auth)))
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	// Key is only allowed for /legitimate/ prefix
	store.Add("user1/legit/key", ts.URL+"/legitimate/", false, false, []byte(credential), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Try path traversal: URL starts with /legitimate/ (passes prefix check)
	// but the server may normalize to /echo/
	traversalURL := ts.URL + "/legitimate/../echo/test"

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    traversalURL,
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/legit/key",
		},
	})

	if resp.Error != nil {
		t.Logf("Path traversal was rejected: %s", resp.Error.Message)
		t.Log("Defense held: path traversal was blocked")
		return
	}

	// Check if the response came from /echo/ (path traversal succeeded)
	if strings.Contains(resp.Body, "echoed_auth") {
		// Path traversal reached the echo handler!
		// Check if credential is visible (or masked)
		if strings.Contains(resp.Body, credential) {
			t.Fatal("VULNERABILITY: path traversal + credential leak — credential visible in echo response")
		}
		if strings.Contains(resp.Body, "key-rest://") {
			t.Log("Path traversal reached echo handler, but credential was masked")
		}
	}

	// Check if it's a redirect (Go normalizes ".." paths)
	if resp.Status == 301 || resp.Status == 302 {
		t.Logf("Server sent redirect (status %d) — path traversal neutralized by server", resp.Status)
		// Check: does the redirect Location header leak any info?
		if loc, ok := resp.Headers["Location"]; ok {
			t.Logf("Redirect Location: %s", loc)
			if strings.Contains(loc, credential) {
				t.Fatal("VULNERABILITY: credential leaked in redirect Location header")
			}
		}
	}

	t.Logf("Response status: %d, body: %s", resp.Status, resp.Body)
}
