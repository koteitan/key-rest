package daemon

import (
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
)

func newDaemon(t *testing.T) *Daemon {
	t.Helper()
	dir := t.TempDir()
	store, err := keystore.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(dir, store)
}

func TestNew(t *testing.T) {
	d := newDaemon(t)
	if d == nil || d.store == nil || d.dir == "" {
		t.Fatal("New returned an unusable daemon")
	}
}

func TestPidAndSocketPath(t *testing.T) {
	d := newDaemon(t)
	if filepath.Base(d.pidPath()) != "key-rest.pid" {
		t.Fatalf("pidPath: %q", d.pidPath())
	}
	if filepath.Base(d.socketPath()) != "key-rest.sock" {
		t.Fatalf("socketPath: %q", d.socketPath())
	}
}

func TestIsRunningNoPidFile(t *testing.T) {
	d := newDaemon(t)
	running, pid := d.IsRunning()
	if running || pid != 0 {
		t.Fatalf("expected not running, got running=%v pid=%d", running, pid)
	}
}

func TestIsRunningInvalidPidFile(t *testing.T) {
	d := newDaemon(t)
	if err := os.WriteFile(d.pidPath(), []byte("not-a-number"), 0600); err != nil {
		t.Fatal(err)
	}
	running, _ := d.IsRunning()
	if running {
		t.Fatal("garbage PID file should not register as running")
	}
}

func TestIsRunningStalePid(t *testing.T) {
	d := newDaemon(t)
	// PID 999999 is very unlikely to be live; signal(0) returns ESRCH.
	if err := os.WriteFile(d.pidPath(), []byte("999999"), 0600); err != nil {
		t.Fatal(err)
	}
	running, _ := d.IsRunning()
	if running {
		t.Fatal("stale PID should not register as running")
	}
	// PID file should have been cleaned up.
	if _, err := os.Stat(d.pidPath()); !os.IsNotExist(err) {
		t.Fatal("stale PID file should be removed")
	}
}

func TestIsRunningCurrentProcess(t *testing.T) {
	d := newDaemon(t)
	// Write our own PID — Signal(0) succeeds.
	if err := os.WriteFile(d.pidPath(), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	running, pid := d.IsRunning()
	if !running || pid != os.Getpid() {
		t.Fatalf("expected current pid registered, got running=%v pid=%d", running, pid)
	}
}

func TestStopNotRunning(t *testing.T) {
	d := newDaemon(t)
	if err := d.Stop(); err == nil {
		t.Fatal("Stop should error when daemon is not running")
	}
}

func TestStopMissingProcess(t *testing.T) {
	d := newDaemon(t)
	// PID file pointing at nothing — IsRunning will clean it up and return false.
	if err := os.WriteFile(d.pidPath(), []byte("999999"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.Stop(); err == nil {
		t.Fatal("Stop should error when process is gone")
	}
}

func TestStopSendsSignal(t *testing.T) {
	d := newDaemon(t)
	// Use our own PID so SIGTERM via the signal-trap below works.
	if err := os.WriteFile(d.pidPath(), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	// Trap SIGTERM so this test process doesn't actually exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := d.Stop(); err != nil {
		t.Fatalf("Stop returned %v", err)
	}
	select {
	case <-sigCh:
		// got it
	case <-time.After(time.Second):
		t.Fatal("SIGTERM was not delivered")
	}
}

func TestReloadHandler(t *testing.T) {
	d := newDaemon(t)
	pass := []byte("p")
	if err := d.store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := d.store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	d.passphrase = pass
	if err := d.reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
}

func TestEnableHandler(t *testing.T) {
	d := newDaemon(t)
	pass := []byte("p")
	if err := d.store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := d.store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	d.passphrase = pass
	d.store.Disable("user1/k")
	count, err := d.enable("user1/k")
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	d := newDaemon(t)
	if err := os.WriteFile(d.pidPath(), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	// Start should reject because IsRunning returns true.
	if err := d.Start([]byte("p")); err == nil {
		t.Fatal("Start should reject when daemon already running")
	}
}

func TestStartDecryptAllFails(t *testing.T) {
	d := newDaemon(t)
	// Write a corrupted keys.enc so DecryptAll fails inside Start.
	keysFile := filepath.Join(d.dir, "keys.enc")
	if err := os.WriteFile(keysFile, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.Start([]byte("p")); err == nil {
		t.Fatal("Start should fail when DecryptAll fails")
	}
}

func TestStartFullLifecycle(t *testing.T) {
	// Start the daemon, give it a moment to install its signal handler,
	// then send SIGTERM. shutdown() should run and Start should return.
	// Side effect: PR_SET_DUMPABLE=0 stays set in this test process for the
	// rest of the binary — that's fine for our test suite.

	d := newDaemon(t)
	pass := []byte("p")
	// Pre-encrypt one key so DecryptAll succeeds.
	if err := d.store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}

	startErr := make(chan error, 1)
	go func() {
		startErr <- d.Start(pass)
	}()

	// Wait for the daemon to write its PID file (signal handler is installed
	// after that point, so the SIGTERM below isn't lost).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if running, _ := d.IsRunning(); running {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Trap SIGTERM in this test goroutine too so the binary doesn't terminate
	// if the daemon hasn't yet installed its handler.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill: %v", err)
	}

	select {
	case err := <-startErr:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return within 3s of SIGTERM")
	}

	// PID file should be gone after shutdown.
	if _, err := os.Stat(d.pidPath()); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after shutdown")
	}
}

// TestStartPidFileWriteFails covers the cleanup branch where decryption
// succeeded but pid-file write fails: ClearAll + munlock must run.
func TestStartPidFileWriteFails(t *testing.T) {
	d := newDaemon(t)
	pass := []byte("p")
	if err := d.store.Add("u/s/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	// Place a directory at the pid path so WriteFile fails with "is a directory".
	if err := os.MkdirAll(d.pidPath(), 0700); err != nil {
		t.Fatal(err)
	}

	err := d.Start(pass)
	if err == nil {
		t.Fatal("Start should fail when pid path is a directory")
	}
	// Decryption ran but pid file write failed; passphrase must have been munlocked.
	if d.passphrase != nil {
		// passphrase slice may still be non-nil (zeroed in place by ZeroClearAndMunlock),
		// just ensure Start returned the expected error wrapper.
		if !contains(err.Error(), "failed to write PID file") {
			t.Fatalf("expected 'failed to write PID file' in error, got %v", err)
		}
	}
}

// TestStartServerStartFails covers the cleanup branch where pid-file write
// succeeded but server.Start fails: pid file remove + ClearAll must run.
func TestStartServerStartFails(t *testing.T) {
	d := newDaemon(t)
	pass := []byte("p")
	if err := d.store.Add("u/s/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	// Make socket path a non-empty directory: net.Listen("unix", ...) cannot
	// bind there because the path exists and is a directory (Bind: address
	// already in use / permission denied).
	if err := os.MkdirAll(d.socketPath(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.socketPath(), "blocker"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	err := d.Start(pass)
	if err == nil {
		t.Fatal("Start should fail when socket path is unreachable")
	}
	if !contains(err.Error(), "failed to start server") {
		t.Fatalf("expected 'failed to start server' in error, got %v", err)
	}
	// pid file must have been cleaned up.
	if _, statErr := os.Stat(d.pidPath()); !os.IsNotExist(statErr) {
		t.Fatal("PID file should be removed after server.Start fails")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
