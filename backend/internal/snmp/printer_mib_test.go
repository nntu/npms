package snmp

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLoadPrinterMIBFileMatchesDefaultDictionary(t *testing.T) {
	mibPath := filepath.Join("..", "..", "..", "Printer-MIB.yaml")
	mib, err := LoadPrinterMIBFile(mibPath)
	if err != nil {
		t.Fatalf("load Printer-MIB.yaml: %v", err)
	}
	if len(mib.Symbols) < 200 {
		t.Fatalf("expected at least 200 symbols in Printer-MIB.yaml, got %d", len(mib.Symbols))
	}

	def := DefaultPrinterMIB()
	criticalSymbols := []string{
		"printmib",
		"prtGeneralTable",
		"prtGeneralEntry",
		"prtMarkerDefaultIndex",
		"prtGeneralPrinterName",
		"prtGeneralSerialNumber",
		"prtMarkerTable",
		"prtMarkerEntry",
		"prtMarkerCounterUnit",
		"prtMarkerLifeCount",
		"prtMarkerPowerOnCount",
		"prtMarkerSuppliesTable",
		"prtMarkerSuppliesEntry",
		"prtMarkerSuppliesDescription",
		"prtMarkerSuppliesSupplyUnit",
		"prtMarkerSuppliesMaxCapacity",
		"prtMarkerSuppliesLevel",
		"prtInputTable",
		"prtInputEntry",
		"prtCoverTable",
		"prtConsoleDisplayBufferText",
		"prtAlertTable",
	}
	for _, sym := range criticalSymbols {
		fromYAML, okYAML := mib.OID(sym)
		fromDef, okDef := def.OID(sym)
		if !okYAML || !okDef || fromYAML != fromDef {
			t.Fatalf("symbol %q mismatch: yaml=(%q,%v) default=(%q,%v)", sym, fromYAML, okYAML, fromDef, okDef)
		}
	}
}

func TestResolveOIDExtractsSymbolTableEntryAndInstance(t *testing.T) {
	mibPath := filepath.Join("..", "..", "..", "Printer-MIB.yaml")
	mib, err := LoadPrinterMIBFile(mibPath)
	if err != nil {
		t.Fatalf("load Printer-MIB.yaml: %v", err)
	}

	res, ok := mib.ResolveOID("1.3.6.1.2.1.43.10.2.1.4.1.1")
	if !ok {
		t.Fatal("expected OID resolution to succeed")
	}
	if res.Symbol != "prtMarkerLifeCount" || res.Table != "prtMarkerTable" || res.Entry != "prtMarkerEntry" || res.Instance != "1.1" {
		t.Fatalf("unexpected resolution: %+v", res)
	}

	serialRes, ok := mib.ResolveOID("1.3.6.1.2.1.43.5.1.1.17.1")
	if !ok || serialRes.Symbol != "prtGeneralSerialNumber" || serialRes.Instance != "1" {
		t.Fatalf("unexpected serial resolution: %+v", serialRes)
	}
}

func TestNormalizeVarBindValueDecodesPrintableUTF8AndRedactsBinary(t *testing.T) {
	if got := NormalizeVarBindValue([]byte("HP LaserJet Enterprise M501dn\x00")); got != "HP LaserJet Enterprise M501dn" {
		t.Fatalf("expected printable string, got %#v", got)
	}
	if got := NormalizeVarBindValue([]byte{0x01, 0xaf}); got != "0x01af" {
		t.Fatalf("expected hex string for binary bytes, got %#v", got)
	}
	if s, ok := StringValue("0x48502d4d353031"); !ok || s != "HP-M501" {
		t.Fatalf("expected hex-encoded ASCII string to decode to HP-M501, got %q (%v)", s, ok)
	}
}

func TestTryReadMarkerByInstanceUsesSingleGetPacket(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "printer-mib", "hp-laserjet-enterprise-m501dn.json")
	client, err := LoadFixtureClient(fixturePath)
	if err != nil {
		t.Fatalf("load fixture client: %v", err)
	}

	row, ok, err := TryReadMarkerByInstance(context.Background(), client, MarkerLifeOID, MarkerUnitOID, "1.1")
	if err != nil {
		t.Fatalf("TryReadMarkerByInstance error: %v", err)
	}
	if !ok || row.Instance != "1.1" || row.Life != 48250 || row.Unit != 7 {
		t.Fatalf("unexpected marker row: %+v (ok=%v)", row, ok)
	}
	if len(client.GetCalls) != 1 || len(client.WalkCalls) != 0 {
		t.Fatalf("expected exactly 1 GET call and 0 WALK calls, got %d GETs and %d WALKs", len(client.GetCalls), len(client.WalkCalls))
	}
}

func TestCollectPrinterMIBSnapshotAcrossModelFixtures(t *testing.T) {
	mibPath := filepath.Join("..", "..", "..", "Printer-MIB.yaml")
	mib, err := LoadPrinterMIBFile(mibPath)
	if err != nil {
		t.Fatalf("load Printer-MIB.yaml: %v", err)
	}

	fixtures := []struct {
		file         string
		wantName     string
		wantSerial   string
		wantMarkers  int
		wantSupplies int
	}{
		{
			file:         "hp-laserjet-enterprise-m501dn.json",
			wantName:     "HP-M501DN-KT01",
			wantSerial:   "REDACTED-M501DN-001",
			wantMarkers:  1,
			wantSupplies: 1,
		},
		{
			file:         "hp-laserjet-pro-m402dn.json",
			wantName:     "HP-M402DN-HC01",
			wantSerial:   "REDACTED-M402DN-002",
			wantMarkers:  1,
			wantSupplies: 1,
		},
		{
			file:         "brother-hl-l5100dn.json",
			wantName:     "BRN-L5100DN-GD01",
			wantSerial:   "REDACTED-BRL5100-003",
			wantMarkers:  1,
			wantSupplies: 2,
		},
		{
			file:         "brother-hl-l6210dw.json",
			wantName:     "BRN-L6210DW-TD01",
			wantSerial:   "REDACTED-BRL6210-004",
			wantMarkers:  1,
			wantSupplies: 1,
		},
		{
			file:         "ricoh-im-2702-multi-marker.json",
			wantName:     "RICOH-IM2702-VP",
			wantSerial:   "REDACTED-RICOH-005",
			wantMarkers:  2,
			wantSupplies: 1,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "printer-mib", tc.file)
			client, err := LoadFixtureClient(path)
			if err != nil {
				t.Fatalf("load fixture %s: %v", tc.file, err)
			}
			snapshot, err := CollectPrinterMIBSnapshot(context.Background(), client, mib)
			if err != nil {
				t.Fatalf("collect snapshot %s: %v", tc.file, err)
			}
			if snapshot.Name != tc.wantName {
				t.Fatalf("expected name %q, got %q", tc.wantName, snapshot.Name)
			}
			if snapshot.Serial != tc.wantSerial {
				t.Fatalf("expected serial %q, got %q", tc.wantSerial, snapshot.Serial)
			}
			if len(snapshot.Markers) != tc.wantMarkers {
				t.Fatalf("expected %d markers, got %d (%+v)", tc.wantMarkers, len(snapshot.Markers), snapshot.Markers)
			}
			if len(snapshot.Supplies) != tc.wantSupplies {
				t.Fatalf("expected %d supplies, got %d (%+v)", tc.wantSupplies, len(snapshot.Supplies), snapshot.Supplies)
			}
		})
	}
}
