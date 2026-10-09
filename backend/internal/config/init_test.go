package config

import (
	"os"
	"path/filepath"
	"testing"

	"npms/backend/internal/security"
)

func TestInitConfigNewFile(t *testing.T) {
	tempDir := t.TempDir()
	targetConfig := filepath.Join(tempDir, "config.yaml")
	templateConfig := filepath.Join(tempDir, "config.example.yaml")

	exampleContent := `database:
  path: ./data/npms.db
server:
  listen: 127.0.0.1:8080
  allowed_origin: http://localhost:8080
  api_token: ""
security:
  encryption_key: replace-with-a-32-byte-base64-or-64-character-hex-key
polling:
  concurrency: 5
  status_interval: 5m
  counter_interval: 15m
report:
  timezone: UTC
retention:
  polling_runs_days: 30
  counter_events_days: 90
  jobs_days: 30
  cleanup_interval: 24h
logging:
  error_file: ./data/npms-errors.log
  daily: true
  max_size_mb: 10
profiles:
  path: ./profiles
`
	if err := os.WriteFile(templateConfig, []byte(exampleContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := InitConfig(InitOptions{
		Path:             targetConfig,
		TemplatePath:     templateConfig,
		Force:            false,
		GenerateAPIToken: true,
	})
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	if result.EncryptionKey == "" {
		t.Fatal("expected non-empty EncryptionKey")
	}
	if _, err := security.ParseKey(result.EncryptionKey); err != nil {
		t.Fatalf("generated EncryptionKey invalid: %v", err)
	}
	if result.APIToken == "" {
		t.Fatal("expected non-empty APIToken")
	}

	loaded, err := Load(targetConfig)
	if err != nil {
		t.Fatalf("Load initialized config failed: %v", err)
	}
	if loaded.Security.EncryptionKey != result.EncryptionKey {
		t.Fatalf("expected loaded key %q, got %q", result.EncryptionKey, loaded.Security.EncryptionKey)
	}
	if loaded.Server.APIToken != result.APIToken {
		t.Fatalf("expected loaded token %q, got %q", result.APIToken, loaded.Server.APIToken)
	}

	// Test force=false error when file exists
	_, err = InitConfig(InitOptions{
		Path:         targetConfig,
		TemplatePath: templateConfig,
		Force:        false,
	})
	if err == nil {
		t.Fatal("expected error when target file already exists and force=false")
	}

	// Test force=true overwriting
	result2, err := InitConfig(InitOptions{
		Path:         targetConfig,
		TemplatePath: templateConfig,
		Force:        true,
	})
	if err != nil {
		t.Fatalf("InitConfig with force=true failed: %v", err)
	}
	if result2.EncryptionKey == result.EncryptionKey {
		t.Fatal("expected new key generated on overwrite")
	}
}

func TestInitConfigWithoutTemplate(t *testing.T) {
	tempDir := t.TempDir()
	targetConfig := filepath.Join(tempDir, "sub", "config.yaml")

	result, err := InitConfig(InitOptions{
		Path:         targetConfig,
		TemplatePath: filepath.Join(tempDir, "nonexistent.yaml"),
		Force:        false,
	})
	if err != nil {
		t.Fatalf("InitConfig without template failed: %v", err)
	}

	if result.EncryptionKey == "" {
		t.Fatal("expected generated encryption key")
	}

	loaded, err := Load(targetConfig)
	if err != nil {
		t.Fatalf("Load initialized config without template failed: %v", err)
	}
	if loaded.Security.EncryptionKey != result.EncryptionKey {
		t.Fatalf("mismatched encryption key")
	}
}
