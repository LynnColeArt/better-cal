package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/booking"
	"github.com/LynnColeArt/better-cal/backend/internal/config"
	"github.com/LynnColeArt/better-cal/backend/internal/db"
)

func main() {
	cfg := config.FromEnv()
	if cfg.DatabaseURL == "" {
		slog.Error("worker requires CALDIY_DATABASE_URL")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database pool failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Ping(ctx, pool); err != nil {
		slog.Error("database ping failed", "error", err)
		os.Exit(1)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	repository := booking.NewPostgresRepository(pool)
	dispatcher := booking.NewPostgresSideEffectDispatcher(pool, repository)
	worker := booking.NewSideEffectWorker(repository, dispatcher)
	result, err := worker.RunOnce(ctx, 25)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			slog.Error("worker context ended", "error", err)
		} else {
			slog.Error("side-effect dispatch failed", "error", err)
		}
		os.Exit(1)
	}
	slog.Info("side-effect dispatch complete", "claimed", result.Claimed, "delivered", result.Delivered, "failed", result.Failed)
}
