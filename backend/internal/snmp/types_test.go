package snmp

import (
	"context"
	"testing"
)

type fakeClient struct{}

func (fakeClient) Get(context.Context, []string) ([]VarBind, error) { return nil, nil }
func (fakeClient) Walk(context.Context, string) ([]VarBind, error)  { return nil, nil }
func (fakeClient) Close() error                                     { return nil }

func TestJoinMarkerRowsUsesExactInstance(t *testing.T) {
	lifeBase := "1.3.6.1.2.1.43.10.2.1.4"
	unitBase := "1.3.6.1.2.1.43.10.2.1.3"
	rows := JoinMarkerRows(
		[]VarBind{{OID: lifeBase + ".1.1", Value: int64(100)}, {OID: lifeBase + ".1.2", Value: int64(200)}},
		[]VarBind{{OID: unitBase + ".1.2", Value: int64(19)}, {OID: unitBase + ".1.1", Value: int64(7)}},
		lifeBase, unitBase,
	)
	if len(rows) != 2 || rows[0].Instance != "1.1" || rows[0].Unit != 7 || rows[1].Unit != 19 {
		t.Fatalf("unexpected joined rows: %#v", rows)
	}
}

func TestJoinMarkerRowsOmitsMissingUnit(t *testing.T) {
	rows := JoinMarkerRows(
		[]VarBind{{OID: MarkerLifeOID + ".1.1", Value: int64(100)}},
		[]VarBind{}, MarkerLifeOID, MarkerUnitOID,
	)
	if len(rows) != 0 {
		t.Fatalf("expected no row without matching unit, got %#v", rows)
	}
}

func TestNumericValueRejectsNegativeAndMalformedValues(t *testing.T) {
	for _, value := range []any{int64(-1), "-1", "123abc", ""} {
		if got, ok := NumericValue(value); ok || got != 0 {
			t.Fatalf("expected invalid non-counter value %v, got %d, %v", value, got, ok)
		}
	}
}

func TestConfigRejectsMissingSecretMaterial(t *testing.T) {
	if err := (Config{Host: "printer", Port: 161, Version: Version2c, TimeoutSeconds: 2, MaxRepetitions: 25}).Validate(); err == nil {
		t.Fatal("expected missing community error")
	}
	if err := (Config{Host: "printer", Port: 161, Version: Version3, TimeoutSeconds: 2, MaxRepetitions: 25}).Validate(); err == nil {
		t.Fatal("expected missing username error")
	}
	if err := (Config{Host: "printer", Port: 161, Version: Version3, Username: "user", PrivProtocol: "AES", PrivPassphrase: "secret", TimeoutSeconds: 2, MaxRepetitions: 25}).Validate(); err == nil {
		t.Fatal("expected privacy without auth error")
	}
}
