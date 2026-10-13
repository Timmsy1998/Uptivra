package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"uptivra/internal/config"
	"uptivra/internal/domain"
	"uptivra/internal/testutil"
)

func TestAuthorizationCRUDAndPublicRedaction(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	origin := "https://status.example.com"
	h := (&Server{Store: s, Config: config.Config{Origin: origin, CronSecret: strings.Repeat("x", 32)}}).Handler()
	call := func(method, path, body, key, requestOrigin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		if requestOrigin != "" {
			r.Header.Set("Origin", requestOrigin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/v1/monitors", "", "", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	write, _ := s.CreateKey(ctx, "write", "write")
	read, _ := s.CreateKey(ctx, "read", "read")
	m := domain.Monitor{Name: "Public API", URL: "https://example.com/private-target", Interval: 60, Timeout: 5, ExpectedStatus: 200, FailureThreshold: 2, RecoveryThreshold: 2, Enabled: true, Public: true, Authorized: true, Headers: map[string]string{"Authorization": "top-secret"}}
	b, _ := json.Marshal(m)
	w := call("POST", "/api/v1/monitors", string(b), read, "", nil)
	if w.Code != 403 {
		t.Fatal("read key wrote", w.Code)
	}
	w = call("POST", "/api/v1/monitors", string(b), write, "", nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	m.Name = "Private service"
	m.Public = false
	b, _ = json.Marshal(m)
	if w = call("POST", "/api/v1/monitors", string(b), write, "", nil); w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = call("GET", "/api/v1/public/status", "", "", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, secret := range []string{"private-target", "top-secret", "Private service", "headers", "url"} {
		if bytes.Contains(w.Body.Bytes(), []byte(secret)) {
			t.Fatal("public leak", secret)
		}
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Public API")) {
		t.Fatal("public monitor missing")
	}
	if w = call("GET", "/api/v1/keys", "", write, "", nil); w.Code != 401 {
		t.Fatal("keys accessible with API token")
	}
	if w = call("GET", "/api/cron", "", "", "", nil); w.Code != 401 {
		t.Fatal("unauthenticated scheduler")
	}
	if e := s.Bootstrap(ctx, "admin@example.com", "test-password-very-long"); e != nil {
		t.Fatal(e)
	}
	login := `{"email":"admin@example.com","password":"test-password-very-long"}`
	if w = call("POST", "/api/v1/auth/login", login, "", "https://evil.example", nil); w.Code != 403 {
		t.Fatal("cross-site login")
	}
	w = call("POST", "/api/v1/auth/login", login, "", origin, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	if w = call("DELETE", "/api/v1/monitors/"+created["id"], "", "", "https://evil.example", cookie); w.Code != 403 {
		t.Fatal("CSRF accepted")
	}
	if w = call("DELETE", "/api/v1/monitors/"+created["id"], "", "", origin, cookie); w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	if w = call("GET", "/api/v1/monitors/"+created["id"], "", read, "", nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
