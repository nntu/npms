package ingestion

import (
	"context"
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
