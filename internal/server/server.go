package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"uptivra/internal/config"
	"uptivra/internal/cryptobox"
	"uptivra/internal/domain"
	"uptivra/internal/store"
	"uptivra/internal/worker"
)

type Server struct {
	Store  *store.Store
	Config config.Config
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if e := s.Store.DB.Ping(r.Context()); e != nil {
			problem(w, 503, "database unavailable")
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/cron", s.cron)
	mux.HandleFunc("GET /api/v1/public/status", s.public)
	mux.HandleFunc("GET /api/v1/public/badge.svg", s.badge)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("GET /api/v1/auth/session", s.auth(true, func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]string{"role": "administrator"})
	}))
	mux.Handle("POST /api/v1/auth/logout", s.auth(true, s.logout))
	mux.Handle("PUT /api/v1/account/password", s.auth(true, s.password))
	mux.Handle("GET /api/v1/monitors", s.auth(false, s.monitors))
	mux.Handle("POST /api/v1/monitors", s.auth(false, s.saveMonitor))
	mux.Handle("GET /api/v1/monitors/{id}", s.auth(false, s.monitor))
	mux.Handle("PUT /api/v1/monitors/{id}", s.auth(false, s.saveMonitor))
	mux.Handle("DELETE /api/v1/monitors/{id}", s.auth(false, s.deleteMonitor))
	mux.Handle("GET /api/v1/monitors/{id}/checks", s.auth(false, s.history))
	mux.Handle("GET /api/v1/incidents", s.auth(false, s.incidents))
	mux.Handle("GET /api/v1/settings", s.auth(false, s.settings))
	mux.Handle("PUT /api/v1/settings", s.auth(false, s.saveSettings))
	mux.Handle("GET /api/v1/channels", s.auth(true, s.channels))
	mux.Handle("POST /api/v1/channels", s.auth(true, s.createChannel))
	mux.Handle("DELETE /api/v1/channels/{id}", s.auth(true, s.deleteChannel))
	mux.Handle("GET /api/v1/keys", s.auth(true, s.keys))
	mux.Handle("POST /api/v1/keys", s.auth(true, s.createKey))
	mux.Handle("DELETE /api/v1/keys/{id}", s.auth(true, s.deleteKey))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		defer func() {
			if recover() != nil {
				slog.Error("request panic", "method", r.Method)
				problem(w, 500, "internal error")
			}
		}()
		mux.ServeHTTP(w, r)
	})
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status != 204 {
		json.NewEncoder(w).Encode(v)
	}
}
func problem(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		problem(w, 415, "application/json required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		problem(w, 400, "invalid JSON or request fields")
		return false
	}
	var extra any
	if e := d.Decode(&extra); e == nil || !errors.Is(e, io.EOF) {
		problem(w, 400, "a single JSON object is required")
		return false
	}
	return true
}
func dbError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, pgx.ErrNoRows) {
		problem(w, 404, "not found")
	} else {
		slog.Error("database operation failed")
		problem(w, 503, "operation unavailable; retry shortly")
	}
	return true
}

const cookieName = "uptivra_session"

