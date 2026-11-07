// Command api runs the TestForge HTTP API.
//
// Usage:
//
//	api serve    start the HTTP server (default); migrates first when AUTO_MIGRATE=true
//	api migrate  apply pending database migrations and exit
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"testforge/internal/platform/config"
	"testforge/internal/platform/database"
	"testforge/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogging(cfg.LogFormat)

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "migrate":
		if err := database.Migrate(ctx, pool); err != nil {
			return err
		}
		slog.Info("migrations applied")
		return nil
	case "serve":
		if cfg.AutoMigrate {
			if err := database.Migrate(ctx, pool); err != nil {
				return err
			}
		}
		handler, err := server.New(pool, cfg)
		if err != nil {
			return err
		}
		return serve(ctx, cfg, handler)
	default:
		return fmt.Errorf("unknown command %q (use serve or migrate)", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, handler http.Handler) error {
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func setupLogging(format string) {
	var h slog.Handler = slog.NewTextHandler(os.Stderr, nil)
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(h))
}
