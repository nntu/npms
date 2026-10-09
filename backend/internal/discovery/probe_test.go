package discovery

import (
	"context"
	"errors"
	"testing"

	"npms/backend/internal/snmp"
)

type fakeClient struct {
	values []snmp.VarBind
	life   []snmp.VarBind
	units  []snmp.VarBind
	err    error
}

func (f fakeClient) Connect() error                                        { return nil }
func (f fakeClient) Get(context.Context, []string) ([]snmp.VarBind, error) { return f.values, nil }
func (f fakeClient) Walk(_ context.Context, oid string) ([]snmp.VarBind, error) {
	if f.err != nil {
		return nil, f.err
	}
	if oid == snmp.MarkerLifeOID {
		return f.life, nil
	}
	return f.units, nil
}

func TestProbeKeepsIdentityWhenMarkerWalkFails(t *testing.T) {
	result, err := Probe(context.Background(), snmp.Config{Host: "192.168.1.21"}, func(snmp.Config) (snmp.Client, error) {
		return fakeClient{
			values: []snmp.VarBind{
				{OID: SysObjectOID, Value: "1.3.6.1.4.1.11"},
				{OID: SysNameOID, Value: "front-printer"},
			},
			err: errors.New("printer-mib unavailable"),
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "front-printer" || len(result.Markers) != 0 {
		t.Fatalf("unexpected result after marker walk failure: %#v", result)
	}
}
func (fakeClient) Close() error { return nil }

func TestProbeReadsPrinterIdentity(t *testing.T) {
	result, err := Probe(context.Background(), snmp.Config{Host: "192.168.1.20"}, func(snmp.Config) (snmp.Client, error) {
		return fakeClient{values: []snmp.VarBind{
			{OID: SysDescrOID, Value: "HP LaserJet"},
			{OID: SysObjectOID, Value: "1.3.6.1.4.1.11"},
			{OID: SysNameOID, Value: "front-printer"},
			{OID: SerialOID, Value: []byte("fixture-serial")},
		}, life: []snmp.VarBind{{OID: snmp.MarkerLifeOID + ".1.1", Value: int64(12345)}}, units: []snmp.VarBind{{OID: snmp.MarkerUnitOID + ".1.1", Value: int64(7)}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Address != "192.168.1.20" || result.Name != "front-printer" || result.SysObjectID != "1.3.6.1.4.1.11" || result.Serial != "fixture-serial" {
		t.Fatalf("unexpected probe result: %#v", result)
	}
	if len(result.Markers) != 1 || result.Markers[0].Life != 12345 || result.Markers[0].Unit != 7 {
		t.Fatalf("unexpected marker result: %#v", result.Markers)
	}
}

func TestProbeRequiresFactory(t *testing.T) {
	if _, err := Probe(context.Background(), snmp.Config{}, nil); err == nil {
		t.Fatal("expected missing factory error")
	}
}
