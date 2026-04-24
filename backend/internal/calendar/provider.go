package calendar

import "context"

type DispatchInput struct {
	Action                  string
	CreatedAt               string
	UID                     string
	Status                  string
	Start                   string
	End                     string
	EventTypeID             int
	RequestID               string
	SelectedCalendarRef     string
	DestinationCalendarRef  string
	ExternalEventID         string
	RescheduleUID           string
	PreviousExternalEventID string
}

type PreparedDispatch struct {
	TargetURL   string
	ContentType string
	Headers     map[string]string
	Body        string
}

type ProviderAdapter interface {
	PrepareDispatch(context.Context, DispatchInput) (PreparedDispatch, error)
}
