package booking

type WebhookTriggerEvent string

const (
	WebhookTriggerBookingCancelled   WebhookTriggerEvent = "BOOKING_CANCELLED"
	WebhookTriggerBookingRescheduled WebhookTriggerEvent = "BOOKING_RESCHEDULED"
	WebhookTriggerBookingConfirmed   WebhookTriggerEvent = "BOOKING_CONFIRMED"
	WebhookTriggerBookingRejected    WebhookTriggerEvent = "BOOKING_REJECTED"
)

type WebhookDeliveryEnvelope struct {
	TriggerEvent WebhookTriggerEvent   `json:"triggerEvent"`
	CreatedAt    string                `json:"createdAt"`
	Payload      BookingWebhookPayload `json:"payload"`
}

type BookingWebhookPayload struct {
	UID                string            `json:"uid"`
	Title              string            `json:"title,omitempty"`
	StartTime          string            `json:"startTime,omitempty"`
	EndTime            string            `json:"endTime,omitempty"`
	Attendees          []WebhookAttendee `json:"attendees,omitempty"`
	Responses          map[string]any    `json:"responses,omitempty"`
	Metadata           map[string]any    `json:"metadata,omitempty"`
	CancellationReason string            `json:"cancellationReason,omitempty"`
	RescheduleUID      string            `json:"rescheduleUid,omitempty"`
	ReschedulingReason string            `json:"reschedulingReason,omitempty"`
	RejectionReason    string            `json:"rejectionReason,omitempty"`
}

type WebhookAttendee struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	TimeZone string `json:"timeZone"`
}

func webhookTriggerEventForSideEffect(name SideEffectName) (WebhookTriggerEvent, bool) {
	switch name {
	case SideEffectWebhookBookingCancelled:
		return WebhookTriggerBookingCancelled, true
	case SideEffectWebhookBookingRescheduled:
		return WebhookTriggerBookingRescheduled, true
	case SideEffectWebhookBookingConfirmed:
		return WebhookTriggerBookingConfirmed, true
	case SideEffectWebhookBookingDeclined:
		return WebhookTriggerBookingRejected, true
	default:
		return "", false
	}
}

func webhookEnvelopeForBooking(effect PlannedSideEffectRecord, booking Booking) (WebhookDeliveryEnvelope, bool) {
	triggerEvent, ok := webhookTriggerEventForSideEffect(effect.Name)
	if !ok {
		return WebhookDeliveryEnvelope{}, false
	}

	payload := BookingWebhookPayload{
		UID:       booking.UID,
		Title:     booking.Title,
		StartTime: booking.Start,
		EndTime:   booking.End,
		Attendees: webhookAttendees(booking.Attendees),
		Responses: objectOrEmpty(booking.Responses),
		Metadata:  objectOrEmpty(booking.Metadata),
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

	return WebhookDeliveryEnvelope{
		TriggerEvent: triggerEvent,
		CreatedAt:    webhookEventCreatedAt(booking),
		Payload:      payload,
	}, true
}

func webhookEventCreatedAt(booking Booking) string {
	if booking.UpdatedAt != "" {
		return booking.UpdatedAt
	}
	return booking.CreatedAt
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return value
}

func webhookAttendees(attendees []Attendee) []WebhookAttendee {
	if len(attendees) == 0 {
		return nil
	}
	result := make([]WebhookAttendee, 0, len(attendees))
	for _, attendee := range attendees {
		result = append(result, WebhookAttendee{
			Name:     attendee.Name,
			Email:    attendee.Email,
			TimeZone: attendee.TimeZone,
		})
	}
	return result
}
