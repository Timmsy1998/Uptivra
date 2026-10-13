package main

import (
	"context"
	"github.com/timmsy1998/uptivra/internal/config"
	"github.com/timmsy1998/uptivra/internal/cryptobox"
	"github.com/timmsy1998/uptivra/internal/server"
	"github.com/timmsy1998/uptivra/internal/store"
	"github.com/timmsy1998/uptivra/internal/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error("command failed", "error", e)
		os.Exit(1)
	}
}
func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	box, e := cryptobox.New(c.EncryptionKey)
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s, e := store.Open(startup, c.DatabaseURL, box)
	if e != nil {
		return e
	}
	defer s.DB.Close()
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "migrate":
		return s.Migrate(ctx)
	case "bootstrap":
		return s.Bootstrap(ctx, os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD"))
	case "tick":
		report, e := worker.Tick(ctx, s)
		slog.Info("tick finished", "claimed", report.Claimed, "recorded", report.Recorded, "delivered", report.Delivered)
		return e
	case "serve":
		srv := &http.Server{Addr: ":" + c.Port, Handler: (&server.Server{Store: s, Config: c}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
		done := make(chan error, 1)
		go func() { done <- srv.ListenAndServe() }()
		slog.Info("listening", "port", c.Port)
		select {
		case e := <-done:
			return e
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			return srv.Shutdown(shutdown)
		}
	default:
		return &commandError{}
	}
}

type commandError struct{}

func (*commandError) Error() string { return "command must be serve, migrate, bootstrap or tick" }
