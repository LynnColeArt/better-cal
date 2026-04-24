package calendars

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	calendarprovider "github.com/LynnColeArt/better-cal/backend/internal/calendar"
	"github.com/LynnColeArt/better-cal/backend/internal/integrations"
)

const (
	fixtureUserID                 = 123
	FixtureCalendarConnectionRef  = "google-calendar-connection-fixture"
	FixtureCalendarAccountRef     = "google-account-fixture"
	FixtureSelectedCalendarRef    = "selected-calendar-fixture"
	FixtureDestinationCalendarRef = "destination-calendar-fixture"
	FixtureTeamCalendarRef        = "team-calendar-fixture"
	catalogSyncTransitionReason   = "provider_catalog_sync"
	statusRefreshTransitionReason = "provider_status_refresh"
	statusWireTimeLayout          = "2006-01-02T15:04:05.000Z"
)

var (
	ErrInvalidSelectedCalendar        = errors.New("invalid selected calendar")
	ErrInvalidSelectedCalendarRef     = errors.New("invalid selected calendar ref")
	ErrInvalidDestinationCalendarRef  = errors.New("invalid destination calendar ref")
	ErrCalendarCatalogEntryNotFound   = errors.New("calendar catalog entry not found")
	ErrCalendarCatalogProviderUnset   = errors.New("calendar catalog provider is not configured")
	ErrCalendarStatusProviderUnset    = errors.New("calendar status provider is not configured")
	ErrInvalidCalendarCatalogSnapshot = errors.New("invalid calendar catalog snapshot")
	ErrInvalidCalendarStatusSnapshot  = errors.New("invalid calendar status snapshot")
)

type CalendarConnection struct {
	ConnectionRef   string `json:"connectionRef"`
	Provider        string `json:"provider"`
	AccountRef      string `json:"accountRef"`
	AccountEmail    string `json:"accountEmail"`
	Status          string `json:"status"`
	StatusCode      string `json:"statusCode,omitempty"`
	StatusCheckedAt string `json:"statusCheckedAt,omitempty"`
}

type CatalogCalendar struct {
	CalendarRef   string `json:"calendarRef"`
	ConnectionRef string `json:"connectionRef"`
	Provider      string `json:"provider"`
	ExternalID    string `json:"externalId"`
	Name          string `json:"name"`
	Primary       bool   `json:"primary"`
	Writable      bool   `json:"writable"`
}

type CalendarConnectionStatusTransition struct {
	ConnectionRef  string
	Provider       string
	PreviousStatus string
	NextStatus     string
	Reason         string
}

type CalendarConnectionStatusUpdate struct {
	ConnectionRef string
	Provider      string
	AccountRef    string
	Status        string
	StatusCode    string
}

type SelectedCalendar struct {
	CalendarRef string `json:"calendarRef"`
	Provider    string `json:"provider"`
	ExternalID  string `json:"externalId"`
	Name        string `json:"name"`
}

type SaveSelectedCalendarRequest struct {
	CalendarRef string `json:"calendarRef"`
	Provider    string `json:"provider"`
	ExternalID  string `json:"externalId"`
	Name        string `json:"name"`
}

type DeleteSelectedCalendarResult struct {
	Removed            bool
	ClearedDestination bool
}

type Repository interface {
	ReadCalendarConnections(ctx context.Context, userID int) ([]CalendarConnection, error)
	SaveCalendarConnection(ctx context.Context, userID int, connection CalendarConnection) (CalendarConnection, error)
	SyncCalendarCatalog(ctx context.Context, userID int, connections []CalendarConnection, catalog []CatalogCalendar, transitionReason string) ([]CalendarConnection, []CatalogCalendar, error)
	RefreshCalendarConnectionStatuses(ctx context.Context, userID int, updates []CalendarConnectionStatusUpdate, checkedAt string, transitionReason string) ([]CalendarConnection, error)
	ReadCatalogCalendars(ctx context.Context, userID int) ([]CatalogCalendar, error)
	SaveCatalogCalendar(ctx context.Context, userID int, calendar CatalogCalendar) (CatalogCalendar, error)
	ReadCatalogCalendar(ctx context.Context, userID int, calendarRef string) (CatalogCalendar, bool, error)
	ReadSelectedCalendars(ctx context.Context, userID int) ([]SelectedCalendar, error)
	SaveSelectedCalendar(ctx context.Context, userID int, calendar SelectedCalendar) (SelectedCalendar, error)
	DeleteSelectedCalendar(ctx context.Context, userID int, calendarRef string) (DeleteSelectedCalendarResult, error)
	ReadDestinationCalendar(ctx context.Context, userID int) (SelectedCalendar, bool, error)
	SetDestinationCalendar(ctx context.Context, userID int, calendarRef string) (SelectedCalendar, bool, error)
}

