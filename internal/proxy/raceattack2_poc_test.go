package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koteitan/key-rest/internal/keystore"
)

// TestReloadAfterTamperDoesNotLeak — regression test.
//
// While an HTTP request is in-flight, an attacker
//   (1) deletes the target entry from keys.enc on disk (no master passphrase
//       needed — it's just a JSON edit), then
//   (2) triggers a keystore reload.
//
// The mask snapshot taken at validation time must still drive the response
// masker, so the credential bytes do not appear unmasked in the response.
func TestReloadAfterTamperDoesNotLeak(t *testing.T) {
	const secretA = "TARGET-CREDENTIAL-AAAAAA"
	const secretB = "OTHER-CREDENTIAL-BBBBBB"

	var serverGate sync.Mutex
	serverGate.Lock()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverGate.Lock()
		serverGate.Unlock()
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"echoed": "` + auth + `"}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	store, _ := keystore.New(dir)
	pass := []byte("test-pass")
	store.Add("user1/target/key", ts.URL+"/", false, false, nil, []byte(secretA), pass)
	store.Add("user1/other/key", ts.URL+"/other/", false, false, nil, []byte(secretB), pass)
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
				"Authorization": "Bearer key-rest://user1/target/key",
			},
		})
		if resp.Error != nil {
			t.Logf("response error: %s", resp.Error.Message)
			return
		}
		responseBody = resp.Body
	}()

	// Let the request reach the upstream gate. The goroutine must complete
	// validation, snapshot, and the TLS write before we tamper with the store.
	time.Sleep(100 * time.Millisecond)

	// Tamper with keys.enc — remove target entry without the master passphrase.
	keysEncPath := filepath.Join(dir, "keys.enc")
	raw, err := os.ReadFile(keysEncPath)
	if err != nil {
		t.Fatal(err)
	}
	var kf struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	if err := json.Unmarshal(raw, &kf); err != nil {
		t.Fatal(err)
	}
	kept := kf.Keys[:0]
	for _, k := range kf.Keys {
		if k["uri"] != "user1/target/key" {
			kept = append(kept, k)
		}
	}
	kf.Keys = kept
	tampered, _ := json.Marshal(kf)
	if err := os.WriteFile(keysEncPath, tampered, 0600); err != nil {
		t.Fatal(err)
	}

	// Trigger reload — target key disappears from the in-memory store.
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}

	// Release the upstream so the response is read and masked AFTER reload.
	serverGate.Unlock()
	wg.Wait()

	t.Logf("response body: %s", responseBody)
	if strings.Contains(responseBody, secretA) {
		t.Errorf("credential leaked despite snapshot: %q in body", secretA)
	}
	if !strings.Contains(responseBody, "key-rest://user1/target/key") {
		t.Errorf("expected masked URI in response, got: %s", responseBody)
	}
	_ = secretB
}
