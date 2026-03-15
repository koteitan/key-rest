// System test: starts test-server, sets up daemon internally,
// and tests all 26 services end-to-end via clients/go client library.
package systemtest

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	keyrest "github.com/koteitan/key-rest/go"
	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/proxy"
	"github.com/koteitan/key-rest/internal/server"
)

// --- Helpers ---

// projectRoot walks up from cwd to find the directory containing go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("project root not found (go.mod)")
	return ""
}

func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

type credential struct {
	service string
	label   string
	value   string
}

func findCred(creds []credential, service, label string) string {
	for _, c := range creds {
		if c.service == service && c.label == label {
			return c.value
		}
	}
	return ""
}

// startTestServer builds the test-server binary, starts it on the given port,
// parses credentials from stdout, and waits for it to accept TLS connections.
func startTestServer(t *testing.T, root string, port int, certPath, keyPath string) ([]credential, *exec.Cmd) {
	t.Helper()

	// Build
	binPath := filepath.Join(root, "test-server", "test-server")
	build := exec.Command("go", "build", "-o", binPath, "./test-server/")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build test-server: %v\n%s", err, out)
	}

	// Start
	cmd := exec.Command(binPath,
		"-port", fmt.Sprint(port),
		"-cert", certPath,
		"-key", keyPath,
		"-gen-cert",
	)
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Parse credentials from stdout
	var creds []credential
	scanner := bufio.NewScanner(stdout)
	inCreds := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "=== Test Credentials") {
			inCreds = true
			continue
		}
		if strings.HasPrefix(line, "====") {
			break
		}
		if inCreds {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				creds = append(creds, credential{
					service: fields[0],
					label:   fields[1],
					value:   fields[len(fields)-1],
				})
			}
		}
	}
	if len(creds) == 0 {
		cmd.Process.Kill()
		t.Fatal("no credentials parsed from test-server stdout")
	}

	// Wait for test-server to accept TLS connections
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 200 * time.Millisecond},
			"tcp",
			fmt.Sprintf("127.0.0.1:%d", port),
			&tls.Config{InsecureSkipVerify: true},
		)
		if err == nil {
			conn.Close()
			return creds, cmd
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill()
	t.Fatal("test-server did not start accepting connections within 5s")
	return nil, nil
}

// --- Service test definitions ---

type keyDef struct {
	uri       string              // key-rest URI identifier (e.g., "t/openai/api-key")
	service   string              // credential service name from test-server
	label     string              // credential label from test-server
	allowOnly *keystore.Placement // fine-grained placement restriction
}

type serviceTest struct {
	name    string
	method  string
	urlPath string            // path with optional key-rest:// placeholders (prepended with baseURL)
	keys    []keyDef
	headers map[string]string // header values with key-rest:// placeholders
	body    string            // request body with key-rest:// placeholders; empty = no body
}