type Store struct {
	mu              sync.Mutex
	repo            Repository
	catalogProvider calendarprovider.CatalogProviderAdapter
	statusProvider  integrations.StatusProviderAdapter
	connections     map[int][]CalendarConnection
	catalog         map[int][]CatalogCalendar
	selected        map[int][]SelectedCalendar
	destination     map[int]string
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
	}
}

func WithCatalogProvider(provider calendarprovider.CatalogProviderAdapter) StoreOption {
	return func(s *Store) {
		s.catalogProvider = provider
	}
}

func WithStatusProvider(provider integrations.StatusProviderAdapter) StoreOption {
	return func(s *Store) {
		s.statusProvider = provider
	}
}

func NewStore(opts ...StoreOption) *Store {
	store := &Store{
		connections: map[int][]CalendarConnection{},
		catalog:     map[int][]CatalogCalendar{},
		selected:    map[int][]SelectedCalendar{},
		destination: map[int]string{},
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

func NewStoreWithRepository(repo Repository, opts ...StoreOption) *Store {
	return NewStore(append([]StoreOption{WithRepository(repo)}, opts...)...)
}

func (s *Store) ReadCalendarConnections(ctx context.Context, userID int) ([]CalendarConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return nil, err
	}
	return append([]CalendarConnection(nil), s.connections[userID]...), nil
}

func (s *Store) ReadCatalogCalendars(ctx context.Context, userID int) ([]CatalogCalendar, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return nil, err
	}
	return append([]CatalogCalendar(nil), s.catalog[userID]...), nil
}

func (s *Store) SyncProviderCatalog(ctx context.Context, userID int) error {
	if s.catalogProvider == nil {
		return ErrCalendarCatalogProviderUnset
	}
	snapshot, err := s.catalogProvider.ReadCatalog(ctx, calendarprovider.CatalogInput{UserID: userID})
	if err != nil {
		return fmt.Errorf("read provider calendar catalog: %w", err)
	}
	connections, err := calendarConnectionsFromProvider(snapshot.Connections)
	if err != nil {
		return err
	}
	catalog, err := catalogCalendarsFromProvider(snapshot.Calendars)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyProviderCatalogLocked(ctx, userID, connections, catalog)
}

func (s *Store) RefreshProviderConnectionStatus(ctx context.Context, userID int) error {
	if s.statusProvider == nil {
		return ErrCalendarStatusProviderUnset
	}
	snapshot, err := s.statusProvider.ReadStatus(ctx, integrations.StatusInput{UserID: userID})
	if err != nil {
		return err
	}
	updates, err := calendarConnectionStatusUpdatesFromProvider(snapshot.CalendarConnections)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return err
	}
	checkedAt := currentConnectionStatusWireTime()
	if err := validateCalendarConnectionStatusUpdates(s.connections[userID], updates); err != nil {
		return err
	}
	if s.repo != nil {
		connections, err := s.repo.RefreshCalendarConnectionStatuses(ctx, userID, updates, checkedAt, statusRefreshTransitionReason)
		if err != nil {
			return err
		}
		s.connections[userID] = cloneCalendarConnections(connections)
		return nil
	}
	s.connections[userID] = applyCalendarConnectionStatusUpdates(s.connections[userID], updates, checkedAt)
	return nil
}

func (s *Store) ReadSelectedCalendars(ctx context.Context, userID int) ([]SelectedCalendar, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return nil, err
	}
	return append([]SelectedCalendar(nil), s.selected[userID]...), nil
}

func (s *Store) SaveSelectedCalendar(ctx context.Context, userID int, req SaveSelectedCalendarRequest) (SelectedCalendar, error) {
	if req.CalendarRef == "" {
		return SelectedCalendar{}, ErrInvalidSelectedCalendar
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return SelectedCalendar{}, err
	}

	catalogCalendar, ok := s.catalogCalendarLocked(userID, req.CalendarRef)
	if !ok {
		return SelectedCalendar{}, ErrCalendarCatalogEntryNotFound
	}
	calendar := toSelectedCalendar(catalogCalendar)
	if s.repo != nil {
		persisted, err := s.repo.SaveSelectedCalendar(ctx, userID, calendar)
		if err != nil {
			return SelectedCalendar{}, err
		}
		calendar = persisted
	}

	items := append([]SelectedCalendar(nil), s.selected[userID]...)
	replaced := false
	for index := range items {
		if items[index].CalendarRef == calendar.CalendarRef {
			items[index] = calendar
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, calendar)
	}
	sortSelectedCalendars(items)
	s.selected[userID] = items
	return calendar, nil
}

