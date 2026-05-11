package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeDaemon is a stub Unix-socket server that mimics the protocol used by
// the daemon (JSON-over-newline). It also writes a pid file matching the
// current process so cmd/key-rest's IsRunning() check returns true. Tests
// configure responses for each request type.
type fakeDaemon struct {
	dir      string
	listener net.Listener
	mu       sync.Mutex
	handler  func(req map[string]any) any // returns response object
	wg       sync.WaitGroup
	stopped  bool
}

func startFakeDaemon(t *testing.T, dir string, handler func(req map[string]any) any) *fakeDaemon {
	t.Helper()
	sockPath := filepath.Join(dir, "key-rest.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// Write our own PID so IsRunning() reports true.
	pidPath := filepath.Join(dir, "key-rest.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	fd := &fakeDaemon{dir: dir, listener: ln, handler: handler}
	fd.wg.Add(1)
	go fd.serve()
	t.Cleanup(fd.stop)
	return fd
}

func (f *fakeDaemon) serve() {
	defer f.wg.Done()
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

// rawBytes lets a fakeDaemon handler bypass JSON marshaling and write the
// given bytes verbatim. Useful to exercise client-side scanner.Scan /
// json.Unmarshal failure paths.
type rawBytes []byte

// closeNow signals the fakeDaemon to close the connection without sending
// any response (covers scanner.Scan returning false).
type closeNow struct{}

func (f *fakeDaemon) handle(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return
		}
		f.mu.Lock()
		h := f.handler
		f.mu.Unlock()
		resp := h(req)
		switch v := resp.(type) {
		case closeNow:
			return
		case rawBytes:
			conn.Write([]byte(v))
			return
		default:
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			if _, err := conn.Write(data); err != nil {
				return
			}
		}
	}
}

// plantPidOnly writes a pid file pointing at our own process so IsRunning
// reports true, but does NOT start a socket server. cmdList/cmdEnable/etc
// then try to dial the missing socket and fail.
func plantPidOnly(t *testing.T, dir string) {
	t.Helper()
	pidPath := filepath.Join(dir, "key-rest.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(pidPath) })
}

func (f *fakeDaemon) stop() {
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		return
	}
	f.stopped = true
	f.mu.Unlock()
	f.listener.Close()
	f.wg.Wait()
	os.Remove(filepath.Join(f.dir, "key-rest.pid"))
}

func TestRunStatusRunningWithFakeDaemon(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		// status doesn't talk to the socket — but checkDaemonVersion does.
		if req["type"] == "version" {
			return map[string]any{"body": version}
		}
		return map[string]any{"body": ""}
	})

	code, out, _ := runArgs("status")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "running") {
		t.Fatalf("expected 'running', got %q", out)
	}
}

func TestRunStatusVersionMismatch(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		return map[string]any{"body": "0.0.0-old"}
	})

	code, _, errOut := runArgs("status")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(errOut, "warning: daemon version 0.0.0-old") {
		t.Fatalf("expected version-mismatch warning, got %q", errOut)
	}
}

func TestRunListRunningWithKeys(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		switch req["type"] {
		case "version":
			return map[string]any{"body": version}
		case "list":
			body := `[{"uri":"u/s/a","url_prefix":"https://a.example/","disabled":false},` +
				`{"uri":"u/s/b","url_prefix":"https://b.example/","disabled":true}]`
			return map[string]any{"body": body}
		}
		return map[string]any{"body": ""}
	})

	code, out, _ := runArgs("list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "key-rest://u/s/a: https://a.example/ enabled") {
		t.Fatalf("expected enabled entry, got %q", out)
	}
	if !strings.Contains(out, "key-rest://u/s/b: https://b.example/ disabled") {
		t.Fatalf("expected disabled entry, got %q", out)
	}
}

func TestRunListRunningEmpty(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "list" {
			return map[string]any{"body": "[]"}
		}
		return map[string]any{"body": version}
	})

	code, out, _ := runArgs("list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "no keys registered") {
		t.Fatalf("expected 'no keys registered', got %q", out)
	}
}

func TestRunListRunningError(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "list" {
			return map[string]any{
				"error": map[string]any{"code": "BAD", "message": "oops"},
			}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("list")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to query daemon") {
		t.Fatalf("expected query-error, got %q", errOut)
	}
}

func TestRunEnableSuccess(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		switch req["type"] {
		case "enable":
			return map[string]any{"body": "3", "status": 200}
		}
		return map[string]any{"body": version}
	})

	code, out, _ := runArgs("enable", "u/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "3 key(s) enabled") {
		t.Fatalf("expected '3 key(s) enabled', got %q", out)
	}
}

func TestRunEnableErrorResponse(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "enable" {
			return map[string]any{"error": map[string]any{"code": "X", "message": "no"}}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("enable", "u/")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to enable") {
		t.Fatalf("expected failed-to-enable, got %q", errOut)
	}
}

func TestRunDisableSuccess(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "disable" {
			return map[string]any{"body": "1"}
		}
		return map[string]any{"body": version}
	})

	code, out, _ := runArgs("disable", "u/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "1 key(s) disabled") {
		t.Fatalf("expected '1 key(s) disabled', got %q", out)
	}
}

func TestRunAddNotifiesDaemon(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	gotReload := false
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "reload" {
			gotReload = true
			return map[string]any{}
		}
		return map[string]any{"body": version}
	})

	code, out, errOut := runArgs("add", "u/s/k", "https://api.example.com/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(out, "key added: u/s/k") {
		t.Fatalf("expected key-added, got %q", out)
	}
	if !gotReload {
		t.Fatal("expected daemon to receive reload request")
	}
}

