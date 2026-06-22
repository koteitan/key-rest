package proxy

import (
	"net/url"
	"strings"
	"testing"
)

// TestPercentMaskStrayBypass demonstrates that a single malformed '%' anywhere
// in a response body disables the ENTIRE percent-decode masking pass
// (maskPercentEncoded bails when url.QueryUnescape errors), so a credential
// reflected only in percent-encoded form leaks. Error/5xx bodies commonly
// contain a literal '%' (e.g. "100% done", "fmt %s"), making this practical.
//
// Credential must contain a non-alphanumeric char (real API keys do, e.g. '-'),
// so percent-encoding hides it from the raw maskCredentials pass; the percent
// pass is then the only catcher — and a stray '%' disables it.
func TestPercentMaskStrayBypass(t *testing.T) {
	cred := []byte("sk-test-LEAKME") // '-' is non-alnum -> percent-encoded by the server
	snap := []credSnapshot{{uri: "t/x/key", urlPrefix: "https://localhost", value: cred}}

	// What the server reflects: percentEncodeAll(cred). Only '-' -> %2D.
	enc := "sk%2Dtest%2DLEAKME"
	if got := percentEncodeAll(string(cred)); got != enc {
		t.Fatalf("sanity: server encoding = %q, expected %q", got, enc)
	}

	apply := func(body string) string {
		// Exact masking sequence proxy.go applies to a response body
		// (transform step is identity with no transform outputs).
		out := maskCredentials(body, snap)
		out = maskTruncatedKeys(out, snap)
		out = maskPercentEncoded(out, snap, nil)
		return out
	}

	// CONTROL: clean body, no stray '%'. Percent pass decodes and masks.
	clean := `{"echo":"` + enc + `","note":"all done"}`
	cleanOut := apply(clean)
	t.Logf("control (no stray %%) -> %s", cleanOut)
	if strings.Contains(cleanOut, enc) || strings.Contains(cleanOut, string(cred)) {
		t.Errorf("control unexpectedly leaked")
	}

	// ATTACK: same body + a stray '%' ("100%" -> invalid escape "% d").
	attack := `{"echo":"` + enc + `","note":"100% done"}`
	attackOut := apply(attack)
	t.Logf("attack  (stray %%)  -> %s", attackOut)

	if strings.Contains(attackOut, enc) {
		recovered, _ := url.QueryUnescape(enc)
		t.Errorf("LEAK: percent-encoded credential survived masking; agent decodes %q -> %q",
			enc, recovered)
	}
}

// percentEncodeAll mirrors test-server/main.go (encode every non-alnum byte).
func percentEncodeAll(s string) string {
	var b strings.Builder
	const hex = "0123456789ABCDEF"
	for _, c := range []byte(s) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xf])
		}
	}
	return b.String()
}
