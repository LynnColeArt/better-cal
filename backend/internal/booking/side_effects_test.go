package booking

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCancelPlansSideEffectsThroughPort(t *testing.T) {
	port := &capturingSideEffectPort{}
	store := NewStore(WithSideEffectPort(port))

	result, ok, err := store.Cancel(context.Background(), "cancel-request", PrimaryFixtureUID, CancelRequest{
		CancellationReason: "Fixture cancellation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not cancelled")
	}
	assertStringSlice(t, result.SideEffects, []string{
		string(SideEffectCalendarCancelled),
		string(SideEffectEmailCancelled),
		string(SideEffectWebhookBookingCancelled),
	})
	if len(port.cancelled) != 1 {
		t.Fatalf("cancel side-effect calls = %d", len(port.cancelled))
	}
	call := port.cancelled[0]
	if call.Booking.UID != PrimaryFixtureUID {
		t.Fatalf("booking uid = %q", call.Booking.UID)
	}
	if call.Booking.Status != "cancelled" {
		t.Fatalf("booking status = %q", call.Booking.Status)
	}
	if call.CancellationReason != "Fixture cancellation" {
		t.Fatalf("cancellation reason = %q", call.CancellationReason)
	}
	assertSideEffectSnapshotIsSecretFree(t, call.Booking)
}

