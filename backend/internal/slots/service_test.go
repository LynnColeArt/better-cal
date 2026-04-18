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
