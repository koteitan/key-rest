package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

// withTempDir sets KEY_REST_DIR to a fresh tempdir for the duration of the
// test so each in-process run() call has an isolated keystore.
func withTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KEY_REST_DIR", dir)
	return dir
}

// withFakePassphrase replaces readPassphraseFn with a deterministic stub
// that returns the supplied responses in order. Restores on cleanup.
func withFakePassphrase(t *testing.T, responses ...string) {
	t.Helper()
	orig := readPassphraseFn
	idx := 0
	readPassphraseFn = func(_ io.Reader, _ io.Writer, _ string) []byte {
		if idx >= len(responses) {
			t.Fatalf("readPassphraseFn called more times than responses provided")
		}
		r := responses[idx]
		idx++
		return []byte(r)
	}
	t.Cleanup(func() { readPassphraseFn = orig })
}

func runArgs(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	full := append([]string{"key-rest"}, args...)
	code := run(full, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunNoArgs(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs()
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "Usage") {
		t.Fatalf("expected Usage in stderr, got %q", errOut)
	}
}

func TestRunVersion(t *testing.T) {
	withTempDir(t)
	code, out, _ := runArgs("version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.HasPrefix(out, "key-rest ") {
		t.Fatalf("expected key-rest prefix, got %q", out)
	}
	if strings.TrimSpace(out) != "key-rest "+version {
		t.Fatalf("expected key-rest %s, got %q", version, out)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("bogus")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "unknown command: bogus") {
		t.Fatalf("expected unknown-command error, got %q", errOut)
	}
	if !strings.Contains(errOut, "Usage") {
		t.Fatalf("expected Usage banner, got %q", errOut)
	}
}

func TestRunStatusStopped(t *testing.T) {
	withTempDir(t)
	code, out, _ := runArgs("status")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(out) != "stopped" {
		t.Fatalf("expected stopped, got %q", out)
	}
}

func TestRunStopWithoutDaemon(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("stop")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if errOut == "" {
		t.Fatal("expected stderr output for stop without daemon")
	}
}

func TestRunListEmpty(t *testing.T) {
	withTempDir(t)
	code, out, _ := runArgs("list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(out) != "no keys registered" {
		t.Fatalf("expected 'no keys registered', got %q", out)
	}
}

func TestRunRemoveMissingArg(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("remove")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "Usage: key-rest remove") {
		t.Fatalf("expected usage, got %q", errOut)
	}
}

func TestRunRemoveNonexistent(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("remove", "user/svc/key")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to remove key") {
		t.Fatalf("expected failed-to-remove, got %q", errOut)
	}
}

func TestRunEnableMissingArg(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("enable")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "Usage: key-rest enable") {
		t.Fatalf("expected usage, got %q", errOut)
	}
}

func TestRunDisableMissingArg(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("disable")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "Usage: key-rest disable") {
		t.Fatalf("expected usage, got %q", errOut)
	}
}

func TestRunEnableNoDaemon(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("enable", "user/")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to enable") {
		t.Fatalf("expected failed-to-enable error, got %q", errOut)
	}
}

func TestRunDisableNoDaemon(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("disable", "user/")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "failed to disable") {
		t.Fatalf("expected failed-to-disable error, got %q", errOut)
	}
}

func TestRunAddBadArgs(t *testing.T) {
	withTempDir(t)
	// missing positional
	code, _, errOut := runArgs("add")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "Usage: key-rest add") {
		t.Fatalf("expected usage, got %q", errOut)
	}
}

func TestRunAddAllowOnlyHeaderRequiresValue(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("add", "--allow-only-header")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "--allow-only-header requires") {
		t.Fatalf("expected flag-error, got %q", errOut)
	}
}

func TestRunAddAllowOnlyQueryRequiresValue(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("add", "--allow-only-query")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "--allow-only-query requires") {
		t.Fatalf("expected flag-error, got %q", errOut)
	}
}

func TestRunAddAllowOnlyFieldRequiresValue(t *testing.T) {
	withTempDir(t)
	code, _, errOut := runArgs("add", "--allow-only-field")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "--allow-only-field requires") {
		t.Fatalf("expected flag-error, got %q", errOut)
	}
}

func TestRunAddSuccessNoFlags(t *testing.T) {
	withTempDir(t)
	withFakePassphrase(t, "passphrase123", "secret-value")
	code, out, errOut := runArgs("add", "user/svc/key", "https://api.example.com/")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(out, "key added: user/svc/key") {
		t.Fatalf("expected key-added message, got %q", out)
	}
}

func TestRunAddSuccessWithAllowFlags(t *testing.T) {
	withTempDir(t)
	withFakePassphrase(t, "pass", "value")
	code, out, errOut := runArgs("add",
		"--allow-only-header", "Authorization",
		"--allow-only-query", "api_key",
		"--allow-only-field", "key",
		"--allow-only-url",
		"--allow-only-body",
		"u/s/k", "https://api.example.com/",
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(out, "key added: u/s/k") {
		t.Fatalf("expected key-added, got %q", out)
	}
}

func TestRunListAfterAdd(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	if code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/"); code != 0 {
		t.Fatalf("add failed: %d %s", code, errOut)
	}
	code, out, _ := runArgs("list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "key-rest://u/s/k") {
		t.Fatalf("expected list to include u/s/k, got %q", out)
	}
	// keystore file should exist
	if _, err := os.Stat(filepath.Join(dir, "keys.enc")); err != nil {
		t.Fatalf("expected keys.enc to exist: %v", err)
	}
}

