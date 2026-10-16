package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"uptivra/internal/domain"
	"uptivra/internal/testutil"
)

func TestAvailabilityAuthorizationAndPublicProjection(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := domain.Monitor{ID: "history", Name: "Safe", URL: "https://example.com/secret-target", Interval: 60, Timeout: 5, ExpectedStatus: 200, FailureThreshold: 2, RecoveryThreshold: 2, Public: true, Enabled: true, Authorized: true}
	if e := s.SaveMonitor(ctx, m, true); e != nil {
		t.Fatal(e)
	}
	token, e := s.CreateKey(ctx, "reader", "read")
	if e != nil {
		t.Fatal(e)
	}
	h := (&Server{Store: s}).Handler()
	call := func(path, key string, want int) {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret-target") {
			t.Fatal("secret exposed")
		}
	}
	call("/api/v1/monitors/history/availability", "", 401)
	call("/api/v1/monitors/history/availability?days=90", token, 200)
	call("/api/v1/public/monitors/history/availability?days=7", "", 200)
	call("/api/v1/public/monitors/history/availability?days=8", "", 400)
	if _, e := s.DB.Exec(ctx, `UPDATE monitors SET public=false`); e != nil {
		t.Fatal(e)
	}
	call("/api/v1/public/monitors/history/availability", "", 404)
	call("/api/v1/monitors/history/availability", token, 200)
}
