package config

import "os"

const (
	defaultAPIKey               = "cal_test_valid_mock"
	defaultOAuthClientID        = "mock-oauth-client"
	defaultPlatformClientID     = "mock-platform-client"
	defaultPlatformClientSecret = "mock-platform-secret"
	defaultWebhookSubscriberURL = "https://example.invalid/caldiy/webhook"
	defaultWebhookSigningKeyRef = "fixture-booking-webhook"
	defaultWebhookSigningSecret = "mock-webhook-signing-secret"
	defaultRequestID            = "mock-request-id"
)

type Config struct {
	Addr                 string
	DatabaseURL          string
	APIKey               string
	OAuthClientID        string
	PlatformClientID     string
	PlatformClientSecret string
	WebhookSubscriberURL string
	WebhookSigningKeyRef string
	WebhookSigningSecret string
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
		WebhookSubscriberURL: env("CALDIY_WEBHOOK_SUBSCRIBER_URL", defaultWebhookSubscriberURL),
		WebhookSigningKeyRef: env("CALDIY_WEBHOOK_SIGNING_KEY_REF", defaultWebhookSigningKeyRef),
		WebhookSigningSecret: env("CALDIY_WEBHOOK_SIGNING_SECRET", defaultWebhookSigningSecret),
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
