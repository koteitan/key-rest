package keystore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewCreatesDir0700 is a security-critical assertion: the parent
// directory MUST be 0700 so that other users on the same machine cannot
// even traverse it to reach the socket / keystore files. This dir-level
// barrier is what makes the (unavoidable) racy chmod in server.Start safe.
func TestNewCreatesDir0700(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "key-rest")
	if _, err := New(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	if mode != 0o700 {
		t.Fatalf("daemon dir must be 0700, got %#o", mode)
	}
}

func TestNewMkdirError(t *testing.T) {
	// Pass a path whose parent is a regular file — MkdirAll fails.
	parent := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(parent, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(filepath.Join(parent, "child")); err == nil {
		t.Fatal("expected error when parent is a file")
	}
}

func TestDefaultDirEnv(t *testing.T) {
	t.Setenv("KEY_REST_DIR", "/tmp/custom-keyrest")
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/custom-keyrest" {
		t.Fatalf("got %q", dir)
	}
}

func TestDefaultDirHome(t *testing.T) {
	t.Setenv("KEY_REST_DIR", "")
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Fatal("DefaultDir should not be empty when HOME is set")
	}
}

// TestDefaultDirNoHome covers the os.UserHomeDir error path. With both
// KEY_REST_DIR and HOME unset, UserHomeDir returns an error on Unix and
// DefaultDir surfaces it.
func TestDefaultDirNoHome(t *testing.T) {
	t.Setenv("KEY_REST_DIR", "")
	t.Setenv("HOME", "")
	_, err := DefaultDir()
	if err == nil {
		t.Fatal("expected error when HOME is unset")
	}
}

func TestLoadCorruptedFile(t *testing.T) {
	store := setupTestStore(t)
	// Write garbage into keys.enc — load() should error on JSON parse.
	if err := os.WriteFile(store.filePath(), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.load(); err == nil {
		t.Fatal("expected error for corrupted keys.enc")
	}
}

func TestDecryptAllCorruptedFile(t *testing.T) {
	store := setupTestStore(t)
	if err := os.WriteFile(store.filePath(), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll([]byte("pass")); err == nil {
		t.Fatal("expected DecryptAll error on corrupted file")
	}
}

func TestDecryptAllCorruptedEncryptedValue(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("pass")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	// Corrupt the on-disk EncryptedValue (invalid base64).
	bad := []byte(`{"keys":[{"uri":"user1/k","url_prefix":"https://e.com/","encrypted_value":"!!not-base64!!"}]}`)
	if err := os.WriteFile(store.filePath(), bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err == nil {
		t.Fatal("expected DecryptAll error for invalid base64")
	}
}

func TestRLockRUnlockDecrypted(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("p")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	store.RLock()
	defer store.RUnlock()
	d := store.Decrypted()
	if len(d) != 1 || d[0].URI != "user1/k" {
		t.Fatalf("unexpected: %+v", d)
	}
}

func TestSaveErrorOnUnwritableDir(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so save() fails.
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700)
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), []byte("p")); err == nil {
		t.Fatal("expected save error on read-only dir")
	}
}

func TestEnableUnknownPrefix(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("p")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	count, err := store.Enable("user1/nonexistent", pass)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 enabled, got %d", count)
	}
}

func TestEnableCorruptedFile(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("p")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	store.Disable("user1/k")
	// Corrupt the file — Enable's load() should fail.
	if err := os.WriteFile(store.filePath(), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enable("user1/k", pass); err == nil {
		t.Fatal("expected Enable error on corrupted file")
	}
}

func TestEnableInvalidEncryptedValue(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("p")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	store.Disable("user1/k")
	// Corrupt the encrypted_value to invalid base64.
	bad := []byte(`{"keys":[{"uri":"user1/k","url_prefix":"https://e.com/","encrypted_value":"!!"}]}`)
	if err := os.WriteFile(store.filePath(), bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enable("user1/k", pass); err == nil {
		t.Fatal("expected Enable error on invalid base64")
	}
}

func TestLoadNonNotExistError(t *testing.T) {
	store := setupTestStore(t)
	// Make keys.enc unreadable so the read errors out (and not with IsNotExist).
	if err := os.WriteFile(store.filePath(), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store.filePath(), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(store.filePath(), 0600)
	if _, err := store.load(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestAddLoadCorruptedFile(t *testing.T) {
	store := setupTestStore(t)
	if err := os.WriteFile(store.filePath(), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), []byte("p")); err == nil {
		t.Fatal("expected Add error on corrupted file")
	}
}

func TestAddReplaceWhileDecrypted(t *testing.T) {
	// Cover the `replaced && s.decrypted != nil` code path inside Add.
	store := setupTestStore(t)
	pass := []byte("p")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v1"), pass); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v2"), pass); err != nil {
		t.Fatal(err)
	}
	dk := store.Lookup("user1/k")
	if dk == nil {
		t.Fatal("key missing")
	}
	if string(dk.Value) != "v2" {
		t.Fatalf("expected v2, got %q", dk.Value)
	}
}

func TestRemoveLoadCorruptedFile(t *testing.T) {
	store := setupTestStore(t)
	if err := os.WriteFile(store.filePath(), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("user1/k"); err == nil {
		t.Fatal("expected Remove error on corrupted file")
	}
}

func TestRemoveSaveError(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), []byte("p")); err != nil {
		t.Fatal(err)
	}
	// Make keys.enc itself read-only so save's WriteFile (O_WRONLY|O_TRUNC) fails.
	if err := os.Chmod(store.filePath(), 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(store.filePath(), 0600)
	if err := store.Remove("user1/k"); err == nil {
		t.Fatal("expected save error")
	}
}

func TestListLoadCorruptedFile(t *testing.T) {
	store := setupTestStore(t)
	if err := os.WriteFile(store.filePath(), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil {
		t.Fatal("expected List error on corrupted file")
	}
}

func TestEnableWrongPassphrase(t *testing.T) {
	store := setupTestStore(t)
	pass := []byte("p")
	if err := store.Add("user1/k", "https://e.com/", false, false, nil, []byte("v"), pass); err != nil {
		t.Fatal(err)
	}
	if err := store.DecryptAll(pass); err != nil {
		t.Fatal(err)
	}
	store.Disable("user1/k")
	if _, err := store.Enable("user1/k", []byte("wrong")); err == nil {
		t.Fatal("expected Enable error with wrong passphrase")
	}
}
