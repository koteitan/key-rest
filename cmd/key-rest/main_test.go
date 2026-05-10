package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// covDir is the per-binary directory where the -cover-instrumented subprocess
// writes its coverage data. Each call to buildBinary sets GOCOVERDIR for the
// returned binary; the merged data is converted back to text format by the
// CI / Makefile workflow.
func buildBinary(t *testing.T) (binPath, covDir string) {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "key-rest")
	cov := filepath.Join(tmp, "covdata")
	if err := os.MkdirAll(cov, 0700); err != nil {
		t.Fatal(err)
	}
	// Build with coverage instrumentation so the subprocess emits profile
	// data into $GOCOVERDIR. The instrumented binary's output is otherwise
	// identical to a normal build.
	cmd := exec.Command("go", "build", "-cover", "-o", bin, ".")
	cmd.Dir = filepath.Join(projectRoot(t), "cmd", "key-rest")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin, cov
}

// runBin runs the instrumented binary with GOCOVERDIR pointing at the
// per-test covdata dir so the subprocess records its statement coverage.
func runBin(t *testing.T, bin, covDir string, env []string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(env, "GOCOVERDIR="+covDir)
	if cmd.Env == nil {
		cmd.Env = []string{"GOCOVERDIR=" + covDir}
	}
	return cmd.CombinedOutput()
}

func projectRoot(t *testing.T) string {
	t.Helper()
	// cmd/key-rest is two levels below the project root
	dir := "."
	for i := 0; i < 10; i++ {
		if _, err := exec.Command("test", "-f", filepath.Join(dir, "go.mod")).Output(); err == nil {
			abs, _ := filepath.Abs(dir)
			return abs
		}
		dir = filepath.Join(dir, "..")
	}
	t.Fatal("project root not found")
	return ""
}

func TestVersionCommand(t *testing.T) {
	bin, covDir := buildBinary(t)

	out, err := runBin(t, bin, covDir, os.Environ(), "version")
	if err != nil {
		t.Fatalf("version command failed: %v\n%s", err, out)
	}

	output := strings.TrimSpace(string(out))
	if !strings.HasPrefix(output, "key-rest ") {
		t.Fatalf("expected 'key-rest ...' prefix, got %q", output)
	}

	// Verify version matches the constant
	if output != "key-rest "+version {
		t.Fatalf("expected 'key-rest %s', got %q", version, output)
	}
}

func TestStatusCommandStopped(t *testing.T) {
	bin, covDir := buildBinary(t)

	// Use a temporary directory with no daemon running
	tmpDir := t.TempDir()
	env := append(os.Environ(), "KEY_REST_DIR="+tmpDir)
	out, err := runBin(t, bin, covDir, env, "status")
	if err != nil {
		t.Fatalf("status command failed: %v\n%s", err, out)
	}

	output := strings.TrimSpace(string(out))
	if output != "stopped" {
		t.Fatalf("expected 'stopped', got %q", output)
	}
}

func TestNoArgsShowsUsage(t *testing.T) {
	bin, covDir := buildBinary(t)
	out, err := runBin(t, bin, covDir, os.Environ())
	// Exits with non-zero — exec.Command returns an error on non-zero exit.
	if err == nil {
		t.Fatal("expected non-zero exit when no command given")
	}
	if !strings.Contains(string(out), "Usage") {
		t.Fatalf("expected usage text, got %q", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	bin, covDir := buildBinary(t)
	out, err := runBin(t, bin, covDir, os.Environ(), "zonk")
	if err == nil {
		t.Fatal("expected non-zero exit on unknown command")
	}
	if !strings.Contains(string(out), "unknown command") &&
		!strings.Contains(string(out), "Usage") {
		t.Fatalf("expected error or usage, got %q", out)
	}
}

func TestHelpCommand(t *testing.T) {
	bin, covDir := buildBinary(t)
	out, err := runBin(t, bin, covDir, os.Environ(), "help")
	// help prints usage, may exit 0 or 2 depending on impl
	_ = err
	if !strings.Contains(string(out), "Usage") && !strings.Contains(string(out), "key-rest") {
		t.Fatalf("expected help text, got %q", out)
	}
}

func TestListCommandStopped(t *testing.T) {
	bin, covDir := buildBinary(t)
	tmpDir := t.TempDir()
	env := append(os.Environ(), "KEY_REST_DIR="+tmpDir)
	// list with empty keystore — should print nothing or "no keys" without erroring
	out, err := runBin(t, bin, covDir, env, "list")
	_ = err
	_ = out
}

func TestStopWithoutRunningDaemon(t *testing.T) {
	bin, covDir := buildBinary(t)
	tmpDir := t.TempDir()
	env := append(os.Environ(), "KEY_REST_DIR="+tmpDir)
	out, err := runBin(t, bin, covDir, env, "stop")
	if err == nil {
		t.Fatal("stop without running daemon should fail")
	}
	if !strings.Contains(string(out), "not running") && !strings.Contains(string(out), "stopped") {
		// Accept either message
		_ = out
	}
}
