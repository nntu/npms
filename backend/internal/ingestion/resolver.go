package ingestion

import (
	"context"
	"fmt"

	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

type CredentialStore interface {
	GetSNMPCredential(context.Context, string) (repository.SNMPCredential, error)
}

type ConfigResolver struct {
	Store CredentialStore
	Box   *security.SecretBox
}

func (r ConfigResolver) Resolve(ctx context.Context, endpoint repository.DeviceEndpoint) (snmp.Config, error) {
	if r.Store == nil || r.Box == nil {
		return snmp.Config{}, fmt.Errorf("credential resolver is not initialized")
	}
	if endpoint.Address == "" || endpoint.Port == 0 {
		return snmp.Config{}, fmt.Errorf("endpoint address and port are required")
	}
	if endpoint.CredentialID == "" {
		return snmp.Config{}, fmt.Errorf("endpoint %s has no credential reference", endpoint.ID)
	}
	stored, err := r.Store.GetSNMPCredential(ctx, endpoint.CredentialID)
	if err != nil {
		return snmp.Config{}, err
	}
	plaintext, err := r.Box.Decrypt(stored.EncryptedSecretMaterial)
	if err != nil {
		return snmp.Config{}, err
	}
	secret, err := security.DecodeSNMPSecret(plaintext)
	if err != nil {
		return snmp.Config{}, err
	}
	if stored.Version != secret.Version {
		return snmp.Config{}, fmt.Errorf("credential version %q does not match secret version %q", stored.Version, secret.Version)
	}
	config := snmp.Config{Host: endpoint.Address, Port: endpoint.Port, Version: snmp.Version(secret.Version), Community: secret.Community, Username: secret.Username, AuthProtocol: secret.AuthProtocol, AuthPassphrase: secret.AuthPassphrase, PrivProtocol: secret.PrivProtocol, PrivPassphrase: secret.PrivPassphrase, TimeoutSeconds: 3, Retries: 1, MaxRepetitions: 25}
	if err := config.Validate(); err != nil {
		return snmp.Config{}, err
	}
	return config, nil
}