func (s *Store) DeleteSelectedCalendar(ctx context.Context, userID int, calendarRef string) (DeleteSelectedCalendarResult, error) {
	if calendarRef == "" {
		return DeleteSelectedCalendarResult{}, ErrInvalidSelectedCalendarRef
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return DeleteSelectedCalendarResult{}, err
	}

	result := DeleteSelectedCalendarResult{}
	if s.repo != nil {
		repoResult, err := s.repo.DeleteSelectedCalendar(ctx, userID, calendarRef)
		if err != nil {
			return DeleteSelectedCalendarResult{}, err
		}
		result = repoResult
	} else if s.destination[userID] == calendarRef {
		delete(s.destination, userID)
		result.ClearedDestination = true
	}

	items := s.selected[userID]
	filtered := items[:0]
	for _, item := range items {
		if item.CalendarRef == calendarRef {
			result.Removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	s.selected[userID] = append([]SelectedCalendar(nil), filtered...)
	if !result.Removed {
		return DeleteSelectedCalendarResult{}, nil
	}
	if result.ClearedDestination {
		delete(s.destination, userID)
	}
	return result, nil
}

func (s *Store) ReadDestinationCalendar(ctx context.Context, userID int) (SelectedCalendar, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return SelectedCalendar{}, false, err
	}

	calendarRef := s.destination[userID]
	if calendarRef == "" {
		return SelectedCalendar{}, false, nil
	}
	for _, calendar := range s.selected[userID] {
		if calendar.CalendarRef == calendarRef {
			return calendar, true, nil
		}
	}
	delete(s.destination, userID)
	return SelectedCalendar{}, false, nil
}

func (s *Store) SetDestinationCalendar(ctx context.Context, userID int, calendarRef string) (SelectedCalendar, bool, error) {
	if calendarRef == "" {
		return SelectedCalendar{}, false, ErrInvalidDestinationCalendarRef
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return SelectedCalendar{}, false, err
	}

	if s.repo != nil {
		calendar, ok, err := s.repo.SetDestinationCalendar(ctx, userID, calendarRef)
		if err != nil {
			return SelectedCalendar{}, false, err
		}
		if !ok {
			return SelectedCalendar{}, false, nil
		}
		s.destination[userID] = calendarRef
		return calendar, true, nil
	}

	for _, calendar := range s.selected[userID] {
		if calendar.CalendarRef == calendarRef {
			s.destination[userID] = calendarRef
			return calendar, true, nil
		}
	}
	return SelectedCalendar{}, false, nil
}

func (s *Store) ensureLoadedLocked(ctx context.Context, userID int) error {
	if _, ok := s.selected[userID]; ok {
		return nil
	}
	if s.repo == nil {
		if _, ok := s.connections[userID]; !ok {
			s.connections[userID] = fixtureCalendarConnections(userID)
		}
		if _, ok := s.catalog[userID]; !ok {
			s.catalog[userID] = fixtureCatalogCalendars(userID)
		}
		s.selected[userID] = fixtureSelectedCalendars(userID)
		if userID == fixtureUserID {
			s.destination[userID] = FixtureDestinationCalendarRef
		}
		return nil
	}

	connections, err := s.repo.ReadCalendarConnections(ctx, userID)
	if err != nil {
		return err
	}
	catalog, err := s.repo.ReadCatalogCalendars(ctx, userID)
	if err != nil {
		return err
	}
	selected, err := s.repo.ReadSelectedCalendars(ctx, userID)
	if err != nil {
		return err
	}
	destination, ok, err := s.repo.ReadDestinationCalendar(ctx, userID)
	if err != nil {
		return err
	}

	if (len(connections) == 0 || len(catalog) == 0) && s.catalogProvider != nil {
		snapshot, err := s.catalogProvider.ReadCatalog(ctx, calendarprovider.CatalogInput{UserID: userID})
		if err != nil {
			return fmt.Errorf("read provider calendar catalog: %w", err)
		}
		providerConnections, err := calendarConnectionsFromProvider(snapshot.Connections)
		if err != nil {
			return err
		}
		providerCatalog, err := catalogCalendarsFromProvider(snapshot.Calendars)
		if err != nil {
			return err
		}
		if err := validateProviderCatalog(providerConnections, providerCatalog); err != nil {
			return err
		}
		if len(providerConnections) > 0 || len(providerCatalog) > 0 {
			if err := s.applyProviderCatalogLocked(ctx, userID, providerConnections, providerCatalog); err != nil {
				return err
			}
			connections = append([]CalendarConnection(nil), s.connections[userID]...)
			catalog = append([]CatalogCalendar(nil), s.catalog[userID]...)
		}
	}
	if len(connections) == 0 && len(catalog) == 0 && userID == fixtureUserID {
		for _, fixtureConnection := range fixtureCalendarConnections(userID) {
			if _, err := s.repo.SaveCalendarConnection(ctx, userID, fixtureConnection); err != nil {
				return err
			}
		}
		for _, fixtureCalendar := range fixtureCatalogCalendars(userID) {
			if _, err := s.repo.SaveCatalogCalendar(ctx, userID, fixtureCalendar); err != nil {
				return err
			}
		}
		connections, err = s.repo.ReadCalendarConnections(ctx, userID)
		if err != nil {
			return err
		}
		catalog, err = s.repo.ReadCatalogCalendars(ctx, userID)
		if err != nil {
			return err
		}
	}
	if len(selected) == 0 && !ok && userID == fixtureUserID {
		for _, fixtureSelected := range fixtureSelectedCalendars(userID) {
			if _, err := s.repo.SaveSelectedCalendar(ctx, userID, fixtureSelected); err != nil {
				return err
			}
		}
		if _, ok, err := s.repo.SetDestinationCalendar(ctx, userID, FixtureDestinationCalendarRef); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("seed destination calendar not found")
		}
		selected, err = s.repo.ReadSelectedCalendars(ctx, userID)
		if err != nil {
			return err
		}
		destination, ok, err = s.repo.ReadDestinationCalendar(ctx, userID)
		if err != nil {
			return err
		}
	}

	s.connections[userID] = append([]CalendarConnection(nil), connections...)
	s.catalog[userID] = append([]CatalogCalendar(nil), catalog...)
	s.selected[userID] = append([]SelectedCalendar(nil), selected...)
	if ok {
		s.destination[userID] = destination.CalendarRef
	}
	return nil
}

