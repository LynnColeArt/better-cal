package calendars

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryRoundTripSelectedAndDestinationCalendars(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 10_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from destination_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from selected_calendars where user_id = $1`, userID)
	})

	first := SelectedCalendar{
		CalendarRef: "alpha-calendar",
		Provider:    "google-calendar-fixture",
		ExternalID:  "google-alpha-calendar",
		Name:        "Alpha Calendar",
	}
	second := SelectedCalendar{
		CalendarRef: "team-calendar",
		Provider:    "google-calendar-fixture",
		ExternalID:  "google-team-calendar",
		Name:        "Team Calendar",
	}

	if _, err := repo.SaveSelectedCalendar(ctx, userID, second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveSelectedCalendar(ctx, userID, first); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.SetDestinationCalendar(ctx, userID, second.CalendarRef); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("destination calendar was not set")
	}

	selected, err := repo.ReadSelectedCalendars(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected calendar count = %d", len(selected))
	}
	if selected[0].CalendarRef != first.CalendarRef || selected[1].CalendarRef != second.CalendarRef {
		t.Fatalf("selected calendars = %#v", selected)
	}

	destination, ok, err := repo.ReadDestinationCalendar(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("destination calendar was not found")
	}
	if destination.CalendarRef != second.CalendarRef {
		t.Fatalf("destination calendar ref = %q", destination.CalendarRef)
	}
}

func TestPostgresRepositoryDeleteSelectedCalendarClearsDestination(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 20_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from destination_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from selected_calendars where user_id = $1`, userID)
	})

	calendar := SelectedCalendar{
		CalendarRef: "team-calendar",
		Provider:    "google-calendar-fixture",
		ExternalID:  "google-team-calendar",
		Name:        "Team Calendar",
	}
	if _, err := repo.SaveSelectedCalendar(ctx, userID, calendar); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.SetDestinationCalendar(ctx, userID, calendar.CalendarRef); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("destination calendar was not set")
	}

	result, err := repo.DeleteSelectedCalendar(ctx, userID, calendar.CalendarRef)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Removed {
		t.Fatal("selected calendar was not removed")
	}
	if !result.ClearedDestination {
		t.Fatal("destination calendar was not cleared")
	}

	if _, ok, err := repo.ReadDestinationCalendar(ctx, userID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("destination calendar unexpectedly remained")
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