func sessionToken(r *http.Request) string {
	c, e := r.Cookie(cookieName)
	if e != nil {
		return ""
	}
	return c.Value
}
func (s *Server) cookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: value, Path: "/api", HttpOnly: true, Secure: strings.HasPrefix(s.Config.Origin, "https://"), SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (s *Server) origin(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != s.Config.Origin {
		problem(w, 403, "request origin rejected")
		return false
	}
	return true
}
func (s *Server) auth(sessionOnly bool, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		hasBearer := strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
		if hasBearer && !sessionOnly {
			scope := s.Store.KeyScope(r.Context(), bearer)
			if scope == "" {
				problem(w, 401, "authentication required")
				return
			}
			if r.Method != "GET" && scope != "write" {
				problem(w, 403, "write scope required")
				return
			}
		} else {
			token := sessionToken(r)
			if token == "" || !s.Store.Session(r.Context(), token) {
				problem(w, 401, "authentication required")
				return
			}
			if r.Method != "GET" && !s.origin(w, r) {
				return
			}
		}
		next(w, r)
	})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.origin(w, r) {
		return
	}
	var v struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &v) {
		return
	}
	if !s.Store.RateLimit(r.Context(), "login:global", 60) || !s.Store.RateLimit(r.Context(), "login:"+v.Email, 10) {
		problem(w, 429, "too many login attempts; try again in 15 minutes")
		return
	}
	token, e := s.Store.Login(r.Context(), v.Email, v.Password)
	if e != nil {
		problem(w, 401, "invalid email or password")
		return
	}
	s.cookie(w, token, 43200)
	respond(w, 200, map[string]string{"role": "administrator"})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if dbError(w, s.Store.Logout(r.Context(), sessionToken(r))) {
		return
	}
	s.cookie(w, "", -1)
	respond(w, 204, nil)
}
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	if !decode(w, r, &v) {
		return
	}
	if !s.Store.RateLimit(r.Context(), "password", 10) {
		problem(w, 429, "too many attempts")
		return
	}
	if e := s.Store.ChangePassword(r.Context(), v.Current, v.Next); e != nil {
		problem(w, 400, "current password incorrect or new password outside 14–72 bytes")
		return
	}
	s.cookie(w, "", -1)
	respond(w, 204, nil)
}
func (s *Server) cron(w http.ResponseWriter, r *http.Request) {
	expected := "Bearer " + s.Config.CronSecret
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		problem(w, 401, "scheduler authentication required")
		return
	}
	report, e := worker.Tick(r.Context(), s.Store)
	if dbError(w, e) {
		return
	}
	slog.Info("scheduler completed", "claimed", report.Claimed, "recorded", report.Recorded, "delivered", report.Delivered)
	respond(w, 200, report)
}
func (s *Server) monitors(w http.ResponseWriter, r *http.Request) {
	ms, e := s.Store.Monitors(r.Context())
	if dbError(w, e) {
		return
	}
	respond(w, 200, ms)
}
func (s *Server) monitor(w http.ResponseWriter, r *http.Request) {
	m, e := s.Store.Monitor(r.Context(), r.PathValue("id"))
	if dbError(w, e) {
		return
	}
	m.Status = m.Health(time.Now())
	respond(w, 200, m)
}
func (s *Server) saveMonitor(w http.ResponseWriter, r *http.Request) {
	var m domain.Monitor
	if !decode(w, r, &m) {
		return
	}
	create := r.Method == "POST"
	if create {
		m.ID = cryptobox.Token()
	} else {
		m.ID = r.PathValue("id")
	}
	if e := m.Validate(); e != nil {
		problem(w, 400, e.Error())
		return
	}
	if dbError(w, s.Store.SaveMonitor(r.Context(), m, create)) {
		return
	}
	code := 200
	if create {
		code = 201
	}
	respond(w, code, map[string]string{"id": m.ID})
}
func (s *Server) deleteMonitor(w http.ResponseWriter, r *http.Request) {
	if dbError(w, s.Store.DeleteMonitor(r.Context(), r.PathValue("id"))) {
		return
	}
	respond(w, 204, nil)
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	if _, e := s.Store.Monitor(r.Context(), r.PathValue("id")); dbError(w, e) {
		return
	}
	cs, e := s.Store.History(r.Context(), r.PathValue("id"))
	if dbError(w, e) {
		return
	}
	respond(w, 200, cs)
}
func (s *Server) incidents(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.Incidents(r.Context(), false)
	if dbError(w, e) {
		return
	}
	respond(w, 200, v)
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.Settings(r.Context())
	if dbError(w, e) {
		return
	}
	respond(w, 200, v)
}
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var v domain.Settings
	if !decode(w, r, &v) {
		return
	}
	if e := s.Store.SaveSettings(r.Context(), v); e != nil {
		problem(w, 400, "invalid settings; retention must be 7–365 days")
		return
	}
	respond(w, 204, nil)
}
func (s *Server) channels(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.Channels(r.Context())
	if dbError(w, e) {
		return
	}
	respond(w, 200, v)
}
func (s *Server) createChannel(w http.ResponseWriter, r *http.Request) {
	var v store.Channel
	if !decode(w, r, &v) {
		return
	}
	if e := s.Store.CreateChannel(r.Context(), v); e != nil {
		problem(w, 400, "invalid channel: require a public HTTP(S) URL, name, provider, and 32–256 byte webhook secret")
		return
	}
	respond(w, 201, map[string]string{"status": "created"})
}
func (s *Server) deleteChannel(w http.ResponseWriter, r *http.Request) {
	if dbError(w, s.Store.DeleteChannel(r.Context(), r.PathValue("id"))) {
		return
	}
	respond(w, 204, nil)
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.Keys(r.Context())
	if dbError(w, e) {
		return
	}
	respond(w, 200, v)
}
func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if !decode(w, r, &v) {
		return
	}
	token, e := s.Store.CreateKey(r.Context(), v.Name, v.Scope)
	if e != nil {
		problem(w, 400, "name and read/write scope required")
		return
	}
	respond(w, 201, map[string]string{"token": token})
}
func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	if dbError(w, s.Store.DeleteKey(r.Context(), r.PathValue("id"))) {
		return
	}
	respond(w, 204, nil)
}
