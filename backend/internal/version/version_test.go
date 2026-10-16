package version_test

import (
	"testing"

	"npms/backend/internal/version"
)

func TestGet(t *testing.T) {
	info := version.Get()
	if info.Version == "" {
		t.Fatalf("expected non-empty version")
	}
}
