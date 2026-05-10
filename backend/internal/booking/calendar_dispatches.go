package booking

type CalendarDispatchAction string

const (
	CalendarDispatchBookingCancelled   CalendarDispatchAction = "BOOKING_CANCELLED"
	CalendarDispatchBookingRescheduled CalendarDispatchAction = "BOOKING_RESCHEDULED"
)

type CalendarDispatchEnvelope struct {
	Action    CalendarDispatchAction  `json:"action"`
	CreatedAt string                  `json:"createdAt"`
	Payload   CalendarDispatchPayload `json:"payload"`
}

type CalendarDispatchPayload struct {
	UID                     string `json:"uid"`
	Status                  string `json:"status"`
	Start                   string `json:"start"`
	End                     string `json:"end"`
	EventTypeID             int    `json:"eventTypeId"`
	RequestID               string `json:"requestId"`
	SelectedCalendarRef     string `json:"selectedCalendarRef,omitempty"`
	DestinationCalendarRef  string `json:"destinationCalendarRef,omitempty"`
	ExternalEventID         string `json:"externalEventId,omitempty"`
	RescheduleUID           string `json:"rescheduleUid,omitempty"`
	PreviousExternalEventID string `json:"previousExternalEventId,omitempty"`
}

func calendarDispatchActionForSideEffect(name SideEffectName) (CalendarDispatchAction, bool) {
	switch name {
	case SideEffectCalendarCancelled:
		return CalendarDispatchBookingCancelled, true
	case SideEffectCalendarRescheduled:
		return CalendarDispatchBookingRescheduled, true
	default:
		return "", false
	}
}

func calendarDispatchEnvelopeForBooking(effect PlannedSideEffectRecord, booking Booking) (CalendarDispatchEnvelope, bool) {
	action, ok := calendarDispatchActionForSideEffect(effect.Name)
	if !ok {
		return CalendarDispatchEnvelope{}, false
	}

	payload := CalendarDispatchPayload{
		UID:                    booking.UID,
		Status:                 booking.Status,
		Start:                  booking.Start,
		End:                    booking.End,
		EventTypeID:            booking.EventTypeID,
		RequestID:              booking.RequestID,
		SelectedCalendarRef:    booking.SelectedCalendarRef,
		DestinationCalendarRef: booking.DestinationCalendarRef,
		ExternalEventID:        booking.ExternalCalendarEventID,
	}
	if payloadValue := payloadString(effect.Payload, "selectedCalendarRef"); payloadValue != "" {
		payload.SelectedCalendarRef = payloadValue
	}
	if payloadValue := payloadString(effect.Payload, "destinationCalendarRef"); payloadValue != "" {
		payload.DestinationCalendarRef = payloadValue
	}
	if payloadValue := payloadString(effect.Payload, "externalEventId"); payloadValue != "" {
		payload.ExternalEventID = payloadValue
	}
	if rescheduleUID := payloadString(effect.Payload, "rescheduleUid"); rescheduleUID != "" {
		payload.RescheduleUID = rescheduleUID
	}
	if previousExternalEventID := payloadString(effect.Payload, "previousExternalEventId"); previousExternalEventID != "" {
		payload.PreviousExternalEventID = previousExternalEventID
	}

	return CalendarDispatchEnvelope{
		Action:    action,
		CreatedAt: webhookEventCreatedAt(booking),
		Payload:   payload,
	}, true
}
