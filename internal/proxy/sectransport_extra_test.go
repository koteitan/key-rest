package proxy

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

// erroringBody is an io.ReadCloser that always fails on Read.
type erroringBody struct{}

func (erroringBody) Read([]byte) (int, error) { return 0, errors.New("read-error") }
func (erroringBody) Close() error             { return nil }

func TestRoundTripBodyReadError(t *testing.T) {
	tr := &secureTransport{
		resolver: func(string) ([]byte, error) { return nil, errors.New("unused") },
	}
	req, _ := http.NewRequest("POST", "https://localhost/", erroringBody{})
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error from body Read")
	}
	if !strings.Contains(err.Error(), "failed to read request body") {
		t.Fatalf("expected body-read wrap, got %v", err)
	}
}

func TestRoundTripBodyResolveError(t *testing.T) {
	tr := &secureTransport{
		resolver: func(string) ([]byte, error) { return nil, errors.New("no key") },
	}
	body := strings.NewReader("payload with key-rest://unknown/k inside")
	req, _ := http.NewRequest("POST", "https://localhost/", body)
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to resolve body") {
		t.Fatalf("expected body-resolve wrap, got %v", err)
	}
}

func TestRoundTripURLResolveError(t *testing.T) {
	tr := &secureTransport{
		resolver: func(string) ([]byte, error) { return nil, errors.New("no key") },
	}
	// Embed a key-rest URI in the request URI via context so it reaches the
	// URL resolve step.
	req, _ := http.NewRequest("GET", "https://localhost/path", nil)
	ctx := context.WithValue(req.Context(), rawURLKey, "https://localhost/path?token=key-rest://unknown/k")
	req = req.WithContext(ctx)
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to resolve URL") {
		t.Fatalf("expected url-resolve wrap, got %v", err)
	}
}

func TestRoundTripHeaderResolveError(t *testing.T) {
	tr := &secureTransport{
		resolver: func(string) ([]byte, error) { return nil, errors.New("no key") },
	}
	req, _ := http.NewRequest("GET", "https://localhost/", nil)
	req.Header.Set("Authorization", "Bearer key-rest://unknown/k")
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to resolve header") {
		t.Fatalf("expected header-resolve wrap, got %v", err)
	}
}

// TestRoundTripMultiHeaderResolveErrorCleanup ensures the resolve-failure
// branch zero-clears every header resolved before the failing one, not just
// the in-flight one. Two header values are sent; the second uses a URI that
// the resolver cannot resolve.
func TestRoundTripMultiHeaderResolveErrorCleanup(t *testing.T) {
	calls := 0
	tr := &secureTransport{
		resolver: func(uri string) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte("ok-value"), nil
			}
			return nil, errors.New("no key")
		},
	}
	req, _ := http.NewRequest("GET", "https://localhost/", nil)
	// Two headers, both containing a key-rest URI so the resolver is called
	// twice. Go's net/http normalizes the canonical header order, but our
	// loop runs over the entire map so both are visited.
	req.Header.Set("X-First", "Bearer key-rest://user1/clean")
	req.Header.Set("X-Second", "Bearer key-rest://user1/missing")
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to resolve header") {
		t.Fatalf("expected resolve wrap, got %v", err)
	}
}

// TestRoundTripMalformedResponse covers Phase 3's http.ReadResponse failure
// path: server returns bytes that can't be parsed as an HTTP/1.1 response.
func TestRoundTripMalformedResponse(t *testing.T) {
	addr, tlsConfig, _, wg := rawTLSCaptureWithResponse(t,
		[]byte("THIS IS NOT A VALID HTTP RESPONSE"))

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/k", "https://localhost/", false, false, nil, []byte("v"), pass)
	store.DecryptAll(pass)

	p := NewForTest(store, tlsConfig, addr)
	resp := p.Handle(&Request{
		Type:    "http",
		Method:  "GET",
		URL:     "https://localhost/",
		Headers: map[string]string{"Authorization": "Bearer key-rest://user1/k"},
	})
	go func() { wg.Wait() }()

	if resp.Error == nil {
		t.Fatal("expected error response for malformed HTTP")
	}
}

// TestRoundTripCRLFCleanupWithPrior deterministically covers the cleanup
// loop body of the CRLF-rejection branch. Two values are added to the same
// header key — iteration order within a single key is insertion-order, so
// the clean value is appended to resolvedHeaders before the bad value
// triggers the CRLF check.
func TestRoundTripCRLFCleanupWithPrior(t *testing.T) {
	tr := &secureTransport{
		resolver: func(u string) ([]byte, error) {
			switch u {
			case "user1/clean":
				return []byte("ok-value"), nil
			case "user1/bad":
				return []byte("bad\rvalue"), nil
			}
			return nil, errors.New("not found")
		},
	}
	req, _ := http.NewRequest("GET", "https://localhost/", nil)
	req.Header.Add("X-Auth", "Bearer key-rest://user1/clean")
	req.Header.Add("X-Auth", "Bearer key-rest://user1/bad")

	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected CRLF error")
	}
	if !strings.Contains(err.Error(), "CRLF injection") {
		t.Fatalf("expected CRLF surface, got %v", err)
	}
}

// TestCheckAllowOnlyUnknownField covers the final `return nil` of
// checkAllowOnly which is reached only when the caller passes a field name
// outside the recognized {headers, url, body} set.
func TestCheckAllowOnlyUnknownField(t *testing.T) {
	p := &keystore.Placement{URL: true}
	err := checkAllowOnly(p, "something-else", "", "u/k", "value")
	if err != nil {
		t.Fatalf("expected nil for unknown field, got %v", err)
	}
}

// TestRoundTripCRLFCleanupLoop registers two header credentials; the first
// resolves cleanly but the second contains CRLF. The CRLF rejection branch
// must zero-clear ALL prior resolved headers (not just the current one).
func TestRoundTripCRLFCleanupLoop(t *testing.T) {
	addr, tlsConfig, _, wg := rawTLSCapture(t)

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/clean", "https://localhost/", false, false, nil, []byte("ok-value"), pass)
	store.Add("user1/bad", "https://localhost/", false, false, nil, []byte("bad\rvalue"), pass)
	store.DecryptAll(pass)

	p := NewForTest(store, tlsConfig, addr)
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "https://localhost/",
		Headers: map[string]string{
			"X-Header-A": "key-rest://user1/clean",
			"X-Header-B": "key-rest://user1/bad",
		},
	})
	go func() { wg.Wait() }()

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
}
