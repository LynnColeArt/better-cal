package calendar

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGoogleFixtureProviderPreparesCancelDispatch(t *testing.T) {
	provider := NewGoogleFixtureProvider()

	prepared, err := provider.PrepareDispatch(context.Background(), DispatchInput{
		Action:    "BOOKING_CANCELLED",
		CreatedAt: "2026-01-01T00:10:00.000Z",
		UID:       "booking-uid",
		RequestID: "request-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ContentType != "application/json" {
		t.Fatalf("content type = %q", prepared.ContentType)
	}
	if prepared.Headers[googleFixtureHeaderProvider] != googleFixtureProviderName {
		t.Fatalf("provider header = %q", prepared.Headers[googleFixtureHeaderProvider])
	}

	var request GoogleFixtureDispatchRequest
	if err := json.Unmarshal([]byte(prepared.Body), &request); err != nil {
		t.Fatal(err)
	}
	if request.Provider != googleFixtureProviderName {
		t.Fatalf("provider = %q", request.Provider)
	}
	if request.Operation != googleFixtureOperationCancelEvent {
		t.Fatalf("operation = %q", request.Operation)
	}
	if request.Event.ID != "booking-uid" {
		t.Fatalf("event id = %q", request.Event.ID)
	}
	if request.Event.Start != "" || request.Event.End != "" || request.Event.PreviousID != "" {
		t.Fatalf("unexpected event payload = %#v", request.Event)
	}
}

func TestGoogleFixtureProviderPreparesRescheduleDispatch(t *testing.T) {
	provider := NewGoogleFixtureProvider()

	prepared, err := provider.PrepareDispatch(context.Background(), DispatchInput{
		Action:        "BOOKING_RESCHEDULED",
		CreatedAt:     "2026-01-01T00:11:00.000Z",
		UID:           "booking-uid",
		RequestID:     "request-id",
		Start:         "2026-05-02T15:00:00.000Z",
		End:           "2026-05-02T15:30:00.000Z",
		RescheduleUID: "previous-booking-uid",
	})
	if err != nil {
		t.Fatal(err)
	}

	var request GoogleFixtureDispatchRequest
	if err := json.Unmarshal([]byte(prepared.Body), &request); err != nil {
		t.Fatal(err)
	}
	if request.Operation != googleFixtureOperationReschedule {
		t.Fatalf("operation = %q", request.Operation)
	}
	if request.Event.ID != "booking-uid" {
		t.Fatalf("event id = %q", request.Event.ID)
	}
	if request.Event.Start != "2026-05-02T15:00:00.000Z" {
		t.Fatalf("event start = %q", request.Event.Start)
	}
	if request.Event.End != "2026-05-02T15:30:00.000Z" {
		t.Fatalf("event end = %q", request.Event.End)
	}
	if request.Event.PreviousID != "previous-booking-uid" {
		t.Fatalf("event previous id = %q", request.Event.PreviousID)
	}
}
