package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"uptivra/internal/domain"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestIncidentUpdateTimelineAndRouting(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"public", "private"} {
		m := domain.Monitor{ID: id, Name: id, Group: "Bots", Type: "heartbeat", Interval: 60, Grace: 30, Enabled: true, Public: id == "public", Authorized: true, FailureThreshold: 1, RecoveryThreshold: 1}
		if err := s.SaveMonitor(ctx, m, true); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB.Exec(ctx, `INSERT INTO incidents(id,monitor_id) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []*string{nil, ptr("Bots"), ptr("Other")} {
		if err := s.CreateChannel(ctx, store.Channel{Name: "hook", Kind: "discord", Group: g, URL: "https://example.com/hook"}); err != nil {
			t.Fatal(err)
		}
	}
	input := domain.IncidentUpdateInput{Status: "identified", Message: "  <script>private text is literal</script>\nFix in progress  "}
	u, err := s.AddIncidentUpdate(ctx, "public", input)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.CreatedAt.IsZero() || u.Message != strings.TrimSpace(input.Message) {
		t.Fatal(u)
	}
	if _, err = s.AddIncidentUpdate(ctx, "private", input); err != nil {
		t.Fatal(err)
	}
	public, err := s.Incidents(ctx, true)
	if err != nil || len(public) != 1 || len(public[0].Updates) != 1 || public[0].Updates[0] != u || public[0].ResolvedAt != nil {
		t.Fatal(public, err)
	}
	all, err := s.Incidents(ctx, false)
	if err != nil || len(all) != 2 {
		t.Fatal(all, err)
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM deliveries`).Scan(&count); err != nil || count != 4 {
		t.Fatal("group routing", count, err)
	}
	rows, err := s.DB.Query(ctx, `SELECT payload FROM deliveries`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var e domain.IncidentEvent
		if err = json.Unmarshal(raw, &e); err != nil || e.Type != "incident.updated" || e.Group != "Bots" || e.Update == nil || e.Update.Message != u.Message {
			t.Fatal(e, err)
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if _, err = s.AddIncidentUpdate(ctx, "missing", input); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	// Recovery does not prevent appending a post-incident explanation.
	if _, err = s.DB.Exec(ctx, `UPDATE incidents SET resolved_at=now() WHERE id='public'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddIncidentUpdate(ctx, "public", domain.IncidentUpdateInput{Status: "update", Message: "Fix confirmed"}); err != nil {
		t.Fatal(err)
	}
	// Roll back the note when enqueueing fails.
	if _, err = s.DB.Exec(ctx, `ALTER TABLE deliveries ADD CONSTRAINT reject_test CHECK (false) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddIncidentUpdate(ctx, "public", input); err == nil {
		t.Fatal("expected delivery failure")
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM incident_updates WHERE incident_id='public'`).Scan(&count); err != nil || count != 2 {
		t.Fatal("atomic rollback", count, err)
	}
}

func TestConcurrentIncidentUpdateLimit(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := domain.Monitor{ID: "bot", Name: "Bot", Type: "heartbeat", Interval: 60, Grace: 30, Enabled: true, Authorized: true, FailureThreshold: 1, RecoveryThreshold: 1}
	if err := s.SaveMonitor(ctx, m, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO incidents(id,monitor_id) VALUES('incident','bot'); INSERT INTO incident_updates(id,incident_id,status,message) SELECT 'note-'||n,'incident','update','Existing note' FROM generate_series(1,99) n`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			_, err := s.AddIncidentUpdate(ctx, "incident", domain.IncidentUpdateInput{Status: "monitoring", Message: "Watching fix"})
			results <- err
		})
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, store.ErrIncidentUpdateLimit) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal("limit race", successes)
	}
}
