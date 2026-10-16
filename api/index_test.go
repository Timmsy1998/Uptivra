package uptivra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uptivra/internal/testutil"
)

func TestInitializeMigratesBeforeReadiness(t *testing.T) {
	s := testutil.UnmigratedDB(t)
	t.Setenv("DATABASE_URL", s.DB.Config().ConnString())
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("A", 43)+"=")
	t.Setenv("CRON_SECRET", strings.Repeat("c", 32))
	t.Setenv("APP_ORIGIN", "https://status.example.com")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ready, err := initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Store.DB.Close()
	w := httptest.NewRecorder()
	ready.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("readiness: %d %s", w.Code, w.Body.String())
	}
	var version int
	if err := s.DB.QueryRow(ctx, `SELECT max(version) FROM schema_versions`).Scan(&version); err != nil || version != 8 {
		t.Fatalf("schema version: %d, err: %v", version, err)
	}
}

func TestDemoSkipsInitializationEvenWithCachedApplication(t *testing.T) {
	t.Setenv("NEXT_PUBLIC_DEMO_MODE", "true")
	t.Setenv("DATABASE_URL", "invalid database URL")
	original := app
	app = http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("demo reached live application") })
	t.Cleanup(func() { app = original })
	w := httptest.NewRecorder()
	Handler(w, httptest.NewRequest(http.MethodGet, "/api/cron", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("demo status: %d", w.Code)
	}
}

func TestInitializationFailureDoesNotCacheApplication(t *testing.T) {
	t.Setenv("NEXT_PUBLIC_DEMO_MODE", "false")
	t.Setenv("DATABASE_URL", "")
	for range 2 {
		w := httptest.NewRecorder()
		Handler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if w.Code != http.StatusServiceUnavailable || app != nil {
			t.Fatalf("failed initialization must remain retryable: %d", w.Code)
		}
	}
}
