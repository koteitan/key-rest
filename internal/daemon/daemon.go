// Package daemon provides process management for key-rest-daemon.
package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/koteitan/key-rest/internal/crypto"
	"github.com/koteitan/key-rest/internal/keystore"
	"github.com/koteitan/key-rest/internal/proxy"
	"github.com/koteitan/key-rest/internal/server"
)

// Daemon manages the key-rest-daemon lifecycle.
type Daemon struct {
	dir        string
	store      *keystore.Store
	server     *server.Server
	passphrase []byte // held in memory for reload
}

// New creates a new Daemon.
func New(dir string, store *keystore.Store) *Daemon {
	return &Daemon{
		dir:   dir,
		store: store,
	}
}

func (d *Daemon) pidPath() string {
	return filepath.Join(d.dir, "key-rest.pid")
}

func (d *Daemon) socketPath() string {
	return filepath.Join(d.dir, "key-rest.sock")
}

// IsRunning checks if the daemon process is running.
func (d *Daemon) IsRunning() (bool, int) {
	data, err := os.ReadFile(d.pidPath())
	if err != nil {
		return false, 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}

	// Check if process exists
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// Signal 0 checks if process exists without actually sending a signal
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process doesn't exist; clean up stale PID file
		os.Remove(d.pidPath())
		os.Remove(d.socketPath())
		return false, 0
	}

	return true, pid
}

// Start starts the daemon in the foreground (blocking).
// The daemon decrypts all keys and listens on the Unix socket.
func (d *Daemon) Start(passphrase []byte) error {
	if running, pid := d.IsRunning(); running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	// Disable core dumps to prevent credential leakage
	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0}); err != nil {
		return fmt.Errorf("failed to disable core dumps: %w", err)
	}

	// Prevent /proc/PID/mem, /proc/PID/maps, /proc/PID/environ access by non-root
	if _, _, errno := syscall.Syscall(syscall.SYS_PRCTL, 4 /* PR_SET_DUMPABLE */, 0, 0); errno != 0 {
		return fmt.Errorf("failed to set PR_SET_DUMPABLE: %w", errno)
	}

	// Decrypt all keys
	if err := d.store.DecryptAll(passphrase); err != nil {
		return fmt.Errorf("failed to decrypt keys: %w", err)
	}

	// Hold passphrase for reload
	d.passphrase = make([]byte, len(passphrase))
	copy(d.passphrase, passphrase)
	crypto.Mlock(d.passphrase)

	// Write PID file
	pid := os.Getpid()
	if err := os.WriteFile(d.pidPath(), []byte(strconv.Itoa(pid)), 0600); err != nil {
		d.store.ClearAll()
		crypto.ZeroClearAndMunlock(d.passphrase)
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Start socket server
	p := proxy.New(d.store)
	d.server = server.New(d.socketPath(), p)
	d.server.ReloadHandler = d.reload
	if err := d.server.Start(); err != nil {
		os.Remove(d.pidPath())
		d.store.ClearAll()
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Printf("daemon started (PID %d)\n", pid)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	d.shutdown()
	return nil
}

// Stop sends SIGTERM to the running daemon process.
func (d *Daemon) Stop() error {
	running, pid := d.IsRunning()
	if !running {
		return fmt.Errorf("daemon is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", pid, err)
	}

	fmt.Printf("sent stop signal to daemon (PID %d)\n", pid)
	return nil
}

func (d *Daemon) reload() error {
	return d.store.DecryptAll(d.passphrase)
}

func (d *Daemon) shutdown() {
	fmt.Println("shutting down...")
	if d.server != nil {
		d.server.Stop()
	}
	d.store.ClearAll()
	crypto.ZeroClearAndMunlock(d.passphrase)
	os.Remove(d.pidPath())
	fmt.Println("daemon stopped")
}
