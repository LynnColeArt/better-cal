package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const wireTimeLayout = "2006-01-02T15:04:05.000Z"

var ErrEmptyAPIKey = errors.New("empty api key")
var ErrEmptyOAuthClientID = errors.New("empty oauth client id")
var ErrEmptyPlatformClientID = errors.New("empty platform client id")
var ErrEmptyPlatformClientSecret = errors.New("empty platform client secret")
var ErrEmptyOAuthAuthorizationCode = errors.New("empty oauth authorization code")

type APIKeyPrincipalRepository interface {
	ReadAPIKeyPrincipal(ctx context.Context, token string) (Principal, bool, error)
}

type OAuthClientRepository interface {
	ReadOAuthClient(ctx context.Context, clientID string) (OAuthClient, bool, error)
}

type OAuthTokenExchangeRepository interface {
	ExchangeOAuthAuthorizationCode(ctx context.Context, req OAuthTokenExchangeRequest, issuedAt time.Time) (OAuthTokenResponse, error)
	ReadOAuthAccessTokenPrincipal(ctx context.Context, token string, now time.Time) (Principal, bool, error)
}

type PlatformClientRecord struct {
	Client       PlatformClient
	SecretSHA256 string
}

type PlatformClientRepository interface {
	ReadPlatformClient(ctx context.Context, clientID string) (PlatformClientRecord, bool, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadAPIKeyPrincipal(ctx context.Context, token string) (Principal, bool, error) {
	if token == "" {
		return Principal{}, false, nil
	}

	var principal Principal
	var createdAt time.Time
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, `
		select user_id, user_uuid, principal_type, username, email, permissions, principal_created_at, principal_updated_at
		from api_key_principals
		where token_sha256 = $1
	`, sha256Hex(token)).Scan(
		&principal.ID,
		&principal.UUID,
		&principal.Type,
		&principal.Username,
		&principal.Email,
		&principal.Permissions,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("read api key principal: %w", err)
	}

	principal.CreatedAt = formatWireTime(createdAt)
	principal.UpdatedAt = formatWireTime(updatedAt)
	return principal, true, nil
}

func (r *PostgresRepository) SaveAPIKeyPrincipal(ctx context.Context, token string, principal Principal) error {
	if token == "" {
		return ErrEmptyAPIKey
	}

	createdAt, err := parseWireTime(principal.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := parseWireTime(principal.UpdatedAt)
	if err != nil {
		return err
	}
	permissions := principal.Permissions
	if permissions == nil {
		permissions = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into api_key_principals (
			token_sha256,
			user_id,
			user_uuid,
			principal_type,
			username,
			email,
			permissions,
			principal_created_at,
			principal_updated_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (token_sha256) do update set
			user_id = excluded.user_id,
			user_uuid = excluded.user_uuid,
			principal_type = excluded.principal_type,
			username = excluded.username,
			email = excluded.email,
			permissions = excluded.permissions,
			principal_created_at = excluded.principal_created_at,
			principal_updated_at = excluded.principal_updated_at,
			updated_at = now()
	`, sha256Hex(token), principal.ID, principal.UUID, principal.Type, principal.Username, principal.Email, permissions, createdAt, updatedAt); err != nil {
		return fmt.Errorf("save api key principal: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ReadOAuthClient(ctx context.Context, clientID string) (OAuthClient, bool, error) {
	if clientID == "" {
		return OAuthClient{}, false, nil
	}

	var client OAuthClient
	var createdAt time.Time
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, `
		select client_id, name, redirect_uris, client_created_at, client_updated_at
		from oauth_clients
		where client_id = $1
	`, clientID).Scan(
		&client.ClientID,
		&client.Name,
		&client.RedirectURIs,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthClient{}, false, nil
	}
	if err != nil {
		return OAuthClient{}, false, fmt.Errorf("read oauth client: %w", err)
	}

	client.CreatedAt = formatWireTime(createdAt)
	client.UpdatedAt = formatWireTime(updatedAt)
	return client, true, nil
}

func (r *PostgresRepository) SaveOAuthClient(ctx context.Context, client OAuthClient) error {
	if client.ClientID == "" {
		return ErrEmptyOAuthClientID
	}

	createdAt, err := parseWireTime(client.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := parseWireTime(client.UpdatedAt)
	if err != nil {
		return err
	}
	redirectURIs := client.RedirectURIs
	if redirectURIs == nil {
		redirectURIs = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into oauth_clients (
			client_id,
			name,
			redirect_uris,
			client_created_at,
			client_updated_at
		)
		values ($1, $2, $3, $4, $5)
		on conflict (client_id) do update set
			name = excluded.name,
			redirect_uris = excluded.redirect_uris,
			client_created_at = excluded.client_created_at,
			client_updated_at = excluded.client_updated_at,
			updated_at = now()
	`, client.ClientID, client.Name, redirectURIs, createdAt, updatedAt); err != nil {
		return fmt.Errorf("save oauth client: %w", err)
	}
	return nil
}

func (r *PostgresRepository) SaveOAuthAuthorizationCode(ctx context.Context, code OAuthAuthorizationCode) error {
	if code.Code == "" {
		return ErrEmptyOAuthAuthorizationCode
	}
	if code.ClientID == "" {
		return ErrEmptyOAuthClientID
	}
	if code.RedirectURI == "" || code.Principal.ID == 0 || code.Principal.UUID == "" || code.Principal.Type == "" {
		return ErrInvalidOAuthTokenRequest
	}
	expiresAt, err := parseWireTime(code.ExpiresAt)
	if err != nil {
		return err
	}
	createdAt, err := parseWireTime(code.CreatedAt)
	if err != nil {
		return err
	}
	permissions := code.Principal.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	scopes := code.Scopes
	if scopes == nil {
		scopes = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into oauth_authorization_codes (
			code_sha256,
			client_id,
			redirect_uri,
			user_id,
			user_uuid,
			principal_type,
			username,
			email,
			permissions,
			scopes,
			expires_at,
			code_created_at,
			consumed_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, null)
		on conflict (code_sha256) do update set
			client_id = excluded.client_id,
			redirect_uri = excluded.redirect_uri,
			user_id = excluded.user_id,
			user_uuid = excluded.user_uuid,
			principal_type = excluded.principal_type,
			username = excluded.username,
			email = excluded.email,
			permissions = excluded.permissions,
			scopes = excluded.scopes,
			expires_at = excluded.expires_at,
			code_created_at = excluded.code_created_at,
			consumed_at = null,
			updated_at = now()
	`, sha256Hex(code.Code), code.ClientID, code.RedirectURI, code.Principal.ID, code.Principal.UUID, code.Principal.Type, code.Principal.Username, code.Principal.Email, permissions, scopes, expiresAt, createdAt); err != nil {
		return fmt.Errorf("save oauth authorization code: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ExchangeOAuthAuthorizationCode(ctx context.Context, req OAuthTokenExchangeRequest, issuedAt time.Time) (OAuthTokenResponse, error) {
	var response OAuthTokenResponse
	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		code, err := readOAuthAuthorizationCodeTx(ctx, tx, req.Code)
		if err != nil {
			return err
		}
		if code.clientID != req.ClientID || code.redirectURI != req.RedirectURI {
			return ErrInvalidOAuthGrant
		}
		if code.consumedAt.Valid {
			return ErrOAuthGrantConsumed
		}
		if !issuedAt.Before(code.expiresAt) {
			return ErrOAuthGrantExpired
		}

		token, err := newOAuthTokenResponse(code.scopes)
		if err != nil {
			return err
		}
		accessExpiresAt := issuedAt.Add(oauthAccessTokenTTL)
		refreshExpiresAt := issuedAt.Add(oauthRefreshTokenTTL)
		if _, err := tx.Exec(ctx, `
			update oauth_authorization_codes
			set consumed_at = $2,
				updated_at = now()
			where code_sha256 = $1
				and consumed_at is null
		`, sha256Hex(req.Code), issuedAt); err != nil {
			return fmt.Errorf("consume oauth authorization code: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			insert into oauth_tokens (
				access_token_sha256,
				refresh_token_sha256,
				client_id,
				user_id,
				user_uuid,
				principal_type,
				username,
				email,
				permissions,
				scopes,
				access_expires_at,
				refresh_expires_at
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, sha256Hex(token.AccessToken), sha256Hex(token.RefreshToken), code.clientID, code.principal.ID, code.principal.UUID, code.principal.Type, code.principal.Username, code.principal.Email, code.principal.Permissions, code.scopes, accessExpiresAt, refreshExpiresAt); err != nil {
			return fmt.Errorf("persist oauth token hashes: %w", err)
		}
		response = token
		return nil
	}); err != nil {
		return OAuthTokenResponse{}, err
	}
	return response, nil
}

func (r *PostgresRepository) ReadOAuthAccessTokenPrincipal(ctx context.Context, token string, now time.Time) (Principal, bool, error) {
	if token == "" {
		return Principal{}, false, nil
	}

	var principal Principal
	var permissions []string
	var scopes []string
	var createdAt time.Time
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, `
		select
			user_id,
			user_uuid,
			principal_type,
			username,
			email,
			permissions,
			scopes,
			created_at,
			updated_at
		from oauth_tokens
		where access_token_sha256 = $1
			and access_expires_at > $2
			and revoked_at is null
	`, sha256Hex(token), now).Scan(
		&principal.ID,
		&principal.UUID,
		&principal.Type,
		&principal.Username,
		&principal.Email,
		&permissions,
		&scopes,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("read oauth access token principal: %w", err)
	}

	principal.Permissions = intersectStrings(permissions, scopes)
	principal.CreatedAt = formatWireTime(createdAt)
	principal.UpdatedAt = formatWireTime(updatedAt)
	return principal, true, nil
}

func (r *PostgresRepository) ReadPlatformClient(ctx context.Context, clientID string) (PlatformClientRecord, bool, error) {
	if clientID == "" {
		return PlatformClientRecord{}, false, nil
	}

	var record PlatformClientRecord
	var createdAt time.Time
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, `
		select client_id, name, organization_id, permissions, policy_permissions, client_created_at, client_updated_at, secret_sha256
		from platform_clients
		where client_id = $1
	`, clientID).Scan(
		&record.Client.ID,
		&record.Client.Name,
		&record.Client.OrganizationID,
		&record.Client.Permissions,
		&record.Client.PolicyPermissions,
		&createdAt,
		&updatedAt,
		&record.SecretSHA256,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlatformClientRecord{}, false, nil
	}
	if err != nil {
		return PlatformClientRecord{}, false, fmt.Errorf("read platform client: %w", err)
	}

	record.Client.CreatedAt = formatWireTime(createdAt)
	record.Client.UpdatedAt = formatWireTime(updatedAt)
	return record, true, nil
}

func (r *PostgresRepository) SavePlatformClient(ctx context.Context, secret string, client PlatformClient) error {
	if client.ID == "" {
		return ErrEmptyPlatformClientID
	}
	if secret == "" {
		return ErrEmptyPlatformClientSecret
	}

	createdAt, err := parseWireTime(client.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := parseWireTime(client.UpdatedAt)
	if err != nil {
		return err
	}
	permissions := client.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	policyPermissions := client.PolicyPermissions
	if policyPermissions == nil {
		policyPermissions = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into platform_clients (
			client_id,
			secret_sha256,
			name,
			organization_id,
			permissions,
			policy_permissions,
			client_created_at,
			client_updated_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (client_id) do update set
			secret_sha256 = excluded.secret_sha256,
			name = excluded.name,
			organization_id = excluded.organization_id,
			permissions = excluded.permissions,
			policy_permissions = excluded.policy_permissions,
			client_created_at = excluded.client_created_at,
			client_updated_at = excluded.client_updated_at,
			updated_at = now()
	`, client.ID, sha256Hex(secret), client.Name, client.OrganizationID, permissions, policyPermissions, createdAt, updatedAt); err != nil {
		return fmt.Errorf("save platform client: %w", err)
	}
	return nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type oauthAuthorizationCodeRecord struct {
	clientID    string
	redirectURI string
	principal   Principal
	scopes      []string
	expiresAt   time.Time
	consumedAt  sql.NullTime
}

func readOAuthAuthorizationCodeTx(ctx context.Context, tx db.Tx, code string) (oauthAuthorizationCodeRecord, error) {
	if code == "" {
		return oauthAuthorizationCodeRecord{}, ErrInvalidOAuthGrant
	}
	var record oauthAuthorizationCodeRecord
	err := tx.QueryRow(ctx, `
		select
			client_id,
			redirect_uri,
			user_id,
			user_uuid,
			principal_type,
			username,
			email,
			permissions,
			scopes,
			expires_at,
			consumed_at
		from oauth_authorization_codes
		where code_sha256 = $1
		for update
	`, sha256Hex(code)).Scan(
		&record.clientID,
		&record.redirectURI,
		&record.principal.ID,
		&record.principal.UUID,
		&record.principal.Type,
		&record.principal.Username,
		&record.principal.Email,
		&record.principal.Permissions,
		&record.scopes,
		&record.expiresAt,
		&record.consumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauthAuthorizationCodeRecord{}, ErrInvalidOAuthGrant
	}
	if err != nil {
		return oauthAuthorizationCodeRecord{}, fmt.Errorf("read oauth authorization code: %w", err)
	}
	return record, nil
}

func parseWireTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse principal timestamp: %w", err)
	}
	return parsed, nil
}

func formatWireTime(value time.Time) string {
	return value.UTC().Format(wireTimeLayout)
}
