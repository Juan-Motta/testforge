// Command worker executes queued runs: it claims them from Postgres, talks to
// the agents under test and stores the results.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"testforge/internal/agent"
	"testforge/internal/connection"
	"testforge/internal/platform/config"
	"testforge/internal/platform/database"
	"testforge/internal/platform/sealbox"
	"testforge/internal/run"
)

func main() {
	if err := runWorker(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func runWorker() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.LogFormat == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	}
	box, err := sealbox.New(cfg.SecretKey)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	text := &run.TextEngine{AllowPrivate: cfg.AllowPrivateNetwork}
	voice := &run.VoiceEngine{AllowPrivate: cfg.AllowPrivateNetwork}
	engines := func(modality string) run.Engine {
		if modality == agent.ModalityText {
			return text
		}
		return voice
	}
	w := run.NewWorker(pool, connection.NewService(pool, box, cfg.AllowPrivateNetwork),
		agent.NewService(pool, box, cfg.AllowPrivateNetwork), engines, run.LocalStorage{Dir: cfg.ArtifactsDir}, cfg.WorkerConcurrency)
	w.EnableJudging(run.JudgeOptions{Concurrency: cfg.JudgeConcurrency, TraceWait: cfg.JudgeTraceWait, AllowPrivate: cfg.AllowPrivateNetwork})
	w.Run(ctx)
	slog.Info("worker stopped")
	return nil
}
