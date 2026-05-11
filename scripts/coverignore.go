// coverignore filters a Go coverage profile by stripping any block whose
// source range covers a line marked with `// cover:ignore`. The marker is
// added by hand to lines that are structurally untestable (defensive
// checks, os.Exit branches, syscall failures, etc.). An explanation may
// follow on the same line, e.g.:
//
//	os.Exit(1) // cover:ignore — terminal Read fatal, can't test in-process
//
// Usage: go run scripts/coverignore.go <profile-path>
//
// Reads the profile from the given path; writes the filtered profile to
// stdout. Module-relative paths are resolved by reading go.mod in the
// current working directory.
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const marker = "// cover:ignore"

// lineHasMarker reports whether s contains the marker as a standalone token —
// i.e. followed by end-of-line or a non-identifier/non-dash character. This
// prevents accidental matches against future directives such as
// "// cover:ignore-block".
func lineHasMarker(s string) bool {
	for i := 0; i <= len(s)-len(marker); i++ {
		if s[i:i+len(marker)] != marker {
			continue
		}
		j := i + len(marker)
		if j == len(s) {
			return true
		}
		c := s[j]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return true
		}
		// reject identifier extension or dash (so cover:ignore-block, etc.,
		// are not picked up here).
		if c == '_' || c == '-' || (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		return true
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: coverignore <profile>")
		os.Exit(2)
	}

	module, err := readModule("go.mod")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ignored, err := collectMarkers(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer in.Close()

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	blockRe := regexp.MustCompile(`^(.+\.go):(\d+)\.\d+,(\d+)\.\d+ \d+ \d+$`)
	kept, removed := 0, 0
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, module+"/") {
			fmt.Fprintln(w, line)
			continue
		}
		rel := strings.TrimPrefix(line, module+"/")
		m := blockRe.FindStringSubmatch(rel)
		if m == nil {
			fmt.Fprintln(w, line)
			continue
		}
		file := m[1]
		startLine, _ := strconv.Atoi(m[2])
		endLine, _ := strconv.Atoi(m[3])

		if hitsIgnore(ignored[file], startLine, endLine) {
			removed++
			continue
		}
		kept++
		fmt.Fprintln(w, line)
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "coverignore: kept %d block(s), removed %d block(s)\n", kept, removed)
}

func readModule(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive missing in %s", path)
}

// collectMarkers walks the source tree starting at root and returns a map
// file -> set of line numbers carrying the ignore marker.
func collectMarkers(root string) (map[string]map[int]bool, error) {
	out := map[string]map[int]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		rel := strings.TrimPrefix(path, "./")
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for ln := 1; sc.Scan(); ln++ {
			if lineHasMarker(sc.Text()) {
				if out[rel] == nil {
					out[rel] = map[int]bool{}
				}
				out[rel][ln] = true
			}
		}
		return sc.Err()
	})
	return out, err
}

func hitsIgnore(set map[int]bool, startLine, endLine int) bool {
	for ln := range set {
		if ln >= startLine && ln <= endLine {
			return true
		}
	}
	return false
}
