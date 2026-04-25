package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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

func (c OAuthClient) Principal() Principal {
	return Principal{
		Type:        "oauth-client",
		Username:    c.ClientID,
		Permissions: []string{"oauth-token:exchange"},
	}
}

type OAuthAuthorizationCode struct {
	Code        string
	ClientID    string
	RedirectURI string
	Principal   Principal
	Scopes      []string
	ExpiresAt   string
	CreatedAt   string
}

type OAuthTokenExchangeRequest struct {
	GrantType    string
	ClientID     string
	Code         string
	RedirectURI  string
	RefreshToken string
}

type OAuthTokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
	Scope        string
}

type OAuthAccessTokenRecord struct {
	Principal Principal
	ExpiresAt time.Time
}

type OAuthRefreshTokenRecord struct {
	ClientID    string
	AccessToken string
	Principal   Principal
	Scopes      []string
	ExpiresAt   time.Time
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
	mu                   sync.Mutex
	apiKey               string
	oauthClientID        string
	platformClientID     string
	platformClientSecret string
	apiKeyPrincipals     APIKeyPrincipalRepository
	oauthClients         OAuthClientRepository
	oauthTokens          OAuthTokenExchangeRepository
	platformClients      PlatformClientRepository
	fixtureOAuthCodes    map[string]OAuthAuthorizationCode
	fixtureOAuthTokens   map[string]OAuthAccessTokenRecord
	fixtureOAuthRefresh  map[string]OAuthRefreshTokenRecord
}

const (
	FixtureWrongOwnerAPIKey       = "cal_test_wrong_owner_mock"
	FixtureOAuthAuthorizationCode = "mock-oauth-authorization-code"
	oauthAccessTokenTTL           = time.Hour
	oauthRefreshTokenTTL          = 30 * 24 * time.Hour
)

var (
	ErrInvalidOAuthTokenRequest   = errors.New("invalid oauth token request")
	ErrUnsupportedOAuthGrantType  = errors.New("unsupported oauth grant type")
	ErrInvalidOAuthClient         = errors.New("invalid oauth client")
	ErrInvalidOAuthGrant          = errors.New("invalid oauth grant")
	ErrOAuthGrantConsumed         = errors.New("oauth grant already consumed")
	ErrOAuthGrantExpired          = errors.New("oauth grant expired")
	ErrInvalidOAuthRedirectURI    = errors.New("invalid oauth redirect uri")
	ErrOAuthTokenGenerationFailed = errors.New("oauth token generation failed")
)

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

