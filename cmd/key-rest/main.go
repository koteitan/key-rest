package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/koteitan/key-rest/internal/crypto"
	"github.com/koteitan/key-rest/internal/daemon"
	"github.com/koteitan/key-rest/internal/keystore"
)

const version = "0.4.3"

// readPassphraseFn is the function used to read a passphrase. Tests may
// override this to inject deterministic passphrases without needing a TTY.
var readPassphraseFn = readPassphrase

// spawnDaemonFn forks a background daemon process by re-executing this binary
// with KEY_REST_FOREGROUND=1 and writing the passphrase to its stdin. Tests
// may override this to avoid spawning a real subprocess.
var spawnDaemonFn = spawnDaemon

// exeResolveFn returns the executable path to re-exec for the daemon. Tests
// may override this so spawnDaemon launches a harmless short-lived program
// rather than re-running the test binary.
var exeResolveFn = os.Executable

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 1
	}

	dir, err := keystore.DefaultDir()
	if err != nil {
		fmt.Fprintf(stderr, "failed to get data directory: %v\n", err)
		return 1
	}

	store, err := keystore.New(dir)
	if err != nil {
		fmt.Fprintf(stderr, "failed to initialize keystore: %v\n", err)
		return 1
	}

	switch args[1] {
	case "version":
		fmt.Fprintln(stdout, "key-rest "+version)
		return 0
	case "start":
		return cmdStart(dir, store, stdin, stdout, stderr)
	case "stop":
		return cmdStop(dir, store, stderr)
	case "status":
		return cmdStatus(dir, store, stdout, stderr)
	case "add":
		return cmdAdd(args, store, dir, stdin, stdout, stderr)
	case "remove":
		return cmdRemove(args, store, dir, stdout, stderr)
	case "enable":
		return cmdEnable(args, dir, stdout, stderr)
	case "disable":
		return cmdDisable(args, dir, stdout, stderr)
	case "list":
		return cmdList(store, dir, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "key-rest %s\n\n", version)
	fmt.Fprintf(w, `Usage: key-rest <command> [arguments]

Commands:
  version                        Show version
  start                          Start the daemon
  stop                           Stop the daemon
  status                         Check daemon status
  add [options] <key-uri> <url-prefix>  Add a key
  remove <key-uri>               Remove a key
  list                           List all keys
  enable <key-uri-prefix>        Enable keys matching the prefix
  disable <key-uri-prefix>       Disable keys matching the prefix

Add options:
  --allow-only-header <header-name>  Allow replacement only in the specified header
  --allow-only-query <query-name>   Allow replacement only in the specified query parameter
  --allow-only-field <field-name>   Allow replacement only in the specified JSON body field
  --allow-only-url            Allow replacement anywhere in the URL
  --allow-only-body           Allow replacement anywhere in the request body

Multiple flags can be specified; replacement is allowed in any of them (OR).
If no flags are specified, replacement is allowed everywhere.
`)
}

func cmdStart(dir string, store *keystore.Store, stdin io.Reader, stdout, stderr io.Writer) int {
	d := daemon.New(dir, store)
	d.Version = version
	if running, pid := d.IsRunning(); running {
		fmt.Fprintf(stderr, "daemon is already running (PID %d)\n", pid)
		return 1
	}

	passphrase := readPassphraseFn(stdin, stderr, "Enter passphrase: ")
	crypto.Mlock(passphrase)
	defer crypto.ZeroClearAndMunlock(passphrase)

	// Fork to background
	if os.Getenv("KEY_REST_FOREGROUND") == "1" {
		// Running in foreground mode (used after fork)
		if err := d.Start(passphrase); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		return 0
	}

	pid, err := spawnDaemonFn(stdout, stderr, passphrase)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "daemon starting in background (PID %d)\n", pid)
	return 0
}

