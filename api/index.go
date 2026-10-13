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
	mu.Lock()
	if app == nil {
		c, e := config.Load()
		if e == nil {
			box, err := cryptobox.New(c.EncryptionKey)
			e = err
			if e == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
				s, err := store.Open(ctx, c.DatabaseURL, box)
				cancel()
				e = err
				if e == nil {
					app = (&server.Server{Store: s, Config: c}).Handler()
				}
			}
		}
		if e != nil {
			mu.Unlock()
			http.Error(w, "service configuration or database unavailable", 503)
			return
		}
	}
	h := app
	mu.Unlock()
	h.ServeHTTP(w, r)
}
