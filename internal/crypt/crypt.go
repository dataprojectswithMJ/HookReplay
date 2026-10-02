// Package crypt provides AES-256-GCM encryption for server-side secrets (§2).
// Secrets are encrypted at rest, decrypted only in the dispatch path, and
// zeroized after use.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Encrypt seals plaintext with AES-256-GCM, returning nonce||ciphertext.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens a blob produced by Encrypt.
func Decrypt(blob, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, fmt.Errorf("crypt: ciphertext too short")
	}
	return gcm.Open(nil, blob[:ns], blob[ns:], nil)
}

// MasterKeyFromEnv returns the 32-byte AES-256 key. It reads
// HOOKREPLAY_MASTER_KEY (hex or base64). If unset, it derives a
// development-only key — never use this in production.
func MasterKeyFromEnv() ([]byte, error) {
	v := os.Getenv("HOOKREPLAY_MASTER_KEY")
	if v == "" {
		sum := sha256.Sum256([]byte("hookreplay-dev-master-key-do-not-use"))
		return sum[:], nil
	}
	if b, err := hex.DecodeString(v); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(v); err == nil && len(b) == 32 {
		return b, nil
	}
	return nil, fmt.Errorf("crypt: HOOKREPLAY_MASTER_KEY must be exactly 32 bytes (hex or base64)")
}
