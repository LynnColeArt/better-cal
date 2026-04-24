package calendars

import (
	"context"
	"testing"
)

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
	userID := 456

	saved, err := store.SaveSelectedCalendar(context.Background(), userID, SaveSelectedCalendarRequest{
		CalendarRef: "team-calendar",
		Provider:    "google-calendar-fixture",
		ExternalID:  "google-team-calendar",
		Name:        "Team Calendar",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.CalendarRef != "team-calendar" {
		t.Fatalf("saved calendar ref = %q", saved.CalendarRef)
	}

	destination, ok, err := store.SetDestinationCalendar(context.Background(), userID, "team-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("saved calendar was not set as destination")
	}
	if destination.CalendarRef != "team-calendar" {
		t.Fatalf("destination calendar ref = %q", destination.CalendarRef)
	}

	result, err := store.DeleteSelectedCalendar(context.Background(), userID, "team-calendar")
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
