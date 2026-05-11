package credentials

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/apps"
	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryRoundTripCredentialMetadata(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 40_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_credential_metadata where user_id = $1`, userID)
	})

	credential := CredentialMetadata{
		CredentialRef: "credential-repository-fixture",
		AppSlug:       "google-calendar",
		AppCategory:   "calendar",
		Provider:      "google-calendar-fixture",
		AccountRef:    "google-account-repository",
		AccountLabel:  "repository@example.test",
		Status:        "active",
		Scopes:        []string{"calendar.read", "calendar.write"},
	}
	if _, err := repo.SaveCredentialMetadata(ctx, userID, credential); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ReadCredentialMetadata(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("credential metadata count = %d", len(items))
	}
	if items[0].CredentialRef != credential.CredentialRef {
		t.Fatalf("credential ref = %q", items[0].CredentialRef)
	}
	if items[0].AccountLabel != credential.AccountLabel {
		t.Fatalf("account label = %q", items[0].AccountLabel)
	}
	if len(items[0].Scopes) != 2 {
		t.Fatalf("scopes = %#v", items[0].Scopes)
	}
	if items[0].CreatedAt == "" || items[0].UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", items[0])
	}
}

func TestPostgresRepositoryRefreshesCredentialStatus(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 45_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_credential_metadata where user_id = $1`, userID)
	})

	credential := CredentialMetadata{
		CredentialRef: "credential-status-fixture",
		AppSlug:       "google-calendar",
		AppCategory:   "calendar",
		Provider:      "google-calendar-fixture",
		AccountRef:    "google-account-status",
		AccountLabel:  "status@example.test",
		Status:        "active",
		Scopes:        []string{"calendar.read"},
	}
	if _, err := repo.SaveCredentialMetadata(ctx, userID, credential); err != nil {
		t.Fatal(err)
	}

	refreshed, err := repo.RefreshCredentialStatuses(ctx, userID, []CredentialStatusUpdate{
		{
			CredentialRef: credential.CredentialRef,
			Provider:      credential.Provider,
			AccountRef:    credential.AccountRef,
			Status:        "reauth_required",
			StatusCode:    "oauth_reauth_required",
		},
	}, "2026-04-24T12:00:00.000Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed) != 1 {
		t.Fatalf("credential metadata count = %d", len(refreshed))
	}
	if refreshed[0].Status != "reauth_required" {
		t.Fatalf("status = %q", refreshed[0].Status)
	}
	if refreshed[0].StatusCode != "oauth_reauth_required" {
		t.Fatalf("status code = %q", refreshed[0].StatusCode)
	}
	if refreshed[0].StatusCheckedAt != "2026-04-24T12:00:00.000Z" {
		t.Fatalf("status checked at = %q", refreshed[0].StatusCheckedAt)
	}
}

