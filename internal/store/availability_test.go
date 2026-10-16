package store_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"
	"uptivra/internal/testutil"
)

func TestAvailabilityCadenceLegacyGapsIncidentsAndPublicAccess(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := monitor("history")
	m.Public = true
	if e := s.SaveMonitor(ctx, m, true); e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	for _, sample := range []struct {
		offset  int
		ok      bool
		cadence any
	}{{0, true, 60}, {30, false, 60}, {180, true, nil}, {300, true, 120}} {
		if _, e := s.DB.Exec(ctx, `INSERT INTO checks(monitor_id,checked_at,ok,latency_ms,status_code,reason,interval_seconds) VALUES($1,$2,$3,0,200,'',$4)`, m.ID, at.Add(time.Duration(sample.offset)*time.Second), sample.ok, sample.cadence); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.DB.Exec(ctx, `INSERT INTO incidents(id,monitor_id,opened_at,resolved_at) VALUES('resolved',$1,$2,$3),('ongoing',$1,$3,NULL)`, m.ID, at, at.Add(5*time.Minute)); e != nil {
		t.Fatal(e)
	}
	for _, days := range []int{7, 30, 90} {
		a, e := s.Availability(ctx, m.ID, days, true)
		if e != nil {
			t.Fatal(e)
		}
		if a.CoveredSeconds != 210 || a.UpSeconds != 150 || a.Uptime == nil || len(a.Daily) != days || a.MissingSeconds != a.PeriodSeconds-210 {
			t.Fatalf("wrong coverage: %+v", a)
		}
		if len(a.Incidents) != 2 || a.Incidents[1].DurationSeconds != 300 || a.Incidents[0].DurationSeconds < 3000 {
			t.Fatal(a.Incidents)
		}
	}
	if _, e := s.DB.Exec(ctx, `UPDATE monitors SET public=false WHERE id=$1`, m.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Availability(ctx, m.ID, 7, true); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("private history exposed", e)
	}
	if _, e := s.Availability(ctx, m.ID, 7, false); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Availability(ctx, m.ID, 8, false); e == nil {
		t.Fatal("invalid period accepted")
	}
}
