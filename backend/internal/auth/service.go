package auth

import (
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
	ID             string
	Name           string
	OrganizationID int
	Permissions    []string
	CreatedAt      string
	UpdatedAt      string
}

type Service struct {
	apiKey               string
	oauthClientID        string
	platformClientID     string
	platformClientSecret string
}

func NewService(cfg config.Config) *Service {
	return &Service{
		apiKey:               cfg.APIKey,
		oauthClientID:        cfg.OAuthClientID,
		platformClientID:     cfg.PlatformClientID,
		platformClientSecret: cfg.PlatformClientSecret,
	}
}

func (s *Service) AuthenticateAPIKey(authorization string) (Principal, bool) {
	if !secureEqual(bearerToken(authorization), s.apiKey) {
		return Principal{}, false
	}

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
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
	}, true
}

func (s *Service) OAuthClient(clientID string) (OAuthClient, bool) {
	if clientID != s.oauthClientID {
		return OAuthClient{}, false
	}

	return OAuthClient{
		ClientID:     clientID,
		Name:         "Fixture OAuth Client",
		RedirectURIs: []string{"https://fixture.example.test/callback"},
		CreatedAt:    "2026-01-01T00:00:00.000Z",
		UpdatedAt:    "2026-01-01T00:00:00.000Z",
	}, true
}

func (s *Service) VerifyPlatformClient(pathClientID string, headerClientID string, secret string) (PlatformClient, bool) {
	if pathClientID != s.platformClientID ||
		headerClientID != s.platformClientID ||
		!secureEqual(secret, s.platformClientSecret) {
		return PlatformClient{}, false
	}

	return PlatformClient{
		ID:             s.platformClientID,
		Name:           "Fixture Platform Client",
		OrganizationID: 456,
		Permissions:    []string{"booking:read", "booking:write"},
		CreatedAt:      "2026-01-01T00:00:00.000Z",
		UpdatedAt:      "2026-01-01T00:00:00.000Z",
	}, true
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
