package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/LynnColeArt/better-cal/backend/internal/config"
)

type Principal struct {
	ID          int
	UUID        string
	Type        string
	Username    string
	Email       string
	Permissions []string
	CreatedAt   string
	UpdatedAt   string
}

type OAuthClient struct {
	ClientID     string
	Name         string
	RedirectURIs []string
	CreatedAt    string
	UpdatedAt    string
}

type PlatformClient struct {
	ID                string
	Name              string
	OrganizationID    int
	Permissions       []string
	PolicyPermissions []string
	CreatedAt         string
	UpdatedAt         string
}

func (c PlatformClient) Principal() Principal {
	return Principal{
		Type:        "platform-client",
		Permissions: c.PolicyPermissions,
	}
}

type Service struct {
	apiKey               string
	oauthClientID        string
	platformClientID     string
	platformClientSecret string
	apiKeyPrincipals     APIKeyPrincipalRepository
	oauthClients         OAuthClientRepository
	platformClients      PlatformClientRepository
}

type ServiceOption func(*Service)

func WithAPIKeyPrincipalRepository(repo APIKeyPrincipalRepository) ServiceOption {
	return func(s *Service) {
		s.apiKeyPrincipals = repo
	}
}

func WithOAuthClientRepository(repo OAuthClientRepository) ServiceOption {
	return func(s *Service) {
		s.oauthClients = repo
	}
}

func WithPlatformClientRepository(repo PlatformClientRepository) ServiceOption {
	return func(s *Service) {
		s.platformClients = repo
	}
}

func NewService(cfg config.Config, opts ...ServiceOption) *Service {
	service := &Service{
		apiKey:               cfg.APIKey,
		oauthClientID:        cfg.OAuthClientID,
		platformClientID:     cfg.PlatformClientID,
		platformClientSecret: cfg.PlatformClientSecret,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *Service) AuthenticateAPIKey(authorization string) (Principal, bool) {
	principal, ok, _ := s.AuthenticateAPIKeyContext(context.Background(), authorization)
	return principal, ok
}

func (s *Service) AuthenticateAPIKeyContext(ctx context.Context, authorization string) (Principal, bool, error) {
	token := bearerToken(authorization)
	if s.apiKeyPrincipals != nil {
		if token == "" {
			return Principal{}, false, nil
		}
		return s.apiKeyPrincipals.ReadAPIKeyPrincipal(ctx, token)
	}

	if !secureEqual(token, s.apiKey) {
		return Principal{}, false, nil
	}

	return FixtureAPIKeyPrincipal(), true, nil
}

func FixtureAPIKeyPrincipal() Principal {
	return Principal{
		ID:       123,
		UUID:     "00000000-0000-4000-8000-000000000123",
		Type:     "user",
		Username: "fixture-user",
		Email:    "fixture-user@example.test",
		Permissions: []string{
			"me:read",
			"oauth-client:read",
			"booking:read",
			"booking:write",
			"slots:read",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
	}
}

func (s *Service) OAuthClient(clientID string) (OAuthClient, bool) {
	client, ok, _ := s.OAuthClientContext(context.Background(), clientID)
	return client, ok
}

func (s *Service) OAuthClientContext(ctx context.Context, clientID string) (OAuthClient, bool, error) {
	if s.oauthClients != nil {
		if clientID == "" {
			return OAuthClient{}, false, nil
		}
		return s.oauthClients.ReadOAuthClient(ctx, clientID)
	}

	if clientID != s.oauthClientID {
		return OAuthClient{}, false, nil
	}

	return FixtureOAuthClient(clientID), true, nil
}

func FixtureOAuthClient(clientID string) OAuthClient {
	return OAuthClient{
		ClientID:     clientID,
		Name:         "Fixture OAuth Client",
		RedirectURIs: []string{"https://fixture.example.test/callback"},
		CreatedAt:    "2026-01-01T00:00:00.000Z",
		UpdatedAt:    "2026-01-01T00:00:00.000Z",
	}
}

func (s *Service) VerifyPlatformClient(pathClientID string, headerClientID string, secret string) (PlatformClient, bool) {
	client, ok, _ := s.VerifyPlatformClientContext(context.Background(), pathClientID, headerClientID, secret)
	return client, ok
}

func (s *Service) VerifyPlatformClientContext(ctx context.Context, pathClientID string, headerClientID string, secret string) (PlatformClient, bool, error) {
	if pathClientID == "" || pathClientID != headerClientID {
		return PlatformClient{}, false, nil
	}

	if s.platformClients != nil {
		record, ok, err := s.platformClients.ReadPlatformClient(ctx, pathClientID)
		if err != nil {
			return PlatformClient{}, false, err
		}
		if !ok {
			_ = matchesSHA256Hex(secret, zeroSHA256Hash)
			return PlatformClient{}, false, nil
		}
		if !matchesSHA256Hex(secret, record.SecretSHA256) {
			return PlatformClient{}, false, nil
		}
		return record.Client, true, nil
	}

	if pathClientID != s.platformClientID || !secureEqual(secret, s.platformClientSecret) {
		return PlatformClient{}, false, nil
	}

	return FixturePlatformClient(s.platformClientID), true, nil
}

func FixturePlatformClient(clientID string) PlatformClient {
	return PlatformClient{
		ID:                clientID,
		Name:              "Fixture Platform Client",
		OrganizationID:    456,
		Permissions:       []string{"booking:read", "booking:write"},
		PolicyPermissions: []string{"platform-client:read"},
		CreatedAt:         "2026-01-01T00:00:00.000Z",
		UpdatedAt:         "2026-01-01T00:00:00.000Z",
	}
}

func bearerToken(authorization string) string {
	if !strings.HasPrefix(authorization, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authorization, "Bearer ")
}

func secureEqual(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

const zeroSHA256Hash = "0000000000000000000000000000000000000000000000000000000000000000"

func matchesSHA256Hex(value string, expectedHash string) bool {
	actualHash := sha256Hex(value)
	validHash := isSHA256Hex(expectedHash)
	if !validHash {
		expectedHash = zeroSHA256Hash
	}
	matches := subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
	return value != "" && validHash && matches
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		switch {
		case char >= '0' && char <= '9':
		case char >= 'a' && char <= 'f':
		default:
			return false
		}
	}
	return true
}
