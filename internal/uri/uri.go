// Package uri handles key-rest:// URI detection, parsing, and substitution.
package uri

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Match represents a found key-rest URI match in a string.
type Match struct {
	Start     int      // byte offset of the full match start
	End       int      // byte offset of the full match end
	Enclosed  bool     // true if matched via {{ }}
	Transform string   // transform function name (e.g., "base64"), empty if none
	KeyURIs   []string // key-rest URIs referenced (without "key-rest://" prefix)
	Literals  []string // interleaved: all args in order (URIs as "key-rest://..." placeholder, literals as-is)
	Args      []Arg    // ordered arguments for transform functions
}

// Arg represents a single argument in a transform function call.
type Arg struct {
	IsURI   bool   // true if this is a key-rest:// URI
	Value   string // URI (without prefix) if IsURI, or literal string value
}

var (
	// Matches {{ ... }} with possible whitespace inside braces
	enclosedRe = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)
	// Matches unenclosed key-rest:// URIs
	unenclosedRe = regexp.MustCompile(`key-rest://[a-zA-Z0-9/_.-]+`)
	// Matches a transform function call: funcname(args...)
	transformRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\((.*)\)$`)
	// Matches a key-rest URI within args
	keyURIRe = regexp.MustCompile(`key-rest://[a-zA-Z0-9/_.-]+`)
)

// FindAll finds all key-rest:// URI references in the given string.
// Enclosed matches ({{ }}) are processed first; their byte ranges are excluded from unenclosed matching.
func FindAll(s string) []Match {
	var matches []Match
	excluded := make([][2]int, 0)

	// 1. Find enclosed matches
	for _, loc := range enclosedRe.FindAllStringSubmatchIndex(s, -1) {
		fullStart, fullEnd := loc[0], loc[1]
		innerStart, innerEnd := loc[2], loc[3]
		inner := s[innerStart:innerEnd]

		m := Match{
			Start:    fullStart,
			End:      fullEnd,
			Enclosed: true,
		}

		// Check for transform function
		if tfMatch := transformRe.FindStringSubmatch(inner); tfMatch != nil {
			m.Transform = tfMatch[1]
			m.Args = parseArgs(tfMatch[2])
			for _, arg := range m.Args {
				if arg.IsURI {
					m.KeyURIs = append(m.KeyURIs, arg.Value)
				}
			}
		} else {
			// Plain enclosed URI: {{ key-rest://user1/service/key }}
			uriMatch := keyURIRe.FindString(inner)
			if uriMatch == "" {
				continue // not a key-rest reference
			}
			uri := strings.TrimPrefix(uriMatch, "key-rest://")
			m.KeyURIs = []string{uri}
			m.Args = []Arg{{IsURI: true, Value: uri}}
		}

		if len(m.KeyURIs) > 0 {
			matches = append(matches, m)
			excluded = append(excluded, [2]int{fullStart, fullEnd})
		}
	}

	// 2. Find unenclosed matches, excluding ranges already matched by enclosed
	for _, loc := range unenclosedRe.FindAllStringIndex(s, -1) {
		start, end := loc[0], loc[1]
		if isExcluded(start, end, excluded) {
			continue
		}
		uri := strings.TrimPrefix(s[start:end], "key-rest://")
		matches = append(matches, Match{
			Start:   start,
			End:     end,
			KeyURIs: []string{uri},
			Args:    []Arg{{IsURI: true, Value: uri}},
		})
	}

	return matches
}

func isExcluded(start, end int, excluded [][2]int) bool {
	for _, ex := range excluded {
		if start >= ex[0] && end <= ex[1] {
			return true
		}
	}
	return false
}

// parseArgs parses the arguments of a transform function call.
// Arguments are comma-separated, and can be key-rest:// URIs or "string literals".
func parseArgs(s string) []Arg {
	var args []Arg
	s = strings.TrimSpace(s)
	if s == "" {
		return args
	}

	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}

		if s[0] == '"' {
			// String literal
			end := strings.Index(s[1:], "\"")
			if end < 0 {
				break // malformed
			}
			args = append(args, Arg{IsURI: false, Value: s[1 : end+1]})
			s = s[end+2:]
		} else if strings.HasPrefix(s, "key-rest://") {
			// URI
			loc := keyURIRe.FindStringIndex(s)
			if loc == nil {
				break
			}
			uri := strings.TrimPrefix(s[loc[0]:loc[1]], "key-rest://")
			args = append(args, Arg{IsURI: true, Value: uri})
			s = s[loc[1]:]
		} else {
			break // unexpected
		}

		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, ",") {
			s = s[1:]
		}
	}

	return args
}

// Resolver is a function that resolves a key URI to its decrypted value.
type Resolver func(uri string) ([]byte, error)

// Replace replaces all key-rest:// URIs in the string using the given resolver.
// Returns the replaced string and any error.
func Replace(s string, resolve Resolver) (string, error) {
	matches := FindAll(s)
	if len(matches) == 0 {
		return s, nil
	}

	// Process matches in reverse order to preserve byte offsets
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]

		replacement, err := resolveMatch(m, resolve)
		if err != nil {
			return "", err
		}

		s = s[:m.Start] + replacement + s[m.End:]
	}

	return s, nil
}

func resolveMatch(m Match, resolve Resolver) (string, error) {
	// Resolve all URI arguments
	resolvedArgs := make([]string, len(m.Args))
	for i, arg := range m.Args {
		if arg.IsURI {
			val, err := resolve(arg.Value)
			if err != nil {
				return "", err
			}
			resolvedArgs[i] = string(val)
		} else {
			resolvedArgs[i] = arg.Value
		}
	}

	if m.Transform != "" {
		return applyTransform(m.Transform, resolvedArgs)
	}

	// No transform: single URI replacement
	if len(resolvedArgs) == 1 {
		return resolvedArgs[0], nil
	}
	return "", errors.New("multiple arguments without transform function")
}

func applyTransform(name string, args []string) (string, error) {
	switch name {
	case "base64":
		concatenated := strings.Join(args, "")
		return base64.StdEncoding.EncodeToString([]byte(concatenated)), nil
	default:
		return "", fmt.Errorf("unknown transform function: %s", name)
	}
}