// --- net.DialTimeout failures (pid file present, no socket) ---

func TestRunListDialFails(t *testing.T) {
	dir := withTempDir(t)
	plantPidOnly(t, dir)

	code, _, errOut := runArgs("list")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to query daemon") {
		t.Fatalf("expected dial-fail surface, got %q", errOut)
	}
}

func TestRunAddSendReloadDialFails(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	plantPidOnly(t, dir)

	code, out, errOut := runArgs("add", "u/s/k", "https://api.example.com/")
	if code != 0 {
		t.Fatalf("expected exit 0 (add succeeds, reload fails as warning), got %d", code)
	}
	if !strings.Contains(out, "key added") {
		t.Fatalf("expected key-added, got %q", out)
	}
	if !strings.Contains(errOut, "warning: failed to notify daemon") {
		t.Fatalf("expected reload warning, got %q", errOut)
	}
}

// --- scanner.Scan failure (daemon closes connection without writing) ---

func TestRunEnableNoResponse(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "enable" {
			return closeNow{}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("enable", "u/")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "no response from daemon") {
		t.Fatalf("expected no-response error, got %q", errOut)
	}
}

func TestRunListNoResponse(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "list" {
			return closeNow{}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("list")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "no response from daemon") {
		t.Fatalf("expected no-response error, got %q", errOut)
	}
}

// --- malformed JSON in wrapper ---

func TestRunEnableMalformedJSON(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "enable" {
			return rawBytes("{not-json\n")
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("enable", "u/")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to enable") {
		t.Fatalf("expected failed-to-enable, got %q", errOut)
	}
}

func TestRunListMalformedJSON(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "list" {
			return rawBytes("{not-json\n")
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("list")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to query daemon") {
		t.Fatalf("expected query-fail surface, got %q", errOut)
	}
}

// --- sendList: wrapper valid, body is not a JSON array ---

func TestRunListMalformedBody(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "list" {
			return map[string]any{"body": "this-is-not-an-array"}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("list")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to query daemon") {
		t.Fatalf("expected query-fail surface, got %q", errOut)
	}
}

// --- checkDaemonVersion: scanner.Scan / Unmarshal failures (silent) ---

func TestRunStatusVersionNoResponse(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "version" {
			return closeNow{}
		}
		return map[string]any{"body": ""}
	})

	code, out, _ := runArgs("status")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "running") {
		t.Fatalf("expected 'running', got %q", out)
	}
}

func TestRunStatusVersionMalformedJSON(t *testing.T) {
	dir := withTempDir(t)
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "version" {
			return rawBytes("{not-json\n")
		}
		return map[string]any{"body": ""}
	})

	code, out, _ := runArgs("status")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "running") {
		t.Fatalf("expected 'running', got %q", out)
	}
}

// --- sendReload error paths (triggered via cmdAdd's reload notification) ---

func TestRunAddReloadNoResponse(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "reload" {
			return closeNow{}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(errOut, "warning: failed to notify daemon") {
		t.Fatalf("expected reload warning, got %q", errOut)
	}
	if !strings.Contains(errOut, "no response from daemon") {
		t.Fatalf("expected no-response surface in warning, got %q", errOut)
	}
}

func TestRunAddReloadMalformedJSON(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "reload" {
			return rawBytes("{not-json\n")
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(errOut, "warning: failed to notify daemon") {
		t.Fatalf("expected reload warning, got %q", errOut)
	}
}

func TestRunAddReloadErrorResponse(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "reload" {
			return map[string]any{"error": map[string]any{"code": "X", "message": "no-can-do"}}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(errOut, "no-can-do") {
		t.Fatalf("expected daemon error in warning, got %q", errOut)
	}
}

// TestRunRemoveReloadFailureWarns covers cmdRemove's sendReload-failure
// warning branch.
func TestRunRemoveReloadFailureWarns(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	// Add a key with no daemon running so the entry exists.
	if err := os.Remove(filepath.Join(dir, "key-rest.pid")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/"); code != 0 {
		t.Fatalf("add failed: %d %s", code, errOut)
	}
	// Now start a fake daemon that errors on reload.
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "reload" {
			return map[string]any{"error": map[string]any{"code": "X", "message": "boom"}}
		}
		return map[string]any{"body": version}
	})

	code, _, errOut := runArgs("remove", "u/s/k")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(errOut, "warning: failed to notify daemon") {
		t.Fatalf("expected reload warning, got %q", errOut)
	}
}

func TestRunRemoveNotifiesDaemon(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	// First add (with no daemon) so the key exists. Stop our pid file
	// trick temporarily by removing the pidfile before add.
	if err := os.Remove(filepath.Join(dir, "key-rest.pid")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/"); code != 0 {
		t.Fatalf("add failed: %d %s", code, errOut)
	}
	// Now plant the pid + start fake daemon to handle reload on remove.
	gotReload := false
	startFakeDaemon(t, dir, func(req map[string]any) any {
		if req["type"] == "reload" {
			gotReload = true
			return map[string]any{}
		}
		return map[string]any{"body": version}
	})

	code, out, _ := runArgs("remove", "u/s/k")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "key removed: u/s/k") {
		t.Fatalf("expected key-removed, got %q", out)
	}
	if !gotReload {
		t.Fatal("expected daemon to receive reload request")
	}
}
