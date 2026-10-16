package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"uptivra/internal/testutil"
)

func commandArgs(t *testing.T, command string) {
	t.Helper()
	original := os.Args
	os.Args = []string{"uptivra", command}
	t.Cleanup(func() { os.Args = original })
}

func TestDemoCommandsSkipDatabase(t *testing.T) {
	t.Setenv("NEXT_PUBLIC_DEMO_MODE", "true")
	t.Setenv("DATABASE_URL", "invalid database URL")
	for _, command := range []string{"serve", "schedule", "tick", "bootstrap", "reset-password", "migrate"} {
		t.Run(command, func(t *testing.T) {
			commandArgs(t, command)
			if err := run(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTickMigratesEmptyDatabaseBeforeWork(t *testing.T) {
	s := testutil.UnmigratedDB(t)
	t.Setenv("NEXT_PUBLIC_DEMO_MODE", "false")
	t.Setenv("DATABASE_URL", s.DB.Config().ConnString())
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("A", 43)+"=")
	t.Setenv("CRON_SECRET", strings.Repeat("c", 32))
	t.Setenv("APP_ORIGIN", "https://status.example.com")
	commandArgs(t, "tick")
	if err := run(); err != nil {
		t.Fatal(err)
	}
	var runs int
	if err := s.DB.QueryRow(context.Background(), `SELECT count(*) FROM scheduler_runs`).Scan(&runs); err != nil || runs != 1 {
		t.Fatalf("scheduler runs: %d, err: %v", runs, err)
	}
}
