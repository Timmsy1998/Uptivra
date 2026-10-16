package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"uptivra/internal/config"
	"uptivra/internal/testutil"
)

func TestSetupOriginCodeAndThrottle(t *testing.T) {
	s := testutil.DB(t)
	h := (&Server{Store: s, Config: config.Config{Origin: "https://example.com", SetupCode: "0123"}}).Handler()
	call := func(origin, code string) int {
		r := httptest.NewRequest("POST", "/api/v1/auth/setup", strings.NewReader(`{"email":"owner@example.com","password":"test-password-very-long","code":"`+code+`"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if got := call("https://evil.example", "0123"); got != 403 {
		t.Fatal(got)
	}
	for range 5 {
		if got := call("https://example.com", "9999"); got != 403 {
			t.Fatal(got)
		}
	}
	if got := call("https://example.com", "0123"); got != 429 {
		t.Fatal(got)
	}
	if _, e := s.DB.Exec(context.Background(), `DELETE FROM rate_limits`); e != nil {
		t.Fatal(e)
	}
	if got := call("https://example.com", "0123"); got != 201 {
		t.Fatal(got)
	}
	if got := call("https://example.com", "0123"); got != 400 {
		t.Fatal(got)
	}
}
