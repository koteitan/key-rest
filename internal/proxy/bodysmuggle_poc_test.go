package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
)

// TestTransferEncodingStrippedFromWire — P-013 regression.
// Agent-supplied Transfer-Encoding must not reach the wire alongside the
// daemon-added Content-Length, otherwise CL.TE smuggling is possible on
// the upstream parser.
func TestTransferEncodingStrippedFromWire(t *testing.T) {
	addr, tlsConfig, captured, wg := rawTLSCapture(t)

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	store.Add("user1/te/key", "https://localhost/", false, false, nil, []byte("TE-PROBE-SECRET"), pass)
	store.DecryptAll(pass)

	p := NewForTest(store, tlsConfig, addr)

	body := "padding"
	resp := p.Handle(&Request{
		Type:   "http",
		Method: "POST",
		URL:    "https://localhost/",
		Headers: map[string]string{
			"Authorization":     "Bearer key-rest://user1/te/key",
			"Transfer-Encoding": "chunked",
		},
		Body: &body,
	})
	if resp.Error != nil {
		t.Fatalf("request failed: %s", resp.Error.Message)
	}

	wg.Wait()
	wire := string(*captured)
	t.Logf("wire payload:\n%s", wire)

	if strings.Contains(strings.ToLower(wire), "transfer-encoding") {
		t.Errorf("agent-supplied Transfer-Encoding reached the wire")
	}
}

// TestValidationRaceDoesNotLeak — P-014 regression.
// Race a Disable in during validateField. The request must either fail
// closed or, if it proceeds, mask the credential in the response.
func TestValidationRaceDoesNotLeak(t *testing.T) {
	const secretValue = "VAL-RACE-SECRET-12345"

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"echoed": "` + auth + `"}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	store.Add("user1/val/key", ts.URL+"/", false, false, nil, []byte(secretValue), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	const attempts = 50
	for i := 0; i < attempts; i++ {
		// Re-enable for each iteration
		store.Add("user1/val/key", ts.URL+"/", false, false, nil, []byte(secretValue), pass)
		store.DecryptAll(pass)

		var resp *Response
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp = p.Handle(&Request{
				Type:   "http",
				Method: "GET",
				URL:    ts.URL + "/",
				Headers: map[string]string{
					"Authorization": "Bearer key-rest://user1/val/key",
				},
			})
		}()

		// Tight race against validation
		go func() {
			time.Sleep(time.Duration(i%10) * time.Microsecond)
			store.Disable("user1/val/key")
		}()

		wg.Wait()

		if resp.Error == nil && strings.Contains(resp.Body, secretValue) {
			t.Fatalf("attempt %d: credential leaked: %q", i, resp.Body)
		}
	}
}
