package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("database:\n  path: ./data/test.db\nserver:\n  listen: 127.0.0.1:8080\n  api_token: test-token\nsecurity:\n  encryption_key: key\npolling:\n  concurrency: 3\n  status_interval: 10s\n  counter_interval: 1m\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(loaded.Database.Path) != "test.db" || loaded.Polling.Concurrency != 3 {
		t.Fatalf("unexpected config: %+v", loaded)
	}
}

func TestRejectUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestRejectsExampleAPITokenPlaceholder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("server:\n  api_token: replace-with-a-random-api-token\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected placeholder API token to be rejected")
	}
}

func TestIANATimezoneValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("server:\n  api_token: test-token\nreport:\n  timezone: Asia/Ho_Chi_Minh\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected Asia/Ho_Chi_Minh timezone to be valid, got error: %v", err)
	}
	if cfg.Report.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("expected timezone Asia/Ho_Chi_Minh, got %s", cfg.Report.Timezone)
	}
}

func TestCustomSNMPAndDiscoveryConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte(`
database:
  path: ./data/test.db
server:
  listen: 127.0.0.1:8080
  api_token: test-token
security:
  encryption_key: key
snmp:
  default_port: 1161
  default_community: internal-read
  default_version: "2c"
  timeout_seconds: 5
  retries: 2
  max_repetitions: 30
discovery:
  rules_file: ./custom-discovery.yaml
  default_cidr: 10.141.6.0/24
  default_community: internal-read
  default_port: 1161
  allowed_cidrs:
    - 10.141.6.0/24
    - 10.141.7.0/24
  max_range_ips: 1024
  concurrency: 30
  timeout_ms: 1500
counter:
  max_delta: 5000
polling:
  concurrency: 10
  status_interval: 5m
  counter_interval: 2h
  max_attempts: 3
  retry_delay: 2s
  lease_duration: 60s
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if cfg.SNMP.DefaultPort != 1161 || cfg.SNMP.DefaultCommunity != "internal-read" || cfg.SNMP.TimeoutSeconds != 5 {
		t.Fatalf("unexpected snmp config: %+v", cfg.SNMP)
	}
	if cfg.Discovery.DefaultCIDR != "10.141.6.0/24" || cfg.Discovery.MaxRangeIPs != 1024 || len(cfg.Discovery.AllowedCIDRs) != 2 {
		t.Fatalf("unexpected discovery config: %+v", cfg.Discovery)
	}
	if cfg.Counter.MaxDelta != 5000 {
		t.Fatalf("unexpected counter config: %+v", cfg.Counter)
	}
	if cfg.Polling.MaxAttempts != 3 {
		t.Fatalf("unexpected polling max attempts: %d", cfg.Polling.MaxAttempts)
	}
	retryDelay, err := cfg.RetryDelay()
	if err != nil || retryDelay.Seconds() != 2 {
		t.Fatalf("unexpected retry delay: %v, err: %v", retryDelay, err)
	}
}

func TestInvalidDiscoveryCIDR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte(`
database:
  path: ./data/test.db
server:
  listen: 127.0.0.1:8080
  api_token: test-token
security:
  encryption_key: key
discovery:
  allowed_cidrs:
    - not-a-cidr
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid CIDR error, got nil")
	}
}
