package counter

import (
	"testing"
	"time"
)

func reading(value int64, at string, epoch string) Reading {
	t, _ := time.Parse(time.RFC3339, at)
	return Reading{RawValue: value, CollectedAt: t, Quality: QualityValid, EpochID: epoch}
}

func TestEvaluateMonotonicAndEqual(t *testing.T) {
	previous := reading(100, "2026-01-01T00:00:00Z", "epoch-1")
	current := reading(125, "2026-01-01T00:15:00Z", "epoch-1")
	decision, err := Evaluate(&previous, current, Policy{MaxDelta: 100})
	if err != nil || decision.Delta == nil || *decision.Delta != 25 || decision.Quality != QualityValid {
		t.Fatalf("unexpected increase decision: %#v, %v", decision, err)
	}
	equal, err := Evaluate(&previous, reading(100, "2026-01-01T00:15:00Z", "epoch-1"), Policy{})
	if err != nil || equal.Delta == nil || *equal.Delta != 0 {
		t.Fatalf("expected zero delta: %#v, %v", equal, err)
	}
}

func TestEvaluateResetAndSpikeNeverProduceDelta(t *testing.T) {
	previous := reading(100, "2026-01-01T00:00:00Z", "epoch-1")
	reset, err := Evaluate(&previous, reading(20, "2026-01-01T00:15:00Z", "epoch-1"), Policy{})
	if err != nil || reset.Delta != nil || reset.EventType != "reset_suspected" || reset.Quality != QualitySuspicious {
		t.Fatalf("unexpected reset decision: %#v, %v", reset, err)
	}
	spike, err := Evaluate(&previous, reading(500, "2026-01-01T00:15:00Z", "epoch-1"), Policy{MaxDelta: 100})
	if err != nil || spike.Delta != nil || spike.EventType != "spike_suspected" {
		t.Fatalf("unexpected spike decision: %#v, %v", spike, err)
	}
}

func TestEvaluateEpochAndUnavailableDoNotCreateDelta(t *testing.T) {
	previous := reading(100, "2026-01-01T00:00:00Z", "epoch-1")
	epoch, err := Evaluate(&previous, reading(120, "2026-01-01T00:15:00Z", "epoch-2"), Policy{})
	if err != nil || epoch.Delta != nil || epoch.EventType != "epoch_changed" {
		t.Fatalf("unexpected epoch decision: %#v, %v", epoch, err)
	}
	current := reading(120, "2026-01-01T00:15:00Z", "epoch-1")
	current.Quality = QualityUnavailable
	unavailable, err := Evaluate(&previous, current, Policy{})
	if err != nil || unavailable.Delta != nil || unavailable.Quality != QualityUnavailable {
		t.Fatalf("unexpected unavailable decision: %#v, %v", unavailable, err)
	}
}

func TestAllocateDailyUsesLocalTimezoneAndEstimatedQuality(t *testing.T) {
	location := time.FixedZone("ICT", 7*60*60)
	start := time.Date(2026, 1, 1, 23, 45, 0, 0, location)
	end := time.Date(2026, 1, 2, 0, 15, 0, 0, location)
	allocations, err := AllocateDaily(start, end, 10, location)
	if err != nil || len(allocations) != 2 {
		t.Fatalf("unexpected allocations: %#v, %v", allocations, err)
	}
	if allocations[0].LocalDate != "2026-01-01" || allocations[1].LocalDate != "2026-01-02" || allocations[0].Delta+allocations[1].Delta != 10 {
		t.Fatalf("unexpected daily split: %#v", allocations)
	}
	if allocations[0].Quality != QualityUnverified || allocations[1].Quality != QualityUnverified {
		t.Fatalf("expected estimated quality: %#v", allocations)
	}
}
