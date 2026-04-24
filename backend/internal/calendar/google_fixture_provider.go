package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	googleFixtureProviderName         = "google-calendar-fixture"
	googleFixtureHeaderProvider       = "X-Cal-Calendar-Provider"
	googleFixtureOperationCancelEvent = "cancel_event"
	googleFixtureOperationReschedule  = "move_event"
)

type GoogleFixtureProvider struct{}

func NewGoogleFixtureProvider() GoogleFixtureProvider {
	return GoogleFixtureProvider{}
}

type GoogleFixtureDispatchRequest struct {
	Provider    string                     `json:"provider"`
	Operation   string                     `json:"operation"`
	RequestedAt string                     `json:"requestedAt"`
	RequestID   string                     `json:"requestId"`
	Event       GoogleFixtureDispatchEvent `json:"event"`
}

type GoogleFixtureDispatchEvent struct {
	ID         string `json:"id"`
	Start      string `json:"start,omitempty"`
	End        string `json:"end,omitempty"`
	PreviousID string `json:"previousId,omitempty"`
}

func (GoogleFixtureProvider) PrepareDispatch(_ context.Context, input DispatchInput) (PreparedDispatch, error) {
	request, err := googleFixtureDispatchRequest(input)
	if err != nil {
		return PreparedDispatch{}, err
	}
	bodyRaw, err := json.Marshal(request)
	if err != nil {
		return PreparedDispatch{}, fmt.Errorf("encode google fixture dispatch request: %w", err)
	}
	return PreparedDispatch{
		ContentType: "application/json",
		Headers: map[string]string{
			googleFixtureHeaderProvider: googleFixtureProviderName,
		},
		Body: string(bodyRaw),
	}, nil
}

func googleFixtureDispatchRequest(input DispatchInput) (GoogleFixtureDispatchRequest, error) {
	request := GoogleFixtureDispatchRequest{
		Provider:    googleFixtureProviderName,
		RequestedAt: input.CreatedAt,
		RequestID:   input.RequestID,
	}

	switch input.Action {
	case "BOOKING_CANCELLED":
		if input.UID == "" || input.RequestID == "" {
			return GoogleFixtureDispatchRequest{}, errors.New("calendar provider dispatch requires booking uid and request id")
		}
		request.Operation = googleFixtureOperationCancelEvent
		request.Event = GoogleFixtureDispatchEvent{
			ID: input.UID,
		}
	case "BOOKING_RESCHEDULED":
		if input.UID == "" || input.RequestID == "" || input.Start == "" || input.End == "" {
			return GoogleFixtureDispatchRequest{}, errors.New("calendar provider reschedule dispatch requires booking uid, request id, start, and end")
		}
		request.Operation = googleFixtureOperationReschedule
		request.Event = GoogleFixtureDispatchEvent{
			ID:         input.UID,
			Start:      input.Start,
			End:        input.End,
			PreviousID: input.RescheduleUID,
		}
	default:
		return GoogleFixtureDispatchRequest{}, fmt.Errorf("calendar provider action %q is unsupported", input.Action)
	}

	return request, nil
}
