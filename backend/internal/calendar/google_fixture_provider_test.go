package calendar

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGoogleFixtureProviderPreparesCancelDispatch(t *testing.T) {
	provider := NewGoogleFixtureProvider()

	prepared, err := provider.PrepareDispatch(context.Background(), DispatchInput{
		Action:                 "BOOKING_CANCELLED",
		CreatedAt:              "2026-01-01T00:10:00.000Z",
		UID:                    "booking-uid",
		RequestID:              "request-id",
		SelectedCalendarRef:    "selected-calendar-fixture",
		DestinationCalendarRef: "destination-calendar-fixture",
		ExternalEventID:        "external-event-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ContentType != "application/json" {
		t.Fatalf("content type = %q", prepared.ContentType)
	}
	if prepared.Headers[googleFixtureHeaderProvider] != googleFixtureProviderName {
		t.Fatalf("provider header = %q", prepared.Headers[googleFixtureHeaderProvider])
	}

	var request GoogleFixtureDispatchRequest
	if err := json.Unmarshal([]byte(prepared.Body), &request); err != nil {
		t.Fatal(err)
	}
	if request.Provider != googleFixtureProviderName {
		t.Fatalf("provider = %q", request.Provider)
	}
	if request.Operation != googleFixtureOperationCancelEvent {
		t.Fatalf("operation = %q", request.Operation)
	}
	if request.Event.ID != "external-event-id" {
		t.Fatalf("event id = %q", request.Event.ID)
	}
	if request.Event.SelectedCalendarRef != "selected-calendar-fixture" {
		t.Fatalf("selected calendar ref = %q", request.Event.SelectedCalendarRef)
	}
	if request.Event.DestinationCalendarRef != "destination-calendar-fixture" {
		t.Fatalf("destination calendar ref = %q", request.Event.DestinationCalendarRef)
	}
	if request.Event.Start != "" || request.Event.End != "" || request.Event.PreviousID != "" {
		t.Fatalf("unexpected event payload = %#v", request.Event)
	}
}

func TestGoogleFixtureProviderReadsCatalog(t *testing.T) {
	provider := NewGoogleFixtureProvider()

	snapshot, err := provider.ReadCatalog(context.Background(), CatalogInput{UserID: 123})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Connections) != 1 {
		t.Fatalf("connection count = %d", len(snapshot.Connections))
	}
	if snapshot.Connections[0].ConnectionRef != googleFixtureConnectionRef {
		t.Fatalf("connection ref = %q", snapshot.Connections[0].ConnectionRef)
	}
	if snapshot.Connections[0].Status != "active" {
		t.Fatalf("connection status = %q", snapshot.Connections[0].Status)
	}
	if len(snapshot.Calendars) != 3 {
		t.Fatalf("calendar count = %d", len(snapshot.Calendars))
	}
	if snapshot.Calendars[2].CalendarRef != "team-calendar-fixture" {
		t.Fatalf("team calendar ref = %q", snapshot.Calendars[2].CalendarRef)
	}
}

func TestGoogleFixtureProviderPreparesRescheduleDispatch(t *testing.T) {
	provider := NewGoogleFixtureProvider()

	prepared, err := provider.PrepareDispatch(context.Background(), DispatchInput{
		Action:                  "BOOKING_RESCHEDULED",
		CreatedAt:               "2026-01-01T00:11:00.000Z",
		UID:                     "booking-uid",
		RequestID:               "request-id",
		Start:                   "2026-05-02T15:00:00.000Z",
		End:                     "2026-05-02T15:30:00.000Z",
		SelectedCalendarRef:     "selected-calendar-fixture",
		DestinationCalendarRef:  "destination-calendar-fixture",
		ExternalEventID:         "external-event-id",
		RescheduleUID:           "previous-booking-uid",
		PreviousExternalEventID: "previous-external-event-id",
	})
	if err != nil {
		t.Fatal(err)
	}

	var request GoogleFixtureDispatchRequest
	if err := json.Unmarshal([]byte(prepared.Body), &request); err != nil {
		t.Fatal(err)
	}
	if request.Operation != googleFixtureOperationReschedule {
		t.Fatalf("operation = %q", request.Operation)
	}
	if request.Event.ID != "external-event-id" {
		t.Fatalf("event id = %q", request.Event.ID)
	}
	if request.Event.Start != "2026-05-02T15:00:00.000Z" {
		t.Fatalf("event start = %q", request.Event.Start)
	}
	if request.Event.End != "2026-05-02T15:30:00.000Z" {
		t.Fatalf("event end = %q", request.Event.End)
	}
	if request.Event.PreviousID != "previous-external-event-id" {
		t.Fatalf("event previous id = %q", request.Event.PreviousID)
	}
	if request.Event.SelectedCalendarRef != "selected-calendar-fixture" {
		t.Fatalf("selected calendar ref = %q", request.Event.SelectedCalendarRef)
	}
	if request.Event.DestinationCalendarRef != "destination-calendar-fixture" {
		t.Fatalf("destination calendar ref = %q", request.Event.DestinationCalendarRef)
	}
}