func (s *Store) applyProviderCatalogLocked(ctx context.Context, userID int, connections []CalendarConnection, catalog []CatalogCalendar) error {
	if err := validateProviderCatalog(connections, catalog); err != nil {
		return err
	}
	if s.repo != nil {
		var err error
		connections, catalog, err = s.repo.SyncCalendarCatalog(ctx, userID, connections, catalog, catalogSyncTransitionReason)
		if err != nil {
			return err
		}
	}
	persistedConnections := append([]CalendarConnection(nil), connections...)
	persistedCatalog := append([]CatalogCalendar(nil), catalog...)
	sortCalendarConnections(persistedConnections)
	sortCatalogCalendars(persistedCatalog)
	s.connections[userID] = persistedConnections
	s.catalog[userID] = persistedCatalog
	s.refreshSelectedForCatalogLocked(userID)
	return nil
}

func (s *Store) catalogCalendarLocked(userID int, calendarRef string) (CatalogCalendar, bool) {
	for _, calendar := range s.catalog[userID] {
		if calendar.CalendarRef == calendarRef {
			return calendar, true
		}
	}
	return CatalogCalendar{}, false
}

func toSelectedCalendar(calendar CatalogCalendar) SelectedCalendar {
	return SelectedCalendar{
		CalendarRef: calendar.CalendarRef,
		Provider:    calendar.Provider,
		ExternalID:  calendar.ExternalID,
		Name:        calendar.Name,
	}
}

func (s *Store) refreshSelectedForCatalogLocked(userID int) {
	catalogByRef := map[string]CatalogCalendar{}
	for _, calendar := range s.catalog[userID] {
		catalogByRef[calendar.CalendarRef] = calendar
	}

	if selected, ok := s.selected[userID]; ok {
		refreshed := selected[:0]
		for _, calendar := range selected {
			catalogCalendar, ok := catalogByRef[calendar.CalendarRef]
			if !ok {
				continue
			}
			refreshed = append(refreshed, toSelectedCalendar(catalogCalendar))
		}
		s.selected[userID] = append([]SelectedCalendar(nil), refreshed...)
	}
	if destinationRef := s.destination[userID]; destinationRef != "" {
		if _, ok := catalogByRef[destinationRef]; !ok {
			delete(s.destination, userID)
		}
	}
}