// spawnDaemon launches a background daemon by re-executing the current binary
// with KEY_REST_FOREGROUND=1, then pipes the passphrase to its stdin and
// returns the child PID.
func spawnDaemon(stdout, stderr io.Writer, passphrase []byte) (int, error) {
	exe, err := exeResolveFn()
	if err != nil {
		return 0, fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(exe, "start")
	cmd.Env = append(os.Environ(), "KEY_REST_FOREGROUND=1")
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start daemon: %w", err)
	}

	stdinPipe.Write(passphrase)
	stdinPipe.Write([]byte("\n"))
	stdinPipe.Close()

	return cmd.Process.Pid, nil
}

func cmdStop(dir string, store *keystore.Store, stderr io.Writer) int {
	d := daemon.New(dir, store)
	if err := d.Stop(); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func cmdStatus(dir string, store *keystore.Store, stdout, stderr io.Writer) int {
	d := daemon.New(dir, store)
	running, pid := d.IsRunning()
	if running {
		fmt.Fprintf(stdout, "running (PID %d)\n", pid)
		checkDaemonVersion(dir, stderr)
	} else {
		fmt.Fprintln(stdout, "stopped")
	}
	return 0
}

func cmdAdd(allArgs []string, store *keystore.Store, dir string, stdin io.Reader, stdout, stderr io.Writer) int {
	args := allArgs[2:]
	var allowOnlyHeaders []string
	var allowOnlyQueries []string
	var allowOnlyFields []string
	allowOnlyURL := false
	allowOnlyBody := false
	hasAllowOnly := false
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--allow-only-url":
			allowOnlyURL = true
			hasAllowOnly = true
		case "--allow-only-body":
			allowOnlyBody = true
			hasAllowOnly = true
		case "--allow-only-header":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "--allow-only-header requires a header name\n")
				return 1
			}
			allowOnlyHeaders = append(allowOnlyHeaders, args[i])
			hasAllowOnly = true
		case "--allow-only-query":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "--allow-only-query requires a parameter name\n")
				return 1
			}
			allowOnlyQueries = append(allowOnlyQueries, args[i])
			hasAllowOnly = true
		case "--allow-only-field":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "--allow-only-field requires a field name\n")
				return 1
			}
			allowOnlyFields = append(allowOnlyFields, args[i])
			hasAllowOnly = true
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 2 {
		fmt.Fprintf(stderr, "Usage: key-rest add [options] <key-uri> <url-prefix>\n")
		return 1
	}

	keyURI := positional[0]
	urlPrefix := positional[1]

	var allowOnly *keystore.Placement
	if hasAllowOnly {
		allowOnly = &keystore.Placement{
			Headers: allowOnlyHeaders,
			Queries: allowOnlyQueries,
			Fields:  allowOnlyFields,
			URL:     allowOnlyURL,
			Body:    allowOnlyBody,
		}
	}

	// Check if daemon is running; if not, need passphrase
	d := daemon.New(dir, store)
	running, _ := d.IsRunning()

	passphrase := readPassphraseFn(stdin, stderr, "Enter passphrase: ")
	crypto.Mlock(passphrase)
	defer crypto.ZeroClearAndMunlock(passphrase)

	value := readPassphraseFn(stdin, stderr, "Enter the key value: ")
	crypto.Mlock(value)
	defer crypto.ZeroClearAndMunlock(value)

	if err := store.Add(keyURI, urlPrefix, false, false, allowOnly, value, passphrase); err != nil {
		fmt.Fprintf(stderr, "failed to add key: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "key added: %s\n", keyURI)

	if running {
		if err := sendReload(dir); err != nil {
			fmt.Fprintf(stderr, "warning: failed to notify daemon: %v (restart daemon to apply)\n", err)
		}
	}
	return 0
}

func cmdRemove(allArgs []string, store *keystore.Store, dir string, stdout, stderr io.Writer) int {
	if len(allArgs) < 3 {
		fmt.Fprintf(stderr, "Usage: key-rest remove <key-uri>\n")
		return 1
	}

	keyURI := allArgs[2]
	if err := store.Remove(keyURI); err != nil {
		fmt.Fprintf(stderr, "failed to remove key: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "key removed: %s\n", keyURI)

	d := daemon.New(dir, store)
	if running, _ := d.IsRunning(); running {
		if err := sendReload(dir); err != nil {
			fmt.Fprintf(stderr, "warning: failed to notify daemon: %v (restart daemon to apply)\n", err)
		}
	}
	return 0
}

func cmdList(store *keystore.Store, dir string, stdout, stderr io.Writer) int {
	// If daemon is running, query it for runtime status (includes enabled/disabled)
	d := daemon.New(dir, store)
	if running, _ := d.IsRunning(); running {
		checkDaemonVersion(dir, stderr)
		statuses, err := sendList(dir)
		if err != nil {
			fmt.Fprintf(stderr, "failed to query daemon: %v\n", err)
			return 1
		}
		if len(statuses) == 0 {
			fmt.Fprintln(stdout, "no keys registered")
			return 0
		}
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].URI < statuses[j].URI
		})
		for _, s := range statuses {
			status := "enabled"
			if s.Disabled {
				status = "disabled"
			}
			fmt.Fprintf(stdout, "key-rest://%s: %s %s%s\n", s.URI, s.URLPrefix, status, formatPlacement(s.AllowOnly, s.AllowURL, s.AllowBody))
		}
		return 0
	}

	// Daemon not running: read from file
	entries, err := store.List()
	if err != nil {
		fmt.Fprintf(stderr, "failed to list keys: %v\n", err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "no keys registered")
		return 0
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].URI < entries[j].URI
	})
	for _, e := range entries {
		fmt.Fprintf(stdout, "key-rest://%s: %s%s\n", e.URI, e.URLPrefix, formatPlacement(e.AllowOnly, e.AllowURL, e.AllowBody))
	}
	return 0
}

