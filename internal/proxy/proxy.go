// Package proxy handles HTTP request proxying with key-rest:// URI substitution.
package proxy

import (
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

// New creates a new Proxy with the given keystore.
func New(store *keystore.Store) *Proxy {
	return &Proxy{
		store: store,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Prevent automatic redirect following to avoid credential leakage
				return http.ErrUseLastResponse
			},
		},
	}
}

// Handle processes a proxy request and returns a response.
func (p *Proxy) Handle(req *Request) *Response {
	if req.Type != "http" {
		return errorResponse("INVALID_REQUEST", "unsupported request type: "+req.Type)
	}

	// Replace URIs in URL
	resolvedURL, err := p.replaceField(req.URL, "url", req.URL)
	if err != nil {
		return toErrorResponse(err)
	}

	// Replace URIs in headers
	resolvedHeaders := make(map[string]string, len(req.Headers))
	for k, v := range req.Headers {
		resolved, err := p.replaceField(v, "headers", req.URL)
		if err != nil {
			return toErrorResponse(err)
		}
		resolvedHeaders[k] = resolved
	}

	// Replace URIs in body
	var bodyReader io.Reader
	if req.Body != nil {
		resolvedBody, err := p.replaceField(*req.Body, "body", req.URL)
		if err != nil {
			return toErrorResponse(err)
		}
		bodyReader = strings.NewReader(resolvedBody)
	}

	// Build HTTP request
	httpReq, err := http.NewRequest(req.Method, resolvedURL, bodyReader)
	if err != nil {
		return errorResponse("HTTP_ERROR", "failed to create request: "+err.Error())
	}
	for k, v := range resolvedHeaders {
		httpReq.Header.Set(k, v)
	}

	// Execute request
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

// replaceField replaces key-rest:// URIs in a field value, checking url_prefix and field restrictions.
func (p *Proxy) replaceField(value, field, requestURL string) (string, error) {
	return uri.Replace(value, func(keyURI string) ([]byte, error) {
		dk := p.store.Lookup(keyURI)
		if dk == nil {
			return nil, &ProxyError{Code: "KEY_NOT_FOUND", Message: fmt.Sprintf("key '%s' not found", keyURI)}
		}

		// Check url_prefix (security constraint)
		if !strings.HasPrefix(requestURL, dk.URLPrefix) {
			return nil, &ProxyError{
				Code:    "URL_PREFIX_MISMATCH",
				Message: fmt.Sprintf("request URL does not match url_prefix for key '%s'", keyURI),
			}
		}

		// Check field restriction
		switch field {
		case "url":
			if !dk.AllowURL {
				return nil, &ProxyError{
					Code:    "FIELD_NOT_ALLOWED",
					Message: fmt.Sprintf("key '%s' is not allowed in URL (use --allow-url)", keyURI),
				}
			}
		case "body":
			if !dk.AllowBody {
				return nil, &ProxyError{
					Code:    "FIELD_NOT_ALLOWED",
					Message: fmt.Sprintf("key '%s' is not allowed in body (use --allow-body)", keyURI),
				}
			}
		}
		// headers: always allowed

		return dk.Value, nil
	})
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

func errorResponse(code, message string) *Response {
	return &Response{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}
