package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"uptivra/internal/config"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestChannelGroupPermissionsAndValidation(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	origin := "https://status.example.com"
	h := (&Server{Store: s, Config: config.Config{Origin: origin}}).Handler()
	var cookie *http.Cookie
	call := func(method, path, body, requestOrigin, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", requestOrigin)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if err := s.Bootstrap(ctx, "admin@example.com", "test-password-very-long"); err != nil {
		t.Fatal(err)
	}
	w := call("POST", "/api/v1/auth/login", `{"email":"admin@example.com","password":"test-password-very-long"}`, origin, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	session := w.Result().Cookies()[0]
	path := "/api/v1/channels/missing/group"
	if w = call("PUT", path, `{"group":"Bots"}`, origin, ""); w.Code != 401 {
		t.Fatal("unauthenticated routing", w.Code)
	}
	for _, scope := range []string{"read", "write"} {
		key, err := s.CreateKey(ctx, scope, scope)
		if err != nil {
			t.Fatal(err)
		}
		if w = call("PUT", path, `{"group":"Bots"}`, origin, key); w.Code != 401 && w.Code != 403 {
			t.Fatal("API key changed routing", w.Code)
		}
	}
	cookie = session
	if w = call("POST", "/api/v1/channels", `{"name":"Bots","kind":"discord","url":"https://example.com/private-hook","group":"Bots"}`, origin, ""); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	cs, err := s.Channels(ctx)
	if err != nil || len(cs) != 1 {
		t.Fatal(cs, err)
	}
	path = "/api/v1/channels/" + cs[0].ID + "/group"
	if w = call("PUT", path, `{"group":null}`, "https://evil.example", ""); w.Code != 403 {
		t.Fatal("CSRF routing", w.Code)
	}
	for _, body := range []string{`{}`, `{"group":7}`, `{"group":false}`, `{"group":"Bots","unexpected":true}`, `{"group":"` + strings.Repeat("é", 41) + `"}`} {
		if w = call("PUT", path, body, origin, ""); w.Code != 400 {
			t.Fatal("invalid group", body, w.Code)
		}
	}
	for _, body := range []string{`{"group":"Bots"}`, `{"group":""}`, `{"group":null}`} {
		if w = call("PUT", path, body, origin, ""); w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
		w = call("GET", "/api/v1/channels", "", origin, "")
		var listed []store.Channel
		if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed) != 1 {
			t.Fatal(err, w.Body.String())
		}
		var input struct {
			Group *string `json:"group"`
		}
		json.Unmarshal([]byte(body), &input)
		saved := listed[0].Group
		if (saved == nil) != (input.Group == nil) || (saved != nil && *saved != *input.Group) {
			t.Fatal("group not reflected", w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private-hook") || strings.Contains(w.Body.String(), "signing_secret") {
			t.Fatal("channel secret exposed")
		}
	}
	if w = call("PUT", "/api/v1/channels/missing/group", `{"group":null}`, origin, ""); w.Code != 404 {
		t.Fatal("missing channel", w.Code)
	}
}
