package credentials

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
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
		select credential_ref, app_slug, app_category, provider, account_ref, account_label, status, status_code, status_checked_at, scopes, created_at, updated_at
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
		var statusCode sql.NullString
		var statusCheckedAt sql.NullTime
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
			&statusCode,
			&statusCheckedAt,
			&credential.Scopes,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential metadata: %w", err)
		}
		if statusCode.Valid {
			credential.StatusCode = statusCode.String
		}
		if statusCheckedAt.Valid {
			credential.StatusCheckedAt = formatWireTime(statusCheckedAt.Time)
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

func (r *PostgresRepository) RefreshCredentialStatuses(ctx context.Context, userID int, updates []CredentialStatusUpdate, checkedAt string) ([]CredentialMetadata, error) {
	if checkedAt == "" {
		checkedAt = currentWireTime()
	}
	if err := validateCredentialStatusUpdatesForRepository(updates); err != nil {
		return nil, err
	}

	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		for _, update := range updates {
			tag, err := tx.Exec(ctx, `
				update integration_credential_metadata
				set status = $5,
					status_code = nullif($6, ''),
					status_checked_at = $7::timestamptz,
					updated_at = now()
				where user_id = $1
					and credential_ref = $2
					and provider = $3
					and account_ref = $4
			`, userID, update.CredentialRef, update.Provider, update.AccountRef, update.Status, update.StatusCode, checkedAt)
			if err != nil {
				return fmt.Errorf("refresh credential status: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return ErrInvalidCredentialStatusSnapshot
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return r.ReadCredentialMetadata(ctx, userID)
}

func validateCredentialStatusUpdatesForRepository(updates []CredentialStatusUpdate) error {
	seen := map[string]struct{}{}
	for _, update := range updates {
		if update.CredentialRef == "" || update.Provider == "" || update.AccountRef == "" || update.Status == "" {
			return ErrInvalidCredentialStatusSnapshot
		}
		if _, ok := seen[update.CredentialRef]; ok {
			return ErrInvalidCredentialStatusSnapshot
		}
		seen[update.CredentialRef] = struct{}{}
	}
	return nil
}

func formatWireTime(value time.Time) string {
	return value.UTC().Format(wireTimeLayout)
}
