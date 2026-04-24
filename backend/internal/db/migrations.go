package db

import (
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationAdvisoryLockKey int64 = 20260424

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return ErrNilPool
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	return WithTx(ctx, pool, func(tx Tx) error {
		if _, err := tx.Exec(ctx, `
			create table if not exists better_cal_schema_migrations (
				version text primary key,
				applied_at timestamptz not null default now()
			)
		`); err != nil {
			return fmt.Errorf("create schema migrations table: %w", err)
		}
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, migrationAdvisoryLockKey); err != nil {
			return fmt.Errorf("acquire migration advisory lock: %w", err)
		}

		for _, name := range names {
			if err := applyMigration(ctx, tx, name); err != nil {
				return err
			}
		}
		return nil
	})
}

func applyMigration(ctx context.Context, tx Tx, name string) error {
	var applied bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from better_cal_schema_migrations where version = $1)`, name).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if applied {
		return nil
	}

	raw, err := migrationFiles.ReadFile(path.Join("migrations", name))
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	if _, err := tx.Exec(ctx, string(raw)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx, `insert into better_cal_schema_migrations (version) values ($1)`, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	return nil
}
