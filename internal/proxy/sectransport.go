// Package proxy: secureTransport implements delayed key replacement for memory security.
// key-rest:// URIs are kept as placeholders through the net/http pipeline.
// Replacement with actual secrets happens only in an mlocked buffer
// immediately before TLS encryption.
package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/koteitan/key-rest/internal/crypto"
	"github.com/koteitan/key-rest/internal/uri"
)

// secureTransport implements http.RoundTripper with delayed key replacement.
type secureTransport struct {
	resolver     uri.Resolver
	tlsConfig    *tls.Config // custom TLS config (nil = system default)
	overrideAddr string      // override dial address (for testing)
}

// connClosingBody wraps a response body to close the TLS connection when done.
type connClosingBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *connClosingBody) Close() error {
	err := b.ReadCloser.Close()
	b.conn.Close()
	return err
}

type resolvedHeader struct {
	key   string
	value []byte
}

func (t *secureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// --- Phase 1: Resolve all fields (key-rest:// → actual keys as []byte) ---

	// Read body (with placeholders)
	var bodyPlaceholder []byte
	if req.Body != nil {
		var err error
		bodyPlaceholder, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	resolvedBody, err := uri.ReplaceBytes(string(bodyPlaceholder), t.resolver)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve body: %w", err)
	}

	// Use raw URL from context if available, to preserve characters like {{ }}
	// that url.Parse would percent-encode (breaking pattern matching).
	requestURI := req.URL.RequestURI()
	if rawURL, ok := ctx.Value(rawURLKey).(string); ok && rawURL != "" {
		if idx := strings.Index(rawURL, "://"); idx >= 0 {
			rest := rawURL[idx+3:]
			if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
				requestURI = rest[slashIdx:]
			}
		}
	}
	resolvedURI, err := uri.ReplaceBytes(requestURI, t.resolver)
	if err != nil {
		crypto.ZeroClear(resolvedBody)
		return nil, fmt.Errorf("failed to resolve URL: %w", err)
	}

	var resolvedHeaders []resolvedHeader
	for key, vals := range req.Header {
		if strings.EqualFold(key, "Host") || strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, val := range vals {
			resolved, err := uri.ReplaceBytes(val, t.resolver)
			if err != nil {
				crypto.ZeroClear(resolvedBody)
				crypto.ZeroClear(resolvedURI)
				for _, h := range resolvedHeaders {
					crypto.ZeroClear(h.value)
				}
				return nil, fmt.Errorf("failed to resolve header %s: %w", key, err)
			}
			resolvedHeaders = append(resolvedHeaders, resolvedHeader{key, resolved})
		}
	}

	// --- Phase 2: Build raw HTTP/1.1 request in mlocked buffer ---

	methodPart := req.Method + " "
	httpVersion := " HTTP/1.1\r\n"
	hostLine := "Host: " + req.URL.Host + "\r\n"
	connLine := "Connection: close\r\n"

	size := len(methodPart) + len(resolvedURI) + len(httpVersion)
	size += len(hostLine)
	size += len(connLine)
	for _, h := range resolvedHeaders {
		size += len(h.key) + 2 + len(h.value) + 2 // "Key: Value\r\n"
	}
	var contentLengthLine string
	if len(resolvedBody) > 0 {
		contentLengthLine = fmt.Sprintf("Content-Length: %d\r\n", len(resolvedBody))
		size += len(contentLengthLine)
	}
	size += 2 // \r\n (end of headers)
	size += len(resolvedBody)

	buf := make([]byte, size)
	crypto.Mlock(buf)
	n := 0
	n += copy(buf[n:], methodPart)
	n += copy(buf[n:], resolvedURI)
	n += copy(buf[n:], httpVersion)
	n += copy(buf[n:], hostLine)
	n += copy(buf[n:], connLine)
	for _, h := range resolvedHeaders {
		n += copy(buf[n:], h.key)
		n += copy(buf[n:], ": ")
		n += copy(buf[n:], h.value)
		n += copy(buf[n:], "\r\n")
	}
	if len(contentLengthLine) > 0 {
		n += copy(buf[n:], contentLengthLine)
	}
	n += copy(buf[n:], "\r\n")
	n += copy(buf[n:], resolvedBody)

	// Zero-clear intermediate resolved buffers immediately
	crypto.ZeroClear(resolvedURI)
	crypto.ZeroClear(resolvedBody)
	for _, h := range resolvedHeaders {
		crypto.ZeroClear(h.value)
	}

	// --- Phase 3: TLS dial, write, read response ---

	addr := req.URL.Host
	if !strings.Contains(addr, ":") {
		addr += ":443"
	}
	if t.overrideAddr != "" {
		addr = t.overrideAddr
	}

	dialer := &tls.Dialer{Config: t.tlsConfig}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		crypto.ZeroClearAndMunlock(buf)
		return nil, fmt.Errorf("TLS dial failed: %w", err)
	}
	conn := rawConn

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	_, err = conn.Write(buf)
	crypto.ZeroClearAndMunlock(buf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	resp.Body = &connClosingBody{ReadCloser: resp.Body, conn: conn}
	return resp, nil
}
