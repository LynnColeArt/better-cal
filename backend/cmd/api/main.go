package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/booking"
	"github.com/LynnColeArt/better-cal/backend/internal/config"
	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/LynnColeArt/better-cal/backend/internal/httpapi"
)

func main() {
	cfg := config.FromEnv()
	serverOptions := []httpapi.Option{}
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err := db.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			cancel()
			slog.Error("database pool failed", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		if err := db.Ping(ctx, pool); err != nil {
			cancel()
			slog.Error("database ping failed", "error", err)
			os.Exit(1)
		}
		if err := db.Migrate(ctx, pool); err != nil {
			cancel()
			slog.Error("database migration failed", "error", err)
			os.Exit(1)
		}
		serverOptions = append(serverOptions, httpapi.WithBookingStore(
			booking.NewStoreWithRepository(booking.NewPostgresRepository(pool)),
		))
		cancel()
		slog.Info("database connection ready")
	}

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: httpapi.NewServer(cfg, serverOptions...),
	}

	slog.Info("starting better-cal api", "addr", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api server failed", "error", err)
		os.Exit(1)
	}
}
