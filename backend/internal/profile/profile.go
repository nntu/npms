package profile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentSchemaVersion = 1

type VerificationStatus string

const (
	VerificationVerified     VerificationStatus = "verified"
	VerificationUnverified   VerificationStatus = "unverified"
	VerificationExperimental VerificationStatus = "experimental"
)

type Profile struct {
	SchemaVersion      int                `yaml:"schema_version"`
	ID                 string             `yaml:"id"`
	Version            int                `yaml:"version"`
	Manufacturer       string             `yaml:"manufacturer"`
	VerificationStatus VerificationStatus `yaml:"verification_status"`
	Match              MatchRules         `yaml:"match"`
	Counters           map[string]Counter `yaml:"counters"`
}

type MatchRules struct {
	SysObjectIDs        []string `yaml:"sys_object_ids"`
	SysObjectIDPrefixes []string `yaml:"sys_object_id_prefixes"`
	ModelPatterns       []string `yaml:"model_patterns"`
}

type Counter struct {
	ValueColumn           string `yaml:"value_column"`
	UnitColumn            string `yaml:"unit_column"`
	Mode                  string `yaml:"mode"`
	Selection             string `yaml:"selection"`
	Aggregation           string `yaml:"aggregation"`
	SemanticType          string `yaml:"semantic_type"`
	Scope                 string `yaml:"scope"`
	Unit                  string `yaml:"unit"`
	RequireUnitValidation bool   `yaml:"require_unit_validation"`
}

type DeviceIdentity struct {
	SysObjectID  string
	Manufacturer string
	Model        string
}

type Resolver struct{ Profiles []Profile }

func LoadDir(path string) ([]Profile, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile directory %q does not exist (please verify profiles.path in config.yaml or create the directory)", path)
		}
		return nil, fmt.Errorf("stat profile directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("profile path %q is not a directory", path)
	}
	profiles := make([]Profile, 0)
	err = filepath.WalkDir(path, func(filePath string, entry os.DirEntry, walkErr error) error {

		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml") {
			return nil
		}
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open profile %q: %w", filePath, err)
		}
		loaded, loadErr := Load(file)
		closeErr := file.Close()
		if loadErr != nil {
			return fmt.Errorf("load profile %q: %w", filePath, loadErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close profile %q: %w", filePath, closeErr)
		}
		profiles = append(profiles, loaded)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read profile directory %q: %w", path, err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("profile directory %q contains no YAML profiles", path)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func Load(r io.Reader) (Profile, error) {
	var p Profile
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Profile{}, errors.New("profile must contain exactly one YAML document")
		}
		return Profile{}, fmt.Errorf("decode trailing profile document: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (p Profile) Validate() error {
	if p.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("profile %q: unsupported schema_version %d", p.ID, p.SchemaVersion)
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`).MatchString(p.ID) {
		return fmt.Errorf("profile id must contain lowercase letters, digits, '.', '_' or '-'")
	}
	if p.Version < 1 {
		return fmt.Errorf("profile %q: version must be positive", p.ID)
	}
	if strings.TrimSpace(p.Manufacturer) == "" {
		return fmt.Errorf("profile %q: manufacturer is required", p.ID)
	}
	if p.VerificationStatus == "" {
		return fmt.Errorf("profile %q: verification_status is required", p.ID)
	}
	if p.VerificationStatus != VerificationVerified && p.VerificationStatus != VerificationUnverified && p.VerificationStatus != VerificationExperimental {
		return fmt.Errorf("profile %q: invalid verification_status %q", p.ID, p.VerificationStatus)
	}
	if len(p.Counters) == 0 {
		return fmt.Errorf("profile %q: at least one counter is required", p.ID)
	}
	for key, counter := range p.Counters {
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`).MatchString(key) {
			return fmt.Errorf("profile %q: invalid counter key %q", p.ID, key)
		}
		if counter.ValueColumn == "" || counter.Mode == "" || counter.SemanticType == "" || counter.Scope == "" || counter.Unit == "" {
			return fmt.Errorf("profile %q counter %q: value_column, mode, semantic_type, scope and unit are required", p.ID, key)
		}
		if counter.Mode != "get" && counter.Mode != "walk" {
			return fmt.Errorf("profile %q counter %q: mode must be get or walk", p.ID, key)
		}
		if counter.Mode == "walk" && counter.UnitColumn == "" && counter.RequireUnitValidation {
			return fmt.Errorf("profile %q counter %q: unit_column is required when unit validation is enabled", p.ID, key)
		}
	}
	for _, pattern := range append(append([]string{}, p.Match.SysObjectIDs...), append(p.Match.SysObjectIDPrefixes, p.Match.ModelPatterns...)...) {
		if pattern == "" {
			return fmt.Errorf("profile %q: match patterns cannot be empty", p.ID)
		}
	}
	for _, pattern := range p.Match.ModelPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("profile %q: invalid model pattern %q: %w", p.ID, pattern, err)
		}
	}
	return nil
}

func (r Resolver) Resolve(identity DeviceIdentity, explicitID string) (Profile, error) {
	seen := make(map[string]struct{}, len(r.Profiles))
	for _, p := range r.Profiles {
		if err := p.Validate(); err != nil {
			return Profile{}, err
		}
		if _, exists := seen[p.ID]; exists {
			return Profile{}, fmt.Errorf("duplicate profile id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
	}
	if explicitID != "" {
		for _, p := range r.Profiles {
			if p.ID == explicitID {
				return p, nil
			}
		}
		return Profile{}, fmt.Errorf("explicit profile %q was not found", explicitID)
	}
	for _, level := range []func(Profile) bool{
		func(p Profile) bool { return contains(p.Match.SysObjectIDs, identity.SysObjectID) },
		func(p Profile) bool { return prefixMatch(p.Match.SysObjectIDPrefixes, identity.SysObjectID) },
		func(p Profile) bool { return vendorModelMatch(p, identity) },
		func(p Profile) bool { return vendorGenericMatch(p, identity) },
		func(p Profile) bool {
			return p.Manufacturer == "generic" && len(p.Match.SysObjectIDs) == 0 && len(p.Match.SysObjectIDPrefixes) == 0 && len(p.Match.ModelPatterns) == 0
		},
	} {
		matches := make([]Profile, 0)
		for _, p := range r.Profiles {
			if level(p) {
				matches = append(matches, p)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
			ids := make([]string, len(matches))
			for i := range matches {
				ids[i] = matches[i].ID
			}
			return Profile{}, fmt.Errorf("ambiguous profile match: %s", strings.Join(ids, ", "))
		}
	}
	return Profile{}, errors.New("no matching profile")
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func prefixMatch(values []string, target string) bool {
	for _, value := range values {
		if strings.HasPrefix(target, value) {
			return true
		}
	}
	return false
}

func vendorModelMatch(p Profile, identity DeviceIdentity) bool {
	if !strings.EqualFold(p.Manufacturer, identity.Manufacturer) || len(p.Match.ModelPatterns) == 0 {
		return false
	}
	for _, pattern := range p.Match.ModelPatterns {
		if regexp.MustCompile(pattern).MatchString(identity.Model) {
			return true
		}
	}
	return false
}

func vendorGenericMatch(p Profile, identity DeviceIdentity) bool {
	return strings.EqualFold(p.Manufacturer, identity.Manufacturer) && len(p.Match.ModelPatterns) == 0 && len(p.Match.SysObjectIDs) == 0 && len(p.Match.SysObjectIDPrefixes) == 0 && p.Manufacturer != "generic"
}