func WithOAuthTokenExchangeRepository(repo OAuthTokenExchangeRepository) ServiceOption {
	return func(s *Service) {
		s.oauthTokens = repo
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
	if cfg.OAuthClientID != "" {
		code := FixtureOAuthAuthorizationCodeRecord(FixtureAPIKeyPrincipal(), cfg.OAuthClientID)
		service.fixtureOAuthCodes = map[string]OAuthAuthorizationCode{
			code.Code: code,
		}
		service.fixtureOAuthTokens = map[string]OAuthAccessTokenRecord{}
		service.fixtureOAuthRefresh = map[string]OAuthRefreshTokenRecord{}
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
		if secureEqual(token, FixtureWrongOwnerAPIKey) {
			return FixtureWrongOwnerAPIKeyPrincipal(), true, nil
		}
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
			"booking:host-action",
			"slots:read",
			"selected-calendars:read",
			"selected-calendars:write",
			"destination-calendars:read",
			"destination-calendars:write",
			"calendar-connections:read",
			"calendars:read",
			"credentials:read",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
	}
}

func FixtureWrongOwnerAPIKeyPrincipal() Principal {
	principal := FixtureAPIKeyPrincipal()
	principal.ID = 999
	principal.UUID = "00000000-0000-4000-8000-000000000999"
	principal.Username = "fixture-wrong-owner"
	principal.Email = "fixture-wrong-owner@example.test"
	return principal
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

func FixtureOAuthAuthorizationCodeRecord(principal Principal, clientID string) OAuthAuthorizationCode {
	if clientID == "" {
		clientID = "mock-oauth-client"
	}
	return OAuthAuthorizationCode{
		Code:        FixtureOAuthAuthorizationCode,
		ClientID:    clientID,
		RedirectURI: FixtureOAuthClient(clientID).RedirectURIs[0],
		Principal:   principal,
		Scopes:      []string{"booking:read", "booking:write", "booking:host-action"},
		ExpiresAt:   "2026-12-31T00:00:00.000Z",
		CreatedAt:   "2026-01-01T00:00:00.000Z",
	}
}

func (s *Service) ExchangeOAuthToken(ctx context.Context, req OAuthTokenExchangeRequest) (OAuthTokenResponse, error) {
	req.GrantType = strings.TrimSpace(req.GrantType)
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.Code = strings.TrimSpace(req.Code)
	req.RedirectURI = strings.TrimSpace(req.RedirectURI)
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)

	if req.GrantType == "" || req.ClientID == "" {
		return OAuthTokenResponse{}, ErrInvalidOAuthTokenRequest
	}
	if req.GrantType != "authorization_code" && req.GrantType != "refresh_token" {
		return OAuthTokenResponse{}, ErrUnsupportedOAuthGrantType
	}
	client, ok, err := s.OAuthClientContext(ctx, req.ClientID)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	if !ok {
		return OAuthTokenResponse{}, ErrInvalidOAuthClient
	}

	switch req.GrantType {
	case "authorization_code":
		if req.Code == "" || req.RedirectURI == "" {
			return OAuthTokenResponse{}, ErrInvalidOAuthTokenRequest
		}
		if !hasRedirectURI(client.RedirectURIs, req.RedirectURI) {
			return OAuthTokenResponse{}, ErrInvalidOAuthRedirectURI
		}
		if s.oauthTokens != nil {
			return s.oauthTokens.ExchangeOAuthAuthorizationCode(ctx, req, time.Now().UTC())
		}
		return s.exchangeFixtureOAuthAuthorizationCode(req, time.Now().UTC())
	case "refresh_token":
		if req.RefreshToken == "" {
			return OAuthTokenResponse{}, ErrInvalidOAuthTokenRequest
		}
		if s.oauthTokens != nil {
			return s.oauthTokens.ExchangeOAuthRefreshToken(ctx, req, time.Now().UTC())
		}
		return s.exchangeFixtureOAuthRefreshToken(req, time.Now().UTC())
	default:
		return OAuthTokenResponse{}, ErrUnsupportedOAuthGrantType
	}
}

func (s *Service) AuthenticateOAuthAccessTokenContext(ctx context.Context, authorization string) (Principal, bool, error) {
	token := bearerToken(authorization)
	if token == "" {
		return Principal{}, false, nil
	}

	if s.oauthTokens != nil {
		return s.oauthTokens.ReadOAuthAccessTokenPrincipal(ctx, token, time.Now().UTC())
	}
	return s.authenticateFixtureOAuthAccessToken(token, time.Now().UTC())
}

func (s *Service) exchangeFixtureOAuthAuthorizationCode(req OAuthTokenExchangeRequest, issuedAt time.Time) (OAuthTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	code, ok := s.fixtureOAuthCodes[req.Code]
	if !ok {
		return OAuthTokenResponse{}, ErrInvalidOAuthGrant
	}
	if code.ClientID != req.ClientID || code.RedirectURI != req.RedirectURI {
		return OAuthTokenResponse{}, ErrInvalidOAuthGrant
	}
	expiresAt, err := parseWireTime(code.ExpiresAt)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	if !issuedAt.Before(expiresAt) {
		return OAuthTokenResponse{}, ErrOAuthGrantExpired
	}
	delete(s.fixtureOAuthCodes, req.Code)
	token, err := newOAuthTokenResponse(code.Scopes)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	if s.fixtureOAuthTokens == nil {
		s.fixtureOAuthTokens = map[string]OAuthAccessTokenRecord{}
	}
	s.fixtureOAuthTokens[token.AccessToken] = OAuthAccessTokenRecord{
		Principal: scopedOAuthPrincipal(code.Principal, code.Scopes),
		ExpiresAt: issuedAt.Add(oauthAccessTokenTTL),
	}
	if s.fixtureOAuthRefresh == nil {
		s.fixtureOAuthRefresh = map[string]OAuthRefreshTokenRecord{}
	}
	s.fixtureOAuthRefresh[token.RefreshToken] = OAuthRefreshTokenRecord{
		ClientID:    code.ClientID,
		AccessToken: token.AccessToken,
		Principal:   code.Principal,
		Scopes:      append([]string(nil), code.Scopes...),
		ExpiresAt:   issuedAt.Add(oauthRefreshTokenTTL),
	}
	return token, nil
}

func (s *Service) exchangeFixtureOAuthRefreshToken(req OAuthTokenExchangeRequest, issuedAt time.Time) (OAuthTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.fixtureOAuthRefresh[req.RefreshToken]
	if !ok {
		return OAuthTokenResponse{}, ErrInvalidOAuthGrant
	}
	if record.ClientID != req.ClientID {
		return OAuthTokenResponse{}, ErrInvalidOAuthGrant
	}
	if !issuedAt.Before(record.ExpiresAt) {
		return OAuthTokenResponse{}, ErrOAuthGrantExpired
	}
	delete(s.fixtureOAuthRefresh, req.RefreshToken)
	delete(s.fixtureOAuthTokens, record.AccessToken)

	token, err := newOAuthTokenResponse(record.Scopes)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	s.fixtureOAuthTokens[token.AccessToken] = OAuthAccessTokenRecord{
		Principal: scopedOAuthPrincipal(record.Principal, record.Scopes),
		ExpiresAt: issuedAt.Add(oauthAccessTokenTTL),
	}
	s.fixtureOAuthRefresh[token.RefreshToken] = OAuthRefreshTokenRecord{
		ClientID:    record.ClientID,
		AccessToken: token.AccessToken,
		Principal:   record.Principal,
		Scopes:      append([]string(nil), record.Scopes...),
		ExpiresAt:   issuedAt.Add(oauthRefreshTokenTTL),
	}
	return token, nil
}

func (s *Service) authenticateFixtureOAuthAccessToken(token string, now time.Time) (Principal, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.fixtureOAuthTokens[token]
	if !ok {
		return Principal{}, false, nil
	}
	if !now.Before(record.ExpiresAt) {
		return Principal{}, false, nil
	}
	return record.Principal, true, nil
}

func hasRedirectURI(redirectURIs []string, redirectURI string) bool {
	for _, candidate := range redirectURIs {
		if candidate == redirectURI {
			return true
		}
	}
	return false
}

func newOAuthTokenResponse(scopes []string) (OAuthTokenResponse, error) {
	accessToken, err := randomOAuthToken("cal_at")
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	refreshToken, err := randomOAuthToken("cal_rt")
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	return OAuthTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(oauthAccessTokenTTL.Seconds()),
		Scope:        strings.Join(scopes, " "),
	}, nil
}

func scopedOAuthPrincipal(principal Principal, scopes []string) Principal {
	scoped := principal
	scoped.Permissions = intersectStrings(principal.Permissions, scopes)
	return scoped
}

func intersectStrings(left []string, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return []string{}
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	result := make([]string, 0, len(left))
	for _, value := range left {
		if _, ok := rightSet[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func randomOAuthToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("%w: %w", ErrOAuthTokenGenerationFailed, err)
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw), nil
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
