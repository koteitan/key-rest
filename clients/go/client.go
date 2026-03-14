// Package keyrest provides a net/http-compatible client that routes requests through key-rest-daemon.
package keyrest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// rawURLKey is a context key for preserving the original URL string
// before net/url percent-encodes characters like {{ }}.
type rawURLKey struct{}

type daemonRequest struct {
	Type    string            `json:"type"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    *string           `json:"body"`
}

type daemonResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Client sends HTTP requests through key-rest-daemon.
type Client struct {
	SocketPath string
}

// NewClient creates a new Client with the default socket path (~/.key-rest/key-rest.sock).
func NewClient() *Client {
	home, _ := os.UserHomeDir()
	return &Client{
		SocketPath: filepath.Join(home, ".key-rest", "key-rest.sock"),
	}
}

// NewRequest creates a new http.Request, preserving the original URL string.
// This ensures that key-rest:// URIs with {{ }} enclosed syntax are not
// percent-encoded by net/url before reaching the daemon.
func NewRequest(method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	ctx := context.WithValue(req.Context(), rawURLKey{}, rawURL)
	return req.WithContext(ctx), nil
}

// Do sends an http.Request through the key-rest-daemon and returns an http.Response.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Build daemon request
	headers := make(map[string]string)
	for k := range req.Header {
		headers[k] = req.Header.Get(k)
	}

	var body *string
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
		s := string(data)
		body = &s
	}

	// Use the original URL string if available (set by NewRequest),
	// otherwise fall back to req.URL.String() which may percent-encode {{ }}.
	urlStr := req.URL.String()
	if raw, ok := req.Context().Value(rawURLKey{}).(string); ok {
		urlStr = raw
	}

	dreq := daemonRequest{
		Type:    "http",
		Method:  req.Method,
		URL:     urlStr,
		Headers: headers,
		Body:    body,
	}

	dresp, err := c.sendToDaemon(dreq)
	if err != nil {
		return nil, err
	}

	if dresp.Error != nil {
		return nil, fmt.Errorf("[%s] %s", dresp.Error.Code, dresp.Error.Message)
	}

	// Build http.Response
	respHeader := make(http.Header)
	for k, v := range dresp.Headers {
		respHeader.Set(k, v)
	}

	return &http.Response{
		StatusCode: dresp.Status,
		Status:     dresp.StatusText,
		Header:     respHeader,
		Body:       io.NopCloser(strings.NewReader(dresp.Body)),
	}, nil
}

func (c *Client) sendToDaemon(req daemonRequest) (*daemonResponse, error) {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to key-rest-daemon: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from daemon")
	}

	var resp daemonResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Get sends a GET request through key-rest-daemon.
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post sends a POST request through key-rest-daemon.
func (c *Client) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// Ensure Client satisfies a reasonable subset of http.Client's interface at compile time.
var _ interface {
	Do(req *http.Request) (*http.Response, error)
} = (*Client)(nil)
