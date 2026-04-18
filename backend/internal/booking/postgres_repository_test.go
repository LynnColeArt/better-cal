package booking

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryPersistsBookingFixture(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-booking-%d", time.Now().UnixNano())
	idempotencyKey := uid + "-idempotency"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	bookingValue := Booking{
		UID:         uid,
		ID:          654,
		Title:       "Repository Fixture",
		Status:      "accepted",
		Start:       "2026-05-03T15:00:00.000Z",
		End:         "2026-05-03T15:30:00.000Z",
		EventTypeID: 1001,
		Attendees: []Attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: "America/Chicago",
			},
		},
		Responses: map[string]any{
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "postgres-repository",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
		RequestID: "repo-test-request",
	}

	if err := repo.SaveCreated(ctx, bookingValue, idempotencyKey); err != nil {
		t.Fatal(err)
	}

	found, ok, err := repo.ReadByUID(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking fixture was not found by uid")
	}
	if found.UID != uid {
		t.Fatalf("uid = %q", found.UID)
	}
	if found.Metadata["fixture"] != "postgres-repository" {
		t.Fatalf("metadata = %#v", found.Metadata)
	}

	replayed, ok, err := repo.ReadByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking fixture was not found by idempotency key")
	}
	if replayed.RequestID != "repo-test-request" {
		t.Fatalf("request id = %q", replayed.RequestID)
	}

	bookingValue.Status = "cancelled"
	if err := repo.Save(ctx, bookingValue); err != nil {
		t.Fatal(err)
	}
	found, ok, err = repo.ReadByUID(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("updated booking fixture was not found")
	}
	if found.Status != "cancelled" {
		t.Fatalf("status = %q", found.Status)
	}
}

func TestPostgresRepositoryReturnsFalseForMissingRows(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	missingUID := fmt.Sprintf("missing-booking-%d", time.Now().UnixNano())
	if _, ok, err := repo.ReadByUID(ctx, missingUID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing uid was found")
	}
	if _, ok, err := repo.ReadByIdempotencyKey(ctx, missingUID+"-idempotency"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing idempotency key was found")
	}
}

func testPostgresRepositoryPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("CALDIY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("CALDIY_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set CALDIY_TEST_DATABASE_URL or CALDIY_DATABASE_URL to run Postgres repository tests")
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
