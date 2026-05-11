package uri

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestReplaceBytesUnenclosed(t *testing.T) {
	s := "Authorization: Bearer key-rest://user1/openai/api-key"
	result, err := ReplaceBytes(s, func(uri string) ([]byte, error) {
		if uri == "user1/openai/api-key" {
			return []byte("sk-real"), nil
		}
		return nil, fmt.Errorf("unknown: %s", uri)
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte("Authorization: Bearer sk-real")
	if !bytes.Equal(result, expected) {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestReplaceBytesEnclosedTransform(t *testing.T) {
	s := `Authorization: Basic {{ base64(key-rest://user1/email, ":", key-rest://user1/token) }}`
	result, err := ReplaceBytes(s, func(uri string) ([]byte, error) {
		switch uri {
		case "user1/email":
			return []byte("user@example.com"), nil
		case "user1/token":
			return []byte("secret123"), nil
		}
		return nil, fmt.Errorf("unknown: %s", uri)
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte("Authorization: Basic dXNlckBleGFtcGxlLmNvbTpzZWNyZXQxMjM=")
	if !bytes.Equal(result, expected) {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestReplaceBytesNoMatch(t *testing.T) {
	s := "no URIs here"
	result, err := ReplaceBytes(s, func(uri string) ([]byte, error) {
		t.Fatal("resolver should not be called")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, []byte(s)) {
		t.Fatalf("got %q, want %q", result, s)
	}
}

func TestReplaceBytesResolverError(t *testing.T) {
	s := "key-rest://user1/missing"
	_, err := ReplaceBytes(s, func(uri string) ([]byte, error) {
		return nil, errors.New("not found")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReplaceBytesUnknownTransform(t *testing.T) {
	s := "{{ sha256(key-rest://user1/key) }}"
	_, err := ReplaceBytes(s, func(uri string) ([]byte, error) {
		return []byte("v"), nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReplaceBytesMultiArgsWithoutTransform(t *testing.T) {
	// Construct a Match by hand that has multiple args but no transform —
	// not reachable via FindAll but exercise the error path in resolveMatchBytes.
	m := Match{
		Args: []Arg{
			{IsURI: true, Value: "user1/a"},
			{IsURI: true, Value: "user1/b"},
		},
	}
	_, err := resolveMatchBytes(m, func(uri string) ([]byte, error) {
		return []byte("x"), nil
	})
	if err == nil {
		t.Fatal("expected error for multi-arg without transform")
	}
}

func TestResolveMatchMultiArgsWithoutTransform(t *testing.T) {
	// String variant of the same defensive path.
	m := Match{
		Args: []Arg{
			{IsURI: true, Value: "user1/a"},
			{IsURI: true, Value: "user1/b"},
		},
	}
	_, err := resolveMatch(m, func(uri string) ([]byte, error) {
		return []byte("x"), nil
	})
	if err == nil {
		t.Fatal("expected error for multi-arg without transform")
	}
}

func TestReplaceBytesPartialResolverFailure(t *testing.T) {
	// First match succeeds, second fails — exercise the zero-clear loop in ReplaceBytes.
	s := "key-rest://user1/ok and key-rest://user1/bad"
	_, err := ReplaceBytes(s, func(uri string) ([]byte, error) {
		if uri == "user1/ok" {
			return []byte("ok-val"), nil
		}
		return nil, errors.New("bad")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveMatchExportedAlias(t *testing.T) {
	m := Match{
		KeyURIs: []string{"user1/k"},
		Args:    []Arg{{IsURI: true, Value: "user1/k"}},
	}
	v, err := ResolveMatch(m, func(uri string) ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Fatalf("got %q, want hello", v)
	}
}

func TestParseArgsMalformedString(t *testing.T) {
	// Unterminated string literal — parseArgs breaks out without panicking.
	args := parseArgs(`"unterminated`)
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %v", args)
	}
}

func TestParseArgsTrailingComma(t *testing.T) {
	// Trailing comma after a literal — exercises the "next iteration sees empty" path.
	args := parseArgs(`"a",`)
	if len(args) != 1 || args[0].Value != "a" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestParseArgsUnexpectedToken(t *testing.T) {
	// Non-quote, non-key-rest token — exercises the fall-through break.
	args := parseArgs(`unexpected`)
	if len(args) != 0 {
		t.Fatalf("expected 0, got %v", args)
	}
}

// TestParseArgsKeyRestWithoutPath covers the `loc == nil` break: the
// HasPrefix check matches `key-rest://`, but the trailing path regex
// requires at least one character so FindStringIndex returns nil.
func TestParseArgsKeyRestWithoutPath(t *testing.T) {
	args := parseArgs(`key-rest://`)
	if len(args) != 0 {
		t.Fatalf("expected 0 args for key-rest:// with empty path, got %v", args)
	}
}

func TestParseArgsEmpty(t *testing.T) {
	if len(parseArgs("")) != 0 {
		t.Fatal("empty input should yield empty args")
	}
	if len(parseArgs("   ")) != 0 {
		t.Fatal("whitespace-only should yield empty args")
	}
}

func TestFindAllEnclosedNonURI(t *testing.T) {
	// {{ ... }} that doesn't reference key-rest at all is dropped.
	matches := FindAll(`{{ noise }}`)
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestZeroClear(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5}
	zeroClear(b)
	for i, v := range b {
		if v != 0 {
			t.Fatalf("byte %d not zeroed: %d", i, v)
		}
	}
}
