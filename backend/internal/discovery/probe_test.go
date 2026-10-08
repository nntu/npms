package discovery

import (
	"context"
	"testing"

	"npms/backend/internal/snmp"
)

type fakeClient struct {
	values []snmp.VarBind
}

func (f fakeClient) Connect() error                                        { return nil }
func (f fakeClient) Get(context.Context, []string) ([]snmp.VarBind, error) { return f.values, nil }
func (fakeClient) Walk(context.Context, string) ([]snmp.VarBind, error)    { return nil, nil }
func (fakeClient) Close() error                                            { return nil }

func TestProbeReadsPrinterIdentity(t *testing.T) {
	result, err := Probe(context.Background(), snmp.Config{Host: "192.168.1.20"}, func(snmp.Config) (snmp.Client, error) {
		return fakeClient{values: []snmp.VarBind{
			{OID: SysDescrOID, Value: "HP LaserJet"},
			{OID: SysObjectOID, Value: "1.3.6.1.4.1.11"},
			{OID: SysNameOID, Value: "front-printer"},
			{OID: SerialOID, Value: []byte("fixture-serial")},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Address != "192.168.1.20" || result.Name != "front-printer" || result.SysObjectID != "1.3.6.1.4.1.11" || result.Serial != "fixture-serial" {
		t.Fatalf("unexpected probe result: %#v", result)
	}
}

func TestProbeRequiresFactory(t *testing.T) {
	if _, err := Probe(context.Background(), snmp.Config{}, nil); err == nil {
		t.Fatal("expected missing factory error")
	}
}
