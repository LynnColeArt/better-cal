package email

import (
	"context"
	"encoding/json"
	"testing"
)

func TestResendFixtureProviderPreparesCancelledDispatch(t *testing.T) {
	provider := NewResendFixtureProvider()

	prepared, err := provider.PrepareDispatch(context.Background(), DispatchInput{
		Action:             "BOOKING_CANCELLED",
		CreatedAt:          "2026-01-01T00:10:00.000Z",
		UID:                "booking-uid",
		RequestID:          "request-id",
		Recipients:         []Recipient{{Name: "Recipient", Email: "recipient@example.test", TimeZone: "America/Chicago"}},
		CancellationReason: "Fixture cancellation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ContentType != "application/json" {
		t.Fatalf("content type = %q", prepared.ContentType)
	}
	if prepared.Headers[resendFixtureHeaderProvider] != resendFixtureProviderName {
		t.Fatalf("provider header = %q", prepared.Headers[resendFixtureHeaderProvider])
	}

	var request ResendFixtureDispatchRequest
	if err := json.Unmarshal([]byte(prepared.Body), &request); err != nil {
		t.Fatal(err)
	}
	if request.Provider != resendFixtureProviderName {
		t.Fatalf("provider = %q", request.Provider)
	}
	if request.Template != "booking-cancelled" {
		t.Fatalf("template = %q", request.Template)
	}
	if len(request.Message.To) != 1 {
		t.Fatalf("recipient count = %d", len(request.Message.To))
	}
	if request.Message.To[0].Email != "recipient@example.test" {
		t.Fatalf("recipient email = %q", request.Message.To[0].Email)
	}
	if request.Message.Variables.CancellationReason != "Fixture cancellation" {
		t.Fatalf("cancellation reason = %q", request.Message.Variables.CancellationReason)
	}
}

func TestResendFixtureProviderPreparesRescheduledDispatch(t *testing.T) {
	provider := NewResendFixtureProvider()

	prepared, err := provider.PrepareDispatch(context.Background(), DispatchInput{
		Action:             "BOOKING_RESCHEDULED",
		CreatedAt:          "2026-01-01T00:11:00.000Z",
		UID:                "booking-uid",
		RequestID:          "request-id",
		Start:              "2026-05-02T15:00:00.000Z",
		End:                "2026-05-02T15:30:00.000Z",
		Recipients:         []Recipient{{Name: "Recipient", Email: "recipient@example.test", TimeZone: "America/Chicago"}},
		RescheduleUID:      "previous-booking-uid",
		ReschedulingReason: "Fixture reschedule",
	})
	if err != nil {
		t.Fatal(err)
	}

	var request ResendFixtureDispatchRequest
	if err := json.Unmarshal([]byte(prepared.Body), &request); err != nil {
		t.Fatal(err)
	}
	if request.Template != "booking-rescheduled" {
		t.Fatalf("template = %q", request.Template)
	}
	if request.Message.Variables.RescheduleUID != "previous-booking-uid" {
		t.Fatalf("reschedule uid = %q", request.Message.Variables.RescheduleUID)
	}
	if request.Message.Variables.ReschedulingReason != "Fixture reschedule" {
		t.Fatalf("rescheduling reason = %q", request.Message.Variables.ReschedulingReason)
	}
}
