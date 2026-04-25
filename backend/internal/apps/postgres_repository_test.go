package apps

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryRoundTripAppCatalog(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	slug := "app-catalog-repository-fixture"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_catalog where app_slug = $1`, slug)
	})

	app := AppMetadata{
		AppSlug:      slug,
		Category:     "calendar",
		Provider:     "repository-provider-fixture",
		Name:         "Repository Fixture",
		Description:  "Repository app catalog fixture.",
		AuthType:     "oauth",
		Capabilities: []string{"calendar.read", "calendar.write"},
	}
	if _, err := repo.SaveAppMetadata(ctx, app); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ReadAppCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found AppMetadata
	for _, item := range items {
		if item.AppSlug == slug {
			found = item
			break
		}
	}
	if found.AppSlug == "" {
		t.Fatalf("saved app %q was not found in catalog: %#v", slug, items)
	}
	if found.Name != app.Name {
		t.Fatalf("name = %q", found.Name)
	}
	if len(found.Capabilities) != 2 {
		t.Fatalf("capabilities = %#v", found.Capabilities)
	}
	if found.CreatedAt == "" || found.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", found)
	}
}

func TestPostgresRepositoryRoundTripInstallIntent(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	slug := "app-install-intent-repository-fixture"
	intentRef := "app-intent-repository-fixture"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_install_intents where install_intent_ref = $1`, intentRef)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_catalog where app_slug = $1`, slug)
	})

	if _, err := repo.SaveAppMetadata(ctx, AppMetadata{
		AppSlug:      slug,
		Category:     "calendar",
		Provider:     "install-intent-provider-fixture",
		Name:         "Install Intent Fixture",
		Description:  "Install intent app catalog fixture.",
		AuthType:     "oauth",
		Capabilities: []string{"calendar.read"},
	}); err != nil {
		t.Fatal(err)
	}

	intent, err := repo.SaveInstallIntent(ctx, AppInstallIntent{
		InstallIntentRef: intentRef,
		UserID:           123,
		AppSlug:          slug,
		Status:           InstallIntentStatusPending,
	})
	if err != nil {
		t.Fatal(err)
	}
	if intent.InstallIntentRef != intentRef {
		t.Fatalf("install intent ref = %q", intent.InstallIntentRef)
	}
	if intent.UserID != 123 {
		t.Fatalf("user id = %d", intent.UserID)
	}
	if intent.AppSlug != slug {
		t.Fatalf("app slug = %q", intent.AppSlug)
	}
	if intent.Status != InstallIntentStatusPending {
		t.Fatalf("status = %q", intent.Status)
	}
	if intent.CreatedAt == "" || intent.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", intent)
	}
}

func TestPostgresAppCatalogTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_catalog'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw_response", "error_body"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app catalog table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallIntentTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_intents'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw_response", "error_body"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install intent table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func testPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("CALDIY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("CALDIY_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set CALDIY_TEST_DATABASE_URL or CALDIY_DATABASE_URL to run Postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
