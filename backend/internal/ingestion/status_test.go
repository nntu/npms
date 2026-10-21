package ingestion

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

type fakeStatusEndpointStore struct {
	endpoints     []repository.DeviceEndpoint
	successCalled bool
	lastSuccessID string
}

func (f *fakeStatusEndpointStore) ListDeviceEndpoints(_ context.Context, _ string) ([]repository.DeviceEndpoint, error) {
	return f.endpoints, nil
}

func (f *fakeStatusEndpointStore) MarkEndpointSuccess(_ context.Context, endpointID string, _ time.Time) error {
	f.successCalled = true
	f.lastSuccessID = endpointID
	return nil
}

func (f *fakeStatusEndpointStore) MarkDeviceOffline(_ context.Context, _ string) error {
	return nil
}

type fakeStatusClient struct {
	values   []snmp.VarBind
	getErr   error
	closeErr error
}

func (f fakeStatusClient) Get(_ context.Context, _ []string) ([]snmp.VarBind, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.values, nil
}

func (f fakeStatusClient) Walk(context.Context, string) ([]snmp.VarBind, error) {
	return nil, nil
}

func (f fakeStatusClient) Close() error {
	return f.closeErr
}

func testResolver(t *testing.T) ConfigResolver {
	t.Helper()
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := security.EncodeSNMPSecret(security.SNMPSecret{Version: "2c", Community: "public"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	return ConfigResolver{
		Store: fakeCredentialStore{credential: repository.SNMPCredential{ID: "cred-1", Version: "2c", EncryptedSecretMaterial: ciphertext}},
		Box:   box,
	}
}

func TestStatusPollerSuccessWithStringSysName(t *testing.T) {
	store := &fakeStatusEndpointStore{
		endpoints: []repository.DeviceEndpoint{
			{ID: "ep-1", DeviceID: "dev-1", Protocol: "snmp", Address: "192.168.1.100", Port: 161, CredentialID: "cred-1"},
		},
	}
	resolver := testResolver(t)
	factory := func(snmp.Config) (snmp.Client, error) {
		return fakeStatusClient{
			values: []snmp.VarBind{
				{OID: "1.3.6.1.2.1.1.5.0", Type: "OctetString", Value: "HP LaserJet M501dn"},
			},
		}, nil
	}

	poller, err := NewStatusPoller(store, resolver, factory)
	if err != nil {
		t.Fatal(err)
	}

	if err := poller.PollDevice(context.Background(), "dev-1"); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !store.successCalled || store.lastSuccessID != "ep-1" {
		t.Fatalf("expected endpoint ep-1 to be marked success, got called=%v id=%s", store.successCalled, store.lastSuccessID)
	}
}

func TestStatusPollerSuccessWithLeadingDotOID(t *testing.T) {
	store := &fakeStatusEndpointStore{
		endpoints: []repository.DeviceEndpoint{
			{ID: "ep-1", DeviceID: "dev-1", Protocol: "snmp", Address: "192.168.1.100", Port: 161, CredentialID: "cred-1"},
		},
	}
	resolver := testResolver(t)
	factory := func(snmp.Config) (snmp.Client, error) {
		return fakeStatusClient{
			values: []snmp.VarBind{
				{OID: ".1.3.6.1.2.1.1.5.0", Type: "OctetString", Value: "Brother HL-L5100DN"},
			},
		}, nil
	}

	poller, err := NewStatusPoller(store, resolver, factory)
	if err != nil {
		t.Fatal(err)
	}

	if err := poller.PollDevice(context.Background(), "dev-1"); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !store.successCalled {
		t.Fatal("expected endpoint to be marked success")
	}
}

func TestStatusPollerFailsOnNoSuchObject(t *testing.T) {
	store := &fakeStatusEndpointStore{
		endpoints: []repository.DeviceEndpoint{
			{ID: "ep-1", DeviceID: "dev-1", Protocol: "snmp", Address: "192.168.1.100", Port: 161, CredentialID: "cred-1"},
		},
	}
	resolver := testResolver(t)
	factory := func(snmp.Config) (snmp.Client, error) {
		return fakeStatusClient{
			values: []snmp.VarBind{
				{OID: "1.3.6.1.2.1.1.5.0", Type: "NoSuchObject", Value: nil},
			},
		}, nil
	}

	poller, err := NewStatusPoller(store, resolver, factory)
	if err != nil {
		t.Fatal(err)
	}

	err = poller.PollDevice(context.Background(), "dev-1")
	if err == nil {
		t.Fatal("expected error for NoSuchObject, got nil")
	}
	if !strings.Contains(err.Error(), "not supported by device") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if store.successCalled {
		t.Fatal("endpoint should not be marked success on error")
	}
}

func TestStatusPollerFailsOnNetworkTimeout(t *testing.T) {
	store := &fakeStatusEndpointStore{
		endpoints: []repository.DeviceEndpoint{
			{ID: "ep-1", DeviceID: "dev-1", Protocol: "snmp", Address: "192.168.1.100", Port: 161, CredentialID: "cred-1"},
		},
	}
	resolver := testResolver(t)
	factory := func(snmp.Config) (snmp.Client, error) {
		return fakeStatusClient{
			getErr: errors.New("request timeout"),
		}, nil
	}

	poller, err := NewStatusPoller(store, resolver, factory)
	if err != nil {
		t.Fatal(err)
	}

	err = poller.PollDevice(context.Background(), "dev-1")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "request timeout") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if store.successCalled {
		t.Fatal("endpoint should not be marked success on error")
	}
}

func TestStatusPollerFailsOnMissingEndpoint(t *testing.T) {
	store := &fakeStatusEndpointStore{
		endpoints: []repository.DeviceEndpoint{},
	}
	resolver := testResolver(t)
	factory := func(snmp.Config) (snmp.Client, error) {
		return fakeStatusClient{}, nil
	}

	poller, err := NewStatusPoller(store, resolver, factory)
	if err != nil {
		t.Fatal(err)
	}

	err = poller.PollDevice(context.Background(), "dev-1")
	if err == nil || !strings.Contains(err.Error(), "has no SNMP endpoint") {
		t.Fatalf("expected no SNMP endpoint error, got: %v", err)
	}
}
