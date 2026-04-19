package booking

import (
	"context"
	"errors"
	"testing"
)

func TestCreateChecksSlotAvailabilityAfterDefaults(t *testing.T) {
	port := &capturingAvailabilityPort{available: true}
	store := NewStore(WithSlotAvailabilityPort(port))

	created, duplicate, err := store.Create(context.Background(), "request-id", CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("create was reported as duplicate")
	}
	if created.Start != FixtureBookingStart {
		t.Fatalf("start = %q", created.Start)
	}
	if len(port.requests) != 1 {
		t.Fatalf("availability checks = %d", len(port.requests))
	}
	request := port.requests[0]
	if request.EventTypeID != FixtureEventTypeID {
		t.Fatalf("event type id = %d", request.EventTypeID)
	}
	if request.Start != FixtureBookingStart {
		t.Fatalf("availability start = %q", request.Start)
	}
	if request.TimeZone != FixtureTimeZone {
		t.Fatalf("time zone = %q", request.TimeZone)
	}
}

func TestCreateRejectsUnavailableSlot(t *testing.T) {
	store := NewStore()

	_, _, err := store.Create(context.Background(), "request-id", CreateRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       "2026-05-01T16:00:00.000Z",
		Attendee: Attendee{
			TimeZone: FixtureTimeZone,
		},
	})

	assertValidationCode(t, err, errCodeSlotUnavailable)
}

func TestCreateReturnsAvailabilityErrors(t *testing.T) {
	store := NewStore(WithSlotAvailabilityPort(&capturingAvailabilityPort{
		err: errors.New("availability unavailable"),
	}))

	_, _, err := store.Create(context.Background(), "request-id", CreateRequest{})
	if err == nil {
		t.Fatal("expected availability error")
	}
	if _, ok := ValidationFromError(err); ok {
		t.Fatalf("error = %v, did not expect validation error", err)
	}
}

func TestCreateIdempotencyReplayDoesNotCheckDiscardedSlot(t *testing.T) {
	store := NewStore()
	created, duplicate, err := store.Create(context.Background(), "first-request", CreateRequest{
		EventTypeID:    FixtureEventTypeID,
		Start:          FixtureBookingStart,
		IdempotencyKey: "availability-replay-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("initial create was reported as duplicate")
	}

	replayed, duplicate, err := store.Create(context.Background(), "second-request", CreateRequest{
		EventTypeID:    FixtureEventTypeID,
		Start:          "2026-05-01T16:00:00.000Z",
		IdempotencyKey: "availability-replay-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate {
		t.Fatal("idempotency replay was not reported as duplicate")
	}
	if replayed.UID != created.UID {
		t.Fatalf("replayed uid = %q, want %q", replayed.UID, created.UID)
	}
}

type capturingAvailabilityPort struct {
	requests  []SlotAvailabilityRequest
	available bool
	err       error
}

func (p *capturingAvailabilityPort) IsSlotAvailable(_ context.Context, req SlotAvailabilityRequest) (bool, error) {
	p.requests = append(p.requests, req)
	if p.err != nil {
		return false, p.err
	}
	return p.available, nil
}