func sortSelectedCalendars(items []SelectedCalendar) {
	slices.SortFunc(items, func(left SelectedCalendar, right SelectedCalendar) int {
		switch {
		case left.CalendarRef < right.CalendarRef:
			return -1
		case left.CalendarRef > right.CalendarRef:
			return 1
		default:
			return 0
		}
	})
}

func sortCalendarConnections(items []CalendarConnection) {
	slices.SortFunc(items, func(left CalendarConnection, right CalendarConnection) int {
		switch {
		case left.ConnectionRef < right.ConnectionRef:
			return -1
		case left.ConnectionRef > right.ConnectionRef:
			return 1
		default:
			return 0
		}
	})
}

func cloneCalendarConnections(items []CalendarConnection) []CalendarConnection {
	cloned := append([]CalendarConnection(nil), items...)
	sortCalendarConnections(cloned)
	return cloned
}

func sortCatalogCalendars(items []CatalogCalendar) {
	slices.SortFunc(items, func(left CatalogCalendar, right CatalogCalendar) int {
		switch {
		case left.CalendarRef < right.CalendarRef:
			return -1
		case left.CalendarRef > right.CalendarRef:
			return 1
		default:
			return 0
		}
	})
}

func calendarConnectionStatusUpdatesFromProvider(items []integrations.CalendarConnectionStatus) ([]CalendarConnectionStatusUpdate, error) {
	updates := make([]CalendarConnectionStatusUpdate, 0, len(items))
	for _, item := range items {
		if item.ConnectionRef == "" || item.Provider == "" || item.AccountRef == "" || item.Status == "" {
			return nil, ErrInvalidCalendarStatusSnapshot
		}
		updates = append(updates, CalendarConnectionStatusUpdate{
			ConnectionRef: item.ConnectionRef,
			Provider:      item.Provider,
			AccountRef:    item.AccountRef,
			Status:        item.Status,
			StatusCode:    item.StatusCode,
		})
	}
	return updates, nil
}

func validateCalendarConnectionStatusUpdates(existing []CalendarConnection, updates []CalendarConnectionStatusUpdate) error {
	existingByRef := map[string]CalendarConnection{}
	for _, connection := range existing {
		existingByRef[connection.ConnectionRef] = connection
	}
	seen := map[string]struct{}{}
	for _, update := range updates {
		if update.ConnectionRef == "" || update.Provider == "" || update.AccountRef == "" || update.Status == "" {
			return ErrInvalidCalendarStatusSnapshot
		}
		if _, ok := seen[update.ConnectionRef]; ok {
			return ErrInvalidCalendarStatusSnapshot
		}
		seen[update.ConnectionRef] = struct{}{}
		connection, ok := existingByRef[update.ConnectionRef]
		if !ok || connection.Provider != update.Provider || connection.AccountRef != update.AccountRef {
			return ErrInvalidCalendarStatusSnapshot
		}
	}
	return nil
}

func applyCalendarConnectionStatusUpdates(existing []CalendarConnection, updates []CalendarConnectionStatusUpdate, checkedAt string) []CalendarConnection {
	updatedByRef := map[string]CalendarConnectionStatusUpdate{}
	for _, update := range updates {
		updatedByRef[update.ConnectionRef] = update
	}
	connections := cloneCalendarConnections(existing)
	for index := range connections {
		update, ok := updatedByRef[connections[index].ConnectionRef]
		if !ok {
			continue
		}
		connections[index].Status = update.Status
		connections[index].StatusCode = update.StatusCode
		connections[index].StatusCheckedAt = checkedAt
	}
	return connections
}

func currentConnectionStatusWireTime() string {
	return time.Now().UTC().Format(statusWireTimeLayout)
}

func calendarConnectionsFromProvider(items []calendarprovider.CatalogConnection) ([]CalendarConnection, error) {
	connections := make([]CalendarConnection, 0, len(items))
	for _, item := range items {
		if item.ConnectionRef == "" || item.Provider == "" || item.AccountRef == "" || item.AccountEmail == "" || item.Status == "" {
			return nil, ErrInvalidCalendarCatalogSnapshot
		}
		connections = append(connections, CalendarConnection{
			ConnectionRef: item.ConnectionRef,
			Provider:      item.Provider,
			AccountRef:    item.AccountRef,
			AccountEmail:  item.AccountEmail,
			Status:        item.Status,
		})
	}
	return connections, nil
}

