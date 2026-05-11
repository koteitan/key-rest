package proxy

import (
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/uri"
)

// rawTLSCaptureWithResponse mirrors rawTLSCapture but writes the supplied
// raw HTTP response bytes back to the client instead of the canned "200 OK".
func rawTLSCaptureWithResponse(t *testing.T, response []byte) (string, *tls.Config, *[]byte, *sync.WaitGroup) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(derBytes)

	pool := x509.NewCertPool()
	pool.AddCert(cert)
	tlsCert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
		Leaf:        cert,
	}
	srvCfg := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	clientCfg := &tls.Config{RootCAs: pool, ServerName: "localhost"}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", srvCfg)
	if err != nil {
		t.Fatal(err)
	}

	var captured []byte
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := io.ReadFull(conn, buf)
		mu.Lock()
		captured = append(captured, buf[:n]...)
		mu.Unlock()

		conn.Write(response)
	}()

	return ln.Addr().String(), clientCfg, &captured, &wg
}

func TestMakeResolverKeyNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	r := makeResolver(store)
	_, err := r("user1/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found', got %v", err)
	}
}

func TestMakeResolverKeyNotAvailable(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass)
	store.DecryptAll(pass)
	// Disable nil-out the decrypted Value while keeping the entry.
	store.Disable("user1/")

	r := makeResolver(store)
	_, err := r("user1/k")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected 'not available', got %v", err)
	}
}

func TestSnapshotCredentialsSkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/disabled", "https://e.com/", false, false, nil, []byte("v"), pass)
	store.Add("user1/active", "https://e.com/", false, false, nil, []byte("v2"), pass)
	store.DecryptAll(pass)
	// Zero out only the first key's decrypted value.
	store.Disable("user1/disabled")

	p := &Proxy{store: store}
	snap := p.snapshotCredentials()
	defer clearSnapshot(snap)
	if len(snap) != 1 {
		t.Fatalf("expected 1 entry after skipping the disabled one, got %d", len(snap))
	}
	if snap[0].uri != "user1/active" {
		t.Fatalf("expected 'user1/active', got %q", snap[0].uri)
	}
}

func TestCollectTransformOutputsSkipsResolveError(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	// No key registered → resolver returns error for any URI. The transform
	// branch with `continue` is exercised.
	p := &Proxy{store: store}
	body := "value: {{ base64(key-rest://user1/missing) }}"
	req := &Request{
		Type:    "http",
		Method:  "POST",
		URL:     "https://e.com/",
		Headers: map[string]string{},
		Body:    &body,
	}
	out := p.collectTransformOutputs(req)
	// resolve error => continue => nothing collected
	if len(out) != 0 {
		t.Fatalf("expected empty outputs, got %v", out)
	}
}

func TestDecompressBodyZstdInvalid(t *testing.T) {
	// Random garbage isn't a valid zstd frame → zstd.NewReader returns error.
	_, err := decompressBody([]byte{0x00, 0x01, 0x02, 0x03, 0x04}, "zstd")
	if err == nil {
		t.Fatal("expected zstd-decode error")
	}
}

// TestHandleReadAllRespBodyFail covers Handle's response-body read failure
// path. We construct a server that returns Content-Length larger than the
// actual payload, so io.ReadAll on the body returns an unexpected-EOF.
func TestHandleResponseBodyReadError(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/k", "https://localhost/", false, false, nil, []byte("v"), pass)
	store.DecryptAll(pass)

	addr, tlsConfig, _, wg := rawTLSCaptureWithResponse(t,
		// Body advertises 100 bytes but only sends 2 before close.
		[]byte("HTTP/1.1 200 OK\r\nContent-Length: 100\r\nConnection: close\r\n\r\nok"))
	p := NewForTest(store, tlsConfig, addr)
	resp := p.Handle(&Request{
		Type:    "http",
		Method:  "GET",
		URL:     "https://localhost/",
		Headers: map[string]string{"Authorization": "Bearer key-rest://user1/k"},
	})
	go func() { wg.Wait() }()

	if resp.Error == nil {
		t.Fatal("expected error response from truncated body")
	}
}

// TestHandleResponseDecompressError covers Handle's decompressBody error
// path. The server declares Content-Encoding: gzip but sends a payload
// that isn't a valid gzip stream.
func TestHandleResponseDecompressError(t *testing.T) {
	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("p")
	store.Add("user1/k", "https://localhost/", false, false, nil, []byte("v"), pass)
	store.DecryptAll(pass)

	resp := []byte(
		"HTTP/1.1 200 OK\r\n" +
			"Content-Encoding: gzip\r\n" +
			"Content-Length: 5\r\n" +
			"Connection: close\r\n\r\n" +
			"hello", // not a valid gzip stream
	)
	addr, tlsConfig, _, wg := rawTLSCaptureWithResponse(t, resp)
	p := NewForTest(store, tlsConfig, addr)
	r := p.Handle(&Request{
		Type:    "http",
		Method:  "GET",
		URL:     "https://localhost/",
		Headers: map[string]string{"Authorization": "Bearer key-rest://user1/k"},
	})
	go func() { wg.Wait() }()

	if r.Error == nil {
		t.Fatal("expected error response from invalid gzip")
	}
}

// Helper: sanity check that valid gzip round-trips through decompressBody.
func TestDecompressBodyGzipRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte("plaintext"))
	w.Close()
	got, err := decompressBody(buf.Bytes(), "gzip")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "plaintext" {
		t.Fatalf("got %q", got)
	}
}

// Use uri.FindAll in a sanity check so the import is present for any
// future tests that need it.
var _ = uri.FindAll
