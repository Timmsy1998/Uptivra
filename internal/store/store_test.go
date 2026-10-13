package store_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"uptivra/internal/domain"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func monitor(id string) domain.Monitor {
	return domain.Monitor{ID: id, Name: "API", URL: "https://example.com", Interval: 60, Timeout: 5, ExpectedStatus: 200, FailureThreshold: 2, RecoveryThreshold: 2, Enabled: true, Authorized: true, Headers: map[string]string{"Authorization": "secret-value"}}
}
func TestClaimsIncidentsAndSecrets(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := monitor("one")
	if e := s.SaveMonitor(ctx, m, true); e != nil {
		t.Fatal(e)
	}
	var config, sec string
	if e := s.DB.QueryRow(ctx, `SELECT config::text,secrets FROM monitors`).Scan(&config, &sec); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(config+sec, "secret-value") {
		t.Fatal("plaintext header persisted")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := []domain.Monitor{}
	for range 8 {
		wg.Go(func() {
			ms, e := s.Claim(ctx)
			if e != nil {
				t.Error(e)
			}
			mu.Lock()
			claimed = append(claimed, ms...)
			mu.Unlock()
		})
	}
	wg.Wait()
	if len(claimed) != 1 {
		t.Fatalf("claimed %d times", len(claimed))
	}
	first := claimed[0]
	c := domain.Check{At: time.Now(), Reason: "status_mismatch", Code: 500}
	if e := s.Record(ctx, first, c); e != nil {
		t.Fatal(e)
	}
	if e := s.Record(ctx, first, c); e != nil {
		t.Fatal(e)
	}
	h, _ := s.History(ctx, m.ID)
	if len(h) != 1 {
		t.Fatal("duplicate result")
	}
	if e := s.CreateChannel(ctx, store.Channel{Name: "Ops", Kind: "webhook", URL: "https://example.com/hook", SigningSecret: strings.Repeat("s", 32)}); e != nil {
		t.Fatal(e)
	}
	run := func(ok bool) {
		t.Helper()
		s.DB.Exec(ctx, `UPDATE monitors SET next_check=now()`)
		ms, e := s.Claim(ctx)
		if e != nil || len(ms) != 1 {
			t.Fatalf("claim: %v %d", e, len(ms))
		}
		if e = s.Record(ctx, ms[0], domain.Check{At: time.Now(), OK: ok, Code: 200}); e != nil {
			t.Fatal(e)
		}
	}
	run(false)
	run(false)
	inc, _ := s.Incidents(ctx, false)
	if len(inc) != 1 || inc[0].ResolvedAt != nil {
		t.Fatal("incident not uniquely opened")
	}
	run(true)
	inc, _ = s.Incidents(ctx, false)
	if inc[0].ResolvedAt != nil {
		t.Fatal("recovered below threshold")
	}
	run(true)
	inc, _ = s.Incidents(ctx, false)
	if inc[0].ResolvedAt == nil {
		t.Fatal("not resolved")
	}
	ds, e := s.ClaimDeliveries(ctx)
	if e != nil || len(ds) != 2 {
		t.Fatalf("outbox %d %v", len(ds), e)
	}
	again, _ := s.ClaimDeliveries(ctx)
	if len(again) != 0 {
		t.Fatal("duplicate delivery claims")
	}
	if e = s.FinishDelivery(ctx, ds[0], false); e != nil {
		t.Fatal(e)
	}
	var attempts int
	var delivered *time.Time
	s.DB.QueryRow(ctx, `SELECT attempts,delivered_at FROM deliveries WHERE id=$1`, ds[0].ID).Scan(&attempts, &delivered)
	if attempts != 1 || delivered != nil {
		t.Fatal("retry state")
	}
	// Editing invalidates an in-flight claim, and secrets round-trip through encryption.
	s.DB.Exec(ctx, `UPDATE monitors SET next_check=now()`)
	ms, _ := s.Claim(ctx)
	m.Name = "Renamed"
	if e = s.SaveMonitor(ctx, m, false); e != nil {
		t.Fatal(e)
	}
	if e = s.Record(ctx, ms[0], domain.Check{At: time.Now(), OK: false}); e != nil {
		t.Fatal(e)
	}
	got, _ := s.Monitor(ctx, m.ID)
	if got.Headers["Authorization"] != "secret-value" || got.Failures != 0 {
		t.Fatal("stale claim or secret corruption")
	}
	if e = s.DeleteMonitor(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	h, _ = s.History(ctx, m.ID)
	if len(h) != 0 {
		t.Fatal("history not cascaded")
	}
}
func TestAuth(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	password := "test-password-long-enough"
	if e := s.Bootstrap(ctx, "admin@example.com", password); e != nil {
		t.Fatal(e)
	}
	if e := s.Bootstrap(ctx, "other@example.com", password); e == nil {
		t.Fatal("second administrator")
	}
	if _, e := s.Login(ctx, "admin@example.com", "incorrect"); e == nil {
		t.Fatal("bad login")
	}
	token, e := s.Login(ctx, "admin@example.com", password)
	if e != nil || !s.Session(ctx, token) {
		t.Fatal("session", e)
	}
	key, e := s.CreateKey(ctx, "CI", "read")
	if e != nil || s.KeyScope(ctx, key) != "read" {
		t.Fatal("key", e)
	}
	if e = s.ChangePassword(ctx, password, "another-long-password"); e != nil {
		t.Fatal(e)
	}
	if s.Session(ctx, token) {
		t.Fatal("sessions survived password change")
	}
	for range 3 {
		if !s.RateLimit(ctx, "login", 3) {
			t.Fatal("early throttle")
		}
	}
	if s.RateLimit(ctx, "login", 3) {
		t.Fatal("missing throttle")
	}
}

func TestExpiredClaimsAndTerminalDelivery(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := monitor("lease")
	m.FailureThreshold = 1
	if err := s.SaveMonitor(ctx, m, true); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChannel(ctx, store.Channel{Name: "Ops", Kind: "webhook", URL: "https://example.com/hook", SigningSecret: strings.Repeat("s", 32)}); err != nil {
		t.Fatal(err)
	}
	old, err := s.Claim(ctx)
	if err != nil || len(old) != 1 {
		t.Fatalf("claim: %v %v", old, err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE monitors SET lease_until=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.Claim(ctx)
	if err != nil || len(fresh) != 1 || fresh[0].Lease == old[0].Lease {
		t.Fatalf("reclaim: %v %v", fresh, err)
	}
	if err = s.Record(ctx, old[0], domain.Check{At: time.Now(), OK: true}); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, m.ID)
	if err != nil || len(history) != 0 {
		t.Fatal("expired worker persisted a result", err)
	}
	if err = s.Record(ctx, fresh[0], domain.Check{At: time.Now(), OK: false}); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 8; attempt++ {
		deliveries, err := s.ClaimDeliveries(ctx)
		if err != nil || len(deliveries) != 1 {
			t.Fatalf("attempt %d: %v %v", attempt, deliveries, err)
		}
		if err = s.FinishDelivery(ctx, deliveries[0], false); err != nil {
			t.Fatal(err)
		}
		// A duplicate completion must not increment attempts or alter the retry deadline.
		if err = s.FinishDelivery(ctx, deliveries[0], true); err != nil {
			t.Fatal(err)
		}
		var attempts int
		var failed, delivered *time.Time
		if err = s.DB.QueryRow(ctx, `SELECT attempts,failed_at,delivered_at FROM deliveries`).Scan(&attempts, &failed, &delivered); err != nil {
			t.Fatal(err)
		}
		if attempts != attempt || delivered != nil || (failed != nil) != (attempt == 8) {
			t.Fatalf("attempt %d: attempts=%d failed=%v delivered=%v", attempt, attempts, failed, delivered)
		}
		if _, err = s.DB.Exec(ctx, `UPDATE deliveries SET next_attempt=now()`); err != nil {
			t.Fatal(err)
		}
	}
	deliveries, err := s.ClaimDeliveries(ctx)
	if err != nil || len(deliveries) != 0 {
		t.Fatal("terminal delivery reclaimed", err)
	}
}

func TestChecksPreserveScheduleAfterMissedTicks(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	if err := s.SaveMonitor(ctx, monitor("cadence"), true); err != nil {
		t.Fatal(err)
	}
	var due time.Time
	if err := s.DB.QueryRow(ctx, `UPDATE monitors SET next_check=now()-interval '125 seconds' WHERE id='cadence' RETURNING next_check`).Scan(&due); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.Claim(ctx)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %v", claimed, err)
	}
	if err := s.Record(ctx, claimed[0], domain.Check{At: time.Now(), OK: true, Code: 200}); err != nil {
		t.Fatal(err)
	}
	var next time.Time
	if err := s.DB.QueryRow(ctx, `SELECT next_check FROM monitors WHERE id='cadence'`).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if !next.Equal(due.Add(180 * time.Second)) {
		t.Fatalf("schedule drift: due %v next %v", due, next)
	}
	again, err := s.Claim(ctx)
	if err != nil || len(again) != 0 {
		t.Fatalf("replayed missed slots: %v %v", again, err)
	}
}

// Hold session insertion at a database barrier while a password change competes.
// Once both finish, a login using the old password must never leave a valid session.
func TestPasswordChangeFencesInFlightLogin(t *testing.T) {
	s := testutil.DB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const password = "original-password-for-race"
	if err := s.Bootstrap(ctx, "race@example.com", password); err != nil {
		t.Fatal(err)
	}
	barrier := time.Now().UnixNano() % 1000000000
	gate, err := s.DB.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Release()
	if _, err = gate.Exec(ctx, `SELECT pg_advisory_lock($1)`, barrier); err != nil {
		t.Fatal(err)
	}
	defer gate.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, barrier)
	_, err = s.DB.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION gate_session() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NEW; END $$; CREATE TRIGGER gate_session BEFORE INSERT ON sessions FOR EACH ROW EXECUTE FUNCTION gate_session()`, barrier))
	if err != nil {
		t.Fatal(err)
	}
	type loginResult struct {
		token string
		err   error
	}
	login := make(chan loginResult, 1)
	go func() {
		token, err := s.Login(ctx, "race@example.com", password)
		login <- loginResult{token, err}
	}()
	for {
		var waiting bool
		if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objid=$1::oid)`, barrier).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("login did not reach insertion barrier")
		case <-time.After(10 * time.Millisecond):
		}
	}
	changed := make(chan error, 1)
	go func() { changed <- s.ChangePassword(ctx, password, "replacement-password-for-race") }()
	// Either the update waits on the account lock, or it commits before insertion.
	// Release the insertion only after observing one of those durable outcomes.
	completed := false
	for !completed {
		select {
		case err = <-changed:
			if err != nil {
				t.Fatal(err)
			}
			completed = true
		default:
			var waiting bool
			err = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'UPDATE accounts SET password_hash=%' AND pid IN (SELECT pid FROM pg_locks WHERE relation='accounts'::regclass))`).Scan(&waiting)
			if err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("password change did not reach update")
			case <-time.After(10 * time.Millisecond):
			}
			continue
		}
		break
	}
	if _, err = gate.Exec(ctx, `SELECT pg_advisory_unlock($1)`, barrier); err != nil {
		t.Fatal(err)
	}
	result := <-login
	if result.err != nil {
		t.Fatal(result.err)
	}
	if !completed {
		if err = <-changed; err != nil {
			t.Fatal(err)
		}
	}
	if s.Session(ctx, result.token) {
		t.Fatal("old-password login survived concurrent password change")
	}
	if _, err = s.Login(ctx, "race@example.com", password); err == nil {
		t.Fatal("old password accepted")
	}
}
