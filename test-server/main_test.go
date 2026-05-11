package main

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// --- Pure helpers ---

func TestRandHex(t *testing.T) {
	s := randHex(4)
	if len(s) != 8 {
		t.Fatalf("expected 8 hex chars, got %d (%q)", len(s), s)
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("non-hex char %q in %q", c, s)
		}
	}
}

func TestM(t *testing.T) {
	got := M("a", 1, "b", "two")
	if got["a"] != 1 || got["b"] != "two" {
		t.Fatalf("got %#v", got)
	}
	// Odd-length input: the trailing unpaired arg is silently dropped.
	got = M("a", 1, "stray")
	if _, ok := got["stray"]; ok {
		t.Fatalf("unpaired key should be ignored, got %#v", got)
	}
}

func TestTruncateKeyShort(t *testing.T) {
	if truncateKey("ab") != "**" {
		t.Fatalf("short input should be all asterisks")
	}
}

func TestTruncateKeyTypicalOpenAIKey(t *testing.T) {
	got := truncateKey("sk-test-abcdef1234")
	// "sk-test-" prefix, then asterisks, then last 4 chars "1234".
	if !strings.HasPrefix(got, "sk-test-") {
		t.Fatalf("prefix lost: %q", got)
	}
	if !strings.HasSuffix(got, "1234") {
		t.Fatalf("suffix lost: %q", got)
	}
	if !strings.Contains(got, "*") {
		t.Fatalf("no asterisks: %q", got)
	}
}

