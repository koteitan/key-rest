// Package crypto provides AES-256-GCM encryption/decryption with PBKDF2 key derivation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"

	"golang.org/x/crypto/pbkdf2"
)

const (
	SaltSize       = 16
	NonceSize      = 12 // standard GCM nonce size
	KeySize        = 32 // AES-256
	PBKDF2Iter     = 600_000
	minCipherLen   = SaltSize + NonceSize + 1 // salt + nonce + at least 1 byte ciphertext
)

// DeriveKey derives a 32-byte AES-256 key from a passphrase and salt using PBKDF2-HMAC-SHA256.
// The caller is responsible for calling ZeroClear on the returned key when done.
func DeriveKey(passphrase, salt []byte) []byte {
	return pbkdf2.Key(passphrase, salt, PBKDF2Iter, KeySize, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM with a key derived from the passphrase.
// Returns salt (16 bytes) || nonce (12 bytes) || ciphertext (includes GCM auth tag).
func Encrypt(plaintext, passphrase []byte) ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	key := DeriveKey(passphrase, salt)
	defer ZeroClear(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// result = salt || nonce || ciphertext
	result := make([]byte, 0, SaltSize+NonceSize+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// Decrypt decrypts data produced by Encrypt using the given passphrase.
// Input format: salt (16 bytes) || nonce (12 bytes) || ciphertext.
func Decrypt(data, passphrase []byte) ([]byte, error) {
	if len(data) < minCipherLen {
		return nil, errors.New("ciphertext too short")
	}

	salt := data[:SaltSize]
	nonce := data[SaltSize : SaltSize+NonceSize]
	ciphertext := data[SaltSize+NonceSize:]

	key := DeriveKey(passphrase, salt)
	defer ZeroClear(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed: wrong passphrase or corrupted data")
	}

	return plaintext, nil
}

// ZeroClear overwrites a byte slice with zeros.
func ZeroClear(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
