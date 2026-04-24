package booking

type EmailDeliveryAction string

const (
	EmailDeliveryBookingCancelled   EmailDeliveryAction = "BOOKING_CANCELLED"
	EmailDeliveryBookingRescheduled EmailDeliveryAction = "BOOKING_RESCHEDULED"
	EmailDeliveryBookingConfirmed   EmailDeliveryAction = "BOOKING_CONFIRMED"
	EmailDeliveryBookingRejected    EmailDeliveryAction = "BOOKING_REJECTED"
)

type EmailDeliveryEnvelope struct {
	Action    EmailDeliveryAction `json:"action"`
	CreatedAt string              `json:"createdAt"`
	Payload   BookingEmailPayload `json:"payload"`
}

type BookingEmailPayload struct {
	UID                string           `json:"uid"`
	Title              string           `json:"title,omitempty"`
	Status             string           `json:"status"`
	Start              string           `json:"start"`
	End                string           `json:"end"`
	EventTypeID        int              `json:"eventTypeId"`
	RequestID          string           `json:"requestId"`
	Recipients         []EmailRecipient `json:"recipients,omitempty"`
	CancellationReason string           `json:"cancellationReason,omitempty"`
	RescheduleUID      string           `json:"rescheduleUid,omitempty"`
	ReschedulingReason string           `json:"reschedulingReason,omitempty"`
	RejectionReason    string           `json:"rejectionReason,omitempty"`
}

type EmailRecipient struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	TimeZone string `json:"timeZone"`
}

func emailDeliveryActionForSideEffect(name SideEffectName) (EmailDeliveryAction, bool) {
	switch name {
	case SideEffectEmailCancelled:
		return EmailDeliveryBookingCancelled, true
	case SideEffectEmailRescheduled:
		return EmailDeliveryBookingRescheduled, true
	case SideEffectEmailConfirmed:
		return EmailDeliveryBookingConfirmed, true
	case SideEffectEmailDeclined:
		return EmailDeliveryBookingRejected, true
	default:
		return "", false
	}
}

func emailDeliveryEnvelopeForBooking(effect PlannedSideEffectRecord, booking Booking) (EmailDeliveryEnvelope, bool) {
	action, ok := emailDeliveryActionForSideEffect(effect.Name)
	if !ok {
		return EmailDeliveryEnvelope{}, false
	}

	payload := BookingEmailPayload{
		UID:         booking.UID,
		Title:       booking.Title,
		Status:      booking.Status,
		Start:       booking.Start,
		End:         booking.End,
		EventTypeID: booking.EventTypeID,
		RequestID:   booking.RequestID,
		Recipients:  emailRecipients(booking.Attendees),
	}
	if payloadValue := payloadString(effect.Payload, "cancellationReason"); payloadValue != "" {
		payload.CancellationReason = payloadValue
	}
	if payloadValue := payloadString(effect.Payload, "rescheduleUid"); payloadValue != "" {
		payload.RescheduleUID = payloadValue
	}
	if payloadValue := payloadString(effect.Payload, "reschedulingReason"); payloadValue != "" {
		payload.ReschedulingReason = payloadValue
	}
	if payloadValue := payloadString(effect.Payload, "reason"); payloadValue != "" {
		payload.RejectionReason = payloadValue
	}

	return EmailDeliveryEnvelope{
		Action:    action,
		CreatedAt: webhookEventCreatedAt(booking),
		Payload:   payload,
	}, true
}

func emailRecipients(attendees []Attendee) []EmailRecipient {
	if len(attendees) == 0 {
		return nil
	}
	result := make([]EmailRecipient, 0, len(attendees))
	for _, attendee := range attendees {
		result = append(result, EmailRecipient{
			Name:     attendee.Name,
			Email:    attendee.Email,
			TimeZone: attendee.TimeZone,
		})
	}
	return result
}
