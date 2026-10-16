// Package uptivra is the Vercel Go Function adapter. All application logic is shared with cmd/uptivra.
// Keep the package and module names aligned: the Vercel Go builder uses the
// package name when synthesizing its module and internal imports must stay local.
package uptivra

import (
	"context"
	"net/http"
	"sync"
	"time"

	"uptivra/internal/config"
	"uptivra/internal/cryptobox"
	"uptivra/internal/server"
	"uptivra/internal/store"
)

var mu sync.Mutex
var app http.Handler

func Handler(w http.ResponseWriter, r *http.Request) {
	if config.DemoMode() {
		http.Error(w, "API disabled in demo mode", http.StatusForbidden)
		return
	}
	// Include cold-start migration time in the invocation budget (maxDuration 60s).
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	mu.Lock()
	if ctx.Err() != nil {
		mu.Unlock()
		http.Error(w, "service initialization unavailable", http.StatusServiceUnavailable)
		return
	}
	if app == nil {
		startup, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		s, e := initialize(startup)
		cancel()
		if e != nil {
			mu.Unlock()
			http.Error(w, "service configuration or database unavailable", 503)
			return
		}
		app = s.Handler()
	}
	h := app
	mu.Unlock()
	h.ServeHTTP(w, r)
}

func initialize(ctx context.Context) (*server.Server, error) {
	c, e := config.Load()
	if e != nil {
		return nil, e
	}
	box, e := cryptobox.New(c.EncryptionKey)
	if e != nil {
		return nil, e
	}
	s, e := store.OpenMigrated(ctx, c.DatabaseURL, box)
	if e != nil {
		return nil, e
	}
	return &server.Server{Store: s, Config: c}, nil
}
