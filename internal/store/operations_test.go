package store_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestDeliveryDiagnosticsAndRetryFencing(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	if e := s.CreateChannel(ctx, store.Channel{Name: "Operations", Kind: "webhook", URL: "https://example.com/private", SigningSecret: strings.Repeat("s", 32)}); e != nil {
		t.Fatal(e)
	}
	cs, e := s.Channels(ctx)
	if e != nil {
		t.Fatal(e)
	}
	id, e := s.TestChannel(ctx, cs[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.TestChannel(ctx, "missing"); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal(e)
	}
	h, e := s.SchedulerHealth(ctx)
	if e != nil || h.Pending != 1 || h.LastRun != nil {
		t.Fatal(h, e)
	}
	for i := 0; i < 8; i++ {
		_, e = s.DB.Exec(ctx, `UPDATE deliveries SET next_attempt=now() WHERE id=$1`, id)
		if e != nil {
			t.Fatal(e)
		}
		ds, e := s.ClaimDeliveries(ctx)
		if e != nil || len(ds) != 1 {
			t.Fatal(ds, e)
		}
		if e = s.FinishDeliveryResult(ctx, ds[0], false, "http_503"); e != nil {
			t.Fatal(e)
		}
		if i == 7 {
			failed, e := s.Deliveries(ctx, "failed")
			if e != nil || len(failed) != 1 || failed[0].Attempts != 8 || failed[0].Outcome != "http_503" || failed[0].LastAttemptAt == nil {
				t.Fatal(failed, e)
			}
			if e = s.RetryDelivery(ctx, id); e != nil {
				t.Fatal(e)
			}
			if e = s.RetryDelivery(ctx, id); !errors.Is(e, pgx.ErrNoRows) {
				t.Fatal("duplicate retry", e)
			}
			if e = s.FinishDeliveryResult(ctx, ds[0], true, "delivered"); e != nil {
				t.Fatal(e)
			}
		}
	}
	pending, e := s.Deliveries(ctx, "pending")
	if e != nil || len(pending) != 1 || pending[0].Attempts != 0 {
		t.Fatal(pending, e)
	}
	ds, e := s.ClaimDeliveries(ctx)
	if e != nil || len(ds) != 1 {
		t.Fatal(ds, e)
	}
	if e = s.FinishDeliveryResult(ctx, ds[0], true, "delivered"); e != nil {
		t.Fatal(e)
	}
	delivered, e := s.Deliveries(ctx, "delivered")
	if e != nil || len(delivered) != 1 {
		t.Fatal(delivered, e)
	}
	if e = s.RetryDelivery(ctx, id); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("retried success", e)
	}
	run, e := s.StartSchedulerRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	h, e = s.SchedulerHealth(ctx)
	if e != nil || h.LastRun == nil || h.LastRun.FinishedAt != nil {
		t.Fatal(h, e)
	}
	if e = s.FinishSchedulerRun(ctx, run, true, 2, 1, 1); e != nil {
		t.Fatal(e)
	}
	h, e = s.SchedulerHealth(ctx)
	if e != nil || h.Pending != 0 || h.LastRun.Success == nil || !*h.LastRun.Success || h.LastRun.Recorded != 1 {
		t.Fatal(h, e)
	}
}
