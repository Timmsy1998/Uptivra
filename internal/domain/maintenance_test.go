package domain

import (
	"testing"
	"time"
)

func TestWeeklyMaintenanceBoundaries(t *testing.T) {
	start := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	m := Maintenance{MonitorID: "one", Title: "Upgrade", StartsAt: start, EndsAt: start.Add(time.Hour), Weekly: true}
	if e := m.Validate(start.Add(-time.Hour)); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		at    time.Time
		shift time.Duration
	}{{start.Add(-time.Minute), 0}, {start, 0}, {m.EndsAt, 7 * 24 * time.Hour}, {start.Add(7 * 24 * time.Hour), 7 * 24 * time.Hour}, {start.Add(8 * 24 * time.Hour), 14 * 24 * time.Hour}} {
		got, ok := m.Occurrence(tc.at)
		if !ok || !got.StartsAt.Equal(start.Add(tc.shift)) {
			t.Fatal(tc, got)
		}
	}
	m.Weekly = false
	if _, ok := m.Occurrence(m.EndsAt); ok {
		t.Fatal("expired one-off")
	}
	m.Weekly = true
	m.EndsAt = start.Add(7 * 24 * time.Hour)
	if m.Validate(start.Add(-time.Hour)) == nil {
		t.Fatal("continuous recurrence accepted")
	}
}
