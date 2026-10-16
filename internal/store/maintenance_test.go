package store_test

import (
	"context"
	"testing"
	"time"
	"uptivra/internal/domain"
	"uptivra/internal/testutil"
)

func TestMaintenanceClaimsFencingRecurrenceAndRedaction(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := monitor("one")
	m.Public = true
	if e := s.SaveMonitor(ctx, m, true); e != nil {
		t.Fatal(e)
	}
	ms, e := s.Claim(ctx)
	if e != nil || len(ms) != 1 {
		t.Fatal(ms, e)
	}
	start := time.Now().Add(time.Hour)
	id, e := s.CreateMaintenance(ctx, domain.Maintenance{MonitorID: m.ID, Title: "Database upgrade", StartsAt: start, EndsAt: start.Add(time.Hour), Weekly: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Record(ctx, ms[0], domain.Check{At: time.Now(), OK: false}); e != nil {
		t.Fatal(e)
	}
	h, e := s.History(ctx, m.ID)
	if e != nil || len(h) != 0 {
		t.Fatal("old lease recorded", h, e)
	}
	// Move the window into the current weekly occurrence, without relying on wall-clock sleeps.
	if _, e = s.DB.Exec(ctx, `UPDATE maintenance_windows SET starts_at=now()-interval '168 hours 30 minutes',ends_at=now()-interval '167 hours 30 minutes' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	ms, e = s.Claim(ctx)
	if e != nil || len(ms) != 0 {
		t.Fatal("claimed in maintenance", ms, e)
	}
	got, e := s.Monitors(ctx)
	if e != nil || got[0].Status != "maintenance" {
		t.Fatal(got, e)
	}
	windows, e := s.Maintenance(ctx, true)
	if e != nil || len(windows) != 1 || !windows[0].EndsAt.After(time.Now()) {
		t.Fatal(windows, e)
	}
	if _, e = s.DB.Exec(ctx, `UPDATE monitors SET public=false,next_check=now()-interval '1 hour'`); e != nil {
		t.Fatal(e)
	}
	windows, e = s.Maintenance(ctx, true)
	if e != nil || len(windows) != 0 {
		t.Fatal("private maintenance exposed", windows, e)
	}
	health, e := s.SchedulerHealth(ctx)
	if e != nil || health.Overdue != 0 {
		t.Fatal(health, e)
	}
	if e = s.DeleteMaintenance(ctx, id); e != nil {
		t.Fatal(e)
	}
	ms, e = s.Claim(ctx)
	if e != nil || len(ms) != 1 {
		t.Fatal("did not resume", ms, e)
	}
	// A window starts between Claim and Record: the recording transaction rechecks it.
	id, e = s.CreateMaintenance(ctx, domain.Maintenance{MonitorID: m.ID, Title: "New window", StartsAt: start, EndsAt: start.Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(ctx, `UPDATE monitors SET lease_token=$1,lease_until=now()+interval '90 seconds' WHERE id=$2`, ms[0].Lease, m.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(ctx, `UPDATE maintenance_windows SET starts_at=now()-interval '1 minute',ends_at=now()+interval '1 minute' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if e = s.Record(ctx, ms[0], domain.Check{At: time.Now()}); e != nil {
		t.Fatal(e)
	}
	h, e = s.History(ctx, m.ID)
	if e != nil || len(h) != 0 {
		t.Fatal("recorded in active window", h, e)
	}
}
