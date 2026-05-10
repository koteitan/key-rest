package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
)

// TestDisableDuringRequestDoesNotLeak — regression test.
//
// While an HTTP request is in-flight (upstream artificially blocked), an
// attacker calls Disable() on the key. The response masker must still
// recognise and replace the credential bytes with their key-rest:// URI;
// otherwise the credential leaks verbatim to the agent.
func TestDisableDuringRequestDoesNotLeak(t *testing.T) {
	const secretValue = "RACE-WIN-SECRET-CREDENTIAL-VALUE"

	var serverGate sync.Mutex
	serverGate.Lock()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the disabling goroutine has run.
		serverGate.Lock()
		serverGate.Unlock()

		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"echoed_authorization": "` + auth + `"}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	store.Add("user1/race/key", ts.URL+"/", false, false, nil, []byte(secretValue), pass)
	store.DecryptAll(pass)

	tlsConfig, addr := testTLSConfig(ts)
	p := NewForTest(store, tlsConfig, addr)

	var responseBody string
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		resp := p.Handle(&Request{
			Type:   "http",
			Method: "GET",
			URL:    ts.URL + "/",
			Headers: map[string]string{
				"Authorization": "Bearer key-rest://user1/race/key",
			},
		})
		if resp.Error != nil {
			t.Logf("response error: %s", resp.Error.Message)
			return
		}
		responseBody = resp.Body
	}()

	// Wait for the request to reach the upstream (gate is held there).
	time.Sleep(100 * time.Millisecond)

	// Disable the key. With the mask snapshot in place, the response masker
	// must still know the bytes to mask; without it, the credential leaks.
	store.Disable("user1/race/key")

	// Release the upstream.
	serverGate.Unlock()
	wg.Wait()

	t.Logf("response body: %s", responseBody)
	if strings.Contains(responseBody, secretValue) {
		t.Errorf("credential leaked despite snapshot: %q in body", secretValue)
	}
	if !strings.Contains(responseBody, "key-rest://user1/race/key") {
		t.Errorf("expected masked URI in response, got: %s", responseBody)
	}
}

// TestDisableDuringRequestNaturalTimingDoesNotLeak — runs the race many times
// at natural localhost timing. With the snapshot fix in place, no run should
// leak the credential.
func TestDisableDuringRequestNaturalTimingDoesNotLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping high-iteration race test in -short mode")
	}
	const secretValue = "PROBE-SECRET-CREDENTIAL-1234"

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
	tlsConfig, addr := testTLSConfig(ts)

	const attempts = 200
	var leaks int32

	for i := 0; i < attempts; i++ {
		store.Add("user1/race/key", ts.URL+"/", false, false, nil, []byte(secretValue), pass)
		store.DecryptAll(pass)
		p := NewForTest(store, tlsConfig, addr)

		var responseBody string
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := p.Handle(&Request{
				Type:   "http",
				Method: "GET",
				URL:    ts.URL + "/",
				Headers: map[string]string{
					"Authorization": "Bearer key-rest://user1/race/key",
				},
			})
			if resp.Error == nil {
				responseBody = resp.Body
			}
		}()

		go func() {
			time.Sleep(50 * time.Microsecond)
			store.Disable("user1/race/key")
		}()

		wg.Wait()

		if strings.Contains(responseBody, secretValue) {
			atomic.AddInt32(&leaks, 1)
		}
	}

	if leaks > 0 {
		t.Errorf("credential leaked in %d / %d attempts despite snapshot", leaks, attempts)
	}
}
