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
