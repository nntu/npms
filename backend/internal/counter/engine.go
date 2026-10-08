package counter

import (
	"fmt"
	"time"
)

type Quality string

const (
	QualityValid       Quality = "valid"
	QualityUnverified  Quality = "unverified"
	QualityUnsupported Quality = "unsupported"
	QualityUnavailable Quality = "unavailable"
	QualitySuspicious  Quality = "suspicious"
)

type Reading struct {
	RawValue    int64
	CollectedAt time.Time
	Quality     Quality
	EpochID     string
}

type Policy struct {
	// MaxDelta is the largest trusted increase between consecutive readings.
	// Zero disables spike detection.
	MaxDelta int64
}

type Decision struct {
	Delta     *int64
	Quality   Quality
	EventType string
	Reason    string
}

func Evaluate(previous *Reading, current Reading, policy Policy) (Decision, error) {
	if err := validateReading(current); err != nil {
		return Decision{}, err
	}
	if previous == nil {
		return Decision{Quality: current.Quality, Reason: "first reading establishes a baseline"}, nil
	}
	if err := validateReading(*previous); err != nil {
		return Decision{}, fmt.Errorf("previous reading: %w", err)
	}
	if !usable(previous.Quality) {
		return Decision{Quality: current.Quality, EventType: "baseline_untrusted", Reason: "previous reading quality is not trusted"}, nil
	}
	if !usable(current.Quality) {
		return Decision{Quality: current.Quality, Reason: "current reading quality is not trusted"}, nil
	}
	if previous.EpochID != current.EpochID {
		return Decision{Quality: current.Quality, EventType: "epoch_changed", Reason: "readings belong to different counter epochs"}, nil
	}
	if current.RawValue < previous.RawValue {
		return Decision{Quality: QualitySuspicious, EventType: "reset_suspected", Reason: "counter decreased"}, nil
	}
	delta := current.RawValue - previous.RawValue
	if policy.MaxDelta > 0 && delta > policy.MaxDelta {
		return Decision{Quality: QualitySuspicious, EventType: "spike_suspected", Reason: "delta exceeds configured maximum"}, nil
	}
	return Decision{Delta: &delta, Quality: current.Quality}, nil
}

func validateReading(reading Reading) error {
	if !validQuality(reading.Quality) {
		return fmt.Errorf("invalid quality %q", reading.Quality)
	}
	if reading.CollectedAt.IsZero() {
		return fmt.Errorf("collected_at is required")
	}
	return nil
}

func validQuality(quality Quality) bool {
	switch quality {
	case QualityValid, QualityUnverified, QualityUnsupported, QualityUnavailable, QualitySuspicious:
		return true
	default:
		return false
	}
}

func usable(quality Quality) bool {
	return quality == QualityValid || quality == QualityUnverified
}

type DailyAllocation struct {
	LocalDate string
	Delta     int64
	Quality   Quality
}

// AllocateDaily distributes a trusted interval across local calendar dates.
// A boundary-spanning interval is always marked estimated because snapshots do
// not prove the exact amount used on either side of midnight.
func AllocateDaily(start, end time.Time, delta int64, location *time.Location) ([]DailyAllocation, error) {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, fmt.Errorf("end must be after start")
	}
	if delta < 0 {
		return nil, fmt.Errorf("delta cannot be negative")
	}
	if location == nil {
		return nil, fmt.Errorf("location is required")
	}
	type segment struct {
		date     string
		duration time.Duration
	}
	segments := make([]segment, 0, 2)
	point := start
	for point.Before(end) {
		local := point.In(location)
		nextMidnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location)
		segmentEnd := end
		if nextMidnight.Before(segmentEnd) {
			segmentEnd = nextMidnight
		}
		segments = append(segments, segment{date: local.Format("2006-01-02"), duration: segmentEnd.Sub(point)})
		point = segmentEnd
	}
	if len(segments) == 1 {
		return []DailyAllocation{{LocalDate: segments[0].date, Delta: delta, Quality: QualityValid}}, nil
	}
	total := end.Sub(start)
	allocated := int64(0)
	result := make([]DailyAllocation, 0, len(segments))
	for i, part := range segments {
		amount := delta * int64(part.duration) / int64(total)
		if i == len(segments)-1 {
			amount = delta - allocated
		}
		allocated += amount
		result = append(result, DailyAllocation{LocalDate: part.date, Delta: amount, Quality: QualityUnverified})
	}
	return result, nil
}