func TestTruncateKeyNoHyphen(t *testing.T) {
	// 12 chars, no hyphen → prefix forced to 3.
	got := truncateKey("abcdefghijkl")
	if got[:3] != "abc" || got[len(got)-4:] != "ijkl" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateKeyPrefixClampedToLeaveSuffix(t *testing.T) {
	// "abcd-ef-g": len=9, last hyphen at index 7 → prefixEnd would be 8,
	// but len-4=5; the clamp at lines 242-244 caps prefixEnd to 5 so the
	// trailing 4 chars (including the hyphen) stay visible.
	got := truncateKey("abcd-ef-g")
	if got[len(got)-4:] != "ef-g" {
		t.Fatalf("expected last 4 chars 'ef-g', got %q (full %q)", got[len(got)-4:], got)
	}
}

func TestPercentEncodeAll(t *testing.T) {
	if percentEncodeAll("abc123") != "abc123" {
		t.Fatal("alphanumeric should pass through")
	}
	if percentEncodeAll("a-b!c") != "a%2Db%21c" {
		t.Fatalf("got %q", percentEncodeAll("a-b!c"))
	}
}

// --- Auth checker factories ---

func reqWithHeader(name, value string) *http.Request {
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set(name, value)
	return r
}

func TestBearerChecker(t *testing.T) {
	f := bearerChecker("tok123")
	if !f(reqWithHeader("Authorization", "Bearer tok123")) {
		t.Fatal("should match")
	}
	if f(reqWithHeader("Authorization", "Bearer wrong")) {
		t.Fatal("should not match")
	}
}

func TestBotChecker(t *testing.T) {
	f := botChecker("xyz")
	if !f(reqWithHeader("Authorization", "Bot xyz")) {
		t.Fatal("match")
	}
	if f(reqWithHeader("Authorization", "Bot wrong")) {
		t.Fatal("no match")
	}
}

func TestRawAuthChecker(t *testing.T) {
	f := rawAuthChecker("raw-token")
	if !f(reqWithHeader("Authorization", "raw-token")) {
		t.Fatal("match")
	}
	if f(reqWithHeader("Authorization", "other")) {
		t.Fatal("no match")
	}
}

func TestHeaderChecker(t *testing.T) {
	f := headerChecker("X-Api-Key", "k")
	if !f(reqWithHeader("X-Api-Key", "k")) {
		t.Fatal("match")
	}
	if f(reqWithHeader("X-Api-Key", "nope")) {
		t.Fatal("no match")
	}
}

func TestQueryChecker(t *testing.T) {
	f := queryChecker("api_key", "secret")
	r := httptest.NewRequest("GET", "/x?api_key=secret", nil)
	if !f(r) {
		t.Fatal("match")
	}
	r = httptest.NewRequest("GET", "/x?api_key=wrong", nil)
	if f(r) {
		t.Fatal("no match")
	}
}

func TestQueryDoubleChecker(t *testing.T) {
	f := queryDoubleChecker("a", "1", "b", "2")
	if !f(httptest.NewRequest("GET", "/?a=1&b=2", nil)) {
		t.Fatal("match")
	}
	if f(httptest.NewRequest("GET", "/?a=1&b=wrong", nil)) {
		t.Fatal("no match")
	}
}

func TestBasicChecker(t *testing.T) {
	f := basicChecker("user", "pass")
	// echo -n "user:pass" | base64 → dXNlcjpwYXNz
	if !f(reqWithHeader("Authorization", "Basic dXNlcjpwYXNz")) {
		t.Fatal("match")
	}
	if f(reqWithHeader("Authorization", "Basic d3Jvbmc=")) {
		t.Fatal("no match")
	}
}

func TestPathTokenChecker(t *testing.T) {
	f := pathTokenChecker("svc", "abc")
	r := httptest.NewRequest("GET", "/svc/botabc/sendMessage", nil)
	if !f(r) {
		t.Fatal("match")
	}
	// Wrong token.
	r = httptest.NewRequest("GET", "/svc/botxxx/sendMessage", nil)
	if f(r) {
		t.Fatal("no match (wrong token)")
	}
	// Missing "bot" prefix.
	r = httptest.NewRequest("GET", "/svc/abc/sendMessage", nil)
	if f(r) {
		t.Fatal("no match (no bot prefix)")
	}
	// Token without trailing slash also accepted (whole tail = token).
	r = httptest.NewRequest("GET", "/svc/botabc", nil)
	if !f(r) {
		t.Fatal("token without trailing slash should match")
	}
}

func TestBodyChecker(t *testing.T) {
	f := bodyChecker("api_key", "secret")
	// Match: JSON body with api_key="secret".
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"api_key":"secret"}`))
	if !f(r) {
		t.Fatal("match")
	}
	// No match: wrong value.
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"api_key":"wrong"}`))
	if f(r) {
		t.Fatal("no match (wrong value)")
	}
	// No match: malformed JSON.
	r = httptest.NewRequest("POST", "/", strings.NewReader(`not-json`))
	if f(r) {
		t.Fatal("no match (bad JSON)")
	}
	// No match: missing field.
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"other":"x"}`))
	if f(r) {
		t.Fatal("no match (missing field)")
	}
}

// --- writeJSON / writeJSONWithEncoding ---

func TestWriteJSONNoRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, M("x", 1))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"x":1`) {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func writeWithEncoding(t *testing.T, encoding string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", nil)
	if encoding != "" {
		r.Header.Set("Accept-Encoding", encoding)
	}
	writeJSONWithEncoding(rec, r, 200, M("k", "v"))
	return rec
}

func TestWriteJSONEncodingDefault(t *testing.T) {
	rec := writeWithEncoding(t, "")
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("no encoding expected")
	}
	if !strings.Contains(rec.Body.String(), `"k":"v"`) {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestWriteJSONEncodingGzip(t *testing.T) {
	rec := writeWithEncoding(t, "gzip")
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("missing gzip encoding")
	}
	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(gr)
	if !strings.Contains(string(data), `"k":"v"`) {
		t.Fatalf("got %q", data)
	}
}

func TestWriteJSONEncodingBrotli(t *testing.T) {
	rec := writeWithEncoding(t, "br")
	if rec.Header().Get("Content-Encoding") != "br" {
		t.Fatal("missing brotli encoding")
	}
	data, _ := io.ReadAll(brotli.NewReader(rec.Body))
	if !strings.Contains(string(data), `"k":"v"`) {
		t.Fatalf("got %q", data)
	}
}

func TestWriteJSONEncodingDeflate(t *testing.T) {
	rec := writeWithEncoding(t, "deflate")
	if rec.Header().Get("Content-Encoding") != "deflate" {
		t.Fatal("missing deflate encoding")
	}
	fr := flate.NewReader(rec.Body)
	defer fr.Close()
	data, _ := io.ReadAll(fr)
	if !strings.Contains(string(data), `"k":"v"`) {
		t.Fatalf("got %q", data)
	}
}

func TestWriteJSONEncodingZstd(t *testing.T) {
	rec := writeWithEncoding(t, "zstd")
	if rec.Header().Get("Content-Encoding") != "zstd" {
		t.Fatal("missing zstd encoding")
	}
	zr, err := zstd.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	data, _ := io.ReadAll(zr)
	if !strings.Contains(string(data), `"k":"v"`) {
		t.Fatalf("got %q", data)
	}
}

// --- OpenAI response factories ---

func TestOpenaiError(t *testing.T) {
	rec := httptest.NewRecorder()
	openaiError("sk-test-abcdef1234")(rec)
	if rec.Code != 401 {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errObj := body["error"].(map[string]any)
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "Incorrect API key") {
		t.Fatalf("unexpected message %q", msg)
	}
}

func TestOpenaiOK(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/openai/v1/chat/completions",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	openaiOK("gpt-test")(rec, r)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "gpt-test" {
		t.Fatalf("model field: %v", body["model"])
	}
}

