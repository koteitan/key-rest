// Package keystore manages encrypted key storage (keys.enc) and in-memory key access.
package keystore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/koteitan/key-rest/internal/crypto"
)

// KeyEntry represents a single key entry in the keystore.
type KeyEntry struct {
	URI            string `json:"uri"`
	URLPrefix      string `json:"url_prefix"`
	AllowURL       bool   `json:"allow_url"`
	AllowBody      bool   `json:"allow_body"`
	EncryptedValue string `json:"encrypted_value"` // base64-encoded salt||nonce||ciphertext
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
	Value     []byte // plaintext key value; caller must ZeroClear when done
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

// DefaultDir returns the default data directory (~/.key-rest/).
func DefaultDir() (string, error) {
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
func (s *Store) Add(uri, urlPrefix string, allowURL, allowBody bool, value, passphrase []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kf, err := s.load()
	if err != nil {
		return err
	}

	// Check for duplicate URI
	for _, k := range kf.Keys {
		if k.URI == uri {
			return errors.New("key already exists: " + uri)
		}
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
		EncryptedValue: base64.StdEncoding.EncodeToString(encrypted),
	}
	kf.Keys = append(kf.Keys, entry)

	if err := s.save(kf); err != nil {
		return err
	}

	// If decrypted keys are loaded in memory, add this one too
	if s.decrypted != nil {
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		s.decrypted = append(s.decrypted, DecryptedKey{
			URI:       uri,
			URLPrefix: urlPrefix,
			AllowURL:  allowURL,
			AllowBody: allowBody,
			Value:     valueCopy,
		})
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
				crypto.ZeroClear(dk.Value)
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

		decrypted = append(decrypted, DecryptedKey{
			URI:       k.URI,
			URLPrefix: k.URLPrefix,
			AllowURL:  k.AllowURL,
			AllowBody: k.AllowBody,
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

// ClearAll zeros out all decrypted keys in memory.
// Called when the daemon stops.
func (s *Store) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearDecryptedLocked()
}

func (s *Store) clearDecryptedLocked() {
	s.clearDecrypted(s.decrypted)
	s.decrypted = nil
}

func (s *Store) clearDecrypted(keys []DecryptedKey) {
	for i := range keys {
		crypto.ZeroClear(keys[i].Value)
	}
}
