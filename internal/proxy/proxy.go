// Package proxy handles HTTP request proxying with key-rest:// URI substitution.
package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/andybalholm/brotli"
	"strings"
	"time"

	"net/url"

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

	// Reject URLs with userinfo (e.g., https://api.example.com@evil.com/)
	// to prevent URL parse inconsistency attacks
	if parsed, err := url.Parse(req.URL); err == nil && parsed.User != nil {
		return errorResponse("INSECURE_REQUEST", "URLs with userinfo (@) are not allowed")
	}

	// Validate all key-rest:// URIs (url_prefix, field restrictions) without resolving
	if err := p.validateField(req.URL, "url", "", req.URL); err != nil {
		return toErrorResponse(err)
	}
	for k, v := range req.Headers {
		if err := p.validateField(v, "headers", k, req.URL); err != nil {
			return toErrorResponse(err)
		}
	}
	if req.Body != nil {
		if err := p.validateField(*req.Body, "body", "", req.URL); err != nil {
			return toErrorResponse(err)
		}
	}

	// Collect transform outputs (e.g., base64-encoded values) for additional
	// response masking. Raw credential masking alone cannot catch these.
	transformOutputs := p.collectTransformOutputs(req)

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

	// Read and decompress response body to ensure masking operates on plaintext.
	// Without decompression, compressed responses bypass credential masking.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorResponse("HTTP_ERROR", "failed to read response body: "+err.Error())
	}
	respBody, err = decompressBody(respBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return errorResponse("HTTP_ERROR", "failed to decompress response body: "+err.Error())
	}
	// Remove Content-Encoding since body is now decompressed
	resp.Header.Del("Content-Encoding")

	// Reverse-substitute credential values back to key-rest:// URIs in response
	// to prevent credential leakage through APIs that echo back auth data.
	// Transform outputs (e.g., base64) are masked first, then raw credentials.
	respBodyStr := p.maskTransformOutputs(string(respBody), transformOutputs)
	respBodyStr = p.maskCredentials(respBodyStr)

	// Build response headers (with credential masking)
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		v := p.maskTransformOutputs(resp.Header.Get(k), transformOutputs)
		respHeaders[k] = p.maskCredentials(v)
	}

	return &Response{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    respHeaders,
		Body:       respBodyStr,
	}
}

