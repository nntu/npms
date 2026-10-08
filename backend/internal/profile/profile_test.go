package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	content := []byte("schema_version: 1\nid: test-profile\nversion: 1\nmanufacturer: test\nverification_status: experimental\nmatch:\n  model_patterns: ['^Test']\ncounters:\n  marker_life:\n    value_column: '1.2.3'\n    mode: get\n    semantic_type: marker_life\n    scope: engine\n    unit: impressions\n")
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ID != "test-profile" {
		t.Fatalf("unexpected profiles: %#v", profiles)
	}
}

func TestRepositoryProfilesLoad(t *testing.T) {
	profiles, err := LoadDir("../../profiles")
	if err != nil {
		t.Fatalf("load repository profiles: %v", err)
	}
	if len(profiles) < 5 {
		t.Fatalf("expected generic plus model profiles, got %d", len(profiles))
	}
}

const validYAML = `schema_version: 1
id: generic-printer-mib
version: 1
manufacturer: generic
verification_status: unverified
match:
  sys_object_id_prefixes: []
  model_patterns: []
counters:
  marker_life:
    value_column: "1.3.6.1.2.1.43.10.2.1.4"
    unit_column: "1.3.6.1.2.1.43.10.2.1.3"
    mode: walk
    selection: validated_marker_rows
    aggregation: none
    semantic_type: marker_life
    scope: engine
    unit: impressions
    require_unit_validation: true
`

func TestLoadValidProfile(t *testing.T) {
	p, err := Load(strings.NewReader(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "generic-printer-mib" || p.Counters["marker_life"].UnitColumn == "" {
		t.Fatalf("unexpected profile: %#v", p)
	}
}

func TestResolverPrecedenceAndAmbiguity(t *testing.T) {
	base, err := Load(strings.NewReader(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	exact := base
	exact.ID = "exact"
	exact.Match.SysObjectIDs = []string{"1.2.3"}
	exact.Manufacturer = "hp"
	vendor := base
	vendor.ID = "vendor"
	vendor.Manufacturer = "hp"
	vendor.Match.ModelPatterns = []string{"M501"}
	got, err := (Resolver{Profiles: []Profile{base, vendor, exact}}).Resolve(DeviceIdentity{SysObjectID: "1.2.3", Manufacturer: "hp", Model: "M501dn"}, "")
	if err != nil || got.ID != "exact" {
		t.Fatalf("expected exact match, got %q, %v", got.ID, err)
	}
	vendor.Match.ModelPatterns = []string{".*"}
	vendor2 := vendor
	vendor2.ID = "vendor-2"
	if _, err := (Resolver{Profiles: []Profile{vendor, vendor2}}).Resolve(DeviceIdentity{Manufacturer: "hp", Model: "M501dn"}, ""); err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	if _, err := Load(strings.NewReader(validYAML + "unknown: true\n")); err == nil {
		t.Fatal("expected unknown field error")
	}
}
