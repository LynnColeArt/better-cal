package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOpenRequiresDatabaseURL(t *testing.T) {
	pool, err := Open(context.Background(), "")
	if pool != nil {
		t.Fatal("expected nil pool")
	}
	if !errors.Is(err, ErrMissingDatabaseURL) {
		t.Fatalf("err = %v", err)
	}
}

func TestPingRequiresPool(t *testing.T) {
	if err := Ping(context.Background(), nil); !errors.Is(err, ErrNilPool) {
		t.Fatalf("err = %v", err)
	}
}

func TestWithTxRequiresPool(t *testing.T) {
	err := WithTx(context.Background(), nil, func(Tx) error {
		t.Fatal("transaction callback should not run")
		return nil
	})
	if !errors.Is(err, ErrNilPool) {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenPingWithComposePostgres(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Ping(ctx, pool); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	for _, tableName := range []string{"booking_fixtures", "bookings", "booking_attendees", "booking_planned_side_effects", "booking_side_effect_dispatch_log", "booking_webhook_deliveries", "booking_webhook_subscriptions", "booking_webhook_delivery_attempts", "booking_calendar_dispatches", "booking_calendar_dispatch_attempts", "api_key_principals", "oauth_clients", "platform_clients"} {
		var exists bool
		if err := pool.QueryRow(ctx, `select exists(select 1 from information_schema.tables where table_name = $1)`, tableName).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("%s table was not created", tableName)
		}
	}
}

func TestWithTxCommitsAndRollsBack(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		create table if not exists better_cal_tx_test (
			id text primary key
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	prefix := fmt.Sprintf("tx-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `drop table if exists better_cal_tx_test`)
	})

	committedID := prefix + "commit"
	if err := WithTx(ctx, pool, func(tx Tx) error {
		_, err := tx.Exec(ctx, `insert into better_cal_tx_test (id) values ($1)`, committedID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertRowCount(t, ctx, pool, committedID, 1)

	rolledBackID := prefix + "rollback"
	sentinel := errors.New("rollback sentinel")
	err = WithTx(ctx, pool, func(tx Tx) error {
		_, err := tx.Exec(ctx, `insert into better_cal_tx_test (id) values ($1)`, rolledBackID)
		if err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
	}
	assertRowCount(t, ctx, pool, rolledBackID, 0)
}

func testPool(t *testing.T) *pgxpool.Pool {
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
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func assertRowCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from better_cal_tx_test where id = $1`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("row count for %q = %d, want %d", id, count, expected)
	}
}
