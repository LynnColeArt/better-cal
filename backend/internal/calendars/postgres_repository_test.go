package calendars

import (
	"context"
	"os"
	"testing"
	"time"

	calendarprovider "github.com/LynnColeArt/better-cal/backend/internal/calendar"
	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryRoundTripConnectionsAndCatalogCalendars(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 10_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_catalog where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connections where user_id = $1`, userID)
	})

	connection := CalendarConnection{
		ConnectionRef: "google-connection",
		Provider:      "google-calendar-fixture",
		AccountRef:    "google-account",
		AccountEmail:  "fixture-user@example.test",
		Status:        "active",
	}
	if _, err := repo.SaveCalendarConnection(ctx, userID, connection); err != nil {
		t.Fatal(err)
	}

	first := CatalogCalendar{
		CalendarRef:   "alpha-calendar",
		ConnectionRef: connection.ConnectionRef,
		Provider:      "google-calendar-fixture",
		ExternalID:    "google-alpha-calendar",
		Name:          "Alpha Calendar",
		Writable:      true,
	}
	second := CatalogCalendar{
		CalendarRef:   "team-calendar",
		ConnectionRef: connection.ConnectionRef,
		Provider:      "google-calendar-fixture",
		ExternalID:    "google-team-calendar",
		Name:          "Team Calendar",
		Primary:       true,
		Writable:      true,
	}
	if _, err := repo.SaveCatalogCalendar(ctx, userID, second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveCatalogCalendar(ctx, userID, first); err != nil {
		t.Fatal(err)
	}

	connections, err := repo.ReadCalendarConnections(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("connection count = %d", len(connections))
	}
	if connections[0].ConnectionRef != connection.ConnectionRef {
		t.Fatalf("connection ref = %q", connections[0].ConnectionRef)
	}

	catalog, err := repo.ReadCatalogCalendars(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 {
		t.Fatalf("catalog calendar count = %d", len(catalog))
	}
	if catalog[0].CalendarRef != first.CalendarRef || catalog[1].CalendarRef != second.CalendarRef {
		t.Fatalf("catalog calendars = %#v", catalog)
	}

	found, ok, err := repo.ReadCatalogCalendar(ctx, userID, second.CalendarRef)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("catalog calendar was not found")
	}
	if found.ConnectionRef != connection.ConnectionRef {
		t.Fatalf("catalog connection ref = %q", found.ConnectionRef)
	}
	if !found.Primary {
		t.Fatal("catalog primary flag was not persisted")
	}
}

func TestPostgresRepositoryRoundTripSelectedAndDestinationCalendars(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 20_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from destination_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from selected_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_catalog where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connections where user_id = $1`, userID)
	})

	connection := CalendarConnection{
		ConnectionRef: "google-connection",
		Provider:      "google-calendar-fixture",
		AccountRef:    "google-account",
		AccountEmail:  "fixture-user@example.test",
		Status:        "active",
	}
	if _, err := repo.SaveCalendarConnection(ctx, userID, connection); err != nil {
		t.Fatal(err)
	}

	firstCatalog := CatalogCalendar{
		CalendarRef:   "alpha-calendar",
		ConnectionRef: connection.ConnectionRef,
		Provider:      "google-calendar-fixture",
		ExternalID:    "google-alpha-calendar",
		Name:          "Alpha Calendar",
		Writable:      true,
	}
	secondCatalog := CatalogCalendar{
		CalendarRef:   "team-calendar",
		ConnectionRef: connection.ConnectionRef,
		Provider:      "google-calendar-fixture",
		ExternalID:    "google-team-calendar",
		Name:          "Team Calendar",
		Writable:      true,
	}
	if _, err := repo.SaveCatalogCalendar(ctx, userID, firstCatalog); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveCatalogCalendar(ctx, userID, secondCatalog); err != nil {
		t.Fatal(err)
	}

	first := toSelectedCalendar(firstCatalog)
	second := toSelectedCalendar(secondCatalog)
	if _, err := repo.SaveSelectedCalendar(ctx, userID, second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveSelectedCalendar(ctx, userID, first); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.SetDestinationCalendar(ctx, userID, second.CalendarRef); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("destination calendar was not set")
	}

	selected, err := repo.ReadSelectedCalendars(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected calendar count = %d", len(selected))
	}
	if selected[0].CalendarRef != first.CalendarRef || selected[1].CalendarRef != second.CalendarRef {
		t.Fatalf("selected calendars = %#v", selected)
	}

	destination, ok, err := repo.ReadDestinationCalendar(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("destination calendar was not found")
	}
	if destination.CalendarRef != second.CalendarRef {
		t.Fatalf("destination calendar ref = %q", destination.CalendarRef)
	}
}

