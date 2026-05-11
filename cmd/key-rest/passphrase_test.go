//go:build linux

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// PTY ioctl request codes for Linux (asm-generic/ioctls.h).
const (
	_TIOCSPTLCK = 0x40045431
	_TIOCGPTN   = 0x80045430
)

// openPty returns a freshly-allocated master/slave PTY pair on Linux. The
// caller is responsible for closing both ends.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}
	var unlock int32 = 0
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(),
		_TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock)))
	if errno != 0 {
		m.Close()
		t.Fatalf("TIOCSPTLCK: %v", errno)
	}
	var n uint32
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, m.Fd(),
		_TIOCGPTN, uintptr(unsafe.Pointer(&n)))
	if errno != 0 {
		m.Close()
		t.Fatalf("TIOCGPTN: %v", errno)
	}
	s, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		t.Fatalf("open slave: %v", err)
	}
	return m, s
}

func TestReadPassphraseGenericReader(t *testing.T) {
	got := readPassphrase(strings.NewReader("hello\nignored"), io.Discard, "prompt: ")
	if string(got) != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestReadPassphraseGenericReaderNoNewline(t *testing.T) {
	got := readPassphrase(strings.NewReader("nofinalnewline"), io.Discard, "prompt: ")
	if string(got) != "nofinalnewline" {
		t.Fatalf("got %q", got)
	}
}

func TestReadPassphrasePipeReader(t *testing.T) {
	// A pipe read end is an *os.File but term.IsTerminal returns false.
	// This exercises the syscall.Read loop path of readPassphrase.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Write([]byte("piped-secret\n"))
		w.Close()
	}()

	got := readPassphrase(r, io.Discard, "prompt: ")
	wg.Wait()
	if string(got) != "piped-secret" {
		t.Fatalf("got %q, want %q", got, "piped-secret")
	}
}

func TestReadPassphraseTTYPath(t *testing.T) {
	// PTY slave is a real terminal so readPassphrase enters the term.MakeRaw
	// branch and dispatches to readPasswordMlocked.
	master, slave := openPty(t)
	defer master.Close()
	defer slave.Close()

	done := make(chan []byte, 1)
	go func() {
		var stderr bytes.Buffer
		done <- readPassphrase(slave, &stderr, "Enter passphrase: ")
		_ = stderr // we don't assert prompt content
	}()

	// Give readPasswordMlocked a moment to enter raw mode before we write.
	time.Sleep(50 * time.Millisecond)
	if _, err := master.Write([]byte("secret\r")); err != nil {
		t.Fatalf("master write: %v", err)
	}

	select {
	case got := <-done:
		if string(got) != "secret" {
			t.Fatalf("got %q, want %q", got, "secret")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readPassphrase did not return within 3s")
	}
}

func TestReadPasswordMlockedBackspace(t *testing.T) {
	// Exercises the backspace branch directly. The function exits via the
	// Enter case once "\r" arrives.
	master, slave := openPty(t)
	defer master.Close()
	defer slave.Close()

	done := make(chan []byte, 1)
	go func() {
		done <- readPasswordMlocked(int(slave.Fd()), io.Discard)
	}()

	time.Sleep(50 * time.Millisecond)
	// Type "abXc", erase 'c' and 'X' with two backspaces (0x7f then 0x08),
	// then send Enter. Expected result: "ab".
	if _, err := master.Write([]byte("abXc\x7f\x08\r")); err != nil {
		t.Fatalf("master write: %v", err)
	}

	select {
	case got := <-done:
		if string(got) != "ab" {
			t.Fatalf("got %q, want %q", got, "ab")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readPasswordMlocked did not return within 3s")
	}
}

func TestReadPasswordMlockedIgnoresNonPrintable(t *testing.T) {
	master, slave := openPty(t)
	defer master.Close()
	defer slave.Close()

	done := make(chan []byte, 1)
	go func() {
		done <- readPasswordMlocked(int(slave.Fd()), io.Discard)
	}()

	time.Sleep(50 * time.Millisecond)
	// Byte 1 (Ctrl-A) is non-printable (<32) and not Backspace/Enter/Ctrl-C.
	// It must be silently ignored — falls through the default case but fails
	// the >= 32 check. Then 'X' is added, then Enter.
	if _, err := master.Write([]byte{1, 'X', '\r'}); err != nil {
		t.Fatalf("master write: %v", err)
	}

	select {
	case got := <-done:
		if string(got) != "X" {
			t.Fatalf("got %q, want %q", got, "X")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readPasswordMlocked did not return within 3s")
	}
}