// --- buildServices ---

func TestBuildServices(t *testing.T) {
	svcs, creds := buildServices()
	if len(svcs) == 0 {
		t.Fatal("expected at least one service")
	}
	if len(creds) == 0 {
		t.Fatal("expected at least one credential entry")
	}
	// Spot-check that openai is wired up.
	if _, ok := svcs["openai"]; !ok {
		t.Fatal("openai service missing")
	}
	// Each service should have a non-nil onOK + onFail + checkAuth.
	for name, svc := range svcs {
		if svc.checkAuth == nil || svc.onOK == nil || svc.onFail == nil {
			t.Fatalf("service %q has nil hook", name)
		}
		if len(svc.creds) == 0 {
			t.Fatalf("service %q has no creds", name)
		}
	}
}

// TestBuildServicesAllHandlersInvocable iterates every service produced by
// buildServices and invokes its onOK and onFail closures so the inline
// response factories defined for each provider get coverage.
func TestBuildServicesAllHandlersInvocable(t *testing.T) {
	svcs, _ := buildServices()
	for name, svc := range svcs {
		// onFail — some services (e.g. Slack) report failure via body fields
		// while keeping HTTP 200, so we just assert the handler wrote a
		// non-empty body without panicking.
		rec := httptest.NewRecorder()
		svc.onFail(rec)
		if rec.Body.Len() == 0 {
			t.Fatalf("%s: onFail wrote empty body", name)
		}
		// onOK with a generic POST body
		rec = httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/"+name+"/x",
			strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
		svc.onOK(rec, req)
		if rec.Body.Len() == 0 {
			t.Fatalf("%s: onOK wrote empty body", name)
		}
	}
}

// --- generateSelfSignedCert ---

func TestGenerateSelfSignedCertWritesFiles(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "c.pem")
	keyPath := filepath.Join(dir, "k.pem")
	cert, err := generateSelfSignedCert(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Certificate == nil {
		t.Fatal("returned cert has no chain")
	}
	// Both files should exist.
	for _, p := range []string{certPath, keyPath} {
		if data, err := os.ReadFile(p); err != nil || len(data) == 0 {
			t.Fatalf("expected %s to be written, err=%v", p, err)
		}
	}
}

func TestGenerateSelfSignedCertSkipsEmptyPath(t *testing.T) {
	// Empty paths exercise the `if certPath != ""` / `if keyPath != ""`
	// false branches: cert generated in memory but not persisted.
	cert, err := generateSelfSignedCert("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cert.Certificate == nil {
		t.Fatal("returned cert has no chain")
	}
}

// --- logHTTPRequest ---

func TestLogHTTPRequestHasBody(t *testing.T) {
	// Capture stdout? Hard without a refactor. Just call and confirm no
	// panic, and that the request body remains readable afterwards.
	r := httptest.NewRequest("POST", "/x", strings.NewReader("payload"))
	r.Header.Set("X-Test", "value")
	logHTTPRequest("svc", r)
	// Body must still be readable for downstream handlers.
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatalf("body lost or altered: %q", data)
	}
}

func TestLogHTTPRequestEmptyBody(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	logHTTPRequest("svc", r)
}

// --- runMain ---

func TestRunMainHelpSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runMain([]string{"help"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected Usage banner in stderr, got %q", stderr.String())
	}
}

func TestRunMainBadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runMain([]string{"--no-such-flag"}, &stdout, &stderr, nil)
	if code != 2 {
		t.Fatalf("expected exit 2 from flag parse error, got %d", code)
	}
}

