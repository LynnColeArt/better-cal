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

type CatalogInput struct {
	UserID int
}

type CatalogSnapshot struct {
	Connections []CatalogConnection
	Calendars   []CatalogCalendar
}

type CatalogConnection struct {
	ConnectionRef string
	Provider      string
	AccountRef    string
	AccountEmail  string
	Status        string
}

type CatalogCalendar struct {
	CalendarRef   string
	ConnectionRef string
	Provider      string
	ExternalID    string
	Name          string
	Primary       bool
	Writable      bool
}

type CatalogProviderAdapter interface {
	ReadCatalog(context.Context, CatalogInput) (CatalogSnapshot, error)
}
