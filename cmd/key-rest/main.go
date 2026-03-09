package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/koteitan/key-rest/internal/crypto"
	"github.com/koteitan/key-rest/internal/daemon"
	"github.com/koteitan/key-rest/internal/keystore"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dir, err := keystore.DefaultDir()
	if err != nil {
		fatalf("failed to get data directory: %v\n", err)
	}

	store, err := keystore.New(dir)
	if err != nil {
		fatalf("failed to initialize keystore: %v\n", err)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("key-rest " + version)
		return
	case "start":
		cmdStart(dir, store)
	case "stop":
		cmdStop(dir, store)
	case "status":
		cmdStatus(dir, store)
	case "add":
		cmdAdd(store, dir)
	case "remove":
		cmdRemove(store, dir)
	case "list":
		cmdList(store)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "key-rest %s\n\n", version)
	fmt.Fprintf(os.Stderr, `Usage: key-rest <command> [arguments]

Commands:
  version                        Show version
  start                          Start the daemon
  stop                           Stop the daemon
  status                         Check daemon status
  add [options] <key-uri> <url-prefix>  Add a key
  remove <key-uri>               Remove a key
  list                           List all keys

Add options:
  --allow-url    Allow replacement within URLs
  --allow-body   Allow replacement within request body
`)
}

func cmdStart(dir string, store *keystore.Store) {
	d := daemon.New(dir, store)
	if running, pid := d.IsRunning(); running {
		fatalf("daemon is already running (PID %d)\n", pid)
	}

	passphrase := readPassphrase("Enter passphrase: ")
	crypto.Mlock(passphrase)
	defer crypto.ZeroClearAndMunlock(passphrase)

	// Fork to background
	if os.Getenv("KEY_REST_FOREGROUND") == "1" {
		// Running in foreground mode (used after fork)
		if err := d.Start(passphrase); err != nil {
			fatalf("%v\n", err)
		}
		return
	}

	// Fork a background process
	exe, err := os.Executable()
	if err != nil {
		fatalf("failed to get executable path: %v\n", err)
	}

	cmd := exec.Command(exe, "start")
	cmd.Env = append(os.Environ(), "KEY_REST_FOREGROUND=1")
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	// Pass passphrase via pipe
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		fatalf("failed to create stdin pipe: %v\n", err)
	}

	if err := cmd.Start(); err != nil {
		fatalf("failed to start daemon: %v\n", err)
	}

	stdinPipe.Write(passphrase)
	stdinPipe.Write([]byte("\n"))
	stdinPipe.Close()

	fmt.Printf("daemon starting in background (PID %d)\n", cmd.Process.Pid)
}

func cmdStop(dir string, store *keystore.Store) {
	d := daemon.New(dir, store)
	if err := d.Stop(); err != nil {
		fatalf("%v\n", err)
	}
}

func cmdStatus(dir string, store *keystore.Store) {
	d := daemon.New(dir, store)
	running, pid := d.IsRunning()
	if running {
		fmt.Printf("running (PID %d)\n", pid)
	} else {
		fmt.Println("stopped")
	}
}

func cmdAdd(store *keystore.Store, dir string) {
	args := os.Args[2:]
	allowURL := false
	allowBody := false
	var positional []string

	for _, arg := range args {
		switch arg {
		case "--allow-url":
			allowURL = true
		case "--allow-body":
			allowBody = true
		default:
			positional = append(positional, arg)
		}
	}

	if len(positional) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: key-rest add [--allow-url] [--allow-body] <key-uri> <url-prefix>\n")
		os.Exit(1)
	}

	keyURI := positional[0]
	urlPrefix := positional[1]

	// Check if daemon is running; if not, need passphrase
	d := daemon.New(dir, store)
	running, _ := d.IsRunning()

	passphrase := readPassphrase("Enter passphrase: ")
	crypto.Mlock(passphrase)
	defer crypto.ZeroClearAndMunlock(passphrase)

	value := readPassphrase("Enter the key value: ")
	crypto.Mlock(value)
	defer crypto.ZeroClearAndMunlock(value)

	if err := store.Add(keyURI, urlPrefix, allowURL, allowBody, value, passphrase); err != nil {
		fatalf("failed to add key: %v\n", err)
	}

	fmt.Printf("key added: %s\n", keyURI)

	if running {
		if err := sendReload(dir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to notify daemon: %v (restart daemon to apply)\n", err)
		}
	}
}

func cmdRemove(store *keystore.Store, dir string) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: key-rest remove <key-uri>\n")
		os.Exit(1)
	}

	keyURI := os.Args[2]
	if err := store.Remove(keyURI); err != nil {
		fatalf("failed to remove key: %v\n", err)
	}

	fmt.Printf("key removed: %s\n", keyURI)

	d := daemon.New(dir, store)
	if running, _ := d.IsRunning(); running {
		if err := sendReload(dir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to notify daemon: %v (restart daemon to apply)\n", err)
		}
	}
}

func cmdList(store *keystore.Store) {
	entries, err := store.List()
	if err != nil {
		fatalf("failed to list keys: %v\n", err)
	}

	if len(entries) == 0 {
		fmt.Println("no keys registered")
		return
	}

	for _, e := range entries {
		flags := ""
		if e.AllowURL {
			flags += " [url]"
		}
		if e.AllowBody {
			flags += " [body]"
		}
		fmt.Printf("%s: %s%s\n", e.URI, e.URLPrefix, flags)
	}
}

func sendReload(dir string) error {
	socketPath := filepath.Join(dir, "key-rest.sock")
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := map[string]string{"type": "reload"}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	conn.Write(data)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return fmt.Errorf("no response from daemon")
	}

	var resp struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("[%s] %s", resp.Error.Code, resp.Error.Message)
	}
	return nil
}

func readPassphrase(prompt string) []byte {
	// Check if stdin is a terminal
	if os.Getenv("KEY_REST_FOREGROUND") == "1" {
		// In foreground mode (forked process), read from stdin pipe
		buf := make([]byte, 4096)
		crypto.Mlock(buf)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			fatalf("failed to read from stdin: %v\n", err)
		}
		// Trim trailing newline
		data := buf[:n]
		if len(data) > 0 && data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}
		result := make([]byte, len(data))
		copy(result, data)
		crypto.ZeroClearAndMunlock(buf)
		return result
	}

	fmt.Fprint(os.Stderr, prompt)
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fatalf("failed to read passphrase: %v\n", err)
	}
	return pass
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