func catalogCalendarsFromProvider(items []calendarprovider.CatalogCalendar) ([]CatalogCalendar, error) {
	calendars := make([]CatalogCalendar, 0, len(items))
	for _, item := range items {
		if item.CalendarRef == "" || item.ConnectionRef == "" || item.Provider == "" || item.ExternalID == "" || item.Name == "" {
			return nil, ErrInvalidCalendarCatalogSnapshot
		}
		calendars = append(calendars, CatalogCalendar{
			CalendarRef:   item.CalendarRef,
			ConnectionRef: item.ConnectionRef,
			Provider:      item.Provider,
			ExternalID:    item.ExternalID,
			Name:          item.Name,
			Primary:       item.Primary,
			Writable:      item.Writable,
		})
	}
	return calendars, nil
}

func validateProviderCatalog(connections []CalendarConnection, catalog []CatalogCalendar) error {
	connectionRefs := map[string]string{}
	for _, connection := range connections {
		if connection.ConnectionRef == "" || connection.Provider == "" || connection.AccountRef == "" || connection.AccountEmail == "" || connection.Status == "" {
			return ErrInvalidCalendarCatalogSnapshot
		}
		if _, ok := connectionRefs[connection.ConnectionRef]; ok {
			return ErrInvalidCalendarCatalogSnapshot
		}
		connectionRefs[connection.ConnectionRef] = connection.Provider
	}

	calendarRefs := map[string]struct{}{}
	externalRefs := map[string]struct{}{}
	for _, calendar := range catalog {
		if calendar.CalendarRef == "" || calendar.ConnectionRef == "" || calendar.Provider == "" || calendar.ExternalID == "" || calendar.Name == "" {
			return ErrInvalidCalendarCatalogSnapshot
		}
		if _, ok := calendarRefs[calendar.CalendarRef]; ok {
			return ErrInvalidCalendarCatalogSnapshot
		}
		calendarRefs[calendar.CalendarRef] = struct{}{}
		connectionProvider, ok := connectionRefs[calendar.ConnectionRef]
		if !ok || connectionProvider != calendar.Provider {
			return ErrInvalidCalendarCatalogSnapshot
		}
		externalRef := calendar.Provider + "\x00" + calendar.ExternalID
		if _, ok := externalRefs[externalRef]; ok {
			return ErrInvalidCalendarCatalogSnapshot
		}
		externalRefs[externalRef] = struct{}{}
	}
	return nil
}

func fixtureCalendarConnections(userID int) []CalendarConnection {
	if userID != fixtureUserID {
		return []CalendarConnection{}
	}
	return []CalendarConnection{
		{
			ConnectionRef: FixtureCalendarConnectionRef,
			Provider:      "google-calendar-fixture",
			AccountRef:    FixtureCalendarAccountRef,
			AccountEmail:  "fixture-user@example.test",
			Status:        "active",
		},
	}
}

func fixtureCatalogCalendars(userID int) []CatalogCalendar {
	if userID != fixtureUserID {
		return []CatalogCalendar{}
	}
	return []CatalogCalendar{
		{
			CalendarRef:   FixtureDestinationCalendarRef,
			ConnectionRef: FixtureCalendarConnectionRef,
			Provider:      "google-calendar-fixture",
			ExternalID:    "google-calendar-destination",
			Name:          "Fixture Destination Calendar",
			Primary:       true,
			Writable:      true,
		},
		{
			CalendarRef:   FixtureSelectedCalendarRef,
			ConnectionRef: FixtureCalendarConnectionRef,
			Provider:      "google-calendar-fixture",
			ExternalID:    "google-calendar-selected",
			Name:          "Fixture Selected Calendar",
			Writable:      true,
		},
		{
			CalendarRef:   FixtureTeamCalendarRef,
			ConnectionRef: FixtureCalendarConnectionRef,
			Provider:      "google-calendar-fixture",
			ExternalID:    "google-calendar-team",
			Name:          "Fixture Team Calendar",
			Writable:      true,
		},
	}
}

func fixtureSelectedCalendars(userID int) []SelectedCalendar {
	if userID != fixtureUserID {
		return []SelectedCalendar{}
	}
	return []SelectedCalendar{
		toSelectedCalendar(fixtureCatalogCalendars(userID)[0]),
		toSelectedCalendar(fixtureCatalogCalendars(userID)[1]),
	}
}
