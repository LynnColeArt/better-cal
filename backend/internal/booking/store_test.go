package booking

import (
	"context"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/slots"
)

func TestCreateBookingAppliesDefaultsAndIdempotency(t *testing.T) {
	store := NewStore()
	created, duplicate, err := store.Create(context.Background(), "first-request", CreateRequest{
		Start: "2026-05-01T15:00:00.000Z",
		Attendee: Attendee{
			Name:     "Fixture Attendee",
			Email:    "fixture-attendee@example.test",
			TimeZone: "America/Chicago",
		},
		IdempotencyKey: "fixture-booking-personal-basic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("first create was reported as duplicate")
	}
	if created.UID != PrimaryFixtureUID {
		t.Fatalf("uid = %q", created.UID)
	}
	if created.End != FixtureBookingEnd {
		t.Fatalf("end = %q", created.End)
	}
	if created.Attendees[0].ID != 321 {
		t.Fatalf("attendee id = %d", created.Attendees[0].ID)
	}

	replayed, duplicate, err := store.Create(context.Background(), "second-request", CreateRequest{
		IdempotencyKey: "fixture-booking-personal-basic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate {
		t.Fatal("second create was not reported as duplicate")
	}
	if replayed.RequestID != "first-request" {
		t.Fatalf("duplicate request id = %q", replayed.RequestID)
	}
}

func TestCreateReplaysRepositoryIdempotency(t *testing.T) {
	repositoryBooking := fixtureBooking("repo-request", Booking{
		UID: "repo-booking",
	})
	store := NewStoreWithRepository(&fakeRepository{
		byIdempotency: map[string]Booking{
			"repo-key": repositoryBooking,
		},
	})

	replayed, duplicate, err := store.Create(context.Background(), "second-request", CreateRequest{
		IdempotencyKey: "repo-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate {
		t.Fatal("repository idempotency replay was not reported as duplicate")
	}
	if replayed.UID != "repo-booking" {
		t.Fatalf("uid = %q", replayed.UID)
	}
	if replayed.RequestID != "repo-request" {
		t.Fatalf("request id = %q", replayed.RequestID)
	}
}

func TestReadEnsuresPrimaryFixtureBooking(t *testing.T) {
	store := NewStore()

	found, ok, err := store.Read(context.Background(), "read-request", PrimaryFixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("primary fixture booking was not found")
	}
	if found.RequestID != "read-request" {
		t.Fatalf("request id = %q", found.RequestID)
	}

	if _, ok, err := store.Read(context.Background(), "read-request", "missing"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing booking was found")
	}
}

func TestReadLoadsRepositoryBooking(t *testing.T) {
	repositoryBooking := fixtureBooking("repo-read-request", Booking{
		UID: "repo-read-booking",
	})
	store := NewStoreWithRepository(&fakeRepository{
		byUID: map[string]Booking{
			"repo-read-booking": repositoryBooking,
		},
	})

	found, ok, err := store.Read(context.Background(), "request-id", "repo-read-booking")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("repository booking was not found")
	}
	if found.RequestID != "repo-read-request" {
		t.Fatalf("request id = %q", found.RequestID)
	}
}

func TestCancelBookingUpdatesState(t *testing.T) {
	store := NewStore()

	result, ok, err := store.Cancel(context.Background(), "cancel-request", PrimaryFixtureUID, CancelRequest{
		CancellationReason: "Fixture cancellation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not cancelled")
	}
	if result.Booking.Status != "cancelled" {
		t.Fatalf("status = %q", result.Booking.Status)
	}
	if len(result.SideEffects) != 3 {
		t.Fatalf("side effects = %v", result.SideEffects)
	}

	found, ok, err := store.Read(context.Background(), "read-request", PrimaryFixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("cancelled booking was not found")
	}
	if found.Status != "cancelled" {
		t.Fatalf("stored status = %q", found.Status)
	}
}

func TestRescheduleBookingCreatesOldAndNewBookings(t *testing.T) {
	store := NewStore()

	result, ok, err := store.Reschedule(context.Background(), "reschedule-request", PrimaryFixtureUID, RescheduleRequest{
		Start: "2026-05-02T15:00:00.000Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not rescheduled")
	}
	if result.OldBooking.Status != "cancelled" {
		t.Fatalf("old status = %q", result.OldBooking.Status)
	}
	if result.NewBooking.UID != RescheduledFixtureUID {
		t.Fatalf("new uid = %q", result.NewBooking.UID)
	}
	if result.NewBooking.Status != "accepted" {
		t.Fatalf("new status = %q", result.NewBooking.Status)
	}
	if result.NewBooking.Start != "2026-05-02T15:00:00.000Z" {
		t.Fatalf("new start = %q", result.NewBooking.Start)
	}
	if result.NewBooking.End != "2026-05-02T15:30:00.000Z" {
		t.Fatalf("new end = %q", result.NewBooking.End)
	}

	found, ok, err := store.Read(context.Background(), "read-request", RescheduledFixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("rescheduled booking was not found")
	}
	if found.UID != RescheduledFixtureUID {
		t.Fatalf("stored uid = %q", found.UID)
	}
}

func TestRescheduleCreatesAcceptedNewBookingAfterCancellation(t *testing.T) {
	store := NewStore()
	if _, ok, err := store.Cancel(context.Background(), "cancel-request", PrimaryFixtureUID, CancelRequest{}); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("booking was not cancelled")
	}

	result, ok, err := store.Reschedule(context.Background(), "reschedule-request", PrimaryFixtureUID, RescheduleRequest{
		Start: slots.FixtureReschedule,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not rescheduled")
	}
	if result.OldBooking.Status != "cancelled" {
		t.Fatalf("old status = %q", result.OldBooking.Status)
	}
	if result.NewBooking.Status != "accepted" {
		t.Fatalf("new status = %q", result.NewBooking.Status)
	}
}

func TestConfirmPendingBooking(t *testing.T) {
	store := NewStore()

	result, ok, err := store.Confirm(context.Background(), "confirm-request", PendingConfirmFixtureUID, ConfirmRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not confirmed")
	}
	if result.Booking.Status != "accepted" {
		t.Fatalf("status = %q", result.Booking.Status)
	}
	if len(result.SideEffects) != 2 {
		t.Fatalf("side effects = %v", result.SideEffects)
	}

	found, ok, err := store.Read(context.Background(), "read-request", PendingConfirmFixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("confirmed booking was not found")
	}
	if found.Status != "accepted" {
		t.Fatalf("stored status = %q", found.Status)
	}
}

func TestDeclinePendingBooking(t *testing.T) {
	store := NewStore()

	result, ok, err := store.Decline(context.Background(), "decline-request", PendingDeclineFixtureUID, DeclineRequest{
		Reason: "Fixture decline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not declined")
	}
	if result.Booking.Status != "rejected" {
		t.Fatalf("status = %q", result.Booking.Status)
	}
	if len(result.SideEffects) != 2 {
		t.Fatalf("side effects = %v", result.SideEffects)
	}
}

func TestCreateDerivesEndFromFixtureDuration(t *testing.T) {
	store := NewStore(WithSlotAvailabilityPort(&capturingAvailabilityPort{available: true}))

	created, duplicate, err := store.Create(context.Background(), "create-request", CreateRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       "2026-08-01T10:15:00.000Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("create was reported as duplicate")
	}
	if created.End != "2026-08-01T10:45:00.000Z" {
		t.Fatalf("end = %q", created.End)
	}
}

func TestLifecycleMethodsReturnFalseForMissingBookings(t *testing.T) {
	store := NewStore()

	if _, ok, err := store.Cancel(context.Background(), "request-id", "missing", CancelRequest{}); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing booking was cancelled")
	}
	if _, ok, err := store.Reschedule(context.Background(), "request-id", "missing", RescheduleRequest{}); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing booking was rescheduled")
	}
	if _, ok, err := store.Confirm(context.Background(), "request-id", "missing", ConfirmRequest{}); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing booking was confirmed")
	}
	if _, ok, err := store.Decline(context.Background(), "request-id", "missing", DeclineRequest{}); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing booking was declined")
	}
}

