package ingestion

import (
	"context"
	"testing"

	"npms/backend/internal/repository"
	"npms/backend/internal/security"
)

type fakeCredentialStore struct{ credential repository.SNMPCredential }

func (f fakeCredentialStore) GetSNMPCredential(context.Context, string) (repository.SNMPCredential, error) {
	return f.credential, nil
}

func TestConfigResolverDecryptsWithoutReturningPlaintextStorage(t *testing.T) {
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := security.EncodeSNMPSecret(security.SNMPSecret{Version: "2c", Community: "private-read-only"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	resolver := ConfigResolver{Store: fakeCredentialStore{credential: repository.SNMPCredential{ID: "cred-1", Version: "2c", EncryptedSecretMaterial: ciphertext}}, Box: box}
	config, err := resolver.Resolve(context.Background(), repository.DeviceEndpoint{ID: "endpoint-1", Address: "192.0.2.10", Port: 161, CredentialID: "cred-1"})
	if err != nil || config.Community != "private-read-only" || config.Host != "192.0.2.10" {
		t.Fatalf("unexpected resolved config: %#v, %v", config, err)
	}
}
