package auth

import (
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/config"
)

func TestAuthenticateAPIKeyRequiresBearerPrefix(t *testing.T) {
	service := NewService(testConfig())

	principal, ok := service.AuthenticateAPIKey("Bearer cal_test_valid_mock")
	if !ok {
		t.Fatal("expected bearer token to authenticate")
	}
	if principal.Email != "fixture-user@example.test" {
		t.Fatalf("email = %q", principal.Email)
	}

	for _, authorization := range []string{"cal_test_valid_mock", "Bearer invalid", "bearer cal_test_valid_mock", ""} {
		if _, ok := service.AuthenticateAPIKey(authorization); ok {
			t.Fatalf("authorization %q unexpectedly authenticated", authorization)
		}
	}
}

func TestAuthenticateAPIKeyRejectsEmptyConfiguredSecret(t *testing.T) {
	cfg := testConfig()
	cfg.APIKey = ""
	service := NewService(cfg)

	if _, ok := service.AuthenticateAPIKey(""); ok {
		t.Fatal("empty authorization unexpectedly authenticated")
	}
	if _, ok := service.AuthenticateAPIKey("Bearer "); ok {
		t.Fatal("empty bearer token unexpectedly authenticated")
	}
}

func TestOAuthClientLookup(t *testing.T) {
	service := NewService(testConfig())

	client, ok := service.OAuthClient("mock-oauth-client")
	if !ok {
		t.Fatal("expected fixture oauth client")
	}
	if client.ClientID != "mock-oauth-client" {
		t.Fatalf("client id = %q", client.ClientID)
	}

	if _, ok := service.OAuthClient("missing-client"); ok {
		t.Fatal("missing client unexpectedly resolved")
	}
}

func TestVerifyPlatformClientRequiresAllCredentials(t *testing.T) {
	service := NewService(testConfig())

	client, ok := service.VerifyPlatformClient("mock-platform-client", "mock-platform-client", "mock-platform-secret")
	if !ok {
		t.Fatal("expected platform client to authenticate")
	}
	if client.ID != "mock-platform-client" {
		t.Fatalf("client id = %q", client.ID)
	}

	cases := []struct {
		name           string
		pathClientID   string
		headerClientID string
		secret         string
	}{
		{name: "wrong path client", pathClientID: "wrong", headerClientID: "mock-platform-client", secret: "mock-platform-secret"},
		{name: "wrong header client", pathClientID: "mock-platform-client", headerClientID: "wrong", secret: "mock-platform-secret"},
		{name: "wrong secret", pathClientID: "mock-platform-client", headerClientID: "mock-platform-client", secret: "wrong"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, ok := service.VerifyPlatformClient(testCase.pathClientID, testCase.headerClientID, testCase.secret); ok {
				t.Fatal("credentials unexpectedly authenticated")
			}
		})
	}
}

func TestVerifyPlatformClientRejectsEmptyConfiguredSecret(t *testing.T) {
	cfg := testConfig()
	cfg.PlatformClientSecret = ""
	service := NewService(cfg)

	if _, ok := service.VerifyPlatformClient("mock-platform-client", "mock-platform-client", ""); ok {
		t.Fatal("empty platform secret unexpectedly authenticated")
	}
}

func testConfig() config.Config {
	return config.Config{
		APIKey:               "cal_test_valid_mock",
		OAuthClientID:        "mock-oauth-client",
		PlatformClientID:     "mock-platform-client",
		PlatformClientSecret: "mock-platform-secret",
	}
}
