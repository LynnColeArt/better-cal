package credentials

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

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