func allServiceTests() []serviceTest {
	return []serviceTest{
		// ---- Bearer token services (13) ----
		{
			name: "openai", method: "POST",
			urlPath: "/openai/v1/chat/completions",
			keys:    []keyDef{{uri: "t/openai/api-key", service: "openai", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/openai/api-key", "Content-Type": "application/json"},
			body:    `{"model":"gpt-4o"}`,
		},
		{
			name: "mistral", method: "POST",
			urlPath: "/mistral/v1/chat/completions",
			keys:    []keyDef{{uri: "t/mistral/api-key", service: "mistral", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/mistral/api-key", "Content-Type": "application/json"},
			body:    `{"model":"mistral-large-latest"}`,
		},
		{
			name: "groq", method: "POST",
			urlPath: "/groq/openai/v1/chat/completions",
			keys:    []keyDef{{uri: "t/groq/api-key", service: "groq", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/groq/api-key", "Content-Type": "application/json"},
			body:    `{"model":"llama-3.3-70b-versatile"}`,
		},
		{
			name: "xai", method: "POST",
			urlPath: "/xai/v1/chat/completions",
			keys:    []keyDef{{uri: "t/xai/api-key", service: "xai", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/xai/api-key", "Content-Type": "application/json"},
			body:    `{"model":"grok-3"}`,
		},
		{
			name: "perplexity", method: "POST",
			urlPath: "/perplexity/chat/completions",
			keys:    []keyDef{{uri: "t/perplexity/api-key", service: "perplexity", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/perplexity/api-key", "Content-Type": "application/json"},
			body:    `{"model":"sonar"}`,
		},
		{
			name: "deepseek", method: "POST",
			urlPath: "/deepseek/chat/completions",
			keys:    []keyDef{{uri: "t/deepseek/api-key", service: "deepseek", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/deepseek/api-key", "Content-Type": "application/json"},
			body:    `{"model":"deepseek-chat"}`,
		},
		{
			name: "openrouter", method: "POST",
			urlPath: "/openrouter/api/v1/chat/completions",
			keys:    []keyDef{{uri: "t/openrouter/api-key", service: "openrouter", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/openrouter/api-key", "Content-Type": "application/json"},
			body:    `{"model":"anthropic/claude-sonnet-4-20250514"}`,
		},
		{
			name: "github", method: "GET",
			urlPath: "/github/user/repos",
			keys:    []keyDef{{uri: "t/github/token", service: "github", label: "token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/github/token"},
		},
		{
			name: "matrix", method: "POST",
			urlPath: "/matrix/_matrix/client/v3/rooms/ROOM/send/m.room.message",
			keys:    []keyDef{{uri: "t/matrix/access-token", service: "matrix", label: "access-token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/matrix/access-token", "Content-Type": "application/json"},
			body:    `{"msgtype":"m.text","body":"test"}`,
		},
		{
			name: "slack", method: "POST",
			urlPath: "/slack/api/chat.postMessage",
			keys:    []keyDef{{uri: "t/slack/bot-token", service: "slack", label: "bot-token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/slack/bot-token", "Content-Type": "application/json"},
			body:    `{"channel":"C01234567","text":"test"}`,
		},
		{
			name: "sentry", method: "GET",
			urlPath: "/sentry/api/0/projects/",
			keys:    []keyDef{{uri: "t/sentry/auth-token", service: "sentry", label: "auth-token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/sentry/auth-token"},
		},
		{
			name: "line", method: "POST",
			urlPath: "/line/v2/bot/message/push",
			keys:    []keyDef{{uri: "t/line/channel-access-token", service: "line", label: "channel-access-token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/line/channel-access-token", "Content-Type": "application/json"},
			body:    `{"to":"U1234567890","messages":[{"type":"text","text":"test"}]}`,
		},
		{
			name: "notion", method: "POST",
			urlPath: "/notion/v1/databases/DB/query",
			keys:    []keyDef{{uri: "t/notion/api-key", service: "notion", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bearer key-rest://t/notion/api-key", "Content-Type": "application/json"},
			body:    `{}`,
		},
		// ---- Custom header services (5) ----
		{
			name: "anthropic", method: "POST",
			urlPath: "/anthropic/v1/messages",
			keys:    []keyDef{{uri: "t/anthropic/api-key", service: "anthropic", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"X-Api-Key"}}}},
			headers: map[string]string{"X-Api-Key": "key-rest://t/anthropic/api-key", "Content-Type": "application/json"},
			body:    `{"model":"claude-sonnet-4-20250514","max_tokens":1}`,
		},
		{
			name: "exa", method: "POST",
			urlPath: "/exa/search",
			keys:    []keyDef{{uri: "t/exa/api-key", service: "exa", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"X-Api-Key"}}}},
			headers: map[string]string{"X-Api-Key": "key-rest://t/exa/api-key", "Content-Type": "application/json"},
			body:    `{"query":"test","type":"neural"}`,
		},
		{
			name: "brave", method: "GET",
			urlPath: "/brave/res/v1/web/search?q=test",
			keys:    []keyDef{{uri: "t/brave/api-key", service: "brave", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"X-Subscription-Token"}}}},
			headers: map[string]string{"X-Subscription-Token": "key-rest://t/brave/api-key"},
		},
		{
			name: "gitlab", method: "GET",
			urlPath: "/gitlab/api/v4/projects",
			keys:    []keyDef{{uri: "t/gitlab/token", service: "gitlab", label: "token", allowOnly: &keystore.Placement{Headers: []string{"Private-Token"}}}},
			headers: map[string]string{"Private-Token": "key-rest://t/gitlab/token"},
		},
		{
			name: "bing", method: "GET",
			urlPath: "/bing/v7.0/search?q=test",
			keys:    []keyDef{{uri: "t/bing/api-key", service: "bing", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Ocp-Apim-Subscription-Key"}}}},
			headers: map[string]string{"Ocp-Apim-Subscription-Key": "key-rest://t/bing/api-key"},
		},
		// ---- Prefix/raw token services (2) ----
		{
			name: "discord", method: "POST",
			urlPath: "/discord/api/v10/channels/CH/messages",
			keys:    []keyDef{{uri: "t/discord/bot-token", service: "discord", label: "bot-token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "Bot key-rest://t/discord/bot-token", "Content-Type": "application/json"},
			body:    `{"content":"test"}`,
		},
		{
			name: "linear", method: "POST",
			urlPath: "/linear/graphql",
			keys:    []keyDef{{uri: "t/linear/api-key", service: "linear", label: "api-key", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}}},
			headers: map[string]string{"Authorization": "key-rest://t/linear/api-key", "Content-Type": "application/json"},
			body:    `{"query":"{ issues { nodes { id title } } }"}`,
		},
		// ---- Query parameter services (3) ----
		{
			name: "gemini", method: "POST",
			urlPath: "/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://t/gemini/api-key",
			keys:    []keyDef{{uri: "t/gemini/api-key", service: "gemini", label: "api-key", allowOnly: &keystore.Placement{Queries: []string{"key"}}}},
			headers: map[string]string{"Content-Type": "application/json"},
			body:    `{"contents":[{"parts":[{"text":"test"}]}]}`,
		},
		{
			name: "google-search", method: "GET",
			urlPath: "/google-search/customsearch/v1?key=key-rest://t/google-search/api-key",
			keys:    []keyDef{{uri: "t/google-search/api-key", service: "google-search", label: "api-key", allowOnly: &keystore.Placement{Queries: []string{"key"}}}},
		},
		{
			name: "trello", method: "GET",
			urlPath: "/trello/1/members/me/boards?key=key-rest://t/trello/api-key&token=key-rest://t/trello/token",
			keys: []keyDef{
				{uri: "t/trello/api-key", service: "trello", label: "api-key", allowOnly: &keystore.Placement{Queries: []string{"key"}}},
				{uri: "t/trello/token", service: "trello", label: "token", allowOnly: &keystore.Placement{Queries: []string{"token"}}},
			},
		},
		// ---- Body field service (1) ----
		{
			name: "tavily", method: "POST",
			urlPath: "/tavily/search",
			keys:    []keyDef{{uri: "t/tavily/api-key", service: "tavily", label: "api-key", allowOnly: &keystore.Placement{Fields: []string{"api_key"}}}},
			headers: map[string]string{"Content-Type": "application/json"},
			body:    `{"api_key":"key-rest://t/tavily/api-key","query":"test","search_depth":"basic"}`,
		},
		// ---- Path embedding service (1) ----
		{
			name: "telegram", method: "POST",
			urlPath: "/telegram/bot{{key-rest://t/telegram/bot-token}}/sendMessage",
			keys:    []keyDef{{uri: "t/telegram/bot-token", service: "telegram", label: "bot-token", allowOnly: &keystore.Placement{URL: true}}},
			headers: map[string]string{"Content-Type": "application/json"},
			body:    `{"chat_id":123456789,"text":"test"}`,
		},
		// ---- Basic auth service (1) ----
		{
			name: "atlassian", method: "GET",
			urlPath: "/atlassian/2.0/repositories/ws/repo/pullrequests",
			keys: []keyDef{
				{uri: "t/atlassian/email", service: "atlassian", label: "email", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}},
				{uri: "t/atlassian/token", service: "atlassian", label: "token", allowOnly: &keystore.Placement{Headers: []string{"Authorization"}}},
			},
			headers: map[string]string{
				"Authorization": `Basic {{ base64(key-rest://t/atlassian/email, ":", key-rest://t/atlassian/token) }}`,
			},
		},
	}
}

// --- Main test ---

func TestAllServices(t *testing.T) {
	root := projectRoot(t)
	port := findFreePort(t)
	tmpDir := t.TempDir()

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Start test-server
	creds, cmd := startTestServer(t, root, port, certPath, keyPath)
	defer cmd.Wait()
	defer cmd.Process.Kill()

	t.Logf("test-server running on port %d with %d credentials", port, len(creds))

	// Load test-server certificate as trusted CA
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to add test-server cert to pool")
	}
	tlsConfig := &tls.Config{RootCAs: certPool}

	// Setup keystore
	storeDir := filepath.Join(tmpDir, "keystore")
	store, err := keystore.New(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("system-test-passphrase")
	baseURL := fmt.Sprintf("https://localhost:%d", port)
	tests := allServiceTests()

	for _, tc := range tests {
		for _, k := range tc.keys {
			value := findCred(creds, k.service, k.label)
			if value == "" {
				t.Fatalf("credential not found: service=%q label=%q", k.service, k.label)
			}
			urlPrefix := baseURL + "/" + k.service + "/"
			if err := store.Add(k.uri, urlPrefix, false, false, k.allowOnly, []byte(value), passphrase); err != nil {
				t.Fatalf("add key %s: %v", k.uri, err)
			}
		}
	}
	if err := store.DecryptAll(passphrase); err != nil {
		t.Fatal(err)
	}
	defer store.ClearAll()

	// Create proxy (no overrideAddr needed; URLs point to localhost:port)
	p := proxy.NewForTest(store, tlsConfig, "")

	// Start Unix socket server
	socketPath := filepath.Join(tmpDir, "test.sock")
	srv := server.New(socketPath, p)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// Run all service tests using clients/go client library
	client := &keyrest.Client{SocketPath: socketPath}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fullURL := baseURL + tc.urlPath

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req, err := keyrest.NewRequest(tc.method, fullURL, bodyReader)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d\nbody: %s", resp.StatusCode, truncate(body, 500))
			}

			// Verify valid JSON response
			var parsed interface{}
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("invalid JSON: %s", truncate(body, 500))
			}

			t.Logf("OK (%d bytes)", len(body))
		})
	}
}

func TestResponseMasking(t *testing.T) {
	root := projectRoot(t)
	port := findFreePort(t)
	tmpDir := t.TempDir()

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	creds, cmd := startTestServer(t, root, port, certPath, keyPath)
	defer cmd.Wait()
	defer cmd.Process.Kill()

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to add test-server cert to pool")
	}
	tlsConfig := &tls.Config{RootCAs: certPool}

	storeDir := filepath.Join(tmpDir, "keystore")
	store, err := keystore.New(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("system-test-passphrase")
	baseURL := fmt.Sprintf("https://localhost:%d", port)

	// Register a key for the echo endpoint
	echoKeyValue := findCred(creds, "openai", "api-key")
	if echoKeyValue == "" {
		t.Fatal("credential not found for echo test")
	}
	if err := store.Add("t/echo/key", baseURL+"/echo/", false, false, &keystore.Placement{Headers: []string{"Authorization"}}, []byte(echoKeyValue), passphrase); err != nil {
		t.Fatalf("add echo key: %v", err)
	}
	if err := store.DecryptAll(passphrase); err != nil {
		t.Fatal(err)
	}
	defer store.ClearAll()

	p := proxy.NewForTest(store, tlsConfig, "")
	socketPath := filepath.Join(tmpDir, "test.sock")
	srv := server.New(socketPath, p)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	client := &keyrest.Client{SocketPath: socketPath}

	req, err := keyrest.NewRequest("GET", baseURL+"/echo/test", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer key-rest://t/echo/key")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d\nbody: %s", resp.StatusCode, truncate(body, 500))
	}

	// Credential MUST NOT appear in response body
	if strings.Contains(string(body), echoKeyValue) {
		t.Fatal("credential leaked in response body — masking failed")
	}

	// key-rest:// URI SHOULD appear (reverse substitution)
	if !strings.Contains(string(body), "key-rest://") {
		t.Fatal("credential was not reverse-substituted in response body")
	}

	t.Logf("OK: credential masked in echo response (%d bytes)", len(body))
}

// TestCompressionMasking verifies that credential masking works correctly
// when the upstream server returns compressed responses.
// This is a regression test for issue #10: brotli-compressed responses
// bypass credential masking because decompressBody does not support brotli.
func TestCompressionMasking(t *testing.T) {
	root := projectRoot(t)
	port := findFreePort(t)
	tmpDir := t.TempDir()

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	creds, cmd := startTestServer(t, root, port, certPath, keyPath)
	defer cmd.Wait()
	defer cmd.Process.Kill()

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to add test-server cert to pool")
	}
	tlsConfig := &tls.Config{RootCAs: certPool}

	storeDir := filepath.Join(tmpDir, "keystore")
	store, err := keystore.New(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("system-test-passphrase")
	baseURL := fmt.Sprintf("https://localhost:%d", port)

	echoKeyValue := findCred(creds, "openai", "api-key")
	if echoKeyValue == "" {
		t.Fatal("credential not found for compression test")
	}
	if err := store.Add("t/echo/key", baseURL+"/echo/", false, false, &keystore.Placement{Headers: []string{"Authorization"}}, []byte(echoKeyValue), passphrase); err != nil {
		t.Fatalf("add echo key: %v", err)
	}
	if err := store.DecryptAll(passphrase); err != nil {
		t.Fatal(err)
	}
	defer store.ClearAll()

	p := proxy.NewForTest(store, tlsConfig, "")
	socketPath := filepath.Join(tmpDir, "test.sock")
	srv := server.New(socketPath, p)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	client := &keyrest.Client{SocketPath: socketPath}

	encodings := []struct {
		name           string
		acceptEncoding string
	}{
		{"identity", ""},
		{"gzip", "gzip"},
		{"deflate", "deflate"},
		{"brotli", "br"},
	}

	for _, tc := range encodings {
		t.Run(tc.name, func(t *testing.T) {
			req, err := keyrest.NewRequest("GET", baseURL+"/echo/test", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer key-rest://t/echo/key")
			if tc.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tc.acceptEncoding)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d\nbody: %s", resp.StatusCode, truncate(body, 500))
			}

			// All encodings MUST be decompressed and masked.
			// If this fails for brotli, it confirms issue #10.
			if strings.Contains(string(body), echoKeyValue) {
				t.Fatalf("credential leaked in %s response — masking failed", tc.name)
			}
			if !strings.Contains(string(body), "key-rest://") {
				t.Fatalf("credential not reverse-substituted in %s response — decompression or masking failed", tc.name)
			}
			t.Logf("OK: credential masked in %s response (%d bytes)", tc.name, len(body))
		})
	}
}

// TestTruncatedKeyMasking verifies that truncated API keys in error messages
// from OpenAI and Stripe are masked. These APIs return errors like:
// "Incorrect API key provided: sk-test-****...abcd"
// where "abcd" is the real suffix. This is a regression test for issue #11.
func TestTruncatedKeyMasking(t *testing.T) {
	root := projectRoot(t)
	port := findFreePort(t)
	tmpDir := t.TempDir()

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	creds, cmd := startTestServer(t, root, port, certPath, keyPath)
	defer cmd.Wait()
	defer cmd.Process.Kill()

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to add test-server cert to pool")
	}
	tlsConfig := &tls.Config{RootCAs: certPool}

	storeDir := filepath.Join(tmpDir, "keystore")
	store, err := keystore.New(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("system-test-passphrase")
	baseURL := fmt.Sprintf("https://localhost:%d", port)

	tests := []struct {
		name    string
		service string
		label   string
		uri     string
		method  string
		urlPath string
		body    string
	}{
		{
			name: "openai", service: "openai", label: "api-key",
			uri: "t/openai/api-key", method: "POST",
			urlPath: "/openai/v1/chat/completions",
			body:    `{"model":"gpt-4o"}`,
		},
		{
			name: "stripe", service: "stripe", label: "api-key",
			uri: "t/stripe/api-key", method: "GET",
			urlPath: "/stripe/v1/charges",
		},
	}

	for _, tc := range tests {
		keyValue := findCred(creds, tc.service, tc.label)
		if keyValue == "" {
			t.Fatalf("%s credential not found", tc.service)
		}
		if err := store.Add(tc.uri, baseURL+"/"+tc.service+"/", false, false, &keystore.Placement{Headers: []string{"Authorization"}}, []byte(keyValue), passphrase); err != nil {
			t.Fatalf("add %s key: %v", tc.service, err)
		}
	}
	if err := store.DecryptAll(passphrase); err != nil {
		t.Fatal(err)
	}
	defer store.ClearAll()

	p := proxy.NewForTest(store, tlsConfig, "")
	socketPath := filepath.Join(tmpDir, "test.sock")
	srv := server.New(socketPath, p)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	client := &keyrest.Client{SocketPath: socketPath}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keyValue := findCred(creds, tc.service, tc.label)

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req, err := keyrest.NewRequest(tc.method, baseURL+tc.urlPath, bodyReader)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			// Send WRONG key to trigger error with truncated real key
			req.Header.Set("Authorization", "Bearer WRONG_KEY")
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			suffix := keyValue[len(keyValue)-4:]
			if strings.Contains(bodyStr, suffix) {
				t.Fatalf("partial credential (suffix %q) leaked in %s error response:\n%s", suffix, tc.name, truncate(body, 500))
			}

			if !strings.Contains(bodyStr, "key-rest://"+tc.uri) {
				t.Fatalf("truncated key was not replaced with key-rest:// URI in %s response:\n%s", tc.name, truncate(body, 500))
			}

			t.Logf("OK: partial key masked in %s error response", tc.name)
		})
	}
}

// TestPercentEncodedMasking verifies that credentials percent-encoded in
// responses are detected and masked. This is a regression test for issue #13.
func TestPercentEncodedMasking(t *testing.T) {
	root := projectRoot(t)
	port := findFreePort(t)
	tmpDir := t.TempDir()

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	creds, cmd := startTestServer(t, root, port, certPath, keyPath)
	defer cmd.Wait()
	defer cmd.Process.Kill()

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to add test-server cert to pool")
	}
	tlsConfig := &tls.Config{RootCAs: certPool}

	storeDir := filepath.Join(tmpDir, "keystore")
	store, err := keystore.New(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("system-test-passphrase")
	baseURL := fmt.Sprintf("https://localhost:%d", port)

	echoKeyValue := findCred(creds, "openai", "api-key")
	if echoKeyValue == "" {
		t.Fatal("credential not found for percent-echo test")
	}
	// Register key for percent-echo endpoint
	if err := store.Add("t/pctecho/key", baseURL+"/percent-echo/", false, false, &keystore.Placement{Headers: []string{"Authorization"}}, []byte(echoKeyValue), passphrase); err != nil {
		t.Fatalf("add key: %v", err)
	}
	if err := store.DecryptAll(passphrase); err != nil {
		t.Fatal(err)
	}
	defer store.ClearAll()

	p := proxy.NewForTest(store, tlsConfig, "")
	socketPath := filepath.Join(tmpDir, "test.sock")
	srv := server.New(socketPath, p)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	client := &keyrest.Client{SocketPath: socketPath}

	req, err := keyrest.NewRequest("GET", baseURL+"/percent-echo/test", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer key-rest://t/pctecho/key")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if strings.Contains(bodyStr, echoKeyValue) {
		t.Fatalf("credential leaked in percent-encoded response:\n%s", truncate(body, 500))
	}

	if !strings.Contains(bodyStr, "key-rest://") {
		t.Fatalf("credential was not masked in percent-encoded response:\n%s", truncate(body, 500))
	}

	t.Logf("OK: percent-encoded credential masked (%d bytes)", len(body))
}

func truncate(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
