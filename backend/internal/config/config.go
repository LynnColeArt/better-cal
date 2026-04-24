package config

import (
	"os"
	"strconv"
)

const (
	defaultAPIKey               = "cal_test_valid_mock"
	defaultOAuthClientID        = "mock-oauth-client"
	defaultPlatformClientID     = "mock-platform-client"
	defaultPlatformClientSecret = "mock-platform-secret"
	defaultCalendarDispatchURL  = "http://webhook-sink:8090/caldiy/calendar-dispatch"
	defaultWebhookSubscriberURL = "https://example.invalid/caldiy/webhook"
	defaultWebhookSigningKeyRef = "fixture-booking-webhook"
	defaultWebhookSigningSecret = "mock-webhook-signing-secret"
	defaultWebhookMaxAttempts   = 3
	defaultRequestID            = "mock-request-id"
)

type Config struct {
	Addr                 string
	DatabaseURL          string
	APIKey               string
	OAuthClientID        string
	PlatformClientID     string
	PlatformClientSecret string
	CalendarDispatchURL  string
	WebhookSubscriberURL string
	WebhookSigningKeyRef string
	WebhookSigningSecret string
	WebhookMaxAttempts   int
	RequestID            string
}

func FromEnv() Config {
	port := env("PORT", "8080")
	host := env("HOST", "127.0.0.1")
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = host + ":" + port
	}

	return Config{
		Addr:                 addr,
		DatabaseURL:          os.Getenv("CALDIY_DATABASE_URL"),
		APIKey:               env("CALDIY_API_KEY", defaultAPIKey),
		OAuthClientID:        env("CALDIY_OAUTH_CLIENT_ID", defaultOAuthClientID),
		PlatformClientID:     env("CALDIY_PLATFORM_CLIENT_ID", defaultPlatformClientID),
		PlatformClientSecret: env("CALDIY_PLATFORM_CLIENT_SECRET", defaultPlatformClientSecret),
		CalendarDispatchURL:  env("CALDIY_CALENDAR_DISPATCH_URL", defaultCalendarDispatchURL),
		WebhookSubscriberURL: env("CALDIY_WEBHOOK_SUBSCRIBER_URL", defaultWebhookSubscriberURL),
		WebhookSigningKeyRef: env("CALDIY_WEBHOOK_SIGNING_KEY_REF", defaultWebhookSigningKeyRef),
		WebhookSigningSecret: env("CALDIY_WEBHOOK_SIGNING_SECRET", defaultWebhookSigningSecret),
		WebhookMaxAttempts:   envInt("CALDIY_WEBHOOK_MAX_ATTEMPTS", defaultWebhookMaxAttempts),
		RequestID:            env("CALDIY_REQUEST_ID", defaultRequestID),
	}
}

func env(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
