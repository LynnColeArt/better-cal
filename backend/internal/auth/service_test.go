package auth

import (
	"context"
	"errors"
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

func TestAuthenticateAPIKeySupportsWrongOwnerFixturePrincipal(t *testing.T) {
	service := NewService(testConfig())

	principal, ok := service.AuthenticateAPIKey("Bearer " + FixtureWrongOwnerAPIKey)
	if !ok {
		t.Fatal("expected wrong-owner fixture token to authenticate")
	}
	if principal.ID != 999 {
		t.Fatalf("principal id = %d", principal.ID)
	}
	if !hasPrincipalPermission(principal.Permissions, "booking:write") || !hasPrincipalPermission(principal.Permissions, "booking:host-action") {
		t.Fatalf("permissions = %#v", principal.Permissions)
	}
}

func TestAuthenticateAPIKeyUsesRepositoryWhenConfigured(t *testing.T) {
	service := NewService(testConfig(), WithAPIKeyPrincipalRepository(&fakeAPIKeyPrincipalRepository{
		byToken: map[string]Principal{
			"db-token": FixtureAPIKeyPrincipal(),
		},
	}))

	principal, ok, err := service.AuthenticateAPIKeyContext(context.Background(), "Bearer db-token")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected repository token to authenticate")
	}
	if principal.Email != "fixture-user@example.test" {
		t.Fatalf("email = %q", principal.Email)
	}

	if _, ok, err := service.AuthenticateAPIKeyContext(context.Background(), "Bearer cal_test_valid_mock"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("config token unexpectedly authenticated while repository was configured")
	}
}

func TestAuthenticateAPIKeyReturnsRepositoryErrors(t *testing.T) {
	sentinel := errors.New("repository unavailable")
	service := NewService(testConfig(), WithAPIKeyPrincipalRepository(&fakeAPIKeyPrincipalRepository{
		err: sentinel,
	}))

	if _, _, err := service.AuthenticateAPIKeyContext(context.Background(), "Bearer db-token"); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
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

func TestOAuthClientUsesRepositoryWhenConfigured(t *testing.T) {
	service := NewService(testConfig(), WithOAuthClientRepository(&fakeOAuthClientRepository{
		byID: map[string]OAuthClient{
			"db-oauth-client": FixtureOAuthClient("db-oauth-client"),
		},
	}))

	client, ok, err := service.OAuthClientContext(context.Background(), "db-oauth-client")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected repository oauth client")
	}
	if client.ClientID != "db-oauth-client" {
		t.Fatalf("client id = %q", client.ClientID)
	}

	if _, ok, err := service.OAuthClientContext(context.Background(), "mock-oauth-client"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("config oauth client unexpectedly resolved while repository was configured")
	}
}

func TestOAuthClientReturnsRepositoryErrors(t *testing.T) {
	sentinel := errors.New("oauth repository unavailable")
	service := NewService(testConfig(), WithOAuthClientRepository(&fakeOAuthClientRepository{
		err: sentinel,
	}))

	if _, _, err := service.OAuthClientContext(context.Background(), "db-oauth-client"); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
	}
}

func TestExchangeOAuthTokenConsumesFixtureAuthorizationCode(t *testing.T) {
	service := NewService(testConfig())
	req := OAuthTokenExchangeRequest{
		GrantType:   "authorization_code",
		ClientID:    "mock-oauth-client",
		Code:        FixtureOAuthAuthorizationCode,
		RedirectURI: "https://fixture.example.test/callback",
	}

	token, err := service.ExchangeOAuthToken(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if token.TokenType != "Bearer" {
		t.Fatalf("token type = %q", token.TokenType)
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		t.Fatalf("token response = %#v", token)
	}
	if token.Scope != "booking:read booking:write booking:host-action" {
		t.Fatalf("scope = %q", token.Scope)
	}

	if _, err := service.ExchangeOAuthToken(context.Background(), req); !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("replay err = %v", err)
	}
}

