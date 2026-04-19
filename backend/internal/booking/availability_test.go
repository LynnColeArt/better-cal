package booking

import (
	"context"
	"errors"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/slots"
)

func TestSlotServiceAvailabilityPortCallsSlotService(t *testing.T) {
	service := &capturingSlotService{available: true}
	port := NewSlotServiceAvailabilityPort(service)

	available, err := port.IsSlotAvailable(context.Background(), SlotAvailabilityRequest{
		RequestID:   "slot-port-request",
		EventTypeID: FixtureEventTypeID,
		Start:       FixtureBookingStart,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("slot service availability port returned unavailable")
	}
	if service.requestID != "slot-port-request" {
		t.Fatalf("request id = %q", service.requestID)
	}
	if service.request.EventTypeID != FixtureEventTypeID {
		t.Fatalf("event type id = %d", service.request.EventTypeID)
	}
	if service.request.Start != FixtureBookingStart {
		t.Fatalf("start = %q", service.request.Start)
	}
	if service.request.TimeZone != FixtureTimeZone {
		t.Fatalf("time zone = %q", service.request.TimeZone)
	}
}

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
	if request.RequestID != "request-id" {
		t.Fatalf("request id = %q", request.RequestID)
	}
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

func TestRescheduleChecksSlotAvailability(t *testing.T) {
	port := &capturingAvailabilityPort{available: true}
	store := NewStore(WithSlotAvailabilityPort(port))

	result, ok, err := store.Reschedule(context.Background(), "reschedule-request", PrimaryFixtureUID, RescheduleRequest{
		Start: slots.FixtureReschedule,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not rescheduled")
	}
	if result.NewBooking.Start != slots.FixtureReschedule {
		t.Fatalf("new start = %q", result.NewBooking.Start)
	}
	if len(port.requests) != 1 {
		t.Fatalf("availability checks = %d", len(port.requests))
	}
	request := port.requests[0]
	if request.RequestID != "reschedule-request" {
		t.Fatalf("request id = %q", request.RequestID)
	}
	if request.EventTypeID != FixtureEventTypeID {
		t.Fatalf("event type id = %d", request.EventTypeID)
	}
	if request.Start != slots.FixtureReschedule {
		t.Fatalf("availability start = %q", request.Start)
	}
	if request.TimeZone != FixtureTimeZone {
		t.Fatalf("time zone = %q", request.TimeZone)
	}
}

func TestRescheduleRejectsUnavailableSlot(t *testing.T) {
	store := NewStore(WithSlotAvailabilityPort(&capturingAvailabilityPort{}))

	result, ok, err := store.Reschedule(context.Background(), "reschedule-request", PrimaryFixtureUID, RescheduleRequest{
		Start: "2026-05-04T15:00:00.000Z",
	})

	if ok {
		t.Fatalf("reschedule succeeded: %#v", result)
	}
	assertValidationCode(t, err, errCodeSlotUnavailable)

	found, foundOK, readErr := store.Read(context.Background(), "read-request", PrimaryFixtureUID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !foundOK {
		t.Fatal("primary booking was not found after rejected reschedule")
	}
	if found.Status != "accepted" {
		t.Fatalf("status = %q", found.Status)
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

type capturingSlotService struct {
	requestID string
	request   slots.AvailabilityRequest
	available bool
	err       error
}

func (s *capturingSlotService) IsAvailable(_ context.Context, requestID string, req slots.AvailabilityRequest) (bool, error) {
	s.requestID = requestID
	s.request = req
	if s.err != nil {
		return false, s.err
	}
	return s.available, nil
}
