package ingestion

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
	"npms/backend/internal/snmp"
)

type fakeCounterClient struct {
	life, units []snmp.VarBind
}

func (f fakeCounterClient) Get(context.Context, []string) ([]snmp.VarBind, error) { return nil, nil }
func (f fakeCounterClient) Walk(_ context.Context, oid string) ([]snmp.VarBind, error) {
	if strings.TrimPrefix(oid, ".") == "1.2.3.4" {
		return f.life, nil
	}
	return f.units, nil
}
func (fakeCounterClient) Close() error { return nil }

func TestReadWalkCounterJoinsExactInstance(t *testing.T) {
	reading, err := readWalkCounter(context.Background(), fakeCounterClient{
		life:  []snmp.VarBind{{OID: ".1.2.3.4.7", Value: int64(321)}},
		units: []snmp.VarBind{{OID: ".1.2.3.5.7", Value: int64(3)}},
	}, repository.CounterDefinition{Key: "marker_life", OID: "1.2.3.4", UnitOID: "1.2.3.5", Mode: "walk", Selection: "validated_marker_rows"}, counter.QualityUnverified)
	if err != nil {
		t.Fatal(err)
	}
	if reading.RawValue != 321 || reading.Quality != counter.QualityUnverified {
		t.Fatalf("unexpected reading: %+v", reading)
	}
}

func TestReadWalkCounterRejectsAmbiguousRows(t *testing.T) {
	_, err := readWalkCounter(context.Background(), fakeCounterClient{
		life:  []snmp.VarBind{{OID: ".1.2.3.4.7", Value: int64(321)}, {OID: ".1.2.3.4.8", Value: int64(654)}},
		units: []snmp.VarBind{{OID: ".1.2.3.5.7", Value: int64(3)}, {OID: ".1.2.3.5.8", Value: int64(3)}},
	}, repository.CounterDefinition{Key: "marker_life", OID: "1.2.3.4", UnitOID: "1.2.3.5", Mode: "walk", Selection: "validated_marker_rows"}, counter.QualityUnverified)
	if err == nil || !strings.Contains(err.Error(), "explicit instance selection") {
		t.Fatalf("expected ambiguous marker error, got %v", err)
	}
}

func TestReadWalkCounterUsesConfiguredInstance(t *testing.T) {
	reading, err := readWalkCounter(context.Background(), fakeCounterClient{
		life:  []snmp.VarBind{{OID: ".1.2.3.4.7", Value: int64(321)}, {OID: ".1.2.3.4.8", Value: int64(654)}},
		units: []snmp.VarBind{{OID: ".1.2.3.5.7", Value: int64(3)}, {OID: ".1.2.3.5.8", Value: int64(3)}},
	}, repository.CounterDefinition{Key: "marker_life", OID: "1.2.3.4", UnitOID: "1.2.3.5", Instance: ".8", Mode: "walk", Selection: "validated_marker_rows"}, counter.QualityUnverified)
	if err != nil {
		t.Fatal(err)
	}
	if reading.RawValue != 654 || reading.EpochID != "8" {
		t.Fatalf("unexpected configured-instance reading: %+v", reading)
	}
}

func TestReadWalkCounterRejectsMissingConfiguredInstance(t *testing.T) {
	_, err := readWalkCounter(context.Background(), fakeCounterClient{
		life:  []snmp.VarBind{{OID: ".1.2.3.4.7", Value: int64(321)}},
		units: []snmp.VarBind{{OID: ".1.2.3.5.7", Value: int64(3)}},
	}, repository.CounterDefinition{Key: "marker_life", OID: "1.2.3.4", UnitOID: "1.2.3.5", Instance: "8", Mode: "walk", Selection: "validated_marker_rows"}, counter.QualityUnverified)
	if err == nil || !strings.Contains(err.Error(), "configured instance") {
		t.Fatalf("expected missing configured instance error, got %v", err)
	}
}

func TestReadWalkCounterFastPathGetOnFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "printer-mib", "brother-hl-l5100dn.json")
	client, err := snmp.LoadFixtureClient(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	reading, err := readWalkCounter(context.Background(), client, repository.CounterDefinition{
		Key:       "marker_life",
		OID:       snmp.MarkerLifeOID,
		UnitOID:   snmp.MarkerUnitOID,
		Instance:  "1.1",
		Mode:      "walk",
		Selection: "validated_marker_rows",
	}, counter.QualityUnverified)
	if err != nil {
		t.Fatal(err)
	}
	if reading.RawValue != 35890 || reading.EpochID != "1.1" {
		t.Fatalf("unexpected reading: %+v", reading)
	}
	if len(client.GetCalls) != 1 || len(client.WalkCalls) != 0 {
		t.Fatalf("expected fast-path 1 GET and 0 WALKs, got %d GETs and %d WALKs", len(client.GetCalls), len(client.WalkCalls))
	}
}

func TestReadWalkCounterMultiMarkerFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "printer-mib", "ricoh-im-2702-multi-marker.json")
	client, err := snmp.LoadFixtureClient(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = readWalkCounter(context.Background(), client, repository.CounterDefinition{
		Key:       "marker_life",
		OID:       snmp.MarkerLifeOID,
		UnitOID:   snmp.MarkerUnitOID,
		Mode:      "walk",
		Selection: "validated_marker_rows",
	}, counter.QualityUnverified)
	if err == nil || !strings.Contains(err.Error(), "explicit instance selection") {
		t.Fatalf("expected ambiguity error on multi-marker Ricoh fixture, got %v", err)
	}

	client2, err := snmp.LoadFixtureClient(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	reading, err := readWalkCounter(context.Background(), client2, repository.CounterDefinition{
		Key:       "marker_life",
		OID:       snmp.MarkerLifeOID,
		UnitOID:   snmp.MarkerUnitOID,
		Instance:  "1.2",
		Mode:      "walk",
		Selection: "validated_marker_rows",
	}, counter.QualityUnverified)
	if err != nil {
		t.Fatal(err)
	}
	if reading.RawValue != 96300 || reading.EpochID != "1.2" {
		t.Fatalf("unexpected reading for instance 1.2: %+v", reading)
	}
	if len(client2.GetCalls) != 1 || len(client2.WalkCalls) != 0 {
		t.Fatalf("expected fast-path 1 GET and 0 WALKs for instance 1.2, got %d GETs and %d WALKs", len(client2.GetCalls), len(client2.WalkCalls))
	}
}

type fallbackMockClient struct {
	getFunc  func(ctx context.Context, oids []string) ([]snmp.VarBind, error)
	walkFunc func(ctx context.Context, oid string) ([]snmp.VarBind, error)
}

func (m *fallbackMockClient) Connect() error { return nil }
func (m *fallbackMockClient) Close() error   { return nil }
func (m *fallbackMockClient) Get(ctx context.Context, oids []string) ([]snmp.VarBind, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, oids)
	}
	return nil, nil
}
func (m *fallbackMockClient) Walk(ctx context.Context, oid string) ([]snmp.VarBind, error) {
	if m.walkFunc != nil {
		return m.walkFunc(ctx, oid)
	}
	return nil, nil
}

func TestReadWalkCounterFallbackWhenUnitColumnMissing(t *testing.T) {
	mock := &fallbackMockClient{
		walkFunc: func(ctx context.Context, oid string) ([]snmp.VarBind, error) {
			if strings.Contains(oid, "43.10.2.1.4") {
				return []snmp.VarBind{{OID: "1.3.6.1.2.1.43.10.2.1.4.1.1", Value: int64(12345)}}, nil
			}
			return nil, nil // Unit column returns nothing
		},
	}

	reading, err := readWalkCounter(context.Background(), mock, repository.CounterDefinition{
		Key:       "marker_life",
		OID:       snmp.MarkerLifeOID,
		UnitOID:   snmp.MarkerUnitOID,
		Mode:      "walk",
		Selection: "validated_marker_rows",
	}, counter.QualityUnverified)
	if err != nil {
		t.Fatalf("unexpected error when unit column missing: %v", err)
	}
	if reading.RawValue != 12345 {
		t.Fatalf("expected RawValue 12345, got %d", reading.RawValue)
	}
}

func TestReadWalkCounterFallbackVendorOID(t *testing.T) {
	mock := &fallbackMockClient{
		walkFunc: func(ctx context.Context, oid string) ([]snmp.VarBind, error) {
			return nil, nil // Printer-MIB walk empty
		},
		getFunc: func(ctx context.Context, oids []string) ([]snmp.VarBind, error) {
			if len(oids) > 0 && oids[0] == "1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0" {
				return []snmp.VarBind{{OID: oids[0], Value: int64(67890)}}, nil
			}
			return nil, nil
		},
	}

	reading, err := readWalkCounter(context.Background(), mock, repository.CounterDefinition{
		Key:       "marker_life",
		OID:       snmp.MarkerLifeOID,
		UnitOID:   snmp.MarkerUnitOID,
		Mode:      "walk",
		Selection: "validated_marker_rows",
	}, counter.QualityUnverified)
	if err != nil {
		t.Fatalf("unexpected error on vendor fallback: %v", err)
	}
	if reading.RawValue != 67890 || reading.EpochID != "hp_engine_total" {
		t.Fatalf("expected RawValue 67890 with epoch hp_engine_total, got %+v", reading)
	}
}
