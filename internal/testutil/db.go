// Package testutil provides isolated schemas on a real PostgreSQL test database.
package testutil

import (
	"context"
	"encoding/base64"
	"github.com/jackc/pgx/v5"
	"github.com/timmsy1998/uptivra/internal/cryptobox"
	"github.com/timmsy1998/uptivra/internal/store"
	"os"
	"testing"
)

func DB(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	box, _ := cryptobox.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	s, e := store.Open(context.Background(), url, box)
	if e != nil {
		t.Fatal(e)
	}
	name := "test_" + cryptobox.Hash(cryptobox.Token())[:16]
	ident := pgx.Identifier{name}.Sanitize()
	if _, e = s.DB.Exec(context.Background(), "CREATE SCHEMA "+ident); e != nil {
		t.Fatal(e)
	}
	s.DB.Close()
	cfg, e := pgx.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.RuntimeParams["search_path"] = name
	s, e = store.Open(context.Background(), cfg.ConnString(), box)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); s.DB.Close() })
	if e = s.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	return s
}
