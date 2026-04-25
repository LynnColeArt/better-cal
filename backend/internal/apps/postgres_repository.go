package apps

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

func (r *PostgresRepository) ReadAppCatalog(ctx context.Context) ([]AppMetadata, error) {
	rows, err := r.pool.Query(ctx, `
		select app_slug, app_category, provider, name, description, auth_type, capabilities, created_at, updated_at
		from integration_app_catalog
		order by app_slug
	`)
	if err != nil {
		return nil, fmt.Errorf("read app catalog: %w", err)
	}
	defer rows.Close()

	catalog := []AppMetadata{}
	for rows.Next() {
		var app AppMetadata
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(
			&app.AppSlug,
			&app.Category,
			&app.Provider,
			&app.Name,
			&app.Description,
			&app.AuthType,
			&app.Capabilities,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan app catalog: %w", err)
		}
		app.CreatedAt = formatWireTime(createdAt)
		app.UpdatedAt = formatWireTime(updatedAt)
		catalog = append(catalog, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read app catalog rows: %w", err)
	}
	return catalog, nil
}

func (r *PostgresRepository) SaveAppMetadata(ctx context.Context, app AppMetadata) (AppMetadata, error) {
	if err := ValidateAppMetadata(app); err != nil {
		return AppMetadata{}, err
	}
	capabilities := app.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into integration_app_catalog (
			app_slug,
			app_category,
			provider,
			name,
			description,
			auth_type,
			capabilities
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (app_slug) do update set
			app_category = excluded.app_category,
			provider = excluded.provider,
			name = excluded.name,
			description = excluded.description,
			auth_type = excluded.auth_type,
			capabilities = excluded.capabilities,
			updated_at = now()
	`, app.AppSlug, app.Category, app.Provider, app.Name, app.Description, app.AuthType, capabilities); err != nil {
		return AppMetadata{}, fmt.Errorf("save app metadata: %w", err)
	}

	items, err := r.ReadAppCatalog(ctx)
	if err != nil {
		return AppMetadata{}, err
	}
	for _, item := range items {
		if item.AppSlug == app.AppSlug {
			return item, nil
		}
	}
	return AppMetadata{}, fmt.Errorf("saved app metadata %q was not found", app.AppSlug)
}

func formatWireTime(value time.Time) string {
	return value.UTC().Format(wireTimeLayout)
}
