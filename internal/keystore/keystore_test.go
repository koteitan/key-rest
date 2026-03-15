package keystore

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return store
}

func TestAddAndList(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("test-passphrase")

	err := store.Add("user1/brave/api-key", "https://api.search.brave.com/", false, false, nil, []byte("brave-key-123"), pass)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URI != "user1/brave/api-key" {
		t.Fatalf("unexpected URI: %s", entries[0].URI)
	}
	if entries[0].URLPrefix != "https://api.search.brave.com/" {
		t.Fatalf("unexpected URLPrefix: %s", entries[0].URLPrefix)
	}
	if entries[0].EncryptedValue != "" {
		t.Fatal("List should not expose encrypted values")
	}
}

func TestAddOverwrite(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")

	if err := store.Add("user1/key", "https://example.com/", false, false, nil, []byte("val"), pass); err != nil {
		t.Fatal(err)
	}

	// Overwrite with new value and different options
	if err := store.Add("user1/key", "https://example2.com/", true, false, nil, []byte("val2"), pass); err != nil {
		t.Fatal(err)
	}

	// Should still have only one entry
	entries, _ := store.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URLPrefix != "https://example2.com/" {
		t.Fatalf("expected overwritten url-prefix, got %s", entries[0].URLPrefix)
	}
	if !entries[0].AllowURL {
		t.Fatal("expected allow_url to be true after overwrite")
	}

	// Verify decrypted value
	store.DecryptAll(pass)
	dk := store.Lookup("user1/key")
	if dk == nil {
		t.Fatal("key not found after overwrite")
	}
	if string(dk.Value) != "val2" {
		t.Fatalf("expected val2, got %s", string(dk.Value))
	}
}

func TestRemove(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")

	store.Add("user1/a", "https://a.com/", false, false, nil, []byte("va"), pass)
	store.Add("user1/b", "https://b.com/", false, false, nil, []byte("vb"), pass)

	if err := store.Remove("user1/a"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	entries, _ := store.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URI != "user1/b" {
		t.Fatalf("wrong remaining entry: %s", entries[0].URI)
	}
}

func TestRemoveNotFound(t *testing.T) {
	store := setupTestStore(t)
	err := store.Remove("nonexistent")
	if err == nil {
		t.Fatal("should fail for nonexistent key")
	}
}

func TestDecryptAllAndLookup(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")

	store.Add("user1/a", "https://a.com/", false, false, nil, []byte("secret-a"), pass)
	store.Add("user1/b", "https://b.com/", true, true, nil, []byte("secret-b"), pass)

	// Create a fresh store to simulate daemon restart
	store2, _ := New(store.dir)
	if err := store2.DecryptAll(pass); err != nil {
		t.Fatalf("DecryptAll failed: %v", err)
	}

	dk := store2.Lookup("user1/a")
	if dk == nil {
		t.Fatal("Lookup returned nil for user1/a")
	}
	if string(dk.Value) != "secret-a" {
		t.Fatalf("unexpected value: %q", dk.Value)
	}
	if dk.AllowURL || dk.AllowBody {
		t.Fatal("user1/a should not allow url or body")
	}

	dk2 := store2.Lookup("user1/b")
	if dk2 == nil {
		t.Fatal("Lookup returned nil for user1/b")
	}
	if string(dk2.Value) != "secret-b" {
		t.Fatalf("unexpected value: %q", dk2.Value)
	}
	if !dk2.AllowURL || !dk2.AllowBody {
		t.Fatal("user1/b should allow url and body")
	}

	if store2.Lookup("nonexistent") != nil {
		t.Fatal("should return nil for nonexistent key")
	}
}

func TestDecryptAllWrongPassphrase(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("correct")

	store.Add("user1/key", "https://example.com/", false, false, nil, []byte("secret"), pass)

	store2, _ := New(store.dir)
	err := store2.DecryptAll([]byte("wrong"))
	if err == nil {
		t.Fatal("DecryptAll should fail with wrong passphrase")
	}
}

func TestClearAll(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")

	store.Add("user1/key", "https://example.com/", false, false, nil, []byte("secret"), pass)
	store.DecryptAll(pass)

	store.ClearAll()

	if store.Lookup("user1/key") != nil {
		t.Fatal("Lookup should return nil after ClearAll")
	}
}

func TestFilePermissions(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")
	store.Add("user1/key", "https://example.com/", false, false, nil, []byte("val"), pass)

	info, err := os.Stat(filepath.Join(store.dir, "keys.enc"))
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("expected 0600, got %o", perm)
	}
}

func TestAddWhileDaemonRunning(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")

	store.Add("user1/a", "https://a.com/", false, false, nil, []byte("val-a"), pass)
	store.DecryptAll(pass)

	// Add a new key while decrypted keys are in memory
	store.Add("user1/b", "https://b.com/", false, false, nil, []byte("val-b"), pass)

	dk := store.Lookup("user1/b")
	if dk == nil {
		t.Fatal("new key should be available via Lookup immediately")
	}
	if string(dk.Value) != "val-b" {
		t.Fatalf("unexpected value: %q", dk.Value)
	}
}

func TestRemoveWhileDaemonRunning(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")

	store.Add("user1/a", "https://a.com/", false, false, nil, []byte("val-a"), pass)
	store.Add("user1/b", "https://b.com/", false, false, nil, []byte("val-b"), pass)
	store.DecryptAll(pass)

	store.Remove("user1/a")

	if store.Lookup("user1/a") != nil {
		t.Fatal("removed key should not be found via Lookup")
	}
	if store.Lookup("user1/b") == nil {
		t.Fatal("remaining key should still be found via Lookup")
	}
}
