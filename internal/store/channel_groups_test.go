package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"uptivra/internal/domain"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestChannelGroupRouting(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	for _, kind := range []string{"discord", "webhook"} {
		for _, group := range []*string{nil, ptr("Bots"), ptr("Web"), ptr(""), ptr("bots"), ptr("Missing")} {
			name := kind + ":all"
			if group != nil {
				name = kind + ":" + *group
			}
			if err := s.CreateChannel(ctx, store.Channel{Name: name, Kind: kind, Group: group, URL: "https://example.com/hooks", SigningSecret: strings.Repeat("s", 32)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, group := range []string{"Bots", "Web", ""} {
		m := monitor([]string{"bot", "web", "ungrouped"}[i])
		m.Group, m.FailureThreshold, m.RecoveryThreshold = group, 1, 1
		if err := s.SaveMonitor(ctx, m, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, ok := range []bool{false, true} {
		if _, err := s.DB.Exec(ctx, `UPDATE monitors SET next_check=now()`); err != nil {
			t.Fatal(err)
		}
		ms, err := s.Claim(ctx)
		if err != nil || len(ms) != 3 {
			t.Fatalf("claim %d: %v", len(ms), err)
		}
		for _, m := range ms {
			if err := s.Record(ctx, m, domain.Check{At: time.Now(), OK: ok, Code: 200}); err != nil {
				t.Fatal(err)
			}
			// Replayed/fenced results must not create duplicate notifications.
			if err := s.Record(ctx, m, domain.Check{At: time.Now(), OK: ok, Code: 200}); err != nil {
				t.Fatal(err)
			}
		}
	}
	var total, mismatched int
	if err := s.DB.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE c.monitor_group IS NOT NULL AND c.monitor_group<>m.config->>'group') FROM deliveries d JOIN channels c ON c.id=d.channel_id JOIN monitors m ON m.id=d.payload->>'monitor_id'`).Scan(&total, &mismatched); err != nil {
		t.Fatal(err)
	}
	if total != 24 || mismatched != 0 {
		t.Fatalf("deliveries=%d mismatched=%d", total, mismatched)
	}
	channels, err := s.Channels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := channels[0]
	var before string
	if err := s.DB.QueryRow(ctx, `SELECT secret_config FROM channels WHERE id=$1`, c.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, group := range []*string{ptr("Missing"), ptr(""), nil} {
		if err := s.UpdateChannelGroup(ctx, c.ID, group); err != nil {
			t.Fatal(err)
		}
		var after string
		var saved *string
		if err := s.DB.QueryRow(ctx, `SELECT secret_config,monitor_group FROM channels WHERE id=$1`, c.ID).Scan(&after, &saved); err != nil {
			t.Fatal(err)
		}
		if before != after || (group == nil) != (saved == nil) || (group != nil && *group != *saved) {
			t.Fatal("routing update changed secrets or failed to save group")
		}
	}
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM deliveries`).Scan(&total); err != nil || total != 24 {
		t.Fatal("routing edit changed queued deliveries", err, total)
	}
	if err := s.UpdateChannelGroup(ctx, "absent", nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("missing channel", err)
	}
	long := strings.Repeat("é", 41)
	if err := s.UpdateChannelGroup(ctx, c.ID, &long); err == nil {
		t.Fatal("oversized group accepted")
	}
	if err := s.CreateChannel(ctx, store.Channel{Name: "Invalid", Kind: "discord", URL: "https://example.com", Group: &long}); err == nil {
		t.Fatal("oversized group created")
	}
}

func ptr(s string) *string { return &s }

func TestChannelGroupMigrationPreservesLegacyDestinations(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	if err := s.CreateChannel(ctx, store.Channel{Name: "Legacy", Kind: "discord", URL: "https://example.com/hook"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `ALTER TABLE channels DROP COLUMN monitor_group; DELETE FROM schema_versions WHERE version=3`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	cs, err := s.Channels(ctx)
	if err != nil || len(cs) != 1 || cs[0].Group != nil {
		t.Fatalf("legacy channels: %+v %v", cs, err)
	}
	var encrypted string
	if err := s.DB.QueryRow(ctx, `SELECT secret_config FROM channels`).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	plain, err := s.Box.Open(encrypted)
	if err != nil || !strings.Contains(plain, "https://example.com/hook") {
		t.Fatal("migration changed destination", err)
	}
}
