package ingestion

import (
	"context"
	"strings"
	"testing"

	"npms/backend/internal/profile"
	"npms/backend/internal/snmp"
)

type mockTesterSNMPClient struct {
	getResponses  map[string][]snmp.VarBind
	walkResponses map[string][]snmp.VarBind
}

func (m mockTesterSNMPClient) Get(_ context.Context, oids []string) ([]snmp.VarBind, error) {
	res := make([]snmp.VarBind, 0, len(oids))
	for _, oid := range oids {
		clean := strings.TrimPrefix(oid, ".")
		if binds, ok := m.getResponses[clean]; ok {
			res = append(res, binds...)
		}
	}
	return res, nil
}

func (m mockTesterSNMPClient) Walk(_ context.Context, oid string) ([]snmp.VarBind, error) {
	clean := strings.TrimPrefix(oid, ".")
	if binds, ok := m.walkResponses[clean]; ok {
		return binds, nil
	}
	return nil, nil
}

func (mockTesterSNMPClient) Close() error { return nil }

func TestProfileTesterPrinterMIBSuccess(t *testing.T) {
	client := mockTesterSNMPClient{
		walkResponses: map[string][]snmp.VarBind{
			"1.3.6.1.2.1.43.10.2.1.4": {
				{OID: "1.3.6.1.2.1.43.10.2.1.4.1.1", Value: int64(12500)},
			},
			"1.3.6.1.2.1.43.10.2.1.3": {
				{OID: "1.3.6.1.2.1.43.10.2.1.3.1.1", Value: int64(8)}, // sheets
			},
		},
	}

	prof := profile.Profile{
		ID: "test-printer-profile",
		Counters: map[string]profile.Counter{
			"marker_life": {
				ValueColumn:  "1.3.6.1.2.1.43.10.2.1.4",
				UnitColumn:   "1.3.6.1.2.1.43.10.2.1.3",
				Mode:         "walk",
				SemanticType: "marker_life",
				Unit:         "sheets",
			},
		},
	}

	tester := &ProfileTester{}
	report := tester.TestProfileWithClient(context.Background(), client, prof)

	if !report.Compatible {
		t.Fatalf("expected compatible=true, got false, msg: %s", report.Message)
	}
	if report.FitnessScore != 100 {
		t.Fatalf("expected fitness_score=100, got %d", report.FitnessScore)
	}
	if len(report.Counters) != 1 {
		t.Fatalf("expected 1 counter result, got %d", len(report.Counters))
	}
	ctr := report.Counters[0]
	if ctr.Status != "ok" || ctr.Value == nil || *ctr.Value != 12500 || ctr.ActualUnit != "sheets" {
		t.Fatalf("unexpected counter result: %+v", ctr)
	}
}

func TestProfileTesterIncompatibleScannerDevice(t *testing.T) {
	// Scanner device does not have printer marker table
	client := mockTesterSNMPClient{
		walkResponses: map[string][]snmp.VarBind{
			"1.3.6.1.2.1.43.10.2.1.4": {}, // Empty / no marker rows
		},
	}

	prof := profile.Profile{
		ID: "brother-hl-l5100dn",
		Counters: map[string]profile.Counter{
			"marker_life": {
				ValueColumn:  "1.3.6.1.2.1.43.10.2.1.4",
				UnitColumn:   "1.3.6.1.2.1.43.10.2.1.3",
				Mode:         "walk",
				SemanticType: "marker_life",
				Unit:         "sheets",
			},
		},
	}

	tester := &ProfileTester{}
	report := tester.TestProfileWithClient(context.Background(), client, prof)

	if report.Compatible {
		t.Fatalf("expected compatible=false for device without marker table, got true")
	}
	if report.FitnessScore != 0 {
		t.Fatalf("expected fitness_score=0, got %d", report.FitnessScore)
	}
	if report.Counters[0].Status != "failed" {
		t.Fatalf("expected status=failed, got %s", report.Counters[0].Status)
	}
}

func TestProfileTesterBrotherScannerGETSuccess(t *testing.T) {
	client := mockTesterSNMPClient{
		getResponses: map[string][]snmp.VarBind{
			"1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3": {
				{OID: "1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3", Value: int64(8842)},
			},
		},
	}

	prof := profile.Profile{
		ID: "brother-ads",
		Counters: map[string]profile.Counter{
			"page_count": {
				ValueColumn:  "1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3",
				Mode:         "get",
				SemanticType: "marker_life",
				Unit:         "pages",
			},
		},
	}

	tester := &ProfileTester{}
	report := tester.TestProfileWithClient(context.Background(), client, prof)

	if !report.Compatible {
		t.Fatalf("expected compatible=true for brother-ads, got false: %s", report.Message)
	}
	if report.FitnessScore != 100 {
		t.Fatalf("expected fitness_score=100, got %d", report.FitnessScore)
	}
	if report.Counters[0].Status != "ok" || *report.Counters[0].Value != 8842 {
		t.Fatalf("unexpected counter result: %+v", report.Counters[0])
	}
}
