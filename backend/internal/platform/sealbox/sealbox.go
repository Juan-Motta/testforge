// Package sealbox encrypts small secrets (provider credentials) at rest with
// AES-256-GCM. Each value is bound to a context string through additional
// authenticated data, so a ciphertext copied to another row does not open.
package sealbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// version prefixes every ciphertext so the format or key can rotate later.
const version = "v1."

var errMalformed = errors.New("sealbox: malformed ciphertext")

// Box seals and opens secrets with one key.
type Box struct {
	aead cipher.AEAD
}

// New builds a Box from a base64-encoded 32-byte key.
func New(base64Key string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Key))
	if err != nil || len(key) != 32 {
		return nil, errors.New("SECRET_KEY must be a base64-encoded 32-byte key (generate one with `openssl rand -base64 32`)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("sealbox: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("sealbox: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext bound to context.
func (b *Box) Seal(plaintext, context string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("sealbox: nonce: %w", err)
	}
	out := b.aead.Seal(nonce, nonce, []byte(plaintext), []byte(context))
	return version + base64.RawURLEncoding.EncodeToString(out), nil
}

// Open decrypts a value produced by Seal with the same context.
func (b *Box) Open(sealed, context string) (string, error) {
	payload, ok := strings.CutPrefix(sealed, version)
	if !ok {
		return "", errMalformed
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(raw) < b.aead.NonceSize()+b.aead.Overhead() {
		return "", errMalformed
	}
	nonce, ct := raw[:b.aead.NonceSize()], raw[b.aead.NonceSize():]
	plain, err := b.aead.Open(nil, nonce, ct, []byte(context))
	if err != nil {
		return "", errors.New("sealbox: cannot decrypt (wrong key or context)")
	}
	return string(plain), nil
}
