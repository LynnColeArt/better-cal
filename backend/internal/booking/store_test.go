package booking

import "testing"

func TestCreateBookingAppliesDefaultsAndIdempotency(t *testing.T) {
	store := NewStore()
	created, duplicate := store.Create("first-request", CreateRequest{
		Start: "2026-05-01T15:00:00.000Z",
		Attendee: Attendee{
			Name:     "Fixture Attendee",
			Email:    "fixture-attendee@example.test",
			TimeZone: "America/Chicago",
		},
		IdempotencyKey: "fixture-booking-personal-basic",
	})
	if duplicate {
		t.Fatal("first create was reported as duplicate")
	}
	if created.UID != PrimaryFixtureUID {
		t.Fatalf("uid = %q", created.UID)
	}
	if created.Attendees[0].ID != 321 {
		t.Fatalf("attendee id = %d", created.Attendees[0].ID)
	}

	replayed, duplicate := store.Create("second-request", CreateRequest{
		IdempotencyKey: "fixture-booking-personal-basic",
	})
	if !duplicate {
		t.Fatal("second create was not reported as duplicate")
	}
	if replayed.RequestID != "first-request" {
		t.Fatalf("duplicate request id = %q", replayed.RequestID)
	}
}

func TestReadEnsuresPrimaryFixtureBooking(t *testing.T) {
	store := NewStore()

	found, ok := store.Read("read-request", PrimaryFixtureUID)
	if !ok {
		t.Fatal("primary fixture booking was not found")
	}
	if found.RequestID != "read-request" {
		t.Fatalf("request id = %q", found.RequestID)
	}

	if _, ok := store.Read("read-request", "missing"); ok {
		t.Fatal("missing booking was found")
	}
}

func TestCancelBookingUpdatesState(t *testing.T) {
	store := NewStore()

	result, ok := store.Cancel("cancel-request", PrimaryFixtureUID, CancelRequest{
		CancellationReason: "Fixture cancellation",
	})
	if !ok {
		t.Fatal("booking was not cancelled")
	}
	if result.Booking.Status != "cancelled" {
		t.Fatalf("status = %q", result.Booking.Status)
	}
	if len(result.SideEffects) != 3 {
		t.Fatalf("side effects = %v", result.SideEffects)
	}

	found, ok := store.Read("read-request", PrimaryFixtureUID)
	if !ok {
		t.Fatal("cancelled booking was not found")
	}
	if found.Status != "cancelled" {
		t.Fatalf("stored status = %q", found.Status)
	}
}

func TestRescheduleBookingCreatesOldAndNewBookings(t *testing.T) {
	store := NewStore()

	result, ok := store.Reschedule("reschedule-request", PrimaryFixtureUID, RescheduleRequest{
		Start: "2026-05-02T15:00:00.000Z",
	})
	if !ok {
		t.Fatal("booking was not rescheduled")
	}
	if result.OldBooking.Status != "cancelled" {
		t.Fatalf("old status = %q", result.OldBooking.Status)
	}
	if result.NewBooking.UID != RescheduledFixtureUID {
		t.Fatalf("new uid = %q", result.NewBooking.UID)
	}
	if result.NewBooking.Start != "2026-05-02T15:00:00.000Z" {
		t.Fatalf("new start = %q", result.NewBooking.Start)
	}

	found, ok := store.Read("read-request", RescheduledFixtureUID)
	if !ok {
		t.Fatal("rescheduled booking was not found")
	}
	if found.UID != RescheduledFixtureUID {
		t.Fatalf("stored uid = %q", found.UID)
	}
}

func TestLifecycleMethodsReturnFalseForMissingBookings(t *testing.T) {
	store := NewStore()

	if _, ok := store.Cancel("request-id", "missing", CancelRequest{}); ok {
		t.Fatal("missing booking was cancelled")
	}
	if _, ok := store.Reschedule("request-id", "missing", RescheduleRequest{}); ok {
		t.Fatal("missing booking was rescheduled")
	}
}
