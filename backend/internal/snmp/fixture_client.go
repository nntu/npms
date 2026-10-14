package snmp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// FixtureClient replays a captured or synthetic PrinterMIBSnapshot as an in-memory snmp.Client.
// It also records GetCalls and WalkCalls to measure SNMP packet/round-trip optimizations.
type FixtureClient struct {
	Snapshot  PrinterMIBSnapshot
	VarBinds  []VarBind
	GetCalls  [][]string
	WalkCalls []string
}

// LoadFixtureSnapshot reads a PrinterMIBSnapshot JSON file from disk.
func LoadFixtureSnapshot(path string) (PrinterMIBSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PrinterMIBSnapshot{}, fmt.Errorf("read fixture %q: %w", path, err)
	}
	var snapshot PrinterMIBSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return PrinterMIBSnapshot{}, fmt.Errorf("decode fixture %q: %w", path, err)
	}
	return snapshot, nil
}

// LoadFixtureClient loads a JSON fixture file and returns a replayable FixtureClient.
func LoadFixtureClient(path string) (*FixtureClient, error) {
	snapshot, err := LoadFixtureSnapshot(path)
	if err != nil {
		return nil, err
	}
	return NewFixtureClient(snapshot), nil
}

// NewFixtureClient creates a replayable SNMP client from a PrinterMIBSnapshot.
func NewFixtureClient(snapshot PrinterMIBSnapshot) *FixtureClient {
	binds := make([]VarBind, 0, len(snapshot.DecodedOIDs))
	for _, d := range snapshot.DecodedOIDs {
		binds = append(binds, VarBind{
			OID:   strings.TrimPrefix(strings.TrimSpace(d.OID), "."),
			Type:  d.Type,
			Value: d.Value,
		})
	}
	sort.Slice(binds, func(i, j int) bool {
		return compareOID(binds[i].OID, binds[j].OID) < 0
	})
	return &FixtureClient{
		Snapshot: snapshot,
		VarBinds: binds,
	}
}

func (f *FixtureClient) Get(ctx context.Context, oids []string) ([]VarBind, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	copied := append([]string(nil), oids...)
	f.GetCalls = append(f.GetCalls, copied)

	byOID := make(map[string]VarBind, len(f.VarBinds))
	for _, vb := range f.VarBinds {
		byOID[strings.TrimPrefix(vb.OID, ".")] = vb
	}

	result := make([]VarBind, 0, len(oids))
	for _, req := range oids {
		clean := strings.TrimPrefix(strings.TrimSpace(req), ".")
		if vb, ok := byOID[clean]; ok {
			result = append(result, vb)
		} else {
			result = append(result, VarBind{
				OID:   clean,
				Type:  "NoSuchInstance",
				Value: nil,
			})
		}
	}
	return result, nil
}

func (f *FixtureClient) Walk(ctx context.Context, baseOID string) ([]VarBind, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cleanBase := strings.TrimPrefix(strings.TrimSpace(baseOID), ".")
	f.WalkCalls = append(f.WalkCalls, cleanBase)

	prefix := cleanBase + "."
	result := make([]VarBind, 0)
	for _, vb := range f.VarBinds {
		cleanOID := strings.TrimPrefix(vb.OID, ".")
		if strings.HasPrefix(cleanOID, prefix) {
			result = append(result, vb)
		}
	}
	return result, nil
}

func (f *FixtureClient) Close() error { return nil }

// TotalRequests returns the total number of SNMP operations (Get + Walk) issued to this client.
func (f *FixtureClient) TotalRequests() int {
	return len(f.GetCalls) + len(f.WalkCalls)
}

func compareOID(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(a, "."), ".")
	pb := strings.Split(strings.TrimPrefix(b, "."), ".")
	limit := len(pa)
	if len(pb) < limit {
		limit = len(pb)
	}
	for i := 0; i < limit; i++ {
		na, errA := strconv.Atoi(pa[i])
		nb, errB := strconv.Atoi(pb[i])
		if errA == nil && errB == nil {
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
		} else if pa[i] != pb[i] {
			return strings.Compare(pa[i], pb[i])
		}
	}
	if len(pa) < len(pb) {
		return -1
	}
	if len(pa) > len(pb) {
		return 1
	}
	return 0
}
