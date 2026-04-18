package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryPersistsAPIKeyPrincipal(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	token := fmt.Sprintf("principal-token-%d", time.Now().UnixNano())
	tokenHash := apiKeyTokenHash(token)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from api_key_principals where token_sha256 = $1`, tokenHash)
	})

	principal := FixtureAPIKeyPrincipal()
	principal.Username = "repo-user"
	principal.Email = "repo-user@example.test"
	principal.Permissions = []string{"me:read", "booking:read"}
	if err := repo.SaveAPIKeyPrincipal(ctx, token, principal); err != nil {
		t.Fatal(err)
	}

	var rawTokenRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from api_key_principals
		where to_jsonb(api_key_principals)::text like '%' || $1 || '%'
	`, token).Scan(&rawTokenRows); err != nil {
		t.Fatal(err)
	}
	if rawTokenRows != 0 {
		t.Fatal("raw api key was stored in api key principal row")
	}

	found, ok, err := repo.ReadAPIKeyPrincipal(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("principal was not found")
	}
	if found.Username != "repo-user" {
		t.Fatalf("username = %q", found.Username)
	}
	if found.Email != "repo-user@example.test" {
		t.Fatalf("email = %q", found.Email)
	}
	if len(found.Permissions) != 2 || found.Permissions[0] != "me:read" || found.Permissions[1] != "booking:read" {
		t.Fatalf("permissions = %#v", found.Permissions)
	}
	if found.CreatedAt != "2026-01-01T00:00:00.000Z" {
		t.Fatalf("created at = %q", found.CreatedAt)
	}

	if _, ok, err := repo.ReadAPIKeyPrincipal(ctx, token+"-missing"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing token unexpectedly found")
	}
}

func TestPostgresRepositoryRejectsEmptyToken(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	if err := repo.SaveAPIKeyPrincipal(ctx, "", FixtureAPIKeyPrincipal()); !errors.Is(err, ErrEmptyAPIKey) {
		t.Fatalf("err = %v", err)
	}
}

func TestPostgresRepositoryPersistsOAuthClientMetadata(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	clientID := fmt.Sprintf("oauth-client-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from oauth_clients where client_id = $1`, clientID)
	})

	client := FixtureOAuthClient(clientID)
	client.RedirectURIs = []string{
		"https://fixture.example.test/callback",
		"https://fixture.example.test/secondary-callback",
	}
	if err := repo.SaveOAuthClient(ctx, client); err != nil {
		t.Fatal(err)
	}

	var secretColumnCount int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from information_schema.columns
		where table_name = 'oauth_clients' and column_name ilike '%secret%'
	`).Scan(&secretColumnCount); err != nil {
		t.Fatal(err)
	}
	if secretColumnCount != 0 {
		t.Fatal("oauth client metadata table has a secret-like column")
	}

	found, ok, err := repo.ReadOAuthClient(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("oauth client was not found")
	}
	if found.ClientID != clientID {
		t.Fatalf("client id = %q", found.ClientID)
	}
	if found.Name != "Fixture OAuth Client" {
		t.Fatalf("name = %q", found.Name)
	}
	if len(found.RedirectURIs) != 2 || found.RedirectURIs[1] != "https://fixture.example.test/secondary-callback" {
		t.Fatalf("redirect uris = %#v", found.RedirectURIs)
	}
	if found.CreatedAt != "2026-01-01T00:00:00.000Z" {
		t.Fatalf("created at = %q", found.CreatedAt)
	}

	if _, ok, err := repo.ReadOAuthClient(ctx, clientID+"-missing"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing oauth client unexpectedly found")
	}
}

func TestPostgresRepositoryRejectsEmptyOAuthClientID(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	if err := repo.SaveOAuthClient(ctx, FixtureOAuthClient("")); !errors.Is(err, ErrEmptyOAuthClientID) {
		t.Fatalf("err = %v", err)
	}
}

func testPostgresPrincipalPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("CALDIY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("CALDIY_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set CALDIY_TEST_DATABASE_URL or CALDIY_DATABASE_URL to run Postgres principal tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}
