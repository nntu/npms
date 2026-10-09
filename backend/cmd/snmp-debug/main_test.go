package main

import (
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
