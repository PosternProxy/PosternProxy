package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// derivationSalt is a fixed, application-specific salt for PBKDF2. A random
// per-record salt would be stronger, but the key must be derivable at startup
// from the passphrase alone (no DB access yet). The high iteration count and
// high-entropy passphrase (64 hex chars when auto-generated) make this safe.
var derivationSalt = []byte("posternproxy-enc-v1")

const pbkdf2Iterations = 600_000 // OWASP 2023 recommendation for SHA-256

// DeriveKey returns a 32-byte AES-256 key from a passphrase using PBKDF2.
func DeriveKey(passphrase string) [32]byte {
	dk := pbkdf2.Key([]byte(passphrase), derivationSalt, pbkdf2Iterations, 32, sha256.New)
	var key [32]byte
	copy(key[:], dk)
	return key
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// nonce+ciphertext blob.
func Encrypt(key [32]byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt decrypts a base64-encoded nonce+ciphertext produced by Encrypt.
func Decrypt(key [32]byte, encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < aead.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := data[:aead.NonceSize()], data[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
