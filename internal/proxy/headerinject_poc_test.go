package proxy

import (
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

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"github.com/koteitan/key-rest/internal/keystore"
)

// rawTLSCapture starts a TLS listener that accepts one connection and
// captures the raw decrypted bytes written by the client.
func rawTLSCapture(t *testing.T) (string, *tls.Config, *[]byte, *sync.WaitGroup) {
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
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatal(err)
	}

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

		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok"))
	}()

	return ln.Addr().String(), clientCfg, &captured, &wg
}

// TestHeaderKeyInjectionRejected — F-001 regression test.
// Header keys with CRLF (or any non-token byte) must be rejected before
// any URI resolution, so the credential is never written to the wire.
func TestHeaderKeyInjectionRejected(t *testing.T) {
	addr, tlsConfig, captured, wg := rawTLSCapture(t)

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	const secretValue = "SUPER-SECRET-CREDENTIAL-XYZ"
	store.Add("user1/ts/key", "https://localhost/", false, false, nil, []byte(secretValue), pass)
	store.DecryptAll(pass)

	p := NewForTest(store, tlsConfig, addr)

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    "https://localhost/",
		Headers: map[string]string{
			"Authorization\r\nLog-Echo": "Bearer key-rest://user1/ts/key",
		},
	})

	if resp.Error == nil {
		t.Fatal("expected INVALID_REQUEST, got nil error")
	}
	if resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", resp.Error.Code)
	}

	// Close the unused listener so the goroutine exits.
	go func() { wg.Wait() }()
	if strings.Contains(string(*captured), secretValue) {
		t.Fatal("credential reached the wire — fix is incomplete")
	}
}

// TestMethodCRLFInjectionRawWire — does the Method field allow CRLF injection?
func TestMethodCRLFInjectionRawWire(t *testing.T) {
	addr, tlsConfig, captured, wg := rawTLSCapture(t)

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	const secretValue = "SUPER-SECRET-CREDENTIAL-XYZ"
	store.Add("user1/ts/key", "https://localhost/", false, false, nil, []byte(secretValue), pass)
	store.DecryptAll(pass)

	p := NewForTest(store, tlsConfig, addr)

	maliciousMethod := "GET / HTTP/1.1\r\nHost: attacker.evil\r\nLog-Echo: key-rest://user1/ts/key\r\nGET"

	resp := p.Handle(&Request{
		Type:   "http",
		Method: maliciousMethod,
		URL:    "https://localhost/",
	})

	t.Logf("response error: %+v", resp.Error)
	t.Logf("response status: %d", resp.Status)

	// Try to drain the listener even if request failed
	go func() { wg.Wait() }()
	wire := string(*captured)
	t.Logf("wire payload (may be empty if request was rejected):\n%s", wire)

	if strings.Contains(wire, "Log-Echo: "+secretValue) {
		t.Errorf("Method CRLF: secret leaked")
	}
}

// TestURLCRLFInjectionRawWire — does the URL field allow CRLF injection?
func TestURLCRLFInjectionRawWire(t *testing.T) {
	addr, tlsConfig, captured, wg := rawTLSCapture(t)

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	const secretValue = "SUPER-SECRET-CREDENTIAL-XYZ"
	store.Add("user1/ts/key", "https://localhost/", true, false, nil, []byte(secretValue), pass)
	store.DecryptAll(pass)

	p := NewForTest(store, tlsConfig, addr)

	maliciousURL := "https://localhost/path\r\nLog-Echo: key-rest://user1/ts/key\r\nX-Junk: foo"

	resp := p.Handle(&Request{
		Type:   "http",
		Method: "GET",
		URL:    maliciousURL,
	})

	t.Logf("response error: %+v", resp.Error)
	t.Logf("response status: %d", resp.Status)

	go func() { wg.Wait() }()
	wire := string(*captured)
	t.Logf("wire payload (may be empty if request was rejected):\n%s", wire)

	if strings.Contains(wire, "Log-Echo: "+secretValue) {
		t.Errorf("URL CRLF: secret leaked via injected header from URL")
	}
}
