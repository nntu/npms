package security

import "testing"

func TestSNMPSecretRoundTrip(t *testing.T) {
	original := SNMPSecret{Version: "3", Username: "printer-reader", AuthProtocol: "SHA", AuthPassphrase: "auth-secret", PrivProtocol: "AES", PrivPassphrase: "priv-secret"}
	encoded, err := EncodeSNMPSecret(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSNMPSecret(encoded)
	if err != nil || decoded.Username != original.Username || decoded.PrivProtocol != "AES" {
		t.Fatalf("unexpected secret: %#v, %v", decoded, err)
	}
}

func TestSNMPSecretRejectsUnknownFieldAndMissingMaterial(t *testing.T) {
	if _, err := DecodeSNMPSecret([]byte(`{"version":"2c","community":"public","unexpected":"x"}`)); err == nil {
		t.Fatal("expected unknown field error")
	}
	if _, err := EncodeSNMPSecret(SNMPSecret{Version: "2c"}); err == nil {
		t.Fatal("expected missing community error")
	}
}
