package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/LynnColeArt/better-cal/backend/internal/config"
	"github.com/LynnColeArt/better-cal/backend/internal/httpapi"
)

func main() {
	cfg := config.FromEnv()
	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: httpapi.NewServer(cfg),
	}

	slog.Info("starting better-cal api", "addr", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api server failed", "error", err)
		os.Exit(1)
	}
}
