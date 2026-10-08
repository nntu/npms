package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWritesOnlyErrorsToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "errors.jsonl")
	closeLog, err := Setup(path, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("not persisted")
	slog.Error("persisted", "device_id", "device-1")
	if err := closeLog(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), "errors-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one daily log file, got %v", files)
	}
	contents, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "persisted") || strings.Contains(text, "not persisted") {
		t.Fatalf("unexpected error log content: %s", text)
	}
}

func TestRotatingWriterSplitsLargeFile(t *testing.T) {
	base := filepath.Join(t.TempDir(), "errors.log")
	writer, err := newRotatingWriter(base, 8, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("67890")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(base), "errors*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected two rotated files, got %v", files)
	}
}