func TestPostgresStoreSyncsProviderCatalogAndRecordsStatusTransition(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 25_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from destination_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from selected_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connection_status_history where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_catalog where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connections where user_id = $1`, userID)
	})

	provider := &mutableCatalogProvider{
		snapshot: calendarprovider.CatalogSnapshot{
			Connections: []calendarprovider.CatalogConnection{
				{
					ConnectionRef: "google-connection",
					Provider:      "google-calendar-fixture",
					AccountRef:    "google-account",
					AccountEmail:  "fixture-user@example.test",
					Status:        "active",
				},
				{
					ConnectionRef: "stale-connection",
					Provider:      "google-calendar-fixture",
					AccountRef:    "stale-account",
					AccountEmail:  "stale-user@example.test",
					Status:        "active",
				},
			},
			Calendars: []calendarprovider.CatalogCalendar{
				{
					CalendarRef:   "team-calendar",
					ConnectionRef: "google-connection",
					Provider:      "google-calendar-fixture",
					ExternalID:    "google-team-calendar",
					Name:          "Team Calendar",
					Writable:      true,
				},
				{
					CalendarRef:   "stale-calendar",
					ConnectionRef: "stale-connection",
					Provider:      "google-calendar-fixture",
					ExternalID:    "google-stale-calendar",
					Name:          "Stale Calendar",
					Writable:      true,
				},
			},
		},
	}
	store := NewStoreWithRepository(repo, WithCatalogProvider(provider))

	if err := store.SyncProviderCatalog(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveSelectedCalendar(ctx, userID, SaveSelectedCalendarRequest{CalendarRef: "team-calendar"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveSelectedCalendar(ctx, userID, SaveSelectedCalendarRequest{CalendarRef: "stale-calendar"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.SetDestinationCalendar(ctx, userID, "stale-calendar"); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("destination calendar was not set")
	}
	provider.snapshot.Connections = []calendarprovider.CatalogConnection{
		{
			ConnectionRef: "google-connection",
			Provider:      "google-calendar-fixture",
			AccountRef:    "google-account",
			AccountEmail:  "fixture-user@example.test",
			Status:        "disconnected",
		},
	}
	provider.snapshot.Calendars = []calendarprovider.CatalogCalendar{
		{
			CalendarRef:   "team-calendar",
			ConnectionRef: "google-connection",
			Provider:      "google-calendar-fixture",
			ExternalID:    "google-team-calendar-updated",
			Name:          "Team Calendar Updated",
			Writable:      true,
		},
	}
	if err := store.SyncProviderCatalog(ctx, userID); err != nil {
		t.Fatal(err)
	}

	connections, err := repo.ReadCalendarConnections(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("connection count = %d", len(connections))
	}
	if connections[0].Status != "disconnected" {
		t.Fatalf("connection status = %q", connections[0].Status)
	}
	catalog, err := repo.ReadCatalogCalendars(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 1 {
		t.Fatalf("catalog count = %d", len(catalog))
	}
	if catalog[0].Name != "Team Calendar Updated" {
		t.Fatalf("catalog name = %q", catalog[0].Name)
	}
	selected, err := repo.ReadSelectedCalendars(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("selected calendar count = %d", len(selected))
	}
	if selected[0].CalendarRef != "team-calendar" || selected[0].ExternalID != "google-team-calendar-updated" {
		t.Fatalf("selected calendar = %#v", selected[0])
	}
	if _, ok, err := repo.ReadDestinationCalendar(ctx, userID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("stale destination calendar was not cleared")
	}

	var previousStatus, nextStatus, reason string
	if err := pool.QueryRow(ctx, `
		select previous_status, next_status, reason
		from calendar_connection_status_history
		where user_id = $1 and connection_ref = $2
		order by id desc
		limit 1
	`, userID, "google-connection").Scan(&previousStatus, &nextStatus, &reason); err != nil {
		t.Fatal(err)
	}
	if previousStatus != "active" || nextStatus != "disconnected" {
		t.Fatalf("status transition = %q -> %q", previousStatus, nextStatus)
	}
	if reason != "provider_catalog_sync" {
		t.Fatalf("transition reason = %q", reason)
	}
}

func TestPostgresRepositoryRefreshesCalendarConnectionStatus(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 27_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connection_status_history where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_catalog where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connections where user_id = $1`, userID)
	})

	connection := CalendarConnection{
		ConnectionRef: "google-connection-status",
		Provider:      "google-calendar-fixture",
		AccountRef:    "google-account-status",
		AccountEmail:  "status@example.test",
		Status:        "active",
	}
	if _, err := repo.SaveCalendarConnection(ctx, userID, connection); err != nil {
		t.Fatal(err)
	}

	connections, err := repo.RefreshCalendarConnectionStatuses(ctx, userID, []CalendarConnectionStatusUpdate{
		{
			ConnectionRef: connection.ConnectionRef,
			Provider:      connection.Provider,
			AccountRef:    connection.AccountRef,
			Status:        "reauth_required",
			StatusCode:    "oauth_reauth_required",
		},
	}, "2026-04-24T12:00:00.000Z", "provider_status_refresh")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("connection count = %d", len(connections))
	}
	if connections[0].Status != "reauth_required" {
		t.Fatalf("status = %q", connections[0].Status)
	}
	if connections[0].StatusCode != "oauth_reauth_required" {
		t.Fatalf("status code = %q", connections[0].StatusCode)
	}
	if connections[0].StatusCheckedAt != "2026-04-24T12:00:00.000Z" {
		t.Fatalf("status checked at = %q", connections[0].StatusCheckedAt)
	}

	var previousStatus, nextStatus, reason string
	if err := pool.QueryRow(ctx, `
		select previous_status, next_status, reason
		from calendar_connection_status_history
		where user_id = $1 and connection_ref = $2
		order by id desc
		limit 1
	`, userID, connection.ConnectionRef).Scan(&previousStatus, &nextStatus, &reason); err != nil {
		t.Fatal(err)
	}
	if previousStatus != "active" || nextStatus != "reauth_required" {
		t.Fatalf("status transition = %q -> %q", previousStatus, nextStatus)
	}
	if reason != "provider_status_refresh" {
		t.Fatalf("transition reason = %q", reason)
	}
}