func TestAuthenticateOAuthAccessTokenUsesScopedFixturePermissions(t *testing.T) {
	service := NewService(testConfig())
	token, err := service.ExchangeOAuthToken(context.Background(), OAuthTokenExchangeRequest{
		GrantType:   "authorization_code",
		ClientID:    "mock-oauth-client",
		Code:        FixtureOAuthAuthorizationCode,
		RedirectURI: "https://fixture.example.test/callback",
	})
	if err != nil {
		t.Fatal(err)
	}

	principal, ok, err := service.AuthenticateOAuthAccessTokenContext(context.Background(), "Bearer "+token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected issued access token to authenticate")
	}
	if !hasPrincipalPermission(principal.Permissions, "booking:read") || !hasPrincipalPermission(principal.Permissions, "booking:write") || !hasPrincipalPermission(principal.Permissions, "booking:host-action") {
		t.Fatalf("permissions = %#v", principal.Permissions)
	}
	if hasPrincipalPermission(principal.Permissions, "me:read") {
		t.Fatalf("oauth access token unexpectedly retained unscoped permission: %#v", principal.Permissions)
	}

	if _, ok, err := service.AuthenticateOAuthAccessTokenContext(context.Background(), "Bearer missing-token"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing oauth access token unexpectedly authenticated")
	}
}

func TestExchangeOAuthTokenRotatesFixtureRefreshToken(t *testing.T) {
	service := NewService(testConfig())
	original, err := service.ExchangeOAuthToken(context.Background(), OAuthTokenExchangeRequest{
		GrantType:   "authorization_code",
		ClientID:    "mock-oauth-client",
		Code:        FixtureOAuthAuthorizationCode,
		RedirectURI: "https://fixture.example.test/callback",
	})
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := service.ExchangeOAuthToken(context.Background(), OAuthTokenExchangeRequest{
		GrantType:    "refresh_token",
		ClientID:     "mock-oauth-client",
		RefreshToken: original.RefreshToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.AccessToken == "" || rotated.RefreshToken == "" {
		t.Fatalf("rotated token response = %#v", rotated)
	}
	if rotated.AccessToken == original.AccessToken || rotated.RefreshToken == original.RefreshToken {
		t.Fatalf("refresh rotation reused token values: original=%#v rotated=%#v", original, rotated)
	}
	if rotated.Scope != original.Scope {
		t.Fatalf("rotated scope = %q, want %q", rotated.Scope, original.Scope)
	}
	if _, ok, err := service.AuthenticateOAuthAccessTokenContext(context.Background(), "Bearer "+original.AccessToken); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("old access token unexpectedly authenticated after refresh rotation")
	}
	if _, ok, err := service.AuthenticateOAuthAccessTokenContext(context.Background(), "Bearer "+rotated.AccessToken); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("rotated access token did not authenticate")
	}
	if _, err := service.ExchangeOAuthToken(context.Background(), OAuthTokenExchangeRequest{
		GrantType:    "refresh_token",
		ClientID:     "mock-oauth-client",
		RefreshToken: original.RefreshToken,
	}); !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("refresh replay err = %v", err)
	}
}