func TestPostgresProviderTokenSecretStoreWritesSealedPayload(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	secretStore := NewPostgresProviderTokenSecretStore(pool, NewFixtureProviderTokenSealer())
	store := NewProviderCredentialStore(repo, secretStore)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 50_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_provider_token_secrets where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_credential_metadata where user_id = $1`, userID)
	})

	receipt, err := store.StoreProviderCredentialSecret(ctx, apps.ProviderCredentialSecret{
		UserID:       userID,
		AppSlug:      "google-calendar",
		AppCategory:  "calendar",
		ProviderSlug: "google-calendar-fixture",
		AccountRef:   "google-account-token-secret",
		AccountLabel: "token-secret@example.test",
		Scopes:       []string{"calendar.read", "calendar.write"},
		TokenPayload: apps.ProviderTokenPayload{
			AccessToken:         "provider-access-token-secret-fixture",
			RefreshToken:        "provider-refresh-token-secret-fixture",
			TokenType:           "Bearer",
			RawProviderResponse: []byte(`{"access_token":"provider-access-token-secret-fixture","refresh_token":"provider-refresh-token-secret-fixture"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.CredentialRef == "" {
		t.Fatalf("credential receipt was empty: %#v", receipt)
	}

	var keyRef string
	var sealedPayload []byte
	var sealedPayloadSHA string
	if err := pool.QueryRow(ctx, `
		select key_ref, sealed_payload, sealed_payload_sha256
		from integration_provider_token_secrets
		where user_id = $1
			and credential_ref = $2
	`, userID, receipt.CredentialRef).Scan(&keyRef, &sealedPayload, &sealedPayloadSHA); err != nil {
		t.Fatal(err)
	}
	if keyRef != "fixture-provider-token-key-v1" {
		t.Fatalf("key ref = %q", keyRef)
	}
	if len(sealedPayload) <= 12 {
		t.Fatalf("sealed payload was too short: %d", len(sealedPayload))
	}
	if !isSHA256Hex(sealedPayloadSHA) {
		t.Fatalf("sealed payload sha = %q", sealedPayloadSHA)
	}
	body := strings.ToLower(string(sealedPayload))
	for _, forbidden := range []string{
		"provider-access-token-secret-fixture",
		"provider-refresh-token-secret-fixture",
		"access_token",
		"refresh_token",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("sealed payload exposed forbidden term %q", forbidden)
		}
	}

	var rawTokenRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from integration_provider_token_secrets
		where user_id = $1
			and (
				to_jsonb(integration_provider_token_secrets)::text like '%' || $2 || '%'
				or to_jsonb(integration_provider_token_secrets)::text like '%' || $3 || '%'
			)
	`, userID, "provider-access-token-secret-fixture", "provider-refresh-token-secret-fixture").Scan(&rawTokenRows); err != nil {
		t.Fatal(err)
	}
	if rawTokenRows != 0 {
		t.Fatal("raw provider tokens were stored in provider token secret rows")
	}
}

func TestPostgresCredentialMetadataTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_credential_metadata'
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
		for _, forbidden := range []string{"secret", "token", "encrypted", "payload", "raw_response", "error_body"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("credential metadata table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresProviderOAuthCallbackStoresSealedTokenSecret(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	appRepo := apps.NewPostgresRepository(pool)
	credentialRepo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 55_000
	slug := fmt.Sprintf("provider-oauth-secret-fixture-%d", userID)
	intentRef := fmt.Sprintf("app-intent-provider-oauth-secret-%d", userID)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_provider_token_secrets where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_credential_metadata where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_install_intents where install_intent_ref = $1`, intentRef)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_catalog where app_slug = $1`, slug)
	})

	if _, err := appRepo.SaveAppMetadata(ctx, apps.AppMetadata{
		AppSlug:      slug,
		Category:     "calendar",
		Provider:     "provider-oauth-secret-fixture",
		Name:         "Provider OAuth Secret Fixture",
		Description:  "Provider OAuth sealed token fixture.",
		AuthType:     "oauth",
		Capabilities: []string{"calendar.read"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := appRepo.SaveInstallIntent(ctx, apps.AppInstallIntent{
		InstallIntentRef: intentRef,
		UserID:           userID,
		AppSlug:          slug,
		Status:           apps.InstallIntentStatusPending,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := appRepo.MarkInstallIntentRequiresExternalAuth(ctx, userID, intentRef); err != nil {
		t.Fatal(err)
	}

	appStore := apps.NewStoreWithRepository(
		appRepo,
		apps.WithProviderOAuthExchangePort(apps.NewFixtureProviderOAuthExchangePort()),
		apps.WithProviderCredentialStore(NewProviderCredentialStore(
			credentialRepo,
			NewPostgresProviderTokenSecretStore(pool, NewFixtureProviderTokenSealer()),
		)),
	)
	descriptor, err := appStore.ReadExternalAuthDescriptor(ctx, userID, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appStore.PreflightExternalAuthCallback(ctx, userID, intentRef, apps.ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: apps.ExternalAuthCallbackStatusAuthorized,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := appStore.HandleProviderOAuthCallback(ctx, apps.ProviderOAuthCallbackRequest{
		StateRef:          descriptor.HandoffStateRef,
		AuthorizationCode: "provider-code-secret-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExchangeStatus != apps.ExternalAuthProviderExchangeCredentialReady {
		t.Fatalf("exchange status = %q", result.ExchangeStatus)
	}

	metadata, err := credentialRepo.ReadCredentialMetadata(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 {
		t.Fatalf("credential metadata count = %d", len(metadata))
	}
	var sealedRows int
	var rawRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from integration_provider_token_secrets
		where user_id = $1
			and credential_ref = $2
	`, userID, metadata[0].CredentialRef).Scan(&sealedRows); err != nil {
		t.Fatal(err)
	}
	if sealedRows != 1 {
		t.Fatalf("sealed token row count = %d", sealedRows)
	}
	if err := pool.QueryRow(ctx, `
		select count(*)
		from integration_provider_token_secrets
		where user_id = $1
			and (
				to_jsonb(integration_provider_token_secrets)::text like '%fixture-provider-access-%'
				or to_jsonb(integration_provider_token_secrets)::text like '%fixture-provider-refresh-%'
				or to_jsonb(integration_provider_token_secrets)::text like '%provider-code-secret-fixture%'
			)
	`, userID).Scan(&rawRows); err != nil {
		t.Fatal(err)
	}
	if rawRows != 0 {
		t.Fatal("provider oauth callback stored raw provider token material")
	}
}

func TestPostgresProviderTokenSecretTableHasNoRawTokenColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_provider_token_secrets'
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
		for _, forbidden := range []string{"access_token", "refresh_token", "raw_provider_response", "provider_response"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("provider token secret table has raw-token column %q", column)
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
