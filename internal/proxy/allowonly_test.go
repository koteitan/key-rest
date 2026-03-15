package proxy

// Tests for --allow-only-* placement restrictions.

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

// TestAllowOnlyHeader verifies that a key with --allow-only-header Authorization
// is accepted in Authorization header but rejected in other headers, URL, and body.
func TestAllowOnlyHeader(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	placement := &keystore.Placement{Headers: []string{"Authorization"}}
	store.Add("user1/ao/key", ts.URL+"/", false, false, placement, []byte("secret-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Allowed: Authorization header
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/ao/key",
		},
	})
	if resp.Error != nil {
		t.Fatalf("expected success for allowed header, got: %s", resp.Error.Message)
	}

	// Rejected: X-Custom header
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"X-Custom": "key-rest://user1/ao/key",
		},
	})
	if resp.Error == nil {
		t.Fatal("expected FIELD_NOT_ALLOWED for disallowed header")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}

	// Rejected: URL
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api?key=key-rest://user1/ao/key",
	})
	if resp.Error == nil {
		t.Fatal("expected FIELD_NOT_ALLOWED for URL placement")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}

	// Rejected: body
	body := `{"api_key":"key-rest://user1/ao/key"}`
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Body:   &body,
	})
	if resp.Error == nil {
		t.Fatal("expected FIELD_NOT_ALLOWED for body placement")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}
}

// TestAllowOnlyHeaderCaseInsensitive verifies header name matching is case-insensitive.
func TestAllowOnlyHeaderCaseInsensitive(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	placement := &keystore.Placement{Headers: []string{"authorization"}}
	store.Add("user1/ci/key", ts.URL+"/", false, false, placement, []byte("val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/ci/key",
		},
	})
	if resp.Error != nil {
		t.Fatalf("case-insensitive header match failed: %s", resp.Error.Message)
	}
}

// TestAllowOnlyQuery verifies that a key with --allow-only-query api_key
// is accepted only in that query parameter.
func TestAllowOnlyQuery(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "qval" {
			t.Errorf("unexpected query param: %s", r.URL.Query().Get("api_key"))
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	placement := &keystore.Placement{Queries: []string{"api_key"}}
	store.Add("user1/qo/key", ts.URL+"/", false, false, placement, []byte("qval"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Allowed: api_key query parameter
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api?api_key=key-rest://user1/qo/key",
	})
	if resp.Error != nil {
		t.Fatalf("expected success for allowed query param, got: %s", resp.Error.Message)
	}

	// Rejected: wrong query parameter name
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api?token=key-rest://user1/qo/key",
	})
	if resp.Error == nil {
		t.Fatal("expected FIELD_NOT_ALLOWED for wrong query param")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}

	// Rejected: in header
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/qo/key",
		},
	})
	if resp.Error == nil {
		t.Fatal("expected FIELD_NOT_ALLOWED for header when only query allowed")
	}
}

// TestAllowOnlyField verifies that a key with --allow-only-field api_key
// is accepted only in that JSON body field.
func TestAllowOnlyField(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(bodyBytes), "field-val") {
			t.Errorf("credential not found in body: %s", string(bodyBytes))
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	placement := &keystore.Placement{Fields: []string{"api_key"}}
	store.Add("user1/fo/key", ts.URL+"/", false, false, placement, []byte("field-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Allowed: api_key field
	body := `{"api_key":"key-rest://user1/fo/key"}`
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Body:   &body,
	})
	if resp.Error != nil {
		t.Fatalf("expected success for allowed field, got: %s", resp.Error.Message)
	}

	// Rejected: wrong field name
	body2 := `{"comment":"key-rest://user1/fo/key"}`
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Body:   &body2,
	})
	if resp.Error == nil {
		t.Fatal("expected FIELD_NOT_ALLOWED for wrong field name")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s", resp.Error.Code)
	}
}

