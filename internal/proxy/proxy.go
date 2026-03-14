// Package proxy handles HTTP request proxying with key-rest:// URI substitution.
package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/uri"
)

// rawURLKey is the context key for passing the raw URL string to secureTransport.
// This avoids url.Parse encoding characters like {{ }} that are needed for pattern matching.
type contextKey string

const rawURLKey contextKey = "rawURL"

// Request is the JSON request from a key-rest client.
type Request struct {
	Type    string            `json:"type"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    *string           `json:"body"`
}

// Response is the JSON response sent back to the client.
type Response struct {
	Status     int               `json:"status,omitempty"`
	StatusText string            `json:"statusText,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Error      *ErrorInfo        `json:"error,omitempty"`
}

// ErrorInfo describes an error in the response.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Proxy handles HTTP proxying with credential injection.
type Proxy struct {
	store  *keystore.Store
	client *http.Client
}

func newClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func makeResolver(store *keystore.Store) uri.Resolver {
	return func(keyURI string) ([]byte, error) {
		dk := store.Lookup(keyURI)
		if dk == nil {
			return nil, fmt.Errorf("key '%s' not found", keyURI)
		}
		return dk.Value, nil
	}
}

// New creates a new Proxy with the given keystore.
func New(store *keystore.Store) *Proxy {
	transport := &secureTransport{
		resolver: makeResolver(store),
	}
	return &Proxy{
		store:  store,
		client: newClient(transport),
	}
}

// NewForTest creates a Proxy configured to connect to a specific TLS server (for testing).
func NewForTest(store *keystore.Store, tlsConfig *tls.Config, addr string) *Proxy {
	transport := &secureTransport{
		resolver:     makeResolver(store),
		tlsConfig:    tlsConfig,
		overrideAddr: addr,
	}
	return &Proxy{
		store:  store,
		client: newClient(transport),
	}
}

// Handle processes a proxy request and returns a response.
func (p *Proxy) Handle(req *Request) *Response {
	if req.Type != "http" {
		return errorResponse("INVALID_REQUEST", "unsupported request type: "+req.Type)
	}

	// Enforce HTTPS to prevent credentials from being sent in plaintext
	if !strings.HasPrefix(req.URL, "https://") {
		return errorResponse("INSECURE_REQUEST", "only HTTPS URLs are allowed (got HTTP)")
	}

	// Validate all key-rest:// URIs (url_prefix, field restrictions) without resolving
	if err := p.validateField(req.URL, "url", req.URL); err != nil {
		return toErrorResponse(err)
	}
	for _, v := range req.Headers {
		if err := p.validateField(v, "headers", req.URL); err != nil {
			return toErrorResponse(err)
		}
	}
	if req.Body != nil {
		if err := p.validateField(*req.Body, "body", req.URL); err != nil {
			return toErrorResponse(err)
		}
	}

	// Build http.Request with key-rest:// placeholders still in place.
	// The secureTransport will resolve them in an mlocked buffer before TLS encryption.
	var bodyReader io.Reader
	if req.Body != nil {
		bodyReader = strings.NewReader(*req.Body)
	}
	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return errorResponse("HTTP_ERROR", "failed to create request: "+err.Error())
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Pass raw URL to secureTransport via context so it can extract the
	// path+query without url.Parse encoding (which mangles {{ }} to %7B%7B...%7D%7D).
	ctx := context.WithValue(httpReq.Context(), rawURLKey, req.URL)
	httpReq = httpReq.WithContext(ctx)

	// Execute request (secureTransport handles delayed replacement)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return errorResponse("HTTP_ERROR", "request failed: "+err.Error())
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorResponse("HTTP_ERROR", "failed to read response body: "+err.Error())
	}

	// Build response headers
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	return &Response{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    respHeaders,
		Body:       string(respBody),
	}
}

// validateField checks that all key-rest:// URIs in a field value pass
// url_prefix and field restriction checks, without resolving actual values.
func (p *Proxy) validateField(value, field, requestURL string) error {
	matches := uri.FindAll(value)
	for _, m := range matches {
		for _, keyURI := range m.KeyURIs {
			dk := p.store.Lookup(keyURI)
			if dk == nil {
				return &ProxyError{Code: "KEY_NOT_FOUND", Message: fmt.Sprintf("key '%s' not found", keyURI)}
			}
			if !hasURLPrefix(requestURL, dk.URLPrefix) {
				return &ProxyError{
					Code:    "URL_PREFIX_MISMATCH",
					Message: fmt.Sprintf("request URL does not match url_prefix for key '%s'", keyURI),
				}
			}
			switch field {
			case "url":
				if !dk.AllowURL {
					return &ProxyError{
						Code:    "FIELD_NOT_ALLOWED",
						Message: fmt.Sprintf("key '%s' is not allowed in URL (use --allow-url)", keyURI),
					}
				}
			case "body":
				if !dk.AllowBody {
					return &ProxyError{
						Code:    "FIELD_NOT_ALLOWED",
						Message: fmt.Sprintf("key '%s' is not allowed in body (use --allow-body)", keyURI),
					}
				}
			}
		}
	}
	return nil
}

// ParseRequest parses a JSON request from raw bytes.
func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ProxyError is a structured error for proxy operations.
type ProxyError struct {
	Code    string
	Message string
}

func (e *ProxyError) Error() string {
	return e.Message
}

func toErrorResponse(err error) *Response {
	var pe *ProxyError
	if errors.As(err, &pe) {
		return errorResponse(pe.Code, pe.Message)
	}
	return errorResponse("INTERNAL_ERROR", err.Error())
}

// hasURLPrefix checks that requestURL starts with prefix at a proper URL boundary.
// This prevents subdomain attacks: prefix "https://api.example.com" must not match
// "https://api.example.com.evil.com/". The character after the prefix must be
// '/', '?', '#', or end of string.
func hasURLPrefix(requestURL, prefix string) bool {
	if !strings.HasPrefix(requestURL, prefix) {
		return false
	}
	if strings.HasSuffix(prefix, "/") {
		return true
	}
	if len(requestURL) == len(prefix) {
		return true
	}
	next := requestURL[len(prefix)]
	return next == '/' || next == '?' || next == '#'
}

func errorResponse(code, message string) *Response {
	return &Response{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}
