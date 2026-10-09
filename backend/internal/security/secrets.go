package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

type SecretBox struct{ gcm cipher.AEAD }

func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &SecretBox{gcm: gcm}, nil
}

// ParseKey accepts base64 (preferred, padded or unpadded) or hexadecimal runtime key material.
func ParseKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("encryption key is required")
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes in base64 or hex")
	}
	return decoded, nil
}

// GenerateKey generates a cryptographically secure random 32-byte key encoded in raw base64.
func GenerateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

// GenerateAPIToken generates a cryptographically secure random 32-byte token encoded in raw URL base64.
func GenerateAPIToken() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}


func (b *SecretBox) Encrypt(plaintext []byte) ([]byte, error) {
	if b == nil || b.gcm == nil {
		return nil, fmt.Errorf("secret box is not initialized")
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate secret nonce: %w", err)
	}
	return b.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (b *SecretBox) Decrypt(ciphertext []byte) ([]byte, error) {
	if b == nil || b.gcm == nil {
		return nil, fmt.Errorf("secret box is not initialized")
	}
	if len(ciphertext) < b.gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted secret is too short")
	}
	nonce, payload := ciphertext[:b.gcm.NonceSize()], ciphertext[b.gcm.NonceSize():]
	plaintext, err := b.gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plaintext, nil
}
