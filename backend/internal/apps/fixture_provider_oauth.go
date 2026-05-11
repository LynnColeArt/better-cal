package apps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type FixtureProviderOAuthExchangePort struct{}

func NewFixtureProviderOAuthExchangePort() *FixtureProviderOAuthExchangePort {
	return &FixtureProviderOAuthExchangePort{}
}

func (p *FixtureProviderOAuthExchangePort) ExchangeProviderOAuth(_ context.Context, input ProviderOAuthExchangeInput) (ProviderOAuthExchangeResult, error) {
	if input.UserID <= 0 ||
		strings.TrimSpace(input.InstallIntentRef) == "" ||
		strings.TrimSpace(input.AppSlug) == "" ||
		strings.TrimSpace(input.ProviderSlug) == "" ||
		strings.TrimSpace(input.StateRef) == "" ||
		strings.TrimSpace(input.CallbackPreflightRef) == "" ||
		strings.TrimSpace(input.AuthorizationCode) == "" {
		return ProviderOAuthExchangeResult{}, ErrInvalidProviderOAuthExchange
	}

	fingerprint := fixtureOAuthFingerprint(input.ProviderSlug, input.StateRef, input.AuthorizationCode)
	scopes := append([]string(nil), input.RequestedScopes...)
	if len(scopes) == 0 {
		scopes = []string{input.ProviderSlug + ".read"}
	}
	return ProviderOAuthExchangeResult{
		AccountRef:   input.ProviderSlug + "-account-" + fingerprint[:12],
		AccountLabel: "fixture-oauth-" + fingerprint[:8] + "@example.test",
		Scopes:       scopes,
		TokenPayload: ProviderTokenPayload{
			AccessToken:         "fixture-provider-access-" + fingerprint,
			RefreshToken:        "fixture-provider-refresh-" + fingerprint,
			TokenType:           "Bearer",
			RawProviderResponse: []byte(`{"status":"fixture_provider_oauth_exchange"}`),
		},
	}, nil
}

func fixtureOAuthFingerprint(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}
