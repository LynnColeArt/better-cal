package calendars

import (
	"context"
	"errors"
	"testing"

	calendarprovider "github.com/LynnColeArt/better-cal/backend/internal/calendar"
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

func TestStoreSyncsProviderCatalog(t *testing.T) {
	provider := &mutableCatalogProvider{
		snapshot: calendarprovider.CatalogSnapshot{
			Connections: []calendarprovider.CatalogConnection{
				{
					ConnectionRef: "provider-connection",
					Provider:      "google-calendar-fixture",
					AccountRef:    "provider-account",
					AccountEmail:  "provider@example.test",
					Status:        "active",
				},
			},
			Calendars: []calendarprovider.CatalogCalendar{
				{
					CalendarRef:   "provider-calendar",
					ConnectionRef: "provider-connection",
					Provider:      "google-calendar-fixture",
					ExternalID:    "provider-external-calendar",
					Name:          "Provider Calendar",
					Writable:      true,
				},
			},
		},
	}
	store := NewStore(WithCatalogProvider(provider))

	if err := store.SyncProviderCatalog(context.Background(), 321); err != nil {
		t.Fatal(err)
	}
	connections, err := store.ReadCalendarConnections(context.Background(), 321)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("connection count = %d", len(connections))
	}
	if connections[0].ConnectionRef != "provider-connection" {
		t.Fatalf("connection ref = %q", connections[0].ConnectionRef)
	}
	catalog, err := store.ReadCatalogCalendars(context.Background(), 321)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 1 {
		t.Fatalf("catalog count = %d", len(catalog))
	}
	if catalog[0].CalendarRef != "provider-calendar" {
		t.Fatalf("calendar ref = %q", catalog[0].CalendarRef)
	}

	saved, err := store.SaveSelectedCalendar(context.Background(), 321, SaveSelectedCalendarRequest{CalendarRef: "provider-calendar"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ExternalID != "provider-external-calendar" {
		t.Fatalf("selected external id = %q", saved.ExternalID)
	}
}

func TestStoreSyncRefreshesSelectedCalendarsFromProviderCatalog(t *testing.T) {
	provider := &mutableCatalogProvider{
		snapshot: calendarprovider.CatalogSnapshot{
			Connections: []calendarprovider.CatalogConnection{
				{
					ConnectionRef: "provider-connection",
					Provider:      "google-calendar-fixture",
					AccountRef:    "provider-account",
					AccountEmail:  "provider@example.test",
					Status:        "active",
				},
			},
			Calendars: []calendarprovider.CatalogCalendar{
				{
					CalendarRef:   "kept-calendar",
					ConnectionRef: "provider-connection",
					Provider:      "google-calendar-fixture",
					ExternalID:    "kept-external-calendar",
					Name:          "Kept Calendar",
					Writable:      true,
				},
				{
					CalendarRef:   "removed-calendar",
					ConnectionRef: "provider-connection",
					Provider:      "google-calendar-fixture",
					ExternalID:    "removed-external-calendar",
					Name:          "Removed Calendar",
					Writable:      true,
				},
			},
		},
	}
	store := NewStore(WithCatalogProvider(provider))
	ctx := context.Background()

	if err := store.SyncProviderCatalog(ctx, 321); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveSelectedCalendar(ctx, 321, SaveSelectedCalendarRequest{CalendarRef: "kept-calendar"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveSelectedCalendar(ctx, 321, SaveSelectedCalendarRequest{CalendarRef: "removed-calendar"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.SetDestinationCalendar(ctx, 321, "removed-calendar"); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("destination calendar was not set")
	}

	provider.snapshot.Calendars = []calendarprovider.CatalogCalendar{
		{
			CalendarRef:   "kept-calendar",
			ConnectionRef: "provider-connection",
			Provider:      "google-calendar-fixture",
			ExternalID:    "kept-external-calendar-updated",
			Name:          "Kept Calendar Updated",
			Writable:      true,
		},
	}
	if err := store.SyncProviderCatalog(ctx, 321); err != nil {
		t.Fatal(err)
	}

	selected, err := store.ReadSelectedCalendars(ctx, 321)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("selected calendar count = %d", len(selected))
	}
	if selected[0].CalendarRef != "kept-calendar" || selected[0].Name != "Kept Calendar Updated" {
		t.Fatalf("selected calendar = %#v", selected[0])
	}
	if _, ok, err := store.ReadDestinationCalendar(ctx, 321); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("stale destination calendar was not cleared")
	}
}

func TestStoreRejectsInvalidProviderCatalogSnapshots(t *testing.T) {
	validConnection := calendarprovider.CatalogConnection{
		ConnectionRef: "provider-connection",
		Provider:      "google-calendar-fixture",
		AccountRef:    "provider-account",
		AccountEmail:  "provider@example.test",
		Status:        "active",
	}
	validCalendar := calendarprovider.CatalogCalendar{
		CalendarRef:   "provider-calendar",
		ConnectionRef: "provider-connection",
		Provider:      "google-calendar-fixture",
		ExternalID:    "provider-external-calendar",
		Name:          "Provider Calendar",
		Writable:      true,
	}

	tests := []struct {
		name        string
		connections []calendarprovider.CatalogConnection
		calendars   []calendarprovider.CatalogCalendar
	}{
		{
			name:        "duplicate connection ref",
			connections: []calendarprovider.CatalogConnection{validConnection, validConnection},
			calendars:   []calendarprovider.CatalogCalendar{validCalendar},
		},
		{
			name:        "duplicate calendar ref",
			connections: []calendarprovider.CatalogConnection{validConnection},
			calendars:   []calendarprovider.CatalogCalendar{validCalendar, validCalendar},
		},
		{
			name:        "provider mismatch",
			connections: []calendarprovider.CatalogConnection{validConnection},
			calendars: []calendarprovider.CatalogCalendar{
				{
					CalendarRef:   "provider-calendar",
					ConnectionRef: "provider-connection",
					Provider:      "outlook-calendar-fixture",
					ExternalID:    "provider-external-calendar",
					Name:          "Provider Calendar",
				},
			},
		},
		{
			name:        "duplicate external ref",
			connections: []calendarprovider.CatalogConnection{validConnection},
			calendars: []calendarprovider.CatalogCalendar{
				validCalendar,
				{
					CalendarRef:   "second-provider-calendar",
					ConnectionRef: "provider-connection",
					Provider:      "google-calendar-fixture",
					ExternalID:    "provider-external-calendar",
					Name:          "Second Provider Calendar",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore(WithCatalogProvider(staticCatalogProvider{
				snapshot: calendarprovider.CatalogSnapshot{
					Connections: tc.connections,
					Calendars:   tc.calendars,
				},
			}))
			err := store.SyncProviderCatalog(context.Background(), 321)
			if !errors.Is(err, ErrInvalidCalendarCatalogSnapshot) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

type staticCatalogProvider struct {
	snapshot calendarprovider.CatalogSnapshot
}

func (p staticCatalogProvider) ReadCatalog(context.Context, calendarprovider.CatalogInput) (calendarprovider.CatalogSnapshot, error) {
	return p.snapshot, nil
}