func formatPlacement(allowOnly *keystore.Placement, allowURL, allowBody bool) string {
	flags := ""
	if allowOnly != nil {
		if allowOnly.URL {
			flags += " [url]"
		}
		if allowOnly.Body {
			flags += " [body]"
		}
		for _, h := range allowOnly.Headers {
			flags += fmt.Sprintf(" [header:%s]", h)
		}
		for _, q := range allowOnly.Queries {
			flags += fmt.Sprintf(" [query:%s]", q)
		}
		for _, f := range allowOnly.Fields {
			flags += fmt.Sprintf(" [field:%s]", f)
		}
	} else {
		if allowURL {
			flags += " [url]"
		}
		if allowBody {
			flags += " [body]"
		}
	}
	return flags
}

func cmdEnable(allArgs []string, dir string, stdout, stderr io.Writer) int {
	if len(allArgs) < 3 {
		fmt.Fprintf(stderr, "Usage: key-rest enable <key-uri-prefix>\n")
		return 1
	}
	checkDaemonVersion(dir, stderr)
	uriPrefix := allArgs[2]
	count, err := sendEnableDisable(dir, "enable", uriPrefix)
	if err != nil {
		fmt.Fprintf(stderr, "failed to enable: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%d key(s) enabled\n", count)
	return 0
}

func cmdDisable(allArgs []string, dir string, stdout, stderr io.Writer) int {
	if len(allArgs) < 3 {
		fmt.Fprintf(stderr, "Usage: key-rest disable <key-uri-prefix>\n")
		return 1
	}
	checkDaemonVersion(dir, stderr)
	uriPrefix := allArgs[2]
	count, err := sendEnableDisable(dir, "disable", uriPrefix)
	if err != nil {
		fmt.Fprintf(stderr, "failed to disable: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%d key(s) disabled\n", count)
	return 0
}

func checkDaemonVersion(dir string, stderr io.Writer) {
	socketPath := filepath.Join(dir, "key-rest.sock")
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	req := map[string]string{"type": "version"}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	conn.Write(data)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	var resp struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return
	}
	if resp.Body != "" && resp.Body != version {
		fmt.Fprintf(stderr, "warning: daemon version %s does not match CLI version %s (restart daemon to update)\n", resp.Body, version)
	}
}

func sendEnableDisable(dir, action, uriPrefix string) (int, error) {
	socketPath := filepath.Join(dir, "key-rest.sock")
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return 0, fmt.Errorf("daemon not running or socket unavailable: %w", err)
	}
	defer conn.Close()

	req := map[string]string{"type": action, "uri_prefix": uriPrefix}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	conn.Write(data)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return 0, fmt.Errorf("no response from daemon")
	}

	var resp struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return 0, err
	}
	if resp.Error != nil {
		return 0, fmt.Errorf("[%s] %s", resp.Error.Code, resp.Error.Message)
	}

	count, _ := strconv.Atoi(resp.Body)
	return count, nil
}

