package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/integrations"
)

const (
	googleFixtureProviderName         = "google-calendar-fixture"
	googleFixtureConnectionRef        = "google-calendar-connection-fixture"
	googleFixtureAccountRef           = "google-account-fixture"
	googleFixtureAccountEmail         = "fixture-user@example.test"
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
	ID                     string `json:"id"`
	Start                  string `json:"start,omitempty"`
	End                    string `json:"end,omitempty"`
	PreviousID             string `json:"previousId,omitempty"`
	SelectedCalendarRef    string `json:"selectedCalendarRef,omitempty"`
	DestinationCalendarRef string `json:"destinationCalendarRef,omitempty"`
}

func (GoogleFixtureProvider) ReadCatalog(_ context.Context, input CatalogInput) (CatalogSnapshot, error) {
	if input.UserID != 123 {
		return CatalogSnapshot{}, nil
	}
	return CatalogSnapshot{
		Connections: []CatalogConnection{
			{
				ConnectionRef: googleFixtureConnectionRef,
				Provider:      googleFixtureProviderName,
				AccountRef:    googleFixtureAccountRef,
				AccountEmail:  googleFixtureAccountEmail,
				Status:        "active",
			},
		},
		Calendars: []CatalogCalendar{
			{
				CalendarRef:   "destination-calendar-fixture",
				ConnectionRef: googleFixtureConnectionRef,
				Provider:      googleFixtureProviderName,
				ExternalID:    "google-calendar-destination",
				Name:          "Fixture Destination Calendar",
				Primary:       true,
				Writable:      true,
			},
			{
				CalendarRef:   "selected-calendar-fixture",
				ConnectionRef: googleFixtureConnectionRef,
				Provider:      googleFixtureProviderName,
				ExternalID:    "google-calendar-selected",
				Name:          "Fixture Selected Calendar",
				Writable:      true,
			},
			{
				CalendarRef:   "team-calendar-fixture",
				ConnectionRef: googleFixtureConnectionRef,
				Provider:      googleFixtureProviderName,
				ExternalID:    "google-calendar-team",
				Name:          "Fixture Team Calendar",
				Writable:      true,
			},
		},
	}, nil
}

func (GoogleFixtureProvider) ReadStatus(_ context.Context, input integrations.StatusInput) (integrations.StatusSnapshot, error) {
	if input.UserID != 123 {
		return integrations.StatusSnapshot{}, nil
	}
	return integrations.StatusSnapshot{
		Credentials: []integrations.CredentialStatus{
			{
				CredentialRef: "google-calendar-credential-fixture",
				Provider:      googleFixtureProviderName,
				AccountRef:    googleFixtureAccountRef,
				Status:        "active",
				StatusCode:    "ok",
			},
		},
		CalendarConnections: []integrations.CalendarConnectionStatus{
			{
				ConnectionRef: googleFixtureConnectionRef,
				Provider:      googleFixtureProviderName,
				AccountRef:    googleFixtureAccountRef,
				Status:        "active",
				StatusCode:    "ok",
			},
		},
	}, nil
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
	eventID := googleFixtureEventID(input)
	event := GoogleFixtureDispatchEvent{
		ID:                     eventID,
		SelectedCalendarRef:    input.SelectedCalendarRef,
		DestinationCalendarRef: input.DestinationCalendarRef,
	}

	switch input.Action {
	case "BOOKING_CANCELLED":
		if eventID == "" || input.RequestID == "" {
			return GoogleFixtureDispatchRequest{}, errors.New("calendar provider dispatch requires an event id and request id")
		}
		request.Operation = googleFixtureOperationCancelEvent
		request.Event = event
	case "BOOKING_RESCHEDULED":
		if eventID == "" || input.RequestID == "" || input.Start == "" || input.End == "" {
			return GoogleFixtureDispatchRequest{}, errors.New("calendar provider reschedule dispatch requires an event id, request id, start, and end")
		}
		request.Operation = googleFixtureOperationReschedule
		event.Start = input.Start
		event.End = input.End
		event.PreviousID = googleFixturePreviousEventID(input)
		request.Event = event
	default:
		return GoogleFixtureDispatchRequest{}, fmt.Errorf("calendar provider action %q is unsupported", input.Action)
	}

	return request, nil
}

func googleFixtureEventID(input DispatchInput) string {
	if input.ExternalEventID != "" {
		return input.ExternalEventID
	}
	return input.UID
}

func googleFixturePreviousEventID(input DispatchInput) string {
	if input.PreviousExternalEventID != "" {
		return input.PreviousExternalEventID
	}
	return input.RescheduleUID
}
