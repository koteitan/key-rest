package main

import (
	"strings"
	"testing"

	"github.com/koteitan/key-rest/internal/keystore"
)

func TestFormatPlacementLegacyAllowURL(t *testing.T) {
	got := formatPlacement(nil, true, false)
	if got != " [url]" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatPlacementLegacyAllowBody(t *testing.T) {
	got := formatPlacement(nil, false, true)
	if got != " [body]" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatPlacementLegacyHeadersDefault(t *testing.T) {
	got := formatPlacement(nil, false, false)
	if got != "" {
		t.Fatalf("got %q (expected empty)", got)
	}
}

func TestFormatPlacementAllowOnlyMixed(t *testing.T) {
	p := &keystore.Placement{
		URL:     true,
		Body:    true,
		Headers: []string{"Authorization", "X-Api-Key"},
		Queries: []string{"key"},
		Fields:  []string{"api_key"},
	}
	got := formatPlacement(p, false, false)
	for _, want := range []string{
		" [url]", " [body]",
		" [header:Authorization]", " [header:X-Api-Key]",
		" [query:key]", " [field:api_key]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestFormatPlacementAllowOnlyEmpty(t *testing.T) {
	p := &keystore.Placement{}
	got := formatPlacement(p, false, false)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
