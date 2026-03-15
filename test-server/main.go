// test-server is a mock HTTPS server that mimics the authentication patterns
// and error responses of all services supported by key-rest examples.
// Credentials are randomly generated at startup and printed to stdout.
// URL structure: https://localhost:PORT/SERVICE_NAME/original-path
package main

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// --- Credential generation ---

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// credEntry is a named credential for display.
type credEntry struct {
	label string // display label (e.g., "openai api-key")
	value string
}

// --- Service definition ---

type mockService struct {
	creds     []credEntry
	checkAuth func(r *http.Request) bool
	onFail    func(w http.ResponseWriter)
	onOK      func(w http.ResponseWriter, r *http.Request)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	writeJSONWithEncoding(w, nil, status, v)
}

// writeJSONWithEncoding writes a JSON response, optionally compressed based on
// the Accept-Encoding header from the request.
func writeJSONWithEncoding(w http.ResponseWriter, r *http.Request, status int, v interface{}) {
	plain, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "json marshal error", 500)
		return
	}
	plain = append(plain, '\n')

	w.Header().Set("Content-Type", "application/json")

	if r == nil {
		w.WriteHeader(status)
		w.Write(plain)
		return
	}

	ae := r.Header.Get("Accept-Encoding")
	switch {
	case strings.Contains(ae, "br"):
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		bw.Write(plain)
		bw.Close()
		w.Header().Set("Content-Encoding", "br")
		w.WriteHeader(status)
		w.Write(buf.Bytes())
	case strings.Contains(ae, "gzip"):
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		gw.Write(plain)
		gw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(status)
		w.Write(buf.Bytes())
	case strings.Contains(ae, "deflate"):
		var buf bytes.Buffer
		fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
		fw.Write(plain)
		fw.Close()
		w.Header().Set("Content-Encoding", "deflate")
		w.WriteHeader(status)
		w.Write(buf.Bytes())
	case strings.Contains(ae, "zstd"):
		var buf bytes.Buffer
		zw, _ := zstd.NewWriter(&buf)
		zw.Write(plain)
		zw.Close()
		w.Header().Set("Content-Encoding", "zstd")
		w.WriteHeader(status)
		w.Write(buf.Bytes())
	default:
		w.WriteHeader(status)
		w.Write(plain)
	}
}

// --- Auth checker factories ---

func bearerChecker(expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == expected
	}
}

func botChecker(expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		return strings.TrimPrefix(r.Header.Get("Authorization"), "Bot ") == expected
	}
}

func rawAuthChecker(expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		return r.Header.Get("Authorization") == expected
	}
}

func headerChecker(headerName, expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		return r.Header.Get(headerName) == expected
	}
}

func queryChecker(paramName, expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		return r.URL.Query().Get(paramName) == expected
	}
}

func queryDoubleChecker(p1, v1, p2, v2 string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		return r.URL.Query().Get(p1) == v1 && r.URL.Query().Get(p2) == v2
	}
}

func basicChecker(user, pass string) func(r *http.Request) bool {
	expected := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return func(r *http.Request) bool {
		auth := r.Header.Get("Authorization")
		return strings.TrimPrefix(auth, "Basic ") == expected
	}
}

func pathTokenChecker(serviceName, expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		path := strings.TrimPrefix(r.URL.Path, "/"+serviceName+"/")
		if !strings.HasPrefix(path, "bot") {
			return false
		}
		token := strings.TrimPrefix(path, "bot")
		if idx := strings.Index(token, "/"); idx >= 0 {
			token = token[:idx]
		}
		return token == expected
	}
}

// bodyChecker reads body and checks a JSON field. It stores body for reuse.
func bodyChecker(field, expected string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return false
		}
		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err != nil {
			return false
		}
		val, ok := m[field]
		return ok && fmt.Sprint(val) == expected
	}
}

// --- Response helper ---

