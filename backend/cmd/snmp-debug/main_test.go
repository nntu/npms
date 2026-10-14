package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"npms/backend/internal/snmp"
)

func TestFormatConsoleShowsHostPrinterAndCurrentValues(t *testing.T) {
	got := formatConsole(output{
		Host: "192.168.1.20",
		Results: []snmp.VarBind{
			{OID: sysNameOID, Value: "Office Printer"},
			{OID: sysObjectOID, Value: "1.3.6.1.4.1.11.2.3.9"},
			{OID: serialOID, Value: "masked-fixture-serial"},
		},
		Markers: []snmp.MarkerRow{{Instance: "1", Unit: 7, Life: 12345}},
	})

	for _, want := range []string{"192.168.1.20", "Office Printer", "1.3.6.1.4.1.11.2.3.9", "masked-fixture-serial", "12345"} {
		if !strings.Contains(got, want) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
}

func TestFormatConsoleReportsMissingCounters(t *testing.T) {
	got := formatConsole(output{Host: "printer.local"})
	if !strings.Contains(got, "no validated marker rows returned") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestRunHelpCommand(t *testing.T) {
	for _, flagArg := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		var stdout, stderr strings.Builder
		err := run(flagArg, &stdout, &stderr)
		if err != nil {
			t.Fatalf("expected nil error for %v, got %v", flagArg, err)
		}
		if !strings.Contains(stdout.String(), "NPMS SNMP Diagnostic CLI (snmp-debug)") {
			t.Fatalf("help output missing usage title:\n%s", stdout.String())
		}
	}
}

func TestRunCheckAndCollectWithFixtureAndPrinterMIB(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "printer-mib", "hp-laserjet-enterprise-m501dn.json")
	mibPath := filepath.Join("..", "..", "..", "Printer-MIB.yaml")

	var checkOut, checkErr strings.Builder
	if err := run([]string{"check", "--fixture", fixturePath, "--mib-file", mibPath}, &checkOut, &checkErr); err != nil {
		t.Fatalf("snmp-debug check failed: %v (stderr: %s)", err, checkErr.String())
	}
	for _, want := range []string{"HP-M501DN-KT01", "REDACTED-M501DN-001", "48250", "Black Cartridge HP CF287A"} {
		if !strings.Contains(checkOut.String(), want) {
			t.Fatalf("check output missing %q:\n%s", want, checkOut.String())
		}
	}

	var collectOut, collectErr strings.Builder
	if err := run([]string{"collect", "--fixture", fixturePath, "--mib-file", mibPath, "--anonymize"}, &collectOut, &collectErr); err != nil {
		t.Fatalf("snmp-debug collect failed: %v (stderr: %s)", err, collectErr.String())
	}
	var snapshot snmp.PrinterMIBSnapshot
	if err := json.Unmarshal([]byte(collectOut.String()), &snapshot); err != nil {
		t.Fatalf("unmarshal collected snapshot: %v", err)
	}
	if snapshot.Serial != "REDACTED-SERIAL" || len(snapshot.Markers) != 1 || snapshot.Markers[0].Life != 48250 || len(snapshot.Supplies) != 1 {
		t.Fatalf("unexpected collected snapshot: %+v", snapshot)
	}
}
