package slots

import (
	"context"
	"testing"
)

func TestReadAvailableReturnsFixtureSlots(t *testing.T) {
	service := NewService()

	result, ok, err := service.ReadAvailable(context.Background(), "slot-request", Request{
		EventTypeID: FixtureEventTypeID,
		Start:       FixtureStart,
		End:         FixtureEnd,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("fixture event type was not found")
	}
	if result.RequestID != "slot-request" {
		t.Fatalf("request id = %q", result.RequestID)
	}
	if result.Slots["2026-05-01"][0].Time != FixtureSlotTime {
		t.Fatalf("slot time = %q", result.Slots["2026-05-01"][0].Time)
	}
}

func TestIsAvailableMatchesFixtureSlot(t *testing.T) {
	service := NewService()

	available, err := service.IsAvailable(context.Background(), "slot-request", AvailabilityRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       FixtureSlotTime,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("fixture slot was not available")
	}
}

func TestIsAvailableRejectsMissingSlot(t *testing.T) {
	service := NewService()

	available, err := service.IsAvailable(context.Background(), "slot-request", AvailabilityRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       "2026-05-01T16:00:00.000Z",
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("missing fixture slot was available")
	}
}

func TestIsAvailableReturnsFalseForUnknownEventType(t *testing.T) {
	service := NewService()

	available, err := service.IsAvailable(context.Background(), "slot-request", AvailabilityRequest{
		EventTypeID: 9999,
		Start:       FixtureSlotTime,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("unknown event type returned available")
	}
}

func TestIsAvailableRequiresStart(t *testing.T) {
	service := NewService()

	_, err := service.IsAvailable(context.Background(), "slot-request", AvailabilityRequest{
		EventTypeID: FixtureEventTypeID,
		TimeZone:    FixtureTimeZone,
	})

	validationErr, ok := ValidationFromError(err)
	if !ok {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "INVALID_START_TIME" {
		t.Fatalf("validation code = %q", validationErr.Code)
	}
}

func TestReadAvailableReturnsFalseForUnknownEventType(t *testing.T) {
	service := NewService()

	_, ok, err := service.ReadAvailable(context.Background(), "slot-request", Request{
		EventTypeID: 9999,
		Start:       FixtureStart,
		End:         FixtureEnd,
		TimeZone:    FixtureTimeZone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("unknown event type returned slots")
	}
}

func TestReadAvailableValidatesInputs(t *testing.T) {
	service := NewService()

	_, _, err := service.ReadAvailable(context.Background(), "slot-request", Request{
		EventTypeID: FixtureEventTypeID,
		Start:       "tomorrow",
		TimeZone:    FixtureTimeZone,
	})

	validationErr, ok := ValidationFromError(err)
	if !ok {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "INVALID_START_TIME" {
		t.Fatalf("validation code = %q", validationErr.Code)
	}
}
