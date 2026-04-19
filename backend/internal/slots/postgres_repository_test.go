package slots

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryPersistsAvailabilitySlots(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventTypeID := 200000 + int(time.Now().UnixNano()%100000)
	slotTime := "2026-06-01T15:00:00.000Z"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from event_types where event_type_id = $1`, eventTypeID)
	})

	repo := NewPostgresRepository(pool)
	if err := repo.SaveEventType(ctx, EventType{
		ID:       eventTypeID,
		Title:    "Repository Availability Fixture",
		Duration: 45,
		TimeZone: FixtureTimeZone,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveAvailabilitySlot(ctx, AvailabilitySlot{
		EventTypeID: eventTypeID,
		Time:        slotTime,
		Duration:    45,
		TimeZone:    FixtureTimeZone,
	}); err != nil {
		t.Fatal(err)
	}

	service := NewService(WithRepository(repo))
	result, ok, err := service.ReadAvailable(ctx, "slot-repo-request", Request{
		EventTypeID: eventTypeID,
		Start:       "2026-06-01T00:00:00.000Z",
		End:         "2026-06-02T00:00:00.000Z",
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("repository event type was not found")
	}
	if result.RequestID != "slot-repo-request" {
		t.Fatalf("request id = %q", result.RequestID)
	}
	if result.Slots["2026-06-01"][0].Time != slotTime {
		t.Fatalf("slot time = %q", result.Slots["2026-06-01"][0].Time)
	}
	if result.Slots["2026-06-01"][0].Duration != 45 {
		t.Fatalf("duration = %d", result.Slots["2026-06-01"][0].Duration)
	}

	available, err := service.IsAvailable(ctx, "slot-repo-request", AvailabilityRequest{
		EventTypeID: eventTypeID,
		Start:       slotTime,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("persisted fixture slot was not available")
	}

	available, err = service.IsAvailable(ctx, "slot-repo-request", AvailabilityRequest{
		EventTypeID: eventTypeID,
		Start:       "2026-06-01T16:00:00.000Z",
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("missing persisted fixture slot was available")
	}
}

func TestPostgresRepositoryFiltersAcceptedBookingBusyTimes(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventTypeID := 300000 + int(time.Now().UnixNano()%100000)
	bookingUID := "busy-slot-booking-" + time.Now().Format("20060102150405.000000000")
	slotTime := "2026-07-01T15:00:00.000Z"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, bookingUID)
		_, _ = pool.Exec(cleanupCtx, `delete from event_types where event_type_id = $1`, eventTypeID)
	})

	repo := NewPostgresRepository(pool)
	if err := repo.SaveEventType(ctx, EventType{
		ID:       eventTypeID,
		Title:    "Busy Time Fixture",
		Duration: 30,
		TimeZone: FixtureTimeZone,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveAvailabilitySlot(ctx, AvailabilitySlot{
		EventTypeID: eventTypeID,
		Time:        slotTime,
		Duration:    30,
		TimeZone:    FixtureTimeZone,
	}); err != nil {
		t.Fatal(err)
	}
	insertAcceptedBooking(t, ctx, pool, bookingUID, eventTypeID, slotTime, "2026-07-01T15:30:00.000Z")

	service := NewService(WithRepository(repo), WithBusyTimeProvider(repo))
	result, ok, err := service.ReadAvailable(ctx, "busy-slot-request", Request{
		EventTypeID: eventTypeID,
		Start:       "2026-07-01T00:00:00.000Z",
		End:         "2026-07-02T00:00:00.000Z",
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("repository event type was not found")
	}
	if len(result.Slots) != 0 {
		t.Fatalf("slots = %#v, want accepted booking to block persisted slot", result.Slots)
	}

	available, err := service.IsAvailable(ctx, "busy-slot-request", AvailabilityRequest{
		EventTypeID: eventTypeID,
		Start:       slotTime,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("accepted booking did not block booking availability")
	}

	if _, err := pool.Exec(ctx, `update bookings set status = 'cancelled' where uid = $1`, bookingUID); err != nil {
		t.Fatal(err)
	}
	available, err = service.IsAvailable(ctx, "busy-slot-request", AvailabilityRequest{
		EventTypeID: eventTypeID,
		Start:       slotTime,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("cancelled booking still blocked booking availability")
	}
}

func TestSeedFixtureAvailabilityIsReadable(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	if err := SeedFixtureAvailability(ctx, repo); err != nil {
		t.Fatal(err)
	}

	service := NewService(WithRepository(repo))
	available, err := service.IsAvailable(ctx, "fixture-seed-request", AvailabilityRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       FixtureSlotTime,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("seeded fixture slot was not available")
	}
}

func TestPostgresRepositoryReturnsFalseForUnknownEventType(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	service := NewService(WithRepository(NewPostgresRepository(pool)))
	_, ok, err := service.ReadAvailable(ctx, "missing-event-type-request", Request{
		EventTypeID: 299999,
		Start:       FixtureStart,
		End:         FixtureEnd,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("unknown event type returned availability")
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

func insertAcceptedBooking(t *testing.T, ctx context.Context, pool *pgxpool.Pool, uid string, eventTypeID int, start string, end string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		insert into bookings (
			uid,
			booking_id,
			title,
			status,
			start_time,
			end_time,
			event_type_id,
			responses,
			metadata,
			created_at_wire,
			updated_at_wire,
			request_id
		)
		values ($1, 987654, 'Busy Time Fixture', 'accepted', $2, $3, $4, '{}', '{}', '2026-01-01T00:00:00.000Z', '2026-01-01T00:00:00.000Z', 'busy-time-test')
	`, uid, start, end, eventTypeID); err != nil {
		t.Fatal(err)
	}
}