func TestExchangeOAuthTokenRejectsInvalidInputs(t *testing.T) {
	service := NewService(testConfig())

	tests := []struct {
		name string
		req  OAuthTokenExchangeRequest
		err  error
	}{
		{
			name: "missing grant",
			req: OAuthTokenExchangeRequest{
				ClientID:    "mock-oauth-client",
				Code:        FixtureOAuthAuthorizationCode,
				RedirectURI: "https://fixture.example.test/callback",
			},
			err: ErrInvalidOAuthTokenRequest,
		},
		{
			name: "unsupported grant",
			req: OAuthTokenExchangeRequest{
				GrantType: "client_credentials",
				ClientID:  "mock-oauth-client",
			},
			err: ErrUnsupportedOAuthGrantType,
		},
		{
			name: "missing refresh token",
			req: OAuthTokenExchangeRequest{
				GrantType: "refresh_token",
				ClientID:  "mock-oauth-client",
			},
			err: ErrInvalidOAuthTokenRequest,
		},
		{
			name: "invalid client",
			req: OAuthTokenExchangeRequest{
				GrantType:   "authorization_code",
				ClientID:    "missing-client",
				Code:        FixtureOAuthAuthorizationCode,
				RedirectURI: "https://fixture.example.test/callback",
			},
			err: ErrInvalidOAuthClient,
		},
		{
			name: "invalid redirect",
			req: OAuthTokenExchangeRequest{
				GrantType:   "authorization_code",
				ClientID:    "mock-oauth-client",
				Code:        FixtureOAuthAuthorizationCode,
				RedirectURI: "https://evil.example.test/callback",
			},
			err: ErrInvalidOAuthRedirectURI,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := service.ExchangeOAuthToken(context.Background(), testCase.req); !errors.Is(err, testCase.err) {
				t.Fatalf("err = %v, want %v", err, testCase.err)
			}
		})
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

func TestVerifyPlatformClientUsesRepositoryWhenConfigured(t *testing.T) {
	service := NewService(testConfig(), WithPlatformClientRepository(&fakePlatformClientRepository{
		byID: map[string]PlatformClientRecord{
			"db-platform-client": {
				Client:       FixturePlatformClient("db-platform-client"),
				SecretSHA256: sha256Hex("db-platform-secret"),
			},
		},
	}))

	client, ok, err := service.VerifyPlatformClientContext(
		context.Background(),
		"db-platform-client",
		"db-platform-client",
		"db-platform-secret",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected repository platform client")
	}
	if client.ID != "db-platform-client" {
		t.Fatalf("client id = %q", client.ID)
	}

	cases := []struct {
		name           string
		pathClientID   string
		headerClientID string
		secret         string
	}{
		{name: "config client not used", pathClientID: "mock-platform-client", headerClientID: "mock-platform-client", secret: "mock-platform-secret"},
		{name: "wrong header client", pathClientID: "db-platform-client", headerClientID: "wrong", secret: "db-platform-secret"},
		{name: "wrong secret", pathClientID: "db-platform-client", headerClientID: "db-platform-client", secret: "wrong"},
		{name: "empty secret", pathClientID: "db-platform-client", headerClientID: "db-platform-client", secret: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, ok, err := service.VerifyPlatformClientContext(context.Background(), testCase.pathClientID, testCase.headerClientID, testCase.secret); err != nil {
				t.Fatal(err)
			} else if ok {
				t.Fatal("credentials unexpectedly authenticated")
			}
		})
	}
}

func TestVerifyPlatformClientReturnsRepositoryErrors(t *testing.T) {
	sentinel := errors.New("platform repository unavailable")
	service := NewService(testConfig(), WithPlatformClientRepository(&fakePlatformClientRepository{
		err: sentinel,
	}))

	if _, _, err := service.VerifyPlatformClientContext(
		context.Background(),
		"db-platform-client",
		"db-platform-client",
		"db-platform-secret",
	); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
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

type fakeAPIKeyPrincipalRepository struct {
	byToken map[string]Principal
	err     error
}

func (r *fakeAPIKeyPrincipalRepository) ReadAPIKeyPrincipal(_ context.Context, token string) (Principal, bool, error) {
	if r.err != nil {
		return Principal{}, false, r.err
	}
	principal, ok := r.byToken[token]
	return principal, ok, nil
}

type fakeOAuthClientRepository struct {
	byID map[string]OAuthClient
	err  error
}

func (r *fakeOAuthClientRepository) ReadOAuthClient(_ context.Context, clientID string) (OAuthClient, bool, error) {
	if r.err != nil {
		return OAuthClient{}, false, r.err
	}
	client, ok := r.byID[clientID]
	return client, ok, nil
}

type fakePlatformClientRepository struct {
	byID map[string]PlatformClientRecord
	err  error
}

func (r *fakePlatformClientRepository) ReadPlatformClient(_ context.Context, clientID string) (PlatformClientRecord, bool, error) {
	if r.err != nil {
		return PlatformClientRecord{}, false, r.err
	}
	record, ok := r.byID[clientID]
	return record, ok, nil
}

func hasPrincipalPermission(permissions []string, expected string) bool {
	for _, permission := range permissions {
		if permission == expected {
			return true
		}
	}
	return false
}
