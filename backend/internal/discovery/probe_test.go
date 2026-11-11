package discovery

import (
	"context"
	"errors"
	"os"
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
	if ricohRes.DeviceType != "printer" {
		t.Fatalf("expected Ricoh IM 2702 to be printer, got %s", ricohRes.DeviceType)
	}
}

func TestClassifyDeviceType(t *testing.T) {
	tests := []struct {
		name             string
		manufacturer     string
		model            string
		devName          string
		descr            string
		sysObjectID      string
		matchedProfileID string
		markerCount      int
		want             string
	}{
		{
			name:             "Brother ADS-2400N scanner",
			manufacturer:     "Brother",
			model:            "Brother ADS-2400N",
			devName:          "BRNB42200441557",
			descr:            "Brother NC-04h, Firmware Ver.U ,MID 8WD-501",
			matchedProfileID: "brother-ads",
			markerCount:      0,
			want:             "scanner",
		},
		{
			name:             "Brother NC-10h network card for ADS-4300N",
			manufacturer:     "Brother",
			model:            "Brother NC-10h",
			devName:          "BRN000EC6962304",
			descr:            "Brother NC-10h, Firmware Ver.2.28, MID 5WD-B03",
			matchedProfileID: "",
			markerCount:      0,
			want:             "scanner",
		},
		{
			name:             "HP ScanJet Enterprise Flow 7000",
			manufacturer:     "HP",
			model:            "HP ScanJet Flow 7000 s3",
			devName:          "SCANJET-7000",
			descr:            "HP ScanJet Flow 7000",
			matchedProfileID: "",
			markerCount:      0,
			want:             "scanner",
		},
		{
			name:             "Fujitsu fi-7160 document scanner",
			manufacturer:     "Fujitsu",
			model:            "fi-7160",
			devName:          "FI-7160",
			descr:            "Fujitsu fi-7160 Scanner",
			matchedProfileID: "",
			markerCount:      0,
			want:             "scanner",
		},
		{
			name:             "HP LaserJet Enterprise M501dn printer",
			manufacturer:     "HP",
			model:            "HP LaserJet Enterprise M501dn",
			devName:          "HP-M501DN",
			descr:            "Hewlett-Packard HP LaserJet Enterprise M501dn",
			matchedProfileID: "hp-laserjet-enterprise-m501dn",
			markerCount:      1,
			want:             "printer",
		},
		{
			name:             "Brother HL-L5100DN printer",
			manufacturer:     "Brother",
			model:            "Brother HL-L5100DN",
			devName:          "BRNB42200B39DB8",
			descr:            "Brother NC-8300h",
			matchedProfileID: "brother-hl-l5100dn",
			markerCount:      1,
			want:             "printer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyDeviceType(tt.manufacturer, tt.model, tt.devName, tt.descr, tt.sysObjectID, tt.matchedProfileID, tt.markerCount)
			if got != tt.want {
				t.Errorf("ClassifyDeviceType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadDiscoveryRules(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "rules.yaml")
	content := []byte(`
vendor_oids:
  "1.3.6.1.4.1.9999": "CustomVendor"
vendor_keywords:
  CustomVendor:
    - "custombrand"
scanner_keywords:
  - "custom-scanner"
`)
	if err := os.WriteFile(tempFile, content, 0o600); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadRules(tempFile)
	if err != nil {
		t.Fatalf("failed to load rules: %v", err)
	}

	if rules.VendorOIDs["1.3.6.1.4.1.9999"] != "CustomVendor" {
		t.Fatalf("expected custom vendor OID to be present")
	}
	// Verify default rules still present
	if rules.VendorOIDs["1.3.6.1.4.1.11"] != "HP" {
		t.Fatalf("expected default HP OID to be preserved")
	}

	oldRules := GetGlobalRules()
	SetGlobalRules(rules)
	defer SetGlobalRules(oldRules)

	mfg, _ := InferPrinterModelAndVendor("1.3.6.1.4.1.9999.1", "", "")
	if mfg != "CustomVendor" {
		t.Fatalf("expected CustomVendor from OID, got %q", mfg)
	}

	devType := ClassifyDeviceType("", "", "my-custom-scanner-device", "", "", "", 0)
	if devType != "scanner" {
		t.Fatalf("expected scanner from custom keyword, got %q", devType)
	}
}
