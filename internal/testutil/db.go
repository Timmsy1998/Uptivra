// Package testutil provides isolated schemas on a real PostgreSQL test database.
package testutil

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"uptivra/internal/cryptobox"
	"uptivra/internal/store"
)

func DB(t *testing.T) *store.Store {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	box, _ := cryptobox.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	s, e := store.Open(context.Background(), databaseURL, box)
	if e != nil {
		t.Fatal(e)
	}
	name := "test_" + cryptobox.Hash(cryptobox.Token())[:16]
	ident := pgx.Identifier{name}.Sanitize()
	if _, e = s.DB.Exec(context.Background(), "CREATE SCHEMA "+ident); e != nil {
		t.Fatal(e)
	}
	s.DB.Close()
	u, e := url.Parse(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	s, e = store.Open(context.Background(), u.String(), box)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); s.DB.Close() })
	if e = s.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	return s
}