func TestReschedulePlansSideEffectsThroughPort(t *testing.T) {
	port := &capturingSideEffectPort{}
	store := NewStore(WithSideEffectPort(port))

	result, ok, err := store.Reschedule(context.Background(), "reschedule-request", PrimaryFixtureUID, RescheduleRequest{
		Start:              "2026-05-02T15:00:00.000Z",
		ReschedulingReason: "Fixture reschedule",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not rescheduled")
	}
	assertStringSlice(t, result.SideEffects, []string{
		string(SideEffectCalendarRescheduled),
		string(SideEffectEmailRescheduled),
		string(SideEffectWebhookBookingRescheduled),
	})
	if len(port.rescheduled) != 1 {
		t.Fatalf("reschedule side-effect calls = %d", len(port.rescheduled))
	}
	call := port.rescheduled[0]
	if call.OldBooking.UID != PrimaryFixtureUID {
		t.Fatalf("old booking uid = %q", call.OldBooking.UID)
	}
	if call.NewBooking.UID != RescheduledFixtureUID {
		t.Fatalf("new booking uid = %q", call.NewBooking.UID)
	}
	if call.ReschedulingReason != "Fixture reschedule" {
		t.Fatalf("rescheduling reason = %q", call.ReschedulingReason)
	}
	assertSideEffectSnapshotIsSecretFree(t, call.OldBooking)
	assertSideEffectSnapshotIsSecretFree(t, call.NewBooking)
}

func TestConfirmPlansSideEffectsThroughPort(t *testing.T) {
	port := &capturingSideEffectPort{}
	store := NewStore(WithSideEffectPort(port))

	result, ok, err := store.Confirm(context.Background(), "confirm-request", PendingConfirmFixtureUID, ConfirmRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not confirmed")
	}
	assertStringSlice(t, result.SideEffects, []string{
		string(SideEffectEmailConfirmed),
		string(SideEffectWebhookBookingConfirmed),
	})
	if len(port.confirmed) != 1 {
		t.Fatalf("confirm side-effect calls = %d", len(port.confirmed))
	}
	call := port.confirmed[0]
	if call.Booking.UID != PendingConfirmFixtureUID {
		t.Fatalf("booking uid = %q", call.Booking.UID)
	}
	if call.Booking.Status != "accepted" {
		t.Fatalf("booking status = %q", call.Booking.Status)
	}
	assertSideEffectSnapshotIsSecretFree(t, call.Booking)
}

func TestDeclinePlansSideEffectsThroughPort(t *testing.T) {
	port := &capturingSideEffectPort{}
	store := NewStore(WithSideEffectPort(port))

	result, ok, err := store.Decline(context.Background(), "decline-request", PendingDeclineFixtureUID, DeclineRequest{
		Reason: "Fixture decline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking was not declined")
	}
	assertStringSlice(t, result.SideEffects, []string{
		string(SideEffectEmailDeclined),
		string(SideEffectWebhookBookingDeclined),
	})
	if len(port.declined) != 1 {
		t.Fatalf("decline side-effect calls = %d", len(port.declined))
	}
	call := port.declined[0]
	if call.Booking.UID != PendingDeclineFixtureUID {
		t.Fatalf("booking uid = %q", call.Booking.UID)
	}
	if call.Booking.Status != "rejected" {
		t.Fatalf("booking status = %q", call.Booking.Status)
	}
	if call.Reason != "Fixture decline" {
		t.Fatalf("decline reason = %q", call.Reason)
	}
	assertSideEffectSnapshotIsSecretFree(t, call.Booking)
}

func TestSideEffectPlanningFailurePreventsStateTransition(t *testing.T) {
	port := &capturingSideEffectPort{err: errors.New("side-effect planner unavailable")}
	store := NewStore(WithSideEffectPort(port))

	_, ok, err := store.Cancel(context.Background(), "cancel-request", PrimaryFixtureUID, CancelRequest{})
	if err == nil {
		t.Fatal("expected side-effect planning error")
	}
	if ok {
		t.Fatal("cancel reported success despite side-effect planning failure")
	}

	found, ok, err := store.Read(context.Background(), "read-request", PrimaryFixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("primary fixture booking was not found")
	}
	if found.Status != "accepted" {
		t.Fatalf("booking status = %q", found.Status)
	}
}

func TestFixtureSideEffectPortPersistsSideEffectPayloadHints(t *testing.T) {
	port := FixtureSideEffectPort{}

	cancelled, err := port.PlanBookingCancelled(context.Background(), BookingCancelledSideEffect{
		Booking: BookingSideEffectSnapshot{
			UID:                     PrimaryFixtureUID,
			RequestID:               "cancel-request",
			SelectedCalendarRef:     FixtureSelectedCalendarRef,
			DestinationCalendarRef:  FixtureDestinationCalendarRef,
			ExternalCalendarEventID: "google-event-primary",
		},
		CancellationReason: "Fixture cancellation",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPayloadValue(t, cancelled[0].Payload, "selectedCalendarRef", FixtureSelectedCalendarRef)
	assertPayloadValue(t, cancelled[0].Payload, "destinationCalendarRef", FixtureDestinationCalendarRef)
	assertPayloadValue(t, cancelled[0].Payload, "externalEventId", "google-event-primary")
	assertPayloadValue(t, cancelled[1].Payload, "cancellationReason", "Fixture cancellation")
	assertPayloadValue(t, cancelled[2].Payload, "cancellationReason", "Fixture cancellation")

	rescheduled, err := port.PlanBookingRescheduled(context.Background(), BookingRescheduledSideEffect{
		OldBooking: BookingSideEffectSnapshot{
			UID:                     PrimaryFixtureUID,
			ExternalCalendarEventID: "google-event-primary",
		},
		NewBooking: BookingSideEffectSnapshot{
			UID:                     RescheduledFixtureUID,
			RequestID:               "reschedule-request",
			SelectedCalendarRef:     FixtureSelectedCalendarRef,
			DestinationCalendarRef:  FixtureDestinationCalendarRef,
			ExternalCalendarEventID: "google-event-rescheduled",
		},
		ReschedulingReason: "Fixture reschedule",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPayloadValue(t, rescheduled[0].Payload, "rescheduleUid", PrimaryFixtureUID)
	assertPayloadValue(t, rescheduled[0].Payload, "selectedCalendarRef", FixtureSelectedCalendarRef)
	assertPayloadValue(t, rescheduled[0].Payload, "destinationCalendarRef", FixtureDestinationCalendarRef)
	assertPayloadValue(t, rescheduled[0].Payload, "externalEventId", "google-event-rescheduled")
	assertPayloadValue(t, rescheduled[0].Payload, "previousExternalEventId", "google-event-primary")
	assertPayloadValue(t, rescheduled[1].Payload, "rescheduleUid", PrimaryFixtureUID)
	assertPayloadValue(t, rescheduled[1].Payload, "reschedulingReason", "Fixture reschedule")
	assertPayloadValue(t, rescheduled[2].Payload, "rescheduleUid", PrimaryFixtureUID)
	assertPayloadValue(t, rescheduled[2].Payload, "reschedulingReason", "Fixture reschedule")

	declined, err := port.PlanBookingDeclined(context.Background(), BookingDeclinedSideEffect{
		Booking: BookingSideEffectSnapshot{UID: PendingDeclineFixtureUID, RequestID: "decline-request"},
		Reason:  "Fixture decline",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPayloadValue(t, declined[0].Payload, "reason", "Fixture decline")
	assertPayloadValue(t, declined[1].Payload, "reason", "Fixture decline")
}

func assertStringSlice(t *testing.T, actual []string, expected []string) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("strings = %#v, want %#v", actual, expected)
	}
}

func assertSideEffectSnapshotIsSecretFree(t *testing.T, snapshot BookingSideEffectSnapshot) {
	t.Helper()
	snapshotType := reflect.TypeOf(snapshot)
	for _, fieldName := range []string{"Attendees", "Responses", "Metadata"} {
		if _, ok := snapshotType.FieldByName(fieldName); ok {
			t.Fatalf("side-effect snapshot exposes %s", fieldName)
		}
	}
}

func assertPayloadValue(t *testing.T, payload map[string]any, key string, expected string) {
	t.Helper()
	value, _ := payload[key].(string)
	if value != expected {
		t.Fatalf("payload[%q] = %q, want %q", key, value, expected)
	}
}

type capturingSideEffectPort struct {
	cancelled   []BookingCancelledSideEffect
	rescheduled []BookingRescheduledSideEffect
	confirmed   []BookingConfirmedSideEffect
	declined    []BookingDeclinedSideEffect
	err         error
}

func (p *capturingSideEffectPort) PlanBookingCancelled(_ context.Context, event BookingCancelledSideEffect) ([]PlannedSideEffect, error) {
	p.cancelled = append(p.cancelled, event)
	if p.err != nil {
		return nil, p.err
	}
	return []PlannedSideEffect{
		{Name: SideEffectCalendarCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectEmailCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectWebhookBookingCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID, Payload: map[string]any{"cancellationReason": event.CancellationReason}},
	}, nil
}

func (p *capturingSideEffectPort) PlanBookingRescheduled(_ context.Context, event BookingRescheduledSideEffect) ([]PlannedSideEffect, error) {
	p.rescheduled = append(p.rescheduled, event)
	if p.err != nil {
		return nil, p.err
	}
	return []PlannedSideEffect{
		{Name: SideEffectCalendarRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID},
		{Name: SideEffectEmailRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID},
		{Name: SideEffectWebhookBookingRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID, Payload: map[string]any{"rescheduleUid": event.OldBooking.UID, "reschedulingReason": event.ReschedulingReason}},
	}, nil
}

func (p *capturingSideEffectPort) PlanBookingConfirmed(_ context.Context, event BookingConfirmedSideEffect) ([]PlannedSideEffect, error) {
	p.confirmed = append(p.confirmed, event)
	if p.err != nil {
		return nil, p.err
	}
	return []PlannedSideEffect{
		{Name: SideEffectEmailConfirmed, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectWebhookBookingConfirmed, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID, Payload: map[string]any{}},
	}, nil
}

func (p *capturingSideEffectPort) PlanBookingDeclined(_ context.Context, event BookingDeclinedSideEffect) ([]PlannedSideEffect, error) {
	p.declined = append(p.declined, event)
	if p.err != nil {
		return nil, p.err
	}
	return []PlannedSideEffect{
		{Name: SideEffectEmailDeclined, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectWebhookBookingDeclined, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID, Payload: map[string]any{"reason": event.Reason}},
	}, nil
}
