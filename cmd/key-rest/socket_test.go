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
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		if _, err := conn.Write(data); err != nil {
			return
		}
	}
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
