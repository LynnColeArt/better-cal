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

type APIKeyPrincipalRepository interface {
	ReadAPIKeyPrincipal(ctx context.Context, token string) (Principal, bool, error)
}

type PostgresPrincipalRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPrincipalRepository(pool *pgxpool.Pool) *PostgresPrincipalRepository {
	return &PostgresPrincipalRepository{pool: pool}
}

func (r *PostgresPrincipalRepository) ReadAPIKeyPrincipal(ctx context.Context, token string) (Principal, bool, error) {
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
	`, apiKeyTokenHash(token)).Scan(
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

func (r *PostgresPrincipalRepository) SaveAPIKeyPrincipal(ctx context.Context, token string, principal Principal) error {
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
	`, apiKeyTokenHash(token), principal.ID, principal.UUID, principal.Type, principal.Username, principal.Email, permissions, createdAt, updatedAt); err != nil {
		return fmt.Errorf("save api key principal: %w", err)
	}
	return nil
}

func apiKeyTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
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
