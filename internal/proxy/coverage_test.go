package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"github.com/koteitan/key-rest/internal/keystore"
)

func TestParseRequestValid(t *testing.T) {
	body := `{"type":"http","method":"GET","url":"https://e.com/"}`
	req, err := ParseRequest([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.Type != "http" {
		t.Fatalf("got type %q", req.Type)
	}
}

func TestParseRequestInvalid(t *testing.T) {
	_, err := ParseRequest([]byte("not-json"))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestProxyErrorString(t *testing.T) {
	e := &ProxyError{Code: "X", Message: "oops"}
	if e.Error() != "oops" {
		t.Fatalf("got %q", e.Error())
	}
}

func TestToErrorResponseGenericError(t *testing.T) {
	resp := toErrorResponse(errors.New("plain error"))
	if resp.Error == nil || resp.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("got %+v", resp.Error)
	}
}

func TestNewProxyConstructor(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	p := New(store)
	if p == nil || p.store != store {
		t.Fatal("New returned an unusable Proxy")
	}
}

func TestCheckRedirectReturnsErrUseLastResponse(t *testing.T) {
	// Verify that the http.Client built by newClient refuses to follow redirects.
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/r/key", "https://localhost/", false, false, nil, []byte("S"), pass)
	store.DecryptAll(pass)

	redirCount := 0
	tsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never be reached because the client doesn't follow redirects.
		redirCount++
		w.WriteHeader(200)
	}))
	defer tsTarget.Close()

	tsRedir := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tsTarget.URL+"/elsewhere", 302)
	}))
	defer tsRedir.Close()

	tlsConfig, addr := testTLSConfig(tsRedir)
	p := NewForTest(store, tlsConfig, addr)
	store.Add("user1/r/key", tsRedir.URL+"/", false, false, nil, []byte("S"), pass)
	store.DecryptAll(pass)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    tsRedir.URL + "/",
		Headers: map[string]string{
			"Authorization": "Bearer key-rest://user1/r/key",
		},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Status != 302 {
		t.Fatalf("expected the 302 to be returned (no follow), got %d", resp.Status)
	}
	if redirCount != 0 {
		t.Fatal("redirect target was followed")
	}
}

func TestDecompressBodyGzip(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("hello"))
	gw.Close()
	out, err := decompressBody(buf.Bytes(), "gzip")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestDecompressBodyDeflate(t *testing.T) {
	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	fw.Write([]byte("hello"))
	fw.Close()
	out, err := decompressBody(buf.Bytes(), "deflate")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestDecompressBodyBrotli(t *testing.T) {
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	bw.Write([]byte("hello"))
	bw.Close()
	out, err := decompressBody(buf.Bytes(), "br")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestDecompressBodyZstd(t *testing.T) {
	var buf bytes.Buffer
	zw, _ := zstd.NewWriter(&buf)
	zw.Write([]byte("hello"))
	zw.Close()
	out, err := decompressBody(buf.Bytes(), "zstd")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestDecompressBodyIdentity(t *testing.T) {
	out, err := decompressBody([]byte("hello"), "")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
	out, err = decompressBody([]byte("hello"), "identity")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestDecompressBodyUnknown(t *testing.T) {
	// Unknown encoding falls through to the default: returns body unchanged.
	out, err := decompressBody([]byte("xyz"), "unknown-enc")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "xyz" {
		t.Fatalf("got %q", out)
	}
}

func TestDecompressBodyMalformed(t *testing.T) {
	// Malformed gzip → error from io.ReadAll.
	if _, err := decompressBody([]byte("not-gzip"), "gzip"); err == nil {
		t.Fatal("expected gzip parse error")
	}
	if _, err := decompressBody([]byte("not-zstd"), "zstd"); err == nil {
		t.Fatal("expected zstd parse error")
	}
}

func TestMaskPercentEncodedNoPercent(t *testing.T) {
	if got := maskPercentEncoded("plain", nil); got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskPercentEncodedDecodesAndMasks(t *testing.T) {
	snap := []credSnapshot{
		{uri: "user1/k", value: []byte("SECRET")},
	}
	encoded := "Bearer%20SECRET"
	got := maskPercentEncoded(encoded, snap)
	if !strings.Contains(got, "key-rest://user1/k") {
		t.Fatalf("expected URI in masked string, got %q", got)
	}
}

func TestMaskPercentEncodedInvalidEscape(t *testing.T) {
	// %XY where XY isn't hex — QueryUnescape returns an error.
	if got := maskPercentEncoded("%ZZ", nil); got != "%ZZ" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestMaskPercentEncodedNoCredentialAfterDecode(t *testing.T) {
	// Has a percent escape, decodes fine, but no credential → return original.
	if got := maskPercentEncoded("%20%20", nil); got != "%20%20" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestContainsCRLFNone(t *testing.T) {
	if containsCRLF([]byte("plain")) {
		t.Fatal("false positive on plain bytes")
	}
}

func TestIsValidHeaderNameRejects(t *testing.T) {
	if isValidHeaderName("") {
		t.Fatal("empty header name should be invalid")
	}
	if isValidHeaderName("with space") {
		t.Fatal("space in header name should be invalid")
	}
	if !isValidHeaderName("X-Foo-Bar.123") {
		t.Fatal("valid token rejected")
	}
}