func M(kv ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

// --- OpenAI-compatible response factories ---

// truncateKey mimics OpenAI's error format: prefix + asterisks + last 4 chars.
// e.g., "sk-test-abc123" → "sk-test-******bc23"
func truncateKey(key string) string {
	if len(key) < 8 {
		return strings.Repeat("*", len(key))
	}
	// Find the end of the prefix portion (up to and including the last hyphen
	// before the secret part, but at least 3 chars)
	prefixEnd := 0
	for i, c := range key {
		if c == '-' {
			prefixEnd = i + 1
		}
	}
	if prefixEnd < 3 {
		prefixEnd = 3
	}
	if prefixEnd > len(key)-4 {
		prefixEnd = len(key) - 4
	}
	return key[:prefixEnd] + strings.Repeat("*", len(key)-prefixEnd-4) + key[len(key)-4:]
}

func openaiError(key string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		writeJSON(w, 401, M(
			"error", M(
				"message", fmt.Sprintf("Incorrect API key provided: %s. You can find your API key at https://platform.openai.com/account/api-keys.", truncateKey(key)),
				"type", "invalid_request_error",
				"param", nil,
				"code", "invalid_api_key",
			),
		))
	}
}

func openaiOK(model string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, M(
			"id", "chatcmpl-test-"+randHex(4),
			"object", "chat.completion",
			"created", time.Now().Unix(),
			"model", model,
			"choices", []interface{}{M(
				"index", 0,
				"message", M("role", "assistant", "content", "[test-server] Authentication successful."),
				"finish_reason", "stop",
			)},
			"usage", M("prompt_tokens", 10, "completion_tokens", 20, "total_tokens", 30),
		))
	}
}

// --- Build all services ---

