package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"uptivra/internal/domain"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestHeartbeatLifecycle(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal("repeat migration", err)
	}
	m := domain.Monitor{ID: "bot", Name: "Bot", Type: "heartbeat", Interval: 60, Grace: 120, Enabled: true, Authorized: true, FailureThreshold: 2, RecoveryThreshold: 2}
	if err := s.SaveMonitor(ctx, m, true); err != nil {
		t.Fatal(err)
	}
	due, err := s.Claim(ctx)
	if err != nil || len(due) != 0 {
		t.Fatal("initial grace", err, due)
	}
	token, err := s.RotateHeartbeatToken(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	var hash string
	if err = s.DB.QueryRow(ctx, `SELECT token_hash FROM heartbeats WHERE monitor_id=$1`, m.ID).Scan(&hash); err != nil || hash == token || strings.Contains(hash, token) {
		t.Fatal("token not hashed", err)
	}
	if err = s.ReceiveHeartbeat(ctx, m.ID, "invalid"); !errors.Is(err, store.ErrHeartbeatToken) {
		t.Fatal(err)
	}
	if err = s.ReceiveHeartbeat(ctx, "other", token); !errors.Is(err, store.ErrHeartbeatToken) {
		t.Fatal("token crossed monitors", err)
	}
	record := func(receive bool) {
		t.Helper()
		if _, err = s.DB.Exec(ctx, `UPDATE monitors SET next_check=now() WHERE id=$1`, m.ID); err != nil {
			t.Fatal(err)
		}
		due, err = s.Claim(ctx)
		if err != nil || len(due) != 1 {
			t.Fatal("claim", err, len(due))
		}
		// Receipt after claim must be visible at recording, even when supplied result says false.
		if receive {
			if err = s.ReceiveHeartbeat(ctx, m.ID, token); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.Record(ctx, due[0], domain.Check{At: time.Now(), OK: !receive}); err != nil {
			t.Fatal(err)
		}
		if err = s.Record(ctx, due[0], domain.Check{At: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	record(true)
	got, err := s.Monitor(ctx, m.ID)
	if err != nil || got.Status != "healthy" || got.Latency != nil || got.P95 != nil {
		t.Fatal(got, err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE heartbeats SET last_received=now()-interval '181 seconds'`); err != nil {
		t.Fatal(err)
	}
	record(false)
	record(false)
	incidents, err := s.Incidents(ctx, false)
	if err != nil || len(incidents) != 1 || incidents[0].ResolvedAt != nil {
		t.Fatal(incidents, err)
	}
	record(true)
	record(true)
	incidents, err = s.Incidents(ctx, false)
	if err != nil || incidents[0].ResolvedAt == nil {
		t.Fatal("not recovered", err)
	}
	history, err := s.History(ctx, m.ID)
	if err != nil || len(history) != 5 || history[0].Reason != "heartbeat_received" || history[2].Reason != "heartbeat_missed" {
		t.Fatal(history, err)
	}
	old := token
	token, err = s.RotateHeartbeatToken(ctx, m.ID)
	if err != nil || token == old {
		t.Fatal(err)
	}
	if err = s.ReceiveHeartbeat(ctx, m.ID, old); !errors.Is(err, store.ErrHeartbeatToken) {
		t.Fatal("old token accepted", err)
	}
	m.Enabled = false
	if err = s.SaveMonitor(ctx, m, false); err != nil {
		t.Fatal(err)
	}
	if err = s.ReceiveHeartbeat(ctx, m.ID, token); !errors.Is(err, store.ErrHeartbeatToken) {
		t.Fatal("disabled accepted", err)
	}
	m.Enabled = true
	if err = s.SaveMonitor(ctx, m, false); err != nil {
		t.Fatal(err)
	}
	if err = s.ReceiveHeartbeat(ctx, m.ID, token); err != nil {
		t.Fatal("edit lost token", err)
	}
	m.Type = "http"
	m.URL = "https://example.com"
	m.Timeout = 5
	m.ExpectedStatus = 200
	if err = s.SaveMonitor(ctx, m, false); err == nil {
		t.Fatal("type mutation accepted")
	}
	if err = s.DeleteMonitor(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.ReceiveHeartbeat(ctx, m.ID, token); !errors.Is(err, store.ErrHeartbeatToken) {
		t.Fatal("deleted accepted", err)
	}
}
