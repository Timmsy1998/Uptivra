package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"uptivra/internal/config"
	"uptivra/internal/cryptobox"
	"uptivra/internal/domain"
	"uptivra/internal/testutil"
)

func TestIncidentUpdatePermissionsAndPublicProjection(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	m := domain.Monitor{ID: "bot", Name: "Bot", Type: "heartbeat", Interval: 60, Grace: 30, Enabled: true, Public: true, Authorized: true, FailureThreshold: 1, RecoveryThreshold: 1}
	if err := s.SaveMonitor(ctx, m, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO incidents(id,monitor_id) VALUES('incident','bot')`); err != nil {
		t.Fatal(err)
	}
	read, err := s.CreateKey(ctx, "read", "read")
	if err != nil {
		t.Fatal(err)
	}
	write, err := s.CreateKey(ctx, "write", "write")
	if err != nil {
		t.Fatal(err)
	}
	h := (&Server{Store: s, Config: config.Config{Origin: "https://example.com"}}).Handler()
	call := func(path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	good := `{"status":"identified","message":"Fix deployed"}`
	// A durable valid browser session still cannot bypass origin validation.
	if _, err = s.DB.Exec(ctx, `INSERT INTO sessions(token_hash,expires_at) VALUES($1,now()+interval '1 hour')`, cryptobox.Hash("test-session")); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "https://evil.example"} {
		r := httptest.NewRequest("POST", "/api/v1/incidents/incident/updates", strings.NewReader(good))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Cookie", "uptivra_session=test-session")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("CSRF", w.Code, w.Body.String())
		}
	}

	for _, tc := range []struct {
		path, body, key string
		code            int
	}{
		{"/api/v1/incidents/incident/updates", good, "", 401},
		{"/api/v1/incidents/incident/updates", good, read, 403},
		{"/api/v1/incidents/incident/updates", `{"status":"resolved","message":"done"}`, write, 400},
		{"/api/v1/incidents/missing/updates", good, write, 404},
		{"/api/v1/incidents/incident/updates", good, write, 201},
	} {
		w := call(tc.path, tc.body, tc.key)
		if w.Code != tc.code {
			t.Fatalf("%d expected %d: %s", w.Code, tc.code, w.Body.String())
		}
	}
	public := func() string {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/public/status", nil))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		return w.Body.String()
	}
	if !strings.Contains(public(), "Fix deployed") {
		t.Fatal("missing public update")
	}
	if _, err = s.DB.Exec(ctx, `UPDATE monitors SET public=false WHERE id='bot'`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(public(), "Fix deployed") {
		t.Fatal("private update exposed")
	}
}