func buildServices() (map[string]*mockService, []credEntry) {
	s := make(map[string]*mockService)
	var allCreds []credEntry

	add := func(name string, svc *mockService) {
		s[name] = svc
		for _, c := range svc.creds {
			allCreds = append(allCreds, credEntry{label: name + " " + c.label, value: c.value})
		}
	}

	addOpenAI := func(name, model, prefix string) {
		key := prefix + randHex(16)
		add(name, &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: bearerChecker(key),
			onFail:    openaiError(key),
			onOK:      openaiOK(model),
		})
	}

	// ---- OpenAI-compatible services ----
	addOpenAI("openai", "gpt-4o", "sk-test-")
	addOpenAI("mistral", "mistral-large-latest", "test-mistral-")
	addOpenAI("groq", "llama-3.3-70b-versatile", "gsk_test")
	addOpenAI("xai", "grok-3", "xai-test-")
	addOpenAI("perplexity", "sonar", "pplx-test-")
	addOpenAI("deepseek", "deepseek-chat", "sk-test-ds-")
	addOpenAI("openrouter", "anthropic/claude-sonnet-4-20250514", "sk-or-v1-test-")

	// ---- Anthropic ----
	{
		key := "sk-ant-api03-test-" + randHex(16)
		add("anthropic", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: headerChecker("X-Api-Key", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("type", "error", "error", M("type", "authentication_error", "message", "invalid x-api-key")))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M(
					"id", "msg_test_"+randHex(4),
					"type", "message",
					"role", "assistant",
					"content", []interface{}{M("type", "text", "text", "[test-server] Authentication successful.")},
					"model", "claude-sonnet-4-20250514",
					"stop_reason", "end_turn",
					"usage", M("input_tokens", 10, "output_tokens", 20),
				))
			},
		})
	}

	// ---- Stripe ----
	{
		key := "rk_live_test" + randHex(16)
		add("stripe", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("error", M(
					"message", fmt.Sprintf("Invalid API Key provided: %s", truncateKey(key)),
					"type", "invalid_request_error",
				)))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("id", "ch_test_"+randHex(4), "object", "charge", "amount", 1000, "currency", "usd", "status", "succeeded"))
			},
		})
	}

	// ---- GitHub ----
	{
		key := "ghp_test" + randHex(16)
		add("github", &mockService{
			creds:     []credEntry{{label: "token", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("message", "Bad credentials", "documentation_url", "https://docs.github.com/rest"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, []interface{}{M("id", 1, "name", "test-repo", "full_name", "user1/test-repo", "private", false)})
			},
		})
	}

	// ---- Matrix ----
	{
		key := "syt_test_" + randHex(16)
		add("matrix", &mockService{
			creds:     []credEntry{{label: "access-token", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("errcode", "M_UNKNOWN_TOKEN", "error", "Invalid access token"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("event_id", "$test_event_"+randHex(4)))
			},
		})
	}

	// ---- Slack ----
	{
		key := "xoxb-test-" + randHex(16)
		add("slack", &mockService{
			creds:     []credEntry{{label: "bot-token", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 200, M("ok", false, "error", "invalid_auth"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("ok", true, "channel", "C01234567", "ts", "1234567890.123456",
					"message", M("text", "[test-server] Authentication successful.")))
			},
		})
	}

	// ---- Sentry ----
	{
		key := "sntrys_test" + randHex(16)
		add("sentry", &mockService{
			creds:     []credEntry{{label: "auth-token", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("detail", "Invalid token"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, []interface{}{M("id", "1", "slug", "test-project", "name", "Test Project")})
			},
		})
	}

	// ---- LINE ----
	{
		key := "test-line-" + randHex(24)
		add("line", &mockService{
			creds:     []credEntry{{label: "channel-access-token", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("message", "Authentication failed. Confirm that the access token in the authorization header is valid."))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("sentMessages", []interface{}{M("id", "1234567890", "quoteToken", "test-qt")}))
			},
		})
	}

	// ---- Notion ----
	{
		key := "ntn_test" + randHex(16)
		add("notion", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: bearerChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("object", "error", "status", 401, "code", "unauthorized", "message", "API token is invalid."))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("object", "list", "results", []interface{}{M("object", "page", "id", "test-page-id")}, "has_more", false))
			},
		})
	}

	// ---- Exa ----
	{
		key := "test-exa-" + randHex(16)
		add("exa", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: headerChecker("X-Api-Key", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("error", "Invalid API key"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("results", []interface{}{M("title", "Test Result", "url", "https://example.com", "text", "[test-server] Authentication successful.")}))
			},
		})
	}

	// ---- Brave ----
	{
		key := "BSAtest" + randHex(16)
		add("brave", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: headerChecker("X-Subscription-Token", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("type", "ErrorResponse", "error", M("id", "UNAUTHORIZED", "message", "Unauthorized", "status", 401)))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M(
					"query", M("original", r.URL.Query().Get("q")),
					"web", M("results", []interface{}{M("title", "Test Result", "url", "https://example.com", "description", "[test-server] Authentication successful.")}),
				))
			},
		})
	}

	// ---- GitLab ----
	{
		key := "glpat-test" + randHex(10)
		add("gitlab", &mockService{
			creds:     []credEntry{{label: "token", value: key}},
			checkAuth: headerChecker("Private-Token", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("message", "401 Unauthorized"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, []interface{}{M("id", 1, "name", "test-project", "path_with_namespace", "user1/test-project")})
			},
		})
	}

	// ---- Bing ----
	{
		key := "test-bing-" + randHex(16)
		add("bing", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: headerChecker("Ocp-Apim-Subscription-Key", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("error", M("code", "401", "message", "Access denied due to invalid subscription key.")))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M(
					"_type", "SearchResponse",
					"webPages", M("value", []interface{}{M("name", "Test Result", "url", "https://example.com", "snippet", "[test-server] Authentication successful.")}),
				))
			},
		})
	}

	// ---- Gemini ----
	{
		key := "AIzaTest" + randHex(16)
		add("gemini", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: queryChecker("key", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 400, M("error", M("code", 400, "message", "API key not valid. Please pass a valid API key.", "status", "INVALID_ARGUMENT")))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M(
					"candidates", []interface{}{M(
						"content", M("parts", []interface{}{M("text", "[test-server] Authentication successful.")}, "role", "model"),
						"finishReason", "STOP",
					)},
				))
			},
		})
	}

	// ---- Google Custom Search ----
	{
		key := "AIzaTest" + randHex(16)
		add("google-search", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: queryChecker("key", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 400, M("error", M("code", 400, "message", "API key not valid. Please pass a valid API key.", "status", "INVALID_ARGUMENT")))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M(
					"kind", "customsearch#search",
					"items", []interface{}{M("title", "Test Result", "link", "https://example.com", "snippet", "[test-server] Authentication successful.")},
				))
			},
		})
	}

	// ---- Trello ----
	{
		apiKey := "test-trello-key-" + randHex(8)
		token := "test-trello-token-" + randHex(16)
		add("trello", &mockService{
			creds:     []credEntry{{label: "api-key", value: apiKey}, {label: "token", value: token}},
			checkAuth: queryDoubleChecker("key", apiKey, "token", token),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("message", "invalid key", "error", "UNAUTHORIZED"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, []interface{}{M("id", "test-board-id", "name", "Test Board", "url", "https://trello.com/b/test")})
			},
		})
	}

	// ---- Tavily ----
	{
		key := "tvly-test" + randHex(16)
		add("tavily", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: bodyChecker("api_key", key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("detail", "Could not validate credentials"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M(
					"query", "test",
					"results", []interface{}{M("title", "Test Result", "url", "https://example.com", "content", "[test-server] Authentication successful.")},
				))
			},
		})
	}

	// ---- Atlassian ----
	{
		email := "test@example.com"
		token := "ATATTtest" + randHex(16)
		add("atlassian", &mockService{
			creds:     []credEntry{{label: "email", value: email}, {label: "token", value: token}},
			checkAuth: basicChecker(email, token),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("type", "error", "error", M("message", "This resource requires authentication.")))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("values", []interface{}{M("id", 1, "title", "Test PR", "state", "OPEN")}))
			},
		})
	}

	// ---- Telegram ----
	{
		token := fmt.Sprintf("%d:%s", 1234567890+time.Now().UnixNano()%1000, "ABCDtest"+randHex(8))
		add("telegram", &mockService{
			creds:     []credEntry{{label: "bot-token", value: token}},
			checkAuth: pathTokenChecker("telegram", token),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("ok", false, "error_code", 401, "description", "Unauthorized"))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("ok", true, "result", M("message_id", 1, "chat", M("id", 123456789), "text", "[test-server] Authentication successful.")))
			},
		})
	}

	// ---- Discord ----
	{
		key := "test-discord-" + randHex(16)
		add("discord", &mockService{
			creds:     []credEntry{{label: "bot-token", value: key}},
			checkAuth: botChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("message", "401: Unauthorized", "code", 0))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("id", "1234567890", "type", 0, "content", "[test-server] Authentication successful.", "channel_id", "123456789"))
			},
		})
	}

	// ---- Linear ----
	{
		key := "lin_api_test" + randHex(16)
		add("linear", &mockService{
			creds:     []credEntry{{label: "api-key", value: key}},
			checkAuth: rawAuthChecker(key),
			onFail: func(w http.ResponseWriter) {
				writeJSON(w, 401, M("errors", []interface{}{M("message", "Authentication required", "extensions", M("code", "UNAUTHENTICATED"))}))
			},
			onOK: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, M("data", M("issues", M("nodes", []interface{}{M("id", "test-id", "title", "Test Issue", "state", M("name", "Todo"))}))))
			},
		})
	}

	return s, allCreds
}

