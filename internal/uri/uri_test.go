package uri

import (
	"fmt"
	"testing"
)

func TestFindAllUnenclosed(t *testing.T) {
	s := "Authorization: Bearer key-rest://user1/openai/api-key"
	matches := FindAll(s)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if m.Enclosed {
		t.Fatal("should not be enclosed")
	}
	if len(m.KeyURIs) != 1 || m.KeyURIs[0] != "user1/openai/api-key" {
		t.Fatalf("unexpected KeyURIs: %v", m.KeyURIs)
	}
}

func TestFindAllEnclosed(t *testing.T) {
	s := "https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage"
	matches := FindAll(s)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if !m.Enclosed {
		t.Fatal("should be enclosed")
	}
	if m.Transform != "" {
		t.Fatalf("unexpected transform: %s", m.Transform)
	}
	if len(m.KeyURIs) != 1 || m.KeyURIs[0] != "user1/telegram/bot-token" {
		t.Fatalf("unexpected KeyURIs: %v", m.KeyURIs)
	}
}

func TestFindAllEnclosedTransform(t *testing.T) {
	s := `Authorization: Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}`
	matches := FindAll(s)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if !m.Enclosed {
		t.Fatal("should be enclosed")
	}
	if m.Transform != "base64" {
		t.Fatalf("expected base64, got %s", m.Transform)
	}
	if len(m.KeyURIs) != 2 {
		t.Fatalf("expected 2 key URIs, got %d", len(m.KeyURIs))
	}
	if m.KeyURIs[0] != "user1/atlassian/email" {
		t.Fatalf("unexpected first URI: %s", m.KeyURIs[0])
	}
	if m.KeyURIs[1] != "user1/atlassian/token" {
		t.Fatalf("unexpected second URI: %s", m.KeyURIs[1])
	}
	if len(m.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(m.Args))
	}
	if m.Args[1].IsURI || m.Args[1].Value != ":" {
		t.Fatalf("unexpected second arg: %+v", m.Args[1])
	}
}

func TestFindAllMultipleUnenclosed(t *testing.T) {
	s := "?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token"
	matches := FindAll(s)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestFindAllMixed(t *testing.T) {
	// Enclosed and unenclosed in the same string
	s := "url: https://api.example.com/bot{{ key-rest://user1/token }}/send header: key-rest://user1/other"
	matches := FindAll(s)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	// First match should be enclosed
	if !matches[0].Enclosed {
		t.Fatal("first match should be enclosed")
	}
	// Second match should be unenclosed
	if matches[1].Enclosed {
		t.Fatal("second match should be unenclosed")
	}
}

func TestFindAllNoMatch(t *testing.T) {
	s := "Authorization: Bearer sk-12345"
	matches := FindAll(s)
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestReplaceUnenclosed(t *testing.T) {
	s := "Authorization: Bearer key-rest://user1/openai/api-key"
	result, err := Replace(s, func(uri string) ([]byte, error) {
		if uri == "user1/openai/api-key" {
			return []byte("sk-real-key-123"), nil
		}
		return nil, fmt.Errorf("unknown URI: %s", uri)
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := "Authorization: Bearer sk-real-key-123"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestReplaceEnclosed(t *testing.T) {
	s := "https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage"
	result, err := Replace(s, func(uri string) ([]byte, error) {
		return []byte("123456:ABC-DEF"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := "https://api.telegram.org/bot123456:ABC-DEF/sendMessage"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestReplaceBase64Transform(t *testing.T) {
	s := `Authorization: Basic {{ base64(key-rest://user1/email, ":", key-rest://user1/token) }}`
	result, err := Replace(s, func(uri string) ([]byte, error) {
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
	// base64("user@example.com:secret123") = "dXNlckBleGFtcGxlLmNvbTpzZWNyZXQxMjM="
	expected := "Authorization: Basic dXNlckBleGFtcGxlLmNvbTpzZWNyZXQxMjM="
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestReplaceMultipleUnenclosed(t *testing.T) {
	s := "?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token"
	result, err := Replace(s, func(uri string) ([]byte, error) {
		switch uri {
		case "user1/trello/api-key":
			return []byte("APIKEY"), nil
		case "user1/trello/token":
			return []byte("TOKEN"), nil
		}
		return nil, fmt.Errorf("unknown: %s", uri)
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := "?key=APIKEY&token=TOKEN"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestReplaceResolverError(t *testing.T) {
	s := "key-rest://user1/missing"
	_, err := Replace(s, func(uri string) ([]byte, error) {
		return nil, fmt.Errorf("key not found: %s", uri)
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReplaceNoMatch(t *testing.T) {
	s := "no URIs here"
	result, err := Replace(s, func(uri string) ([]byte, error) {
		t.Fatal("resolver should not be called")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != s {
		t.Fatalf("got %q, want %q", result, s)
	}
}

func TestReplaceUnknownTransform(t *testing.T) {
	s := "{{ sha256(key-rest://user1/key) }}"
	_, err := Replace(s, func(uri string) ([]byte, error) {
		return []byte("value"), nil
	})
	if err == nil {
		t.Fatal("expected error for unknown transform")
	}
}