func TestRunMainCertLoadOrGenerateFails(t *testing.T) {
	// Point cert path at a directory we can't write into.
	dir := t.TempDir()
	// Create a file at the cert path so os.Stat succeeds, then make it
	// unloadable by writing garbage.
	certPath := dir + "/c.pem"
	keyPath := dir + "/k.pem"
	os.WriteFile(certPath, []byte("not a pem"), 0644)
	os.WriteFile(keyPath, []byte("not a pem"), 0644)

	var stdout, stderr bytes.Buffer
	code := runMain([]string{
		"--cert=" + certPath,
		"--key=" + keyPath,
	}, &stdout, &stderr, nil)
	if code != 1 {
		t.Fatalf("expected exit 1 from cert load failure, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Certificate error") {
		t.Fatalf("expected cert-error message, got %q", stderr.String())
	}
}

func TestRunMainServeAndShutdown(t *testing.T) {
	dir := t.TempDir()
	// No --gen-cert and no existing cert at this path → runMain falls into
	// the `else` branch and generates one on the fly.
	certPath := dir + "/c.pem"
	keyPath := dir + "/k.pem"

	var stdout, stderr bytes.Buffer
	shutdown := make(chan struct{})
	done := make(chan int, 1)
	go func() {
		done <- runMain([]string{
			"--port=0",
			"--cert=" + certPath,
			"--key=" + keyPath,
			"--log-request",
		}, &stdout, &stderr, shutdown)
	}()

	// Wait for the server to write the listening address line.
	addr := waitForListenAddr(t, &stderr, 3*time.Second)

	// Exercise every handler branch by sending HTTPS requests. We accept
	// the server's self-signed cert.
	cli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Root handler: known path.
	resp, err := cli.Get("https://" + addr + "/")
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("root status %d", resp.StatusCode)
	}

	// Root handler: unknown path → 404 branch.
	resp, err = cli.Get("https://" + addr + "/no-such-service")
	if err != nil {
		t.Fatalf("unknown: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("unknown status %d", resp.StatusCode)
	}

	// Echo handler.
	resp, err = cli.Get("https://" + addr + "/echo/anything")
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	resp.Body.Close()

	// Percent-echo handler.
	resp, err = cli.Get("https://" + addr + "/percent-echo/")
	if err != nil {
		t.Fatalf("percent-echo: %v", err)
	}
	resp.Body.Close()

	// Service handler — onFail branch (no Authorization).
	resp, err = cli.Get("https://" + addr + "/openai/v1/chat/completions")
	if err != nil {
		t.Fatalf("openai fail: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("openai without auth should fail")
	}

	close(shutdown)

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runMain did not return within 3s of shutdown")
	}

	if !strings.Contains(stdout.String(), "=== Test Credentials ===") {
		t.Fatalf("missing credentials banner in stdout: %q", stdout.String())
	}
}

// TestRunMainGenCertFlag covers the `--gen-cert` branch of the cert-loading
// switch.
func TestRunMainGenCertFlag(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	shutdown := make(chan struct{})
	done := make(chan int, 1)
	go func() {
		done <- runMain([]string{
			"--port=0",
			"--cert=" + filepath.Join(dir, "c.pem"),
			"--key=" + filepath.Join(dir, "k.pem"),
			"--gen-cert",
		}, &stdout, &stderr, shutdown)
	}()
	waitForListenAddr(t, &stderr, 3*time.Second)
	close(shutdown)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runMain hung")
	}
}

// TestRunMainAuthSuccess covers the `if svc.checkAuth(r) { svc.onOK(...) }`
// branch by extracting the openai key from the credentials banner and
// sending an authenticated request.
func TestRunMainAuthSuccess(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	shutdown := make(chan struct{})
	done := make(chan int, 1)
	go func() {
		done <- runMain([]string{
			"--port=0",
			"--cert=" + filepath.Join(dir, "c.pem"),
			"--key=" + filepath.Join(dir, "k.pem"),
		}, &stdout, &stderr, shutdown)
	}()
	addr := waitForListenAddr(t, &stderr, 3*time.Second)

	// Parse the printed credentials banner for the openai api-key.
	key := extractCred(t, stdout.String(), "openai api-key")

	cli := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	req, _ := http.NewRequest("POST", "https://"+addr+"/openai/v1/chat/completions",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("auth'd openai got status %d", resp.StatusCode)
	}

	close(shutdown)
	<-done
}

// extractCred parses the "=== Test Credentials ===" banner and returns the
// value associated with the given label.
func extractCred(t *testing.T, s, label string) string {
	t.Helper()
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			return fields[len(fields)-1]
		}
	}
	t.Fatalf("could not find credential %q in stdout: %q", label, s)
	return ""
}

// TestRunMainListenFails covers the tls.Listen failure branch.
func TestRunMainListenFails(t *testing.T) {
	// Port 1 below 1024 typically requires root; on unprivileged test runs
	// tls.Listen fails with EACCES. Combined with --gen-cert so the cert
	// is generated in-memory regardless of paths.
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runMain([]string{
		"--port=1",
		"--cert=" + filepath.Join(dir, "c.pem"),
		"--key=" + filepath.Join(dir, "k.pem"),
		"--gen-cert",
	}, &stdout, &stderr, nil)
	if code != 1 {
		t.Skipf("expected exit 1 from privileged port; runner may have privilege, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Failed to listen") {
		t.Fatalf("expected listen-fail message, got %q", stderr.String())
	}
}

// waitForListenAddr extracts "running on https://ADDR" from stderr buffer.
func waitForListenAddr(t *testing.T, stderr *bytes.Buffer, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s := stderr.String()
		if idx := strings.Index(s, "running on https://"); idx >= 0 {
			rest := s[idx+len("running on https://"):]
			end := strings.IndexByte(rest, '\n')
			if end >= 0 {
				return rest[:end]
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start within %v; stderr=%q", timeout, stderr.String())
	return ""
}
