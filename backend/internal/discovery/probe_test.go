package discovery

import (
	"context"
	"errors"
	"path/filepath"
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
			{OID: SysLocationOID, Value: "Phòng Kế toán - Tầng 2"},
			{OID: SerialOID, Value: []byte("fixture-serial")},
		}, life: []snmp.VarBind{{OID: snmp.MarkerLifeOID + ".1.1", Value: int64(12345)}}, units: []snmp.VarBind{{OID: snmp.MarkerUnitOID + ".1.1", Value: int64(7)}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Address != "192.168.1.20" || result.Name != "front-printer" || result.SysObjectID != "1.3.6.1.4.1.11" || result.Serial != "fixture-serial" || result.Location != "Phòng Kế toán - Tầng 2" {
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

func TestProbeReadsPrinterMIBTableInstanceSerialAndFallbackName(t *testing.T) {
	m501Path := filepath.Join("..", "..", "testdata", "printer-mib", "hp-laserjet-enterprise-m501dn.json")
	m501Client, err := snmp.LoadFixtureClient(m501Path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Probe(context.Background(), snmp.Config{Host: "192.0.2.10"}, func(snmp.Config) (snmp.Client, error) {
		return m501Client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Serial != "REDACTED-M501DN-001" || res.Name != "HP-M501DN-KT01" || res.Manufacturer != "HP" || res.Model != "HP LaserJet Enterprise M501dn" || len(res.Markers) != 1 || res.Markers[0].Life != 48250 {
		t.Fatalf("unexpected M501dn probe result: %+v", res)
	}
	if len(m501Client.GetCalls) != 1 {
		t.Fatalf("expected 1 batched GET call for identity+serial(.1), got %d", len(m501Client.GetCalls))
	}

	ricohPath := filepath.Join("..", "..", "testdata", "printer-mib", "ricoh-im-2702-multi-marker.json")
	ricohClient, err := snmp.LoadFixtureClient(ricohPath)
	if err != nil {
		t.Fatal(err)
	}
	ricohRes, err := Probe(context.Background(), snmp.Config{Host: "192.0.2.14"}, func(snmp.Config) (snmp.Client, error) {
		return ricohClient, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ricohRes.Name != "RICOH-IM2702-VP" || ricohRes.Serial != "REDACTED-RICOH-005" || ricohRes.Manufacturer != "Ricoh" || ricohRes.Model != "RICOH IM 2702" || len(ricohRes.Markers) != 2 {
		t.Fatalf("unexpected Ricoh probe result: %+v", ricohRes)
	}
}