func TestPostgresRepositoryDeleteSelectedCalendarClearsDestination(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	userID := int(time.Now().UnixNano()%1_000_000_000) + 30_000
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from destination_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from selected_calendars where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_catalog where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from calendar_connections where user_id = $1`, userID)
	})

	connection := CalendarConnection{
		ConnectionRef: "google-connection",
		Provider:      "google-calendar-fixture",
		AccountRef:    "google-account",
		AccountEmail:  "fixture-user@example.test",
		Status:        "active",
	}
	if _, err := repo.SaveCalendarConnection(ctx, userID, connection); err != nil {
		t.Fatal(err)
	}

	catalog := CatalogCalendar{
		CalendarRef:   "team-calendar",
		ConnectionRef: connection.ConnectionRef,
		Provider:      "google-calendar-fixture",
		ExternalID:    "google-team-calendar",
		Name:          "Team Calendar",
		Writable:      true,
	}
	if _, err := repo.SaveCatalogCalendar(ctx, userID, catalog); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveSelectedCalendar(ctx, userID, toSelectedCalendar(catalog)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.SetDestinationCalendar(ctx, userID, catalog.CalendarRef); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("destination calendar was not set")
	}

	result, err := repo.DeleteSelectedCalendar(ctx, userID, catalog.CalendarRef)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Removed {
		t.Fatal("selected calendar was not removed")
	}
	if !result.ClearedDestination {
		t.Fatal("destination calendar was not cleared")
	}

	if _, ok, err := repo.ReadDestinationCalendar(ctx, userID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("destination calendar unexpectedly remained")
	}
}

type mutableCatalogProvider struct {
	snapshot calendarprovider.CatalogSnapshot
}

func (p *mutableCatalogProvider) ReadCatalog(context.Context, calendarprovider.CatalogInput) (calendarprovider.CatalogSnapshot, error) {
	return p.snapshot, nil
}

func testPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("CALDIY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("CALDIY_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set CALDIY_TEST_DATABASE_URL or CALDIY_DATABASE_URL to run Postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
