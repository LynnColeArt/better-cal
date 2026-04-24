package credentials

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const wireTimeLayout = "2006-01-02T15:04:05.000Z"

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadCredentialMetadata(ctx context.Context, userID int) ([]CredentialMetadata, error) {
	rows, err := r.pool.Query(ctx, `
		select credential_ref, app_slug, app_category, provider, account_ref, account_label, status, scopes, created_at, updated_at
		from integration_credential_metadata
		where user_id = $1
		order by credential_ref
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("read credential metadata: %w", err)
	}
	defer rows.Close()

	credentials := []CredentialMetadata{}
	for rows.Next() {
		var credential CredentialMetadata
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(
			&credential.CredentialRef,
			&credential.AppSlug,
			&credential.AppCategory,
			&credential.Provider,
			&credential.AccountRef,
			&credential.AccountLabel,
			&credential.Status,
			&credential.Scopes,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential metadata: %w", err)
		}
		credential.CreatedAt = formatWireTime(createdAt)
		credential.UpdatedAt = formatWireTime(updatedAt)
		credentials = append(credentials, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read credential metadata rows: %w", err)
	}
	return credentials, nil
}

func (r *PostgresRepository) SaveCredentialMetadata(ctx context.Context, userID int, credential CredentialMetadata) (CredentialMetadata, error) {
	if err := ValidateCredentialMetadata(credential); err != nil {
		return CredentialMetadata{}, err
	}
	scopes := credential.Scopes
	if scopes == nil {
		scopes = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into integration_credential_metadata (
			user_id,
			credential_ref,
			app_slug,
			app_category,
			provider,
			account_ref,
			account_label,
			status,
			scopes
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (user_id, credential_ref) do update set
			app_slug = excluded.app_slug,
			app_category = excluded.app_category,
			provider = excluded.provider,
			account_ref = excluded.account_ref,
			account_label = excluded.account_label,
			status = excluded.status,
			scopes = excluded.scopes,
			updated_at = now()
	`, userID, credential.CredentialRef, credential.AppSlug, credential.AppCategory, credential.Provider, credential.AccountRef, credential.AccountLabel, credential.Status, scopes); err != nil {
		return CredentialMetadata{}, fmt.Errorf("save credential metadata: %w", err)
	}

	items, err := r.ReadCredentialMetadata(ctx, userID)
	if err != nil {
		return CredentialMetadata{}, err
	}
	for _, item := range items {
		if item.CredentialRef == credential.CredentialRef {
			return item, nil
		}
	}
	return CredentialMetadata{}, fmt.Errorf("saved credential metadata %q was not found", credential.CredentialRef)
}

func formatWireTime(value time.Time) string {
	return value.UTC().Format(wireTimeLayout)
}
