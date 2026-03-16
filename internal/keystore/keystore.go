// Package keystore manages encrypted key storage (keys.enc) and in-memory key access.
package keystore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/koteitan/key-rest/internal/crypto"
)

// Placement defines where a credential may appear in a request.
// nil means legacy mode (use AllowURL/AllowBody flags).
type Placement struct {
	Headers []string `json:"headers,omitempty"` // allowed header names (case-insensitive)
	Queries []string `json:"queries,omitempty"` // allowed URL query parameter names
	Fields  []string `json:"fields,omitempty"`  // allowed JSON body field names
	URL     bool     `json:"url,omitempty"`     // allow anywhere in URL
	Body    bool     `json:"body,omitempty"`    // allow anywhere in body
}

// KeyEntry represents a single key entry in the keystore.
type KeyEntry struct {
	URI            string     `json:"uri"`
	URLPrefix      string     `json:"url_prefix"`
	AllowURL       bool       `json:"allow_url"`
	AllowBody      bool       `json:"allow_body"`
	AllowOnly      *Placement `json:"allow_only,omitempty"`
	EncryptedValue string     `json:"encrypted_value"` // base64-encoded salt||nonce||ciphertext
}

// keysFile is the on-disk JSON structure.
type keysFile struct {
	Keys []KeyEntry `json:"keys"`
}

// DecryptedKey holds a decrypted key in memory.
type DecryptedKey struct {
	URI       string
	URLPrefix string
	AllowURL  bool
	AllowBody bool
	AllowOnly *Placement
	Value     []byte // plaintext key value; caller must ZeroClear when done
	Disabled  bool   // true if key is disabled (Value is nil)
}

// KeyStatus represents the runtime status of a key for listing.
type KeyStatus struct {
	URI       string     `json:"uri"`
	URLPrefix string     `json:"url_prefix"`
	AllowURL  bool       `json:"allow_url,omitempty"`
	AllowBody bool       `json:"allow_body,omitempty"`
	AllowOnly *Placement `json:"allow_only,omitempty"`
	Disabled  bool       `json:"disabled"`
}

// Store manages the key-rest keystore.
type Store struct {
	mu        sync.RWMutex
	dir       string         // ~/.key-rest/
	decrypted []DecryptedKey // in-memory decrypted keys (only when daemon is running)
}

// New creates a new Store rooted at the given directory.
// The directory is created if it does not exist (permissions 0700).
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// DefaultDir returns the data directory.
// If KEY_REST_DIR is set, it is used; otherwise ~/.key-rest/.
func DefaultDir() (string, error) {
	if dir := os.Getenv("KEY_REST_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".key-rest"), nil
}

func (s *Store) filePath() string {
	return filepath.Join(s.dir, "keys.enc")
}

// load reads the keys.enc file. Returns empty keysFile if the file does not exist.
func (s *Store) load() (*keysFile, error) {
	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &keysFile{}, nil
		}
		return nil, err
	}
	var kf keysFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, err
	}
	return &kf, nil
}

// save writes the keysFile to disk with permissions 0600.
func (s *Store) save(kf *keysFile) error {
	data, err := json.MarshalIndent(kf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), data, 0600)
}

// Add encrypts a key value and adds it to the keystore.
func (s *Store) Add(uri, urlPrefix string, allowURL, allowBody bool, allowOnly *Placement, value, passphrase []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kf, err := s.load()
	if err != nil {
		return err
	}

	encrypted, err := crypto.Encrypt(value, passphrase)
	if err != nil {
		return err
	}

	entry := KeyEntry{
		URI:            uri,
		URLPrefix:      urlPrefix,
		AllowURL:       allowURL,
		AllowBody:      allowBody,
		AllowOnly:      allowOnly,
		EncryptedValue: base64.StdEncoding.EncodeToString(encrypted),
	}

	// Overwrite if URI already exists, otherwise append
	replaced := false
	for i, k := range kf.Keys {
		if k.URI == uri {
			kf.Keys[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		kf.Keys = append(kf.Keys, entry)
	}

	if err := s.save(kf); err != nil {
		return err
	}

	// If decrypted keys are loaded in memory, update or add
	if s.decrypted != nil {
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		crypto.Mlock(valueCopy)
		dk := DecryptedKey{
			URI:       uri,
			URLPrefix: urlPrefix,
			AllowURL:  allowURL,
			AllowBody: allowBody,
			AllowOnly: allowOnly,
			Value:     valueCopy,
		}
		if replaced {
			for i := range s.decrypted {
				if s.decrypted[i].URI == uri {
					crypto.ZeroClearAndMunlock(s.decrypted[i].Value)
					s.decrypted[i] = dk
					break
				}
			}
		} else {
			s.decrypted = append(s.decrypted, dk)
		}
	}

	return nil
}

// Remove removes a key from the keystore by URI.
func (s *Store) Remove(uri string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kf, err := s.load()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]KeyEntry, 0, len(kf.Keys))
	for _, k := range kf.Keys {
		if k.URI == uri {
			found = true
			continue
		}
		filtered = append(filtered, k)
	}
	if !found {
		return errors.New("key not found: " + uri)
	}

	kf.Keys = filtered
	if err := s.save(kf); err != nil {
		return err
	}

	// Remove from in-memory decrypted keys
	if s.decrypted != nil {
		for i, dk := range s.decrypted {
			if dk.URI == uri {
				crypto.ZeroClearAndMunlock(dk.Value)
				s.decrypted = append(s.decrypted[:i], s.decrypted[i+1:]...)
				break
			}
		}
	}

	return nil
}

