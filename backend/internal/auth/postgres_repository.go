package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const wireTimeLayout = "2006-01-02T15:04:05.000Z"

var ErrEmptyAPIKey = errors.New("empty api key")
var ErrEmptyOAuthClientID = errors.New("empty oauth client id")
var ErrEmptyPlatformClientID = errors.New("empty platform client id")
var ErrEmptyPlatformClientSecret = errors.New("empty platform client secret")

type APIKeyPrincipalRepository interface {
	ReadAPIKeyPrincipal(ctx context.Context, token string) (Principal, bool, error)
}

type OAuthClientRepository interface {
	ReadOAuthClient(ctx context.Context, clientID string) (OAuthClient, bool, error)
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