// TestAllowOnlyURLAndBody verifies --allow-only-url and --allow-only-body.
func TestAllowOnlyURLAndBody(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")

	// Key with URL-only permission
	store.Add("user1/url-only/key", ts.URL+"/", false, false,
		&keystore.Placement{URL: true}, []byte("url-val"), pass)

	// Key with body-only permission
	store.Add("user1/body-only/key", ts.URL+"/", false, false,
		&keystore.Placement{Body: true}, []byte("body-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// URL key in URL: allowed
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api?k=key-rest://user1/url-only/key",
	})
	if resp.Error != nil {
		t.Fatalf("URL key in URL should be allowed: %s", resp.Error.Message)
	}

	// URL key in body: rejected
	body := `{"k":"key-rest://user1/url-only/key"}`
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Body:   &body,
	})
	if resp.Error == nil || resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatal("URL key in body should be rejected")
	}

	// Body key in body: allowed
	body2 := `{"k":"key-rest://user1/body-only/key"}`
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Body:   &body2,
	})
	if resp.Error != nil {
		t.Fatalf("body key in body should be allowed: %s", resp.Error.Message)
	}

	// Body key in URL: rejected
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api?k=key-rest://user1/body-only/key",
	})
	if resp.Error == nil || resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatal("body key in URL should be rejected")
	}
}

// TestAttack_AllowOnlyContentEmbedding verifies that an agent cannot embed
// credentials in non-auth fields (like comments/descriptions) when allow-only
// restrictions are in place.
func TestAttack_AllowOnlyContentEmbedding(t *testing.T) {
	const credential = "sk-attack-content-embedding"

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf(`{"received":%s}`, string(bodyBytes))))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")

	// Key restricted to Authorization header only
	placement := &keystore.Placement{Headers: []string{"Authorization"}}
	store.Add("user1/embed/key", ts.URL+"/", false, false, placement, []byte(credential), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Attack: agent tries to embed the credential in a comment field in the body
	body := `{"comment":"key-rest://user1/embed/key","data":"hello"}`
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: &body,
	})

	if resp.Error == nil {
		t.Fatal("VULNERABILITY: credential embedding in body was not blocked by allow-only restriction")
	}
	if resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatalf("expected FIELD_NOT_ALLOWED, got %s: %s", resp.Error.Code, resp.Error.Message)
	}

	t.Log("Defense held: content embedding attack blocked by allow-only-header restriction")
}

// TestAllowOnlyMultipleHeaders verifies multiple allowed headers work correctly.
func TestAllowOnlyMultipleHeaders(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")

	placement := &keystore.Placement{Headers: []string{"Authorization", "X-Api-Key"}}
	store.Add("user1/mh/key", ts.URL+"/", false, false, placement, []byte("mh-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Allowed: Authorization
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/mh/key",
		},
	})
	if resp.Error != nil {
		t.Fatalf("Authorization should be allowed: %s", resp.Error.Message)
	}

	// Allowed: X-Api-Key
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"X-Api-Key": "key-rest://user1/mh/key",
		},
	})
	if resp.Error != nil {
		t.Fatalf("X-Api-Key should be allowed: %s", resp.Error.Message)
	}

	// Rejected: X-Other
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"X-Other": "key-rest://user1/mh/key",
		},
	})
	if resp.Error == nil {
		t.Fatal("X-Other should be rejected")
	}
}

// TestLegacyModeBackwardsCompat verifies that keys without AllowOnly (nil)
// continue to work with the legacy AllowURL/AllowBody flags.
func TestLegacyModeBackwardsCompat(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")

	// Legacy key: AllowURL=true, AllowBody=false, AllowOnly=nil
	store.Add("user1/legacy/key", ts.URL+"/", true, false, nil, []byte("legacy-val"), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	// Header always allowed in legacy mode
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/legacy/key",
		},
	})
	if resp.Error != nil {
		t.Fatalf("legacy header should work: %s", resp.Error.Message)
	}

	// URL allowed (AllowURL=true)
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    ts.URL + "/api?k=key-rest://user1/legacy/key",
	})
	if resp.Error != nil {
		t.Fatalf("legacy URL should work: %s", resp.Error.Message)
	}

	// Body rejected (AllowBody=false)
	body := `{"k":"key-rest://user1/legacy/key"}`
	resp = p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    ts.URL + "/api",
		Body:   &body,
	})
	if resp.Error == nil || resp.Error.Code != "FIELD_NOT_ALLOWED" {
		t.Fatal("legacy body should be rejected when AllowBody=false")
	}
}
