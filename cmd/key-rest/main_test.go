package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "key-rest")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(projectRoot(t), "cmd", "key-rest")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
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
	bin := buildBinary(t)

	out, err := exec.Command(bin, "version").CombinedOutput()
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
	bin := buildBinary(t)

	// Use a temporary directory with no daemon running
	tmpDir := t.TempDir()
	cmd := exec.Command(bin, "status")
	cmd.Env = append(cmd.Environ(), "KEY_REST_DIR="+tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status command failed: %v\n%s", err, out)
	}

	output := strings.TrimSpace(string(out))
	if output != "stopped" {
		t.Fatalf("expected 'stopped', got %q", output)
	}
}