// List returns the URIs and URL prefixes of all keys (without decrypting values).
func (s *Store) List() ([]KeyEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kf, err := s.load()
	if err != nil {
		return nil, err
	}

	// Return entries without encrypted values
	result := make([]KeyEntry, len(kf.Keys))
	for i, k := range kf.Keys {
		result[i] = KeyEntry{
			URI:       k.URI,
			URLPrefix: k.URLPrefix,
			AllowURL:  k.AllowURL,
			AllowBody: k.AllowBody,
			AllowOnly: k.AllowOnly,
		}
	}
	return result, nil
}

// DecryptAll decrypts all keys and holds them in memory.
// Called when the daemon starts.
func (s *Store) DecryptAll(passphrase []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kf, err := s.load()
	if err != nil {
		return err
	}

	decrypted := make([]DecryptedKey, 0, len(kf.Keys))
	for _, k := range kf.Keys {
		raw, err := base64.StdEncoding.DecodeString(k.EncryptedValue)
		if err != nil {
			s.clearDecrypted(decrypted)
			return err
		}

		value, err := crypto.Decrypt(raw, passphrase)
		if err != nil {
			s.clearDecrypted(decrypted)
			return err
		}
		crypto.Mlock(value)

		decrypted = append(decrypted, DecryptedKey{
			URI:       k.URI,
			URLPrefix: k.URLPrefix,
			AllowURL:  k.AllowURL,
			AllowBody: k.AllowBody,
			AllowOnly: k.AllowOnly,
			Value:     value,
		})
	}

	s.clearDecryptedLocked()
	s.decrypted = decrypted
	return nil
}

// Lookup finds a decrypted key by URI. Returns nil if not found or not decrypted.
func (s *Store) Lookup(uri string) *DecryptedKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.decrypted {
		if s.decrypted[i].URI == uri {
			return &s.decrypted[i]
		}
	}
	return nil
}

// RLock acquires a read lock on the store.
func (s *Store) RLock() { s.mu.RLock() }

// RUnlock releases a read lock on the store.
func (s *Store) RUnlock() { s.mu.RUnlock() }

// Decrypted returns the in-memory decrypted keys slice.
// The caller must hold at least a read lock (via RLock).
func (s *Store) Decrypted() []DecryptedKey { return s.decrypted }

// ClearAll zeros out all decrypted keys in memory.
// Called when the daemon stops.
func (s *Store) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearDecryptedLocked()
}

// Disable disables all decrypted keys whose URI starts with uriPrefix.
// Zero-clears plaintext values immediately. Returns the number of keys disabled.
func (s *Store) Disable(uriPrefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for i := range s.decrypted {
		if strings.HasPrefix(s.decrypted[i].URI, uriPrefix) && !s.decrypted[i].Disabled {
			crypto.ZeroClearAndMunlock(s.decrypted[i].Value)
			s.decrypted[i].Value = nil
			s.decrypted[i].Disabled = true
			count++
		}
	}
	return count
}

// Enable re-enables all decrypted keys whose URI starts with uriPrefix.
// Re-decrypts values from the keys.enc file. Returns the number of keys enabled.
func (s *Store) Enable(uriPrefix string, passphrase []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kf, err := s.load()
	if err != nil {
		return 0, err
	}

	count := 0
	for i := range s.decrypted {
		if strings.HasPrefix(s.decrypted[i].URI, uriPrefix) && s.decrypted[i].Disabled {
			for _, ke := range kf.Keys {
				if ke.URI == s.decrypted[i].URI {
					raw, err := base64.StdEncoding.DecodeString(ke.EncryptedValue)
					if err != nil {
						return count, err
					}
					value, err := crypto.Decrypt(raw, passphrase)
					if err != nil {
						return count, err
					}
					crypto.Mlock(value)
					s.decrypted[i].Value = value
					s.decrypted[i].Disabled = false
					count++
					break
				}
			}
		}
	}
	return count, nil
}

// ListStatus returns the runtime status of all decrypted keys.
func (s *Store) ListStatus() []KeyStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]KeyStatus, len(s.decrypted))
	for i, dk := range s.decrypted {
		result[i] = KeyStatus{
			URI:       dk.URI,
			URLPrefix: dk.URLPrefix,
			AllowURL:  dk.AllowURL,
			AllowBody: dk.AllowBody,
			AllowOnly: dk.AllowOnly,
			Disabled:  dk.Disabled,
		}
	}
	return result
}

func (s *Store) clearDecryptedLocked() {
	s.clearDecrypted(s.decrypted)
	s.decrypted = nil
}

func (s *Store) clearDecrypted(keys []DecryptedKey) {
	for i := range keys {
		crypto.ZeroClearAndMunlock(keys[i].Value)
	}
}
