package security

import (
	"bytes"
	"testing"
)

func TestSecretBoxRoundTripAndTamperDetection(t *testing.T) {
	box, err := NewSecretBox(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte("community-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ciphertext, []byte("community-secret")) {
		t.Fatal("secret must not be stored plaintext")
	}
	plaintext, err := box.Decrypt(ciphertext)
	if err != nil || string(plaintext) != "community-secret" {
		t.Fatalf("unexpected plaintext: %q, %v", plaintext, err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := box.Decrypt(ciphertext); err == nil {
		t.Fatal("expected tamper detection")
	}
}

func TestParseKey(t *testing.T) {
	if key, err := ParseKey("BwECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8"); err != nil || len(key) != 32 {
		t.Fatalf("base64 key failed: %v", err)
	}
	if key, err := ParseKey("BwECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="); err != nil || len(key) != 32 {
		t.Fatalf("padded base64 key failed: %v", err)
	}
	if key, err := ParseKey("0707070707070707070707070707070707070707070707070707070707070707"); err != nil || len(key) != 32 {
		t.Fatalf("hex key failed: %v", err)
	}
	if _, err := ParseKey("short"); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestGenerateKeyAndToken(t *testing.T) {
	keyStr, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	parsed, err := ParseKey(keyStr)
	if err != nil || len(parsed) != 32 {
		t.Fatalf("ParseKey failed on generated key %q: %v", keyStr, err)
	}

	tokStr, err := GenerateAPIToken()
	if err != nil || len(tokStr) < 40 {
		t.Fatalf("GenerateAPIToken failed: %v, tok=%q", err, tokStr)
	}
}

func TestSNMPSecretRejectsPrivacyWithoutAuth(t *testing.T) {
	secret := SNMPSecret{Version: "3", Username: "user", PrivProtocol: "AES", PrivPassphrase: "secret"}
	if err := secret.Validate(); err == nil {
		t.Fatal("expected privacy without auth error")
	}
}
