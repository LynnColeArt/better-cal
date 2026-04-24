package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	resendFixtureProviderName   = "resend-email-fixture"
	resendFixtureHeaderProvider = "X-Cal-Email-Provider"
)

type ResendFixtureProvider struct{}

func NewResendFixtureProvider() ResendFixtureProvider {
	return ResendFixtureProvider{}
}

type ResendFixtureDispatchRequest struct {
	Provider    string                       `json:"provider"`
	Template    string                       `json:"template"`
	RequestedAt string                       `json:"requestedAt"`
	RequestID   string                       `json:"requestId"`
	Message     ResendFixtureDispatchMessage `json:"message"`
}

type ResendFixtureDispatchMessage struct {
	To        []ResendFixtureRecipient `json:"to"`
	Variables ResendFixtureVariables   `json:"variables"`
}

type ResendFixtureRecipient struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	TimeZone string `json:"timeZone"`
}

type ResendFixtureVariables struct {
	UID                string `json:"uid"`
	Title              string `json:"title,omitempty"`
	Status             string `json:"status"`
	Start              string `json:"start"`
	End                string `json:"end"`
	EventTypeID        int    `json:"eventTypeId"`
	CancellationReason string `json:"cancellationReason,omitempty"`
	RescheduleUID      string `json:"rescheduleUid,omitempty"`
	ReschedulingReason string `json:"reschedulingReason,omitempty"`
	RejectionReason    string `json:"rejectionReason,omitempty"`
}

func (ResendFixtureProvider) PrepareDispatch(_ context.Context, input DispatchInput) (PreparedDispatch, error) {
	request, err := resendFixtureDispatchRequest(input)
	if err != nil {
		return PreparedDispatch{}, err
	}
	bodyRaw, err := json.Marshal(request)
	if err != nil {
		return PreparedDispatch{}, fmt.Errorf("encode resend fixture dispatch request: %w", err)
	}
	return PreparedDispatch{
		ContentType: "application/json",
		Headers: map[string]string{
			resendFixtureHeaderProvider: resendFixtureProviderName,
		},
		Body: string(bodyRaw),
	}, nil
}

func resendFixtureDispatchRequest(input DispatchInput) (ResendFixtureDispatchRequest, error) {
	if input.RequestID == "" || input.UID == "" {
		return ResendFixtureDispatchRequest{}, errors.New("email provider dispatch requires booking uid and request id")
	}
	if len(input.Recipients) == 0 {
		return ResendFixtureDispatchRequest{}, errors.New("email provider dispatch requires at least one recipient")
	}

	return ResendFixtureDispatchRequest{
		Provider:    resendFixtureProviderName,
		Template:    resendFixtureTemplate(input.Action),
		RequestedAt: input.CreatedAt,
		RequestID:   input.RequestID,
		Message: ResendFixtureDispatchMessage{
			To: resendFixtureRecipients(input.Recipients),
			Variables: ResendFixtureVariables{
				UID:                input.UID,
				Title:              input.Title,
				Status:             input.Status,
				Start:              input.Start,
				End:                input.End,
				EventTypeID:        input.EventTypeID,
				CancellationReason: input.CancellationReason,
				RescheduleUID:      input.RescheduleUID,
				ReschedulingReason: input.ReschedulingReason,
				RejectionReason:    input.RejectionReason,
			},
		},
	}, nil
}

func resendFixtureTemplate(action string) string {
	switch action {
	case "BOOKING_CANCELLED":
		return "booking-cancelled"
	case "BOOKING_RESCHEDULED":
		return "booking-rescheduled"
	case "BOOKING_CONFIRMED":
		return "booking-confirmed"
	case "BOOKING_REJECTED":
		return "booking-rejected"
	default:
		return "booking-update"
	}
}

func resendFixtureRecipients(recipients []Recipient) []ResendFixtureRecipient {
	result := make([]ResendFixtureRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		result = append(result, ResendFixtureRecipient{
			Name:     recipient.Name,
			Email:    recipient.Email,
			TimeZone: recipient.TimeZone,
		})
	}
	return result
}
