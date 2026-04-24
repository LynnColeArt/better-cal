package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/auth"
	"github.com/LynnColeArt/better-cal/backend/internal/booking"
	calendarprovider "github.com/LynnColeArt/better-cal/backend/internal/calendar"
	"github.com/LynnColeArt/better-cal/backend/internal/calendars"
	"github.com/LynnColeArt/better-cal/backend/internal/config"
	"github.com/LynnColeArt/better-cal/backend/internal/credentials"
	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/LynnColeArt/better-cal/backend/internal/httpapi"
	"github.com/LynnColeArt/better-cal/backend/internal/slots"
)

func main() {
	cfg := config.FromEnv()
	slotService := slots.NewService()
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
		authRepository := auth.NewPostgresRepository(pool)
		if err := authRepository.SaveAPIKeyPrincipal(ctx, cfg.APIKey, auth.FixtureAPIKeyPrincipal()); err != nil {
			cancel()
			slog.Error("api key principal seed failed", "error", err)
			os.Exit(1)
		}
		if err := authRepository.SaveAPIKeyPrincipal(ctx, auth.FixtureWrongOwnerAPIKey, auth.FixtureWrongOwnerAPIKeyPrincipal()); err != nil {
			cancel()
			slog.Error("wrong-owner api key principal seed failed", "error", err)
			os.Exit(1)
		}
		if err := authRepository.SaveOAuthClient(ctx, auth.FixtureOAuthClient(cfg.OAuthClientID)); err != nil {
			cancel()
			slog.Error("oauth client seed failed", "error", err)
			os.Exit(1)
		}
		if err := authRepository.SavePlatformClient(ctx, cfg.PlatformClientSecret, auth.FixturePlatformClient(cfg.PlatformClientID)); err != nil {
			cancel()
			slog.Error("platform client seed failed", "error", err)
			os.Exit(1)
		}
		slotRepository := slots.NewPostgresRepository(pool)
		if err := slots.SeedFixtureAvailability(ctx, slotRepository); err != nil {
			cancel()
			slog.Error("slot availability seed failed", "error", err)
			os.Exit(1)
		}
		credentialRepository := credentials.NewPostgresRepository(pool)
		if err := credentials.SeedFixtureMetadata(ctx, credentialRepository); err != nil {
			cancel()
			slog.Error("credential metadata seed failed", "error", err)
			os.Exit(1)
		}
		integrationProvider := calendarprovider.NewGoogleFixtureProvider()
		credentialStore := credentials.NewStoreWithRepository(
			credentialRepository,
			credentials.WithStatusProvider(integrationProvider),
		)
		if err := credentialStore.RefreshProviderStatus(ctx, auth.FixtureAPIKeyPrincipal().ID); err != nil {
			cancel()
			slog.Error("credential status refresh failed", "error", err)
			os.Exit(1)
		}
		webhookSubscriptionStore := booking.NewPostgresWebhookSubscriptionStore(pool)
		if err := booking.SeedWebhookSubscriptions(ctx, webhookSubscriptionStore, booking.FixtureWebhookSubscriptions(cfg.WebhookSubscriberURL, cfg.WebhookSigningKeyRef)); err != nil {
			cancel()
			slog.Error("webhook subscription seed failed", "error", err)
			os.Exit(1)
		}
		slotService = slots.NewService(
			slots.WithRepository(slotRepository),
			slots.WithBusyTimeProvider(slotRepository),
		)
		serverOptions = append(serverOptions, httpapi.WithAuthService(
			auth.NewService(
				cfg,
				auth.WithAPIKeyPrincipalRepository(authRepository),
				auth.WithOAuthClientRepository(authRepository),
				auth.WithPlatformClientRepository(authRepository),
			),
		))
		serverOptions = append(serverOptions, httpapi.WithBookingStore(
			booking.NewStoreWithRepository(
				booking.NewPostgresRepository(pool),
				booking.WithSlotAvailabilityPort(booking.NewSlotServiceAvailabilityPort(slotService)),
			),
		))
		serverOptions = append(serverOptions, httpapi.WithCredentialStore(
			credentialStore,
		))
		calendarStore := calendars.NewStoreWithRepository(
			calendars.NewPostgresRepository(pool),
			calendars.WithCatalogProvider(integrationProvider),
			calendars.WithStatusProvider(integrationProvider),
		)
		if err := calendarStore.SyncProviderCatalog(ctx, auth.FixtureAPIKeyPrincipal().ID); err != nil {
			cancel()
			slog.Error("calendar catalog provider sync failed", "error", err)
			os.Exit(1)
		}
		if err := calendarStore.RefreshProviderConnectionStatus(ctx, auth.FixtureAPIKeyPrincipal().ID); err != nil {
			cancel()
			slog.Error("calendar connection status refresh failed", "error", err)
			os.Exit(1)
		}
		serverOptions = append(serverOptions, httpapi.WithCalendarStore(calendarStore))
		cancel()
		slog.Info("database connection ready")
	}
	serverOptions = append(serverOptions, httpapi.WithSlotService(slotService))

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