// validateField checks that all key-rest:// URIs in a field value pass
// url_prefix and field restriction checks, without resolving actual values.
// fieldName is the header name (for field="headers") or empty for url/body.
func (p *Proxy) validateField(value, field, fieldName, requestURL string) error {
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
			if err := checkPlacement(dk, field, fieldName, keyURI, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkPlacement validates that a key-rest:// URI appears in an allowed location.
func checkPlacement(dk *keystore.DecryptedKey, field, fieldName, keyURI, value string) error {
	if dk.AllowOnly != nil {
		return checkAllowOnly(dk.AllowOnly, field, fieldName, keyURI, value)
	}
	// Legacy mode: AllowURL/AllowBody flags
	switch field {
	case "url":
		if !dk.AllowURL {
			return &ProxyError{
				Code:    "FIELD_NOT_ALLOWED",
				Message: fmt.Sprintf("key '%s' is not allowed in URL (use --allow-only-url)", keyURI),
			}
		}
	case "body":
		if !dk.AllowBody {
			return &ProxyError{
				Code:    "FIELD_NOT_ALLOWED",
				Message: fmt.Sprintf("key '%s' is not allowed in body (use --allow-only-body)", keyURI),
			}
		}
	}
	return nil
}

// checkAllowOnly validates placement using the new allow-only restrictions.
func checkAllowOnly(p *keystore.Placement, field, fieldName, keyURI, value string) error {
	switch field {
	case "headers":
		if len(p.Headers) == 0 {
			return &ProxyError{
				Code:    "FIELD_NOT_ALLOWED",
				Message: fmt.Sprintf("key '%s' is not allowed in headers", keyURI),
			}
		}
		for _, h := range p.Headers {
			if strings.EqualFold(h, fieldName) {
				return nil
			}
		}
		return &ProxyError{
			Code:    "FIELD_NOT_ALLOWED",
			Message: fmt.Sprintf("key '%s' is only allowed in header(s): %s", keyURI, strings.Join(p.Headers, ", ")),
		}
	case "url":
		if p.URL {
			return nil
		}
		if len(p.Queries) > 0 {
			if isInAllowedQuery(value, keyURI, p.Queries) {
				return nil
			}
			return &ProxyError{
				Code:    "FIELD_NOT_ALLOWED",
				Message: fmt.Sprintf("key '%s' is only allowed in query parameter(s): %s", keyURI, strings.Join(p.Queries, ", ")),
			}
		}
		return &ProxyError{
			Code:    "FIELD_NOT_ALLOWED",
			Message: fmt.Sprintf("key '%s' is not allowed in URL", keyURI),
		}
	case "body":
		if p.Body {
			return nil
		}
		if len(p.Fields) > 0 {
			if isInAllowedField(value, keyURI, p.Fields) {
				return nil
			}
			return &ProxyError{
				Code:    "FIELD_NOT_ALLOWED",
				Message: fmt.Sprintf("key '%s' is only allowed in body field(s): %s", keyURI, strings.Join(p.Fields, ", ")),
			}
		}
		return &ProxyError{
			Code:    "FIELD_NOT_ALLOWED",
			Message: fmt.Sprintf("key '%s' is not allowed in body", keyURI),
		}
	}
	return nil
}

// isInAllowedQuery checks if all occurrences of a key-rest:// URI in the URL
// appear only in values of allowed query parameter names.
func isInAllowedQuery(rawURL, keyURI string, allowedParams []string) bool {
	// Extract query string from URL
	qIdx := strings.Index(rawURL, "?")
	if qIdx < 0 {
		return false
	}
	queryStr := rawURL[qIdx+1:]
	// Remove fragment
	if fIdx := strings.Index(queryStr, "#"); fIdx >= 0 {
		queryStr = queryStr[:fIdx]
	}

	keyPattern := "key-rest://" + keyURI
	enclosedPattern := "{{ key-rest://" + keyURI

	for _, param := range strings.Split(queryStr, "&") {
		eqIdx := strings.Index(param, "=")
		if eqIdx < 0 {
			continue
		}
		name := param[:eqIdx]
		val := param[eqIdx+1:]

		if strings.Contains(val, keyPattern) || strings.Contains(val, enclosedPattern) {
			allowed := false
			for _, ap := range allowedParams {
				if name == ap {
					allowed = true
					break
				}
			}
			if !allowed {
				return false
			}
		}
	}
	return true
}

// isInAllowedField checks if all occurrences of a key-rest:// URI in the body
// appear only in values of allowed top-level JSON field names.
func isInAllowedField(body, keyURI string, allowedFields []string) bool {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return false
	}

	keyPattern := "key-rest://" + keyURI

	for fieldName, rawVal := range parsed {
		if strings.Contains(string(rawVal), keyPattern) {
			allowed := false
			for _, af := range allowedFields {
				if fieldName == af {
					allowed = true
					break
				}
			}
			if !allowed {
				return false
			}
		}
	}
	return true
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

// maskCredentials replaces any decrypted key values in s with their key-rest:// URIs.
// It also masks JSON-escaped forms to prevent exfiltration via escaped reflection.
// Credentials are sorted longest-first to prevent a short credential from
// partially matching inside a longer one and leaking the remaining suffix.
func (p *Proxy) maskCredentials(s string) string {
	p.store.RLock()
	decrypted := p.store.Decrypted()
	p.store.RUnlock()

	// Sort by credential value length (longest first) to prevent
	// substring collisions from leaking partial credential data.
	sorted := make([]keystore.DecryptedKey, len(decrypted))
	copy(sorted, decrypted)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Value) > len(sorted[j].Value)
	})

	for _, dk := range sorted {
		if len(dk.Value) > 0 {
			raw := string(dk.Value)
			replacement := "key-rest://" + dk.URI
			// Mask JSON-escaped form first (longer, more specific)
			jsonBytes, _ := json.Marshal(raw)
			jsonEscaped := string(jsonBytes[1 : len(jsonBytes)-1])
			if jsonEscaped != raw {
				s = strings.ReplaceAll(s, jsonEscaped, replacement)
			}
			// Then mask raw form
			s = strings.ReplaceAll(s, raw, replacement)
		}
	}
	return s
}

// collectTransformOutputs resolves all transform expressions (e.g., base64)
// in the request and returns a map from resolved value → original template.
func (p *Proxy) collectTransformOutputs(req *Request) map[string]string {
	resolver := makeResolver(p.store)
	outputs := map[string]string{}

	collectFrom := func(s string) {
		for _, m := range uri.FindAll(s) {
			if m.Transform == "" {
				continue
			}
			resolved, err := uri.ResolveMatch(m, resolver)
			if err != nil {
				continue
			}
			original := s[m.Start:m.End]
			outputs[resolved] = original
		}
	}

	collectFrom(req.URL)
	for _, v := range req.Headers {
		collectFrom(v)
	}
	if req.Body != nil {
		collectFrom(*req.Body)
	}

	return outputs
}

// maskTransformOutputs replaces resolved transform values in s with their original templates.
func (p *Proxy) maskTransformOutputs(s string, outputs map[string]string) string {
	for resolved, original := range outputs {
		s = strings.ReplaceAll(s, resolved, original)
	}
	return s
}

// decompressBody decompresses the response body based on Content-Encoding.
// Returns the original body unchanged if no encoding or unsupported encoding.
func decompressBody(body []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip", "x-gzip":
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	case "deflate":
		r := flate.NewReader(bytes.NewReader(body))
		defer r.Close()
		return io.ReadAll(r)
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(body)))
	case "", "identity":
		return body, nil
	default:
		return body, nil
	}
}

func errorResponse(code, message string) *Response {
	return &Response{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}
