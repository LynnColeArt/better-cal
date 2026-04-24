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
	tokenHash := sha256Hex(token)
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

func TestPostgresRepositoryExchangesOAuthAuthorizationCodeOnce(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	clientID := fmt.Sprintf("oauth-token-client-%d", time.Now().UnixNano())
	codeValue := fmt.Sprintf("oauth-code-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from oauth_tokens where client_id = $1`, clientID)
		_, _ = pool.Exec(cleanupCtx, `delete from oauth_authorization_codes where client_id = $1`, clientID)
		_, _ = pool.Exec(cleanupCtx, `delete from oauth_clients where client_id = $1`, clientID)
	})

	client := FixtureOAuthClient(clientID)
	if err := repo.SaveOAuthClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	code := FixtureOAuthAuthorizationCodeRecord(FixtureAPIKeyPrincipal(), clientID)
	code.Code = codeValue
	if err := repo.SaveOAuthAuthorizationCode(ctx, code); err != nil {
		t.Fatal(err)
	}

	token, err := repo.ExchangeOAuthAuthorizationCode(ctx, OAuthTokenExchangeRequest{
		GrantType:   "authorization_code",
		ClientID:    clientID,
		Code:        codeValue,
		RedirectURI: code.RedirectURI,
	}, time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		t.Fatalf("token response = %#v", token)
	}
	if token.Scope != "booking:read booking:write" {
		t.Fatalf("scope = %q", token.Scope)
	}
	principal, ok, err := repo.ReadOAuthAccessTokenPrincipal(ctx, token.AccessToken, time.Date(2026, 4, 24, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("issued oauth access token did not authenticate")
	}
	if principal.ID != FixtureAPIKeyPrincipal().ID || principal.Email != "fixture-user@example.test" {
		t.Fatalf("principal = %#v", principal)
	}
	if len(principal.Permissions) != 2 || principal.Permissions[0] != "booking:read" || principal.Permissions[1] != "booking:write" {
		t.Fatalf("scoped permissions = %#v", principal.Permissions)
	}
	if _, ok, err := repo.ReadOAuthAccessTokenPrincipal(ctx, token.AccessToken+"-missing", time.Date(2026, 4, 24, 12, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing oauth access token unexpectedly authenticated")
	}

	if _, err := repo.ExchangeOAuthAuthorizationCode(ctx, OAuthTokenExchangeRequest{
		GrantType:   "authorization_code",
		ClientID:    clientID,
		Code:        codeValue,
		RedirectURI: code.RedirectURI,
	}, time.Date(2026, 4, 24, 12, 1, 0, 0, time.UTC)); !errors.Is(err, ErrOAuthGrantConsumed) {
		t.Fatalf("replay err = %v", err)
	}

	var rawCodeRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from oauth_authorization_codes
		where to_jsonb(oauth_authorization_codes)::text like '%' || $1 || '%'
	`, codeValue).Scan(&rawCodeRows); err != nil {
		t.Fatal(err)
	}
	if rawCodeRows != 0 {
		t.Fatal("raw oauth authorization code was stored")
	}

	var rawTokenRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from oauth_tokens
		where to_jsonb(oauth_tokens)::text like '%' || $1 || '%'
			or to_jsonb(oauth_tokens)::text like '%' || $2 || '%'
	`, token.AccessToken, token.RefreshToken).Scan(&rawTokenRows); err != nil {
		t.Fatal(err)
	}
	if rawTokenRows != 0 {
		t.Fatal("raw oauth access or refresh token was stored")
	}

	var consumed bool
	if err := pool.QueryRow(ctx, `
		select consumed_at is not null
		from oauth_authorization_codes
		where code_sha256 = $1
	`, sha256Hex(codeValue)).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if !consumed {
		t.Fatal("authorization code was not marked consumed")
	}

	if _, err := pool.Exec(ctx, `
		update oauth_tokens
		set access_expires_at = $2
		where access_token_sha256 = $1
	`, sha256Hex(token.AccessToken), time.Date(2026, 4, 24, 11, 59, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.ReadOAuthAccessTokenPrincipal(ctx, token.AccessToken, time.Date(2026, 4, 24, 12, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("expired oauth access token unexpectedly authenticated")
	}

	if _, err := pool.Exec(ctx, `
		update oauth_tokens
		set access_expires_at = $2,
			revoked_at = $3
		where access_token_sha256 = $1
	`, sha256Hex(token.AccessToken), time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC), time.Date(2026, 4, 24, 12, 2, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.ReadOAuthAccessTokenPrincipal(ctx, token.AccessToken, time.Date(2026, 4, 24, 12, 3, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("revoked oauth access token unexpectedly authenticated")
	}
}

func TestPostgresRepositoryPersistsPlatformClientWithHashedSecret(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	clientID := fmt.Sprintf("platform-client-%d", time.Now().UnixNano())
	secret := fmt.Sprintf("platform-secret-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from platform_clients where client_id = $1`, clientID)
	})

	client := FixturePlatformClient(clientID)
	client.Permissions = []string{"booking:read"}
	client.PolicyPermissions = []string{"platform-client:read"}
	if err := repo.SavePlatformClient(ctx, secret, client); err != nil {
		t.Fatal(err)
	}

	var rawSecretRows int
	var hashLength int
	var storedHash string
	if err := pool.QueryRow(ctx, `
		select
			count(*) filter (where to_jsonb(platform_clients)::text like '%' || $2 || '%'),
			length(secret_sha256),
			secret_sha256
		from platform_clients
		where client_id = $1
		group by secret_sha256
	`, clientID, secret).Scan(&rawSecretRows, &hashLength, &storedHash); err != nil {
		t.Fatal(err)
	}
	if rawSecretRows != 0 {
		t.Fatal("raw platform client secret was stored in platform client row")
	}
	if hashLength != 64 {
		t.Fatalf("hash length = %d", hashLength)
	}
	if storedHash != sha256Hex(secret) {
		t.Fatal("stored platform client secret hash did not match expected digest")
	}

	record, ok, err := repo.ReadPlatformClient(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("platform client was not found")
	}
	if record.Client.ID != clientID {
		t.Fatalf("client id = %q", record.Client.ID)
	}
	if record.Client.OrganizationID != 456 {
		t.Fatalf("organization id = %d", record.Client.OrganizationID)
	}
	if len(record.Client.Permissions) != 1 || record.Client.Permissions[0] != "booking:read" {
		t.Fatalf("permissions = %#v", record.Client.Permissions)
	}
	if !matchesSHA256Hex(secret, record.SecretSHA256) {
		t.Fatal("stored platform secret hash did not verify")
	}

	service := NewService(testConfig(), WithPlatformClientRepository(repo))
	verified, ok, err := service.VerifyPlatformClientContext(ctx, clientID, clientID, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("platform client did not verify through repository")
	}
	if verified.ID != clientID {
		t.Fatalf("verified client id = %q", verified.ID)
	}
	if _, ok, err := service.VerifyPlatformClientContext(ctx, clientID, clientID, secret+"-wrong"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("wrong platform client secret unexpectedly verified")
	}
}

func TestPostgresRepositoryRejectsEmptyPlatformClientSeedInputs(t *testing.T) {
	pool := testPostgresPrincipalPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	if err := repo.SavePlatformClient(ctx, "secret", FixturePlatformClient("")); !errors.Is(err, ErrEmptyPlatformClientID) {
		t.Fatalf("empty client id err = %v", err)
	}
	if err := repo.SavePlatformClient(ctx, "", FixturePlatformClient("platform-client")); !errors.Is(err, ErrEmptyPlatformClientSecret) {
		t.Fatalf("empty secret err = %v", err)
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
