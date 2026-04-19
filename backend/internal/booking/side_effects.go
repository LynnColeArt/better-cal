package booking

import "context"

type SideEffectName string

const (
	SideEffectCalendarCancelled         SideEffectName = "calendar.cancelled"
	SideEffectEmailCancelled            SideEffectName = "email.cancelled"
	SideEffectWebhookBookingCancelled   SideEffectName = "webhook.booking.cancelled"
	SideEffectCalendarRescheduled       SideEffectName = "calendar.rescheduled"
	SideEffectEmailRescheduled          SideEffectName = "email.rescheduled"
	SideEffectWebhookBookingRescheduled SideEffectName = "webhook.booking.rescheduled"
	SideEffectEmailConfirmed            SideEffectName = "email.confirmed"
	SideEffectWebhookBookingConfirmed   SideEffectName = "webhook.booking.confirmed"
	SideEffectEmailDeclined             SideEffectName = "email.declined"
	SideEffectWebhookBookingDeclined    SideEffectName = "webhook.booking.declined"
)

type SideEffectPort interface {
	PlanBookingCancelled(context.Context, BookingCancelledSideEffect) ([]PlannedSideEffect, error)
	PlanBookingRescheduled(context.Context, BookingRescheduledSideEffect) ([]PlannedSideEffect, error)
	PlanBookingConfirmed(context.Context, BookingConfirmedSideEffect) ([]PlannedSideEffect, error)
	PlanBookingDeclined(context.Context, BookingDeclinedSideEffect) ([]PlannedSideEffect, error)
}

type PlannedSideEffect struct {
	Name       SideEffectName
	BookingUID string
	RequestID  string
}

type BookingCancelledSideEffect struct {
	Booking            BookingSideEffectSnapshot
	CancellationReason string
}

type BookingRescheduledSideEffect struct {
	OldBooking         BookingSideEffectSnapshot
	NewBooking         BookingSideEffectSnapshot
	ReschedulingReason string
}

type BookingConfirmedSideEffect struct {
	Booking BookingSideEffectSnapshot
}

type BookingDeclinedSideEffect struct {
	Booking BookingSideEffectSnapshot
	Reason  string
}

type BookingSideEffectSnapshot struct {
	UID         string
	Status      string
	Start       string
	End         string
	EventTypeID int
	RequestID   string
}

type FixtureSideEffectPort struct{}

func (FixtureSideEffectPort) PlanBookingCancelled(_ context.Context, event BookingCancelledSideEffect) ([]PlannedSideEffect, error) {
	return []PlannedSideEffect{
		{Name: SideEffectCalendarCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectEmailCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectWebhookBookingCancelled, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
	}, nil
}

func (FixtureSideEffectPort) PlanBookingRescheduled(_ context.Context, event BookingRescheduledSideEffect) ([]PlannedSideEffect, error) {
	return []PlannedSideEffect{
		{Name: SideEffectCalendarRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID},
		{Name: SideEffectEmailRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID},
		{Name: SideEffectWebhookBookingRescheduled, BookingUID: event.NewBooking.UID, RequestID: event.NewBooking.RequestID},
	}, nil
}

func (FixtureSideEffectPort) PlanBookingConfirmed(_ context.Context, event BookingConfirmedSideEffect) ([]PlannedSideEffect, error) {
	return []PlannedSideEffect{
		{Name: SideEffectEmailConfirmed, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectWebhookBookingConfirmed, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
	}, nil
}

func (FixtureSideEffectPort) PlanBookingDeclined(_ context.Context, event BookingDeclinedSideEffect) ([]PlannedSideEffect, error) {
	return []PlannedSideEffect{
		{Name: SideEffectEmailDeclined, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
		{Name: SideEffectWebhookBookingDeclined, BookingUID: event.Booking.UID, RequestID: event.Booking.RequestID},
	}, nil
}

func sideEffectSnapshot(booking Booking) BookingSideEffectSnapshot {
	return BookingSideEffectSnapshot{
		UID:         booking.UID,
		Status:      booking.Status,
		Start:       booking.Start,
		End:         booking.End,
		EventTypeID: booking.EventTypeID,
		RequestID:   booking.RequestID,
	}
}

func sideEffectNames(effects []PlannedSideEffect) []string {
	names := make([]string, 0, len(effects))
	for _, effect := range effects {
		names = append(names, string(effect.Name))
	}
	return names
}