type fakeRepository struct {
	byUID         map[string]Booking
	byIdempotency map[string]Booking
	sideEffects   []PlannedSideEffect
}

func (f *fakeRepository) ReadByUID(_ context.Context, uid string) (Booking, bool, error) {
	bookingValue, ok := f.byUID[uid]
	return bookingValue, ok, nil
}

func (f *fakeRepository) ReadByIdempotencyKey(_ context.Context, key string) (Booking, bool, error) {
	bookingValue, ok := f.byIdempotency[key]
	return bookingValue, ok, nil
}

func (f *fakeRepository) SaveCreated(_ context.Context, bookingValue Booking, idempotencyKey string, effects []PlannedSideEffect) (Booking, bool, error) {
	if idempotencyKey != "" {
		if existing, ok := f.byIdempotency[idempotencyKey]; ok {
			return existing, true, nil
		}
	}
	if f.byUID == nil {
		f.byUID = make(map[string]Booking)
	}
	f.byUID[bookingValue.UID] = bookingValue
	f.sideEffects = append(f.sideEffects, effects...)
	if idempotencyKey == "" {
		return bookingValue, false, nil
	}
	if f.byIdempotency == nil {
		f.byIdempotency = make(map[string]Booking)
	}
	f.byIdempotency[idempotencyKey] = bookingValue
	return bookingValue, false, nil
}

func (f *fakeRepository) Save(_ context.Context, effects []PlannedSideEffect, bookings ...Booking) error {
	if f.byUID == nil {
		f.byUID = make(map[string]Booking)
	}
	for _, bookingValue := range bookings {
		f.byUID[bookingValue.UID] = bookingValue
	}
	f.sideEffects = append(f.sideEffects, effects...)
	return nil
}
