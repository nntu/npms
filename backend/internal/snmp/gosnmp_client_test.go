package snmp

import "testing"

func TestJSONValueBytesAreRedactedToNonSecretRepresentation(t *testing.T) {
	if got := jsonValue([]byte{0x01, 0xaf}); got != "0x01af" {
		t.Fatalf("unexpected byte representation: %v", got)
	}
}
