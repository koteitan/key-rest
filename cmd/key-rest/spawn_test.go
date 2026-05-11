package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"syscall"
	"testing"
	"time"
)

// withFakeExe replaces exeResolveFn for the duration of the test so
// spawnDaemon uses a controllable binary path / error.
func withFakeExe(t *testing.T, path string, retErr error) {
	t.Helper()
	orig := exeResolveFn
	exeResolveFn = func() (string, error) { return path, retErr }
	t.Cleanup(func() { exeResolveFn = orig })
}

// reapPid blocks until the given pid has been reaped, or times out. We
// must reap children of the test process or they become zombies.
func reapPid(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var ws syscall.WaitStatus
		got, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
		if err != nil {
			return // ECHILD or already reaped
		}
		if got == pid {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Last-ditch: blocking wait
	var ws syscall.WaitStatus
	syscall.Wait4(pid, &ws, 0, nil)
}

func TestSpawnDaemonExecResolveFails(t *testing.T) {
	withFakeExe(t, "", errors.New("no exe"))

	pid, err := spawnDaemon(io.Discard, io.Discard, []byte("p"))
	if err == nil {
		t.Fatal("expected error from exe-resolve failure")
	}
	if pid != 0 {
		t.Fatalf("expected pid=0, got %d", pid)
	}
	if !strings.Contains(err.Error(), "failed to get executable path") {
		t.Fatalf("expected wrap message, got %v", err)
	}
}

func TestSpawnDaemonCmdStartFails(t *testing.T) {
	withFakeExe(t, "/no/such/binary-xyz", nil)

	pid, err := spawnDaemon(io.Discard, io.Discard, []byte("p"))
	if err == nil {
		t.Fatal("expected error when executable does not exist")
	}
	if pid != 0 {
		t.Fatalf("expected pid=0, got %d", pid)
	}
	if !strings.Contains(err.Error(), "failed to start daemon") {
		t.Fatalf("expected wrap message, got %v", err)
	}
}

func TestSpawnDaemonSuccess(t *testing.T) {
	// /bin/true exits immediately and ignores stdin — a safe stand-in for a
	// "child process that gets a passphrase but doesn't need one".
	withFakeExe(t, "/bin/true", nil)

	var stdout, stderr bytes.Buffer
	pid, err := spawnDaemon(&stdout, &stderr, []byte("ignored"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("expected positive pid, got %d", pid)
	}
	// Reap so we don't leave a zombie.
	reapPid(t, pid)
}