func TestRunRemoveAfterAdd(t *testing.T) {
	withTempDir(t)
	withFakePassphrase(t, "pp", "vv")
	if code, _, errOut := runArgs("add", "u/s/k", "https://api.example.com/"); code != 0 {
		t.Fatalf("add failed: %d %s", code, errOut)
	}
	code, out, _ := runArgs("remove", "u/s/k")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "key removed: u/s/k") {
		t.Fatalf("expected key-removed, got %q", out)
	}
}

// withFakeSpawn replaces spawnDaemonFn for the duration of the test so we
// can exercise the fork branch of cmdStart without launching a real
// subprocess.
func withFakeSpawn(t *testing.T, pid int, retErr error) *[]byte {
	t.Helper()
	orig := spawnDaemonFn
	captured := []byte{}
	spawnDaemonFn = func(_, _ io.Writer, passphrase []byte) (int, error) {
		captured = append(captured, passphrase...)
		return pid, retErr
	}
	t.Cleanup(func() { spawnDaemonFn = orig })
	return &captured
}

func TestRunStartForkSuccess(t *testing.T) {
	withTempDir(t)
	withFakePassphrase(t, "secret-pp")
	captured := withFakeSpawn(t, 12345, nil)

	code, out, errOut := runArgs("start")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(out, "daemon starting in background (PID 12345)") {
		t.Fatalf("expected pid-12345 message, got %q", out)
	}
	if string(*captured) != "secret-pp" {
		t.Fatalf("expected passphrase to reach spawn, got %q", string(*captured))
	}
}

func TestRunStartForkFails(t *testing.T) {
	withTempDir(t)
	withFakePassphrase(t, "pp")
	withFakeSpawn(t, 0, errSpawnFailed{})

	code, _, errOut := runArgs("start")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "spawn-failed") {
		t.Fatalf("expected spawn-failed in stderr, got %q", errOut)
	}
}

type errSpawnFailed struct{}

func (errSpawnFailed) Error() string { return "spawn-failed" }

// TestRunStartForegroundDecryptFails covers the KEY_REST_FOREGROUND branch:
// when re-entering with FOREGROUND=1, cmdStart calls daemon.Start which calls
// store.DecryptAll. With a corrupted keystore, DecryptAll fails and cmdStart
// returns 1 after writing the daemon error to stderr.
func TestRunStartForegroundDecryptFails(t *testing.T) {
	dir := withTempDir(t)
	withFakePassphrase(t, "pp")
	t.Setenv("KEY_REST_FOREGROUND", "1")
	// Corrupted keys.enc → DecryptAll fails inside daemon.Start.
	if err := os.WriteFile(filepath.Join(dir, "keys.enc"), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}

	code, _, errOut := runArgs("start")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if errOut == "" {
		t.Fatal("expected stderr output describing daemon start failure")
	}
}

func TestRunStartAlreadyRunning(t *testing.T) {
	dir := withTempDir(t)
	// Plant a pid file pointing at our own pid so IsRunning returns true.
	pidPath := filepath.Join(dir, "key-rest.pid")
	mypid := []byte{}
	for _, c := range []byte(itoaPid()) {
		mypid = append(mypid, c)
	}
	if err := os.WriteFile(pidPath, mypid, 0600); err != nil {
		t.Fatal(err)
	}
	withFakePassphrase(t /* no responses needed: should bail before reading */)
	code, _, errOut := runArgs("start")
	if code != 1 {
		t.Fatalf("expected exit 1 (already running), got %d", code)
	}
	if !strings.Contains(errOut, "already running") {
		t.Fatalf("expected already-running message, got %q", errOut)
	}
}

func itoaPid() string {
	pid := os.Getpid()
	if pid < 0 {
		pid = -pid
	}
	if pid == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for pid > 0 {
		i--
		buf[i] = byte('0' + pid%10)
		pid /= 10
	}
	return string(buf[i:])
}

func TestFormatPlacementLegacyOnlyURLBody(t *testing.T) {
	got := formatPlacement(nil, true, true)
	if !strings.Contains(got, " [url]") || !strings.Contains(got, " [body]") {
		t.Fatalf("expected url+body, got %q", got)
	}
}

func TestRunStatusInvalidDir(t *testing.T) {
	// Use a non-writable parent path so DefaultDir() fails.
	t.Setenv("KEY_REST_DIR", "/proc/1/cannot-create-here")
	var stdout, stderr bytes.Buffer
	code := run([]string{"key-rest", "status"}, strings.NewReader(""), &stdout, &stderr)
	// keystore.New tries to mkdir; if it fails we get exit 1 and an error on stderr.
	// On systems where /proc/1 is writable (unlikely), the call could succeed —
	// in that case just verify there was no panic.
	if code == 1 && stderr.Len() == 0 {
		t.Fatalf("exit 1 should produce stderr message")
	}
}

// Ensure store-level operations don't break the run() entrypoint when
// keystore.New succeeds against a real tempdir.
func TestRunWithRealKeystore(t *testing.T) {
	withTempDir(t)
	store, err := keystore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if entries, err := store.List(); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("fresh store should be empty, got %d", len(entries))
	}
}
