package booking

import (
	"context"
	"testing"
)

func TestCreateRejectsUnsupportedEventType(t *testing.T) {
	store := NewStore()

	_, _, err := store.Create(context.Background(), "request-id", CreateRequest{
		EventTypeID: 9999,
		Start:       "2026-05-01T15:00:00.000Z",
	})

	assertValidationCode(t, err, errCodeInvalidEventType)
}

func TestCreateRejectsInvalidAttendeeEmail(t *testing.T) {
	store := NewStore()

	_, _, err := store.Create(context.Background(), "request-id", CreateRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       "2026-05-01T15:00:00.000Z",
		Attendee: Attendee{
			Email: "not-an-email",
		},
	})

	assertValidationCode(t, err, errCodeInvalidAttendeeEmail)
}

func TestCreateRejectsSecretBearingMetadata(t *testing.T) {
	store := NewStore()

	_, _, err := store.Create(context.Background(), "request-id", CreateRequest{
		EventTypeID: FixtureEventTypeID,
		Start:       "2026-05-01T15:00:00.000Z",
		Metadata: map[string]any{
			"nested": map[string]any{
				"clientSecret": "do-not-echo",
			},
		},
	})

	assertValidationCode(t, err, errCodeSecretField)
}

func TestCreateIdempotencyReplayDoesNotValidateDiscardedPayload(t *testing.T) {
	store := NewStore()
	created, duplicate, err := store.Create(context.Background(), "first-request", CreateRequest{
		EventTypeID:    FixtureEventTypeID,
		Start:          "2026-05-01T15:00:00.000Z",
		IdempotencyKey: "replay-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("initial create was reported as duplicate")
	}

	replayed, duplicate, err := store.Create(context.Background(), "second-request", CreateRequest{
		EventTypeID:    9999,
		IdempotencyKey: "replay-key",
		Metadata: map[string]any{
			"clientSecret": "discarded",
		},
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

func TestRescheduleRejectsInvalidStartTime(t *testing.T) {
	store := NewStore()

	_, _, err := store.Reschedule(context.Background(), "request-id", PrimaryFixtureUID, RescheduleRequest{
		Start: "next tuesday",
	})

	assertValidationCode(t, err, errCodeInvalidStartTime)
}

func TestConfirmRejectsAcceptedBooking(t *testing.T) {
	store := NewStore()

	_, _, err := store.Confirm(context.Background(), "request-id", PrimaryFixtureUID, ConfirmRequest{})

	assertValidationCode(t, err, errCodeInvalidBookingStatus)
}

func TestDeclineRejectsAcceptedBooking(t *testing.T) {
	store := NewStore()

	_, _, err := store.Decline(context.Background(), "request-id", PrimaryFixtureUID, DeclineRequest{})

	assertValidationCode(t, err, errCodeInvalidBookingStatus)
}

func assertValidationCode(t *testing.T, err error, expectedCode string) {
	t.Helper()
	validationErr, ok := ValidationFromError(err)
	if !ok {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != expectedCode {
		t.Fatalf("validation code = %q, want %q", validationErr.Code, expectedCode)
	}
}
