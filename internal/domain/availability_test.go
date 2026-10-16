package domain

import (
	"testing"
	"time"
)

func TestAvailabilityClipsSplitsAndSeparatesGaps(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := NewAvailability("one", 7, 30, now)
	a.AddCoverage(a.From.Add(-time.Minute), a.From.Add(time.Minute), true)
	midnight := now.Truncate(24 * time.Hour)
	a.AddCoverage(midnight.Add(-time.Minute), midnight.Add(time.Minute), false)
	a.AddCoverage(now.Add(-time.Minute), now.Add(time.Hour), true)
	a.Finish()
	if len(a.Daily) != 7 || a.CoveredSeconds != 240 || a.UpSeconds != 120 || a.Uptime == nil || *a.Uptime != 50 {
		t.Fatalf("unexpected coverage: %+v", a)
	}
	if a.Daily[5].CoveredSeconds != 60 || a.Daily[6].CoveredSeconds != 120 || a.MissingSeconds != a.PeriodSeconds-240 {
		t.Fatal(a)
	}
	if a.Daily[1].Uptime != nil {
		t.Fatal("missing treated as measured")
	}
	empty := NewAvailability("empty", 90, 30, now)
	empty.Finish()
	if empty.Uptime != nil || empty.MissingSeconds != empty.PeriodSeconds || len(empty.Daily) != 90 {
		t.Fatal(empty)
	}
}
