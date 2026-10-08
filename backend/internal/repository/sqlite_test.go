package repository

import (
	"testing"

	"npms/backend/internal/counter"
)

func TestDeviceValidation(t *testing.T) {
	if err := (Device{DisplayName: "Printer", Status: "unknown"}).Validate(); err == nil {
		t.Fatal("expected missing id error")
	}
	if err := (Device{ID: "device-1", Status: "unknown"}).Validate(); err == nil {
		t.Fatal("expected missing display name error")
	}
	if err := (Device{ID: "device-1", DisplayName: "Printer", Status: "unknown"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryQualityValidation(t *testing.T) {
	for _, quality := range []counter.Quality{counter.QualityValid, counter.QualityUnverified, counter.QualityUnsupported, counter.QualityUnavailable, counter.QualitySuspicious} {
		if !validQuality(quality) {
			t.Fatalf("quality should be accepted: %q", quality)
		}
	}
}
