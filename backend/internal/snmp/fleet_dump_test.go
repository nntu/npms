package snmp

import (
	"context"
	"path/filepath"
	"testing"
)

type mockSNMPClient struct {
	getFunc  func(ctx context.Context, oids []string) ([]VarBind, error)
	walkFunc func(ctx context.Context, rootOID string) ([]VarBind, error)
}

func (m *mockSNMPClient) Connect() error { return nil }
func (m *mockSNMPClient) Close() error   { return nil }
func (m *mockSNMPClient) Get(ctx context.Context, oids []string) ([]VarBind, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, oids)
	}
	return nil, nil
}
func (m *mockSNMPClient) Walk(ctx context.Context, rootOID string) ([]VarBind, error) {
	if m.walkFunc != nil {
		return m.walkFunc(ctx, rootOID)
	}
	return nil, nil
}

func TestDumpPrinterCountersWithFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "printer-mib", "hp-laserjet-enterprise-m501dn.json")
	client, err := LoadFixtureClient(fixturePath)
	if err != nil {
		t.Fatalf("load fixture client: %v", err)
	}

	dump := FleetPrinterDump{
		Address: "192.168.1.50",
		Port:    161,
	}

	DumpPrinterCounters(context.Background(), client, &dump)

	if dump.Status != "online" {
		t.Fatalf("expected status 'online', got %q (err: %s)", dump.Status, dump.Error)
	}
	if dump.SysDescr == "" {
		t.Fatal("expected SysDescr to be non-empty")
	}
	if dump.Model == "" {
		t.Fatal("expected Model to be non-empty")
	}
	if len(dump.Markers) == 0 {
		t.Fatal("expected Markers to be found in fixture")
	}
	if len(dump.CandidateCounters) == 0 {
		t.Fatal("expected CandidateCounters to be detected")
	}

	// Verify candidate counter has life count 48250
	foundLife := false
	for _, c := range dump.CandidateCounters {
		if c.Value == 48250 {
			foundLife = true
			break
		}
	}
	if !foundLife {
		t.Fatalf("expected candidate counter with value 48250, got %+v", dump.CandidateCounters)
	}
}

func TestDumpPrinterCountersVendorFallback(t *testing.T) {
	// Mock a printer that does not have Printer-MIB marker table, but has HP vendor counter
	mock := &mockSNMPClient{
		getFunc: func(ctx context.Context, oids []string) ([]VarBind, error) {
			var res []VarBind
			for _, o := range oids {
				switch o {
				case SysDescrOID:
					res = append(res, VarBind{OID: o, Type: "OctetString", Value: "HP LaserJet Pro M404dn"})
				case SysObjectOID:
					res = append(res, VarBind{OID: o, Type: "ObjectIdentifier", Value: "1.3.6.1.4.1.11.2.3.9.1"})
				case SysNameOID:
					res = append(res, VarBind{OID: o, Type: "OctetString", Value: "HPM404"})
				case SysLocationOID:
					res = append(res, VarBind{OID: o, Type: "OctetString", Value: "Floor 2"})
				case "1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0":
					res = append(res, VarBind{OID: o, Type: "Integer", Value: int64(15420)})
				}
			}
			return res, nil
		},
		walkFunc: func(ctx context.Context, rootOID string) ([]VarBind, error) {
			// Returns empty for standard marker tables
			return nil, nil
		},
	}

	dump := FleetPrinterDump{
		Address: "10.0.0.12",
		Port:    161,
	}

	DumpPrinterCounters(context.Background(), mock, &dump)

	if dump.Status != "online" {
		t.Fatalf("expected status 'online', got %q", dump.Status)
	}
	if dump.Manufacturer != "HP" {
		t.Fatalf("expected manufacturer 'HP', got %q", dump.Manufacturer)
	}

	foundVendor := false
	for _, c := range dump.CandidateCounters {
		if c.Key == "hp_total_page_count" && c.Value == 15420 {
			foundVendor = true
			break
		}
	}
	if !foundVendor {
		t.Fatalf("expected vendor counter hp_total_page_count with value 15420, got: %+v", dump.CandidateCounters)
	}
}

func TestGenerateFleetDumpReportAndAnonymize(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "printer-mib", "hp-laserjet-enterprise-m501dn.json")
	client, err := LoadFixtureClient(fixturePath)
	if err != nil {
		t.Fatalf("load fixture client: %v", err)
	}

	targets := []FleetPrinterDump{
		{Address: "192.168.1.50", Port: 161},
		{Address: "192.168.1.51", Port: 161},
	}

	clientFactory := func(cfg Config) (Client, error) {
		return client, nil
	}

	report := GenerateFleetDumpReport(context.Background(), clientFactory, targets, 2)
	if report.PrinterCount != 2 {
		t.Fatalf("expected 2 printers, got %d", report.PrinterCount)
	}
	if report.OnlineCount != 2 {
		t.Fatalf("expected 2 online, got %d", report.OnlineCount)
	}

	anon := AnonymizeFleetReport(report)
	for _, p := range anon.Printers {
		if p.Address != "192.168.xxx.xxx" {
			t.Fatalf("expected masked address '192.168.xxx.xxx', got %q", p.Address)
		}
		if len(p.Serial) > 3 && p.Serial[3:] != "****" {
			// serial should be masked
			for _, r := range p.Serial[3:] {
				if r != '*' {
					t.Fatalf("expected masked serial, got %q", p.Serial)
				}
			}
		}
	}
}