// --- TLS Certificate ---

func generateSelfSignedCert(certPath, keyPath string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:     []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	if certPath != "" {
		os.WriteFile(certPath, certPEM, 0644)
		log.Printf("Certificate saved to %s", certPath)
	}
	if keyPath != "" {
		os.WriteFile(keyPath, keyPEM, 0600)
		log.Printf("Private key saved to %s", keyPath)
	}

	return tls.X509KeyPair(certPEM, keyPEM)
}

// --- Request logging ---

func logHTTPRequest(service string, r *http.Request) {
	fmt.Printf("--- [%s] %s %s ---\n", service, r.Method, r.URL.String())
	for name, values := range r.Header {
		for _, v := range values {
			fmt.Printf("  %s: %s\n", name, v)
		}
	}
	if r.Body != nil && r.ContentLength != 0 {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			fmt.Printf("  Body: %s\n", string(body))
			// Replace body so handlers can still read it
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}
	fmt.Println()
}

// --- Main ---

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: test-server [options]\n\nA mock HTTPS server that mimics authentication of all 26 supported services.\n\nOptions:\n")
		flag.PrintDefaults()
	}

	port := flag.Int("port", 9443, "HTTPS port")
	certFile := flag.String("cert", "test-server/cert.pem", "TLS certificate file path")
	keyFile := flag.String("key", "test-server/key.pem", "TLS private key file path")
	genCert := flag.Bool("gen-cert", false, "generate a new self-signed certificate (overwrites existing)")
	logRequest := flag.Bool("log-request", false, "log incoming requests to stdout")
	flag.Parse()

	if flag.NArg() > 0 && flag.Arg(0) == "help" {
		flag.Usage()
		os.Exit(0)
	}

	// Load or generate TLS cert
	var tlsCert tls.Certificate
	var err error
	if *genCert {
		tlsCert, err = generateSelfSignedCert(*certFile, *keyFile)
	} else if _, e := os.Stat(*certFile); e == nil {
		tlsCert, err = tls.LoadX509KeyPair(*certFile, *keyFile)
		log.Printf("Loaded certificate from %s", *certFile)
	} else {
		tlsCert, err = generateSelfSignedCert(*certFile, *keyFile)
	}
	if err != nil {
		log.Fatalf("Certificate error: %v", err)
	}

	// Build services and print credentials
	svcs, allCreds := buildServices()

	fmt.Println("=== Test Credentials ===")
	maxLabel := 0
	for _, c := range allCreds {
		if len(c.label) > maxLabel {
			maxLabel = len(c.label)
		}
	}
	for _, c := range allCreds {
		fmt.Printf("  %-*s  %s\n", maxLabel, c.label, c.value)
	}
	fmt.Println("========================")

	// Register handlers
	mux := http.NewServeMux()
	for name, svc := range svcs {
		name, svc := name, svc
		mux.HandleFunc("/"+name+"/", func(w http.ResponseWriter, r *http.Request) {
			if *logRequest {
				logHTTPRequest(name, r)
			}
			if svc.checkAuth(r) {
				svc.onOK(w, r)
			} else {
				svc.onFail(w)
			}
		})
	}

	// Echo handler — reflects all request headers in the response body
	mux.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		if *logRequest {
			logHTTPRequest("echo", r)
		}
		headers := make(map[string]string)
		for name, vals := range r.Header {
			headers[name] = vals[0]
		}
		writeJSONWithEncoding(w, r, 200, M("headers", headers, "method", r.Method, "path", r.URL.Path))
	})

	// Root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if *logRequest {
			logHTTPRequest("(root)", r)
		}
		if r.URL.Path != "/" {
			writeJSON(w, 404, M("error", "unknown service. Use /SERVICE_NAME/..."))
			return
		}
		names := make([]string, 0, len(svcs))
		for name := range svcs {
			names = append(names, name)
		}
		writeJSON(w, 200, M("services", names, "usage", fmt.Sprintf("https://localhost:%d/SERVICE_NAME/...", *port)))
	})

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	ln, err := tls.Listen("tcp", addr, &tls.Config{Certificates: []tls.Certificate{tlsCert}})
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Mock API server running on https://localhost:%d", *port)
	log.Printf("Services: %d registered", len(svcs))
	log.Fatal((&http.Server{Handler: mux}).Serve(ln))
}