func sendList(dir string) ([]keystore.KeyStatus, error) {
	socketPath := filepath.Join(dir, "key-rest.sock")
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := map[string]string{"type": "list"}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	conn.Write(data)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from daemon")
	}

	var resp struct {
		Body  string `json:"body"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("[%s] %s", resp.Error.Code, resp.Error.Message)
	}

	var statuses []keystore.KeyStatus
	if err := json.Unmarshal([]byte(resp.Body), &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
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

// readPassphrase reads a passphrase from stdin. If stdin is os.Stdin and a
// terminal, echo is disabled and the prompt is written to stderr. Otherwise
// (piped input, test injection) one line is read from stdin.
func readPassphrase(stdin io.Reader, stderr io.Writer, prompt string) []byte {
	// If caller passed os.Stdin and it's a terminal, use raw mode.
	if f, ok := stdin.(*os.File); ok && f != nil {
		fd := int(f.Fd())
		if term.IsTerminal(fd) {
			fmt.Fprint(stderr, prompt)
			pass := readPasswordMlocked(fd, stderr)
			fmt.Fprintln(stderr)
			return pass
		}
		// Non-terminal *os.File: read one line via syscall (avoids buffered reader
		// gobbling pipe data the child process may need).
		buf := make([]byte, 4096)
		crypto.Mlock(buf)
		n := 0
		var oneByte [1]byte
		for n < len(buf) {
			nr, err := syscall.Read(fd, oneByte[:])
			if nr == 1 {
				if oneByte[0] == '\n' {
					break
				}
				buf[n] = oneByte[0]
				n++
			}
			if err != nil {
				break
			}
		}
		result := make([]byte, n)
		copy(result, buf[:n])
		crypto.ZeroClearAndMunlock(buf)
		crypto.Mlock(result)
		return result
	}

	// Generic io.Reader (test injection): read one line.
	br := bufio.NewReader(stdin)
	line, _ := br.ReadString('\n')
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	return []byte(line)
}

// readPasswordMlocked reads a password from a terminal with echo disabled.
// All buffers are mlocked from allocation. The returned slice is mlocked;
// the caller is responsible for ZeroClearAndMunlock.
func readPasswordMlocked(fd int, stderr io.Writer) []byte {
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(stderr, "failed to set terminal raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 4096)
	crypto.Mlock(buf)
	n := 0

	var oneByte [1]byte
	for {
		_, err := syscall.Read(fd, oneByte[:])
		if err != nil {
			crypto.ZeroClearAndMunlock(buf)
			fmt.Fprintf(stderr, "failed to read from terminal: %v\n", err)
			os.Exit(1)
		}

		switch oneByte[0] {
		case '\n', '\r':
			// Enter: done
			result := make([]byte, n)
			copy(result, buf[:n])
			crypto.ZeroClearAndMunlock(buf)
			crypto.Mlock(result)
			return result
		case 3:
			// Ctrl-C: abort
			crypto.ZeroClearAndMunlock(buf)
			term.Restore(fd, oldState)
			fmt.Fprintln(stderr)
			os.Exit(1)
			return nil
		case 127, 8:
			// Backspace / Delete
			if n > 0 {
				n--
				buf[n] = 0
			}
		default:
			if oneByte[0] >= 32 && n < len(buf) {
				buf[n] = oneByte[0]
				n++
			}
		}
	}
}
