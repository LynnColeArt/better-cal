package calendars

import (
	"context"
	"errors"
	"testing"
)

func TestStoreReadsFixtureConnectionsAndCatalog(t *testing.T) {
	store := NewStore()

	connections, err := store.ReadCalendarConnections(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("connection count = %d", len(connections))
	}
	if connections[0].ConnectionRef != FixtureCalendarConnectionRef {
		t.Fatalf("connection ref = %q", connections[0].ConnectionRef)
	}

	catalog, err := store.ReadCatalogCalendars(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 3 {
		t.Fatalf("catalog count = %d", len(catalog))
	}
	if catalog[2].CalendarRef != FixtureTeamCalendarRef {
		t.Fatalf("third catalog ref = %q", catalog[2].CalendarRef)
	}
}

func TestStoreReadsFixtureCalendarsAndDestination(t *testing.T) {
	store := NewStore()

	selected, err := store.ReadSelectedCalendars(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected calendar count = %d", len(selected))
	}
	if selected[0].CalendarRef != FixtureDestinationCalendarRef {
		t.Fatalf("first selected calendar ref = %q", selected[0].CalendarRef)
	}

	destination, ok, err := store.ReadDestinationCalendar(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("fixture destination calendar was not found")
	}
	if destination.CalendarRef != FixtureDestinationCalendarRef {
		t.Fatalf("destination calendar ref = %q", destination.CalendarRef)
	}
}

func TestStoreSaveSetDestinationAndDeleteSelectedCalendar(t *testing.T) {
	store := NewStore()
	userID := fixtureUserID

	saved, err := store.SaveSelectedCalendar(context.Background(), userID, SaveSelectedCalendarRequest{
		CalendarRef: FixtureTeamCalendarRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.CalendarRef != FixtureTeamCalendarRef {
		t.Fatalf("saved calendar ref = %q", saved.CalendarRef)
	}
	if saved.ExternalID != "google-calendar-team" {
		t.Fatalf("saved external id = %q", saved.ExternalID)
	}

	destination, ok, err := store.SetDestinationCalendar(context.Background(), userID, FixtureTeamCalendarRef)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("saved calendar was not set as destination")
	}
	if destination.CalendarRef != FixtureTeamCalendarRef {
		t.Fatalf("destination calendar ref = %q", destination.CalendarRef)
	}

	result, err := store.DeleteSelectedCalendar(context.Background(), userID, FixtureTeamCalendarRef)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Removed {
		t.Fatal("selected calendar was not removed")
	}
	if !result.ClearedDestination {
		t.Fatal("destination calendar was not cleared")
	}

	if _, ok, err := store.ReadDestinationCalendar(context.Background(), userID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("destination calendar still existed after delete")
	}
}

func TestStoreRejectsUnknownCatalogCalendar(t *testing.T) {
	store := NewStore()

	_, err := store.SaveSelectedCalendar(context.Background(), fixtureUserID, SaveSelectedCalendarRequest{
		CalendarRef: "missing-calendar",
	})
	if !errors.Is(err, ErrCalendarCatalogEntryNotFound) {
		t.Fatalf("err = %v", err)
	}
}
