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

type capturingSideEffectPort struct {
	cancelled   []BookingCancelledSideEffect
	rescheduled []BookingRescheduledSideEffect
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
		{Name: SideEffectWebhookBookingCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
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
		{Name: SideEffectWebhookBookingRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID},
	}, nil
}
