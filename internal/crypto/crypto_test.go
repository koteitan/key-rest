package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	passphrase := []byte("test-passphrase")
	plaintext := []byte("my-secret-api-key-12345")

	ciphertext, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if len(ciphertext) < minCipherLen {
		t.Fatalf("ciphertext too short: %d bytes", len(ciphertext))
	}

	decrypted, err := Decrypt(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted text does not match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	passphrase := []byte("correct-passphrase")
	wrong := []byte("wrong-passphrase")
	plaintext := []byte("secret")

	ciphertext, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(ciphertext, wrong)
	if err == nil {
		t.Fatal("Decrypt should fail with wrong passphrase")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := Decrypt([]byte("short"), []byte("pass"))
	if err == nil {
		t.Fatal("Decrypt should fail with short ciphertext")
	}
}

func TestEncryptDifferentCiphertexts(t *testing.T) {
	passphrase := []byte("pass")
	plaintext := []byte("same-plaintext")

	c1, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions of the same plaintext should produce different ciphertexts")
	}
}

func TestZeroClear(t *testing.T) {
	buf := []byte{1, 2, 3, 4, 5}
	ZeroClear(buf)
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("byte %d not cleared: %d", i, b)
		}
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	passphrase := []byte("passphrase")
	salt := []byte("0123456789abcdef") // 16 bytes

	k1 := DeriveKey(passphrase, salt)
	k2 := DeriveKey(passphrase, salt)

	if !bytes.Equal(k1, k2) {
		t.Fatal("DeriveKey should be deterministic with same passphrase and salt")
	}

	if len(k1) != KeySize {
		t.Fatalf("key length should be %d, got %d", KeySize, len(k1))
	}
}

func TestDeriveKeyDifferentSalts(t *testing.T) {
	passphrase := []byte("passphrase")
	salt1 := []byte("0123456789abcdef")
	salt2 := []byte("fedcba9876543210")

	k1 := DeriveKey(passphrase, salt1)
	k2 := DeriveKey(passphrase, salt2)

	if bytes.Equal(k1, k2) {
		t.Fatal("different salts should produce different keys")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	passphrase := []byte("pass")
	plaintext := []byte("")

	ciphertext, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptCorruptedData(t *testing.T) {
	passphrase := []byte("pass")
	plaintext := []byte("secret")

	ciphertext, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt a byte in the ciphertext portion
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = Decrypt(ciphertext, passphrase)
	if err == nil {
		t.Fatal("Decrypt should fail with corrupted ciphertext")
	}
}
