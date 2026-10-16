package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestStartupMigrationsConcurrentAndRepeatable(t *testing.T) {
	s := testutil.UnmigratedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			ready, err := store.OpenMigrated(ctx, s.DB.Config().ConnString(), s.Box)
			if err != nil {
				t.Error(err)
				return
			}
			ready.DB.Close()
		})
	}
	wg.Wait()
	var versions int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM schema_versions`).Scan(&versions); err != nil || versions != 8 {
		t.Fatalf("versions = %d, err = %v", versions, err)
	}
	// Simulate an existing version-7 installation with real monitor data.
	if err := s.SaveMonitor(ctx, monitor("startup-monitor"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `ALTER TABLE checks DROP COLUMN interval_seconds; DELETE FROM schema_versions WHERE version=8`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		ready, err := store.OpenMigrated(ctx, s.DB.Config().ConnString(), s.Box)
		if err != nil {
			t.Fatal(err)
		}
		ready.DB.Close()
	}
	if _, err := s.DB.Exec(ctx, `SELECT interval_seconds FROM checks`); err != nil {
		t.Fatal(err)
	}
	var monitors int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM monitors`).Scan(&monitors); err != nil || monitors != 1 {
		t.Fatalf("existing monitors = %d, err = %v", monitors, err)
	}
}

func TestStartupMigrationFailureRollsBackAndCanRetry(t *testing.T) {
	s := testutil.UnmigratedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// A conflicting relation makes base-schema creation fail midway through.
	if _, err := s.DB.Exec(ctx, `CREATE TABLE monitors (wrong_column text)`); err != nil {
		t.Fatal(err)
	}
	ready, err := store.OpenMigrated(ctx, s.DB.Config().ConnString(), s.Box)
	if err == nil || ready != nil {
		t.Fatal("failed migration returned a usable store")
	}
	var versions *string
	if err := s.DB.QueryRow(ctx, `SELECT to_regclass('schema_versions')::text`).Scan(&versions); err != nil || versions != nil {
		t.Fatalf("failed migration did not roll back: %v, %v", versions, err)
	}
	if _, err := s.DB.Exec(ctx, `DROP TABLE monitors`); err != nil {
		t.Fatal(err)
	}
	ready, err = store.OpenMigrated(ctx, s.DB.Config().ConnString(), s.Box)
	if err != nil {
		t.Fatal(err)
	}
	ready.DB.Close()
}

func TestStartupMigrationLockHonorsDeadline(t *testing.T) {
	s := testutil.UnmigratedDB(t)
	tx, err := s.DB.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `SELECT pg_advisory_xact_lock(884127)`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ready, err := store.OpenMigrated(ctx, s.DB.Config().ConnString(), s.Box)
	if err == nil || ready != nil || ctx.Err() == nil {
		t.Fatalf("startup must fail closed on lock timeout: store=%v, err=%v, context=%v", ready, err, ctx.Err())
	}
}
