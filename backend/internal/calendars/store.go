package calendars

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

const (
	fixtureUserID                 = 123
	FixtureCalendarConnectionRef  = "google-calendar-connection-fixture"
	FixtureCalendarAccountRef     = "google-account-fixture"
	FixtureSelectedCalendarRef    = "selected-calendar-fixture"
	FixtureDestinationCalendarRef = "destination-calendar-fixture"
	FixtureTeamCalendarRef        = "team-calendar-fixture"
)

var (
	ErrInvalidSelectedCalendar       = errors.New("invalid selected calendar")
	ErrInvalidSelectedCalendarRef    = errors.New("invalid selected calendar ref")
	ErrInvalidDestinationCalendarRef = errors.New("invalid destination calendar ref")
	ErrCalendarCatalogEntryNotFound  = errors.New("calendar catalog entry not found")
)

type CalendarConnection struct {
	ConnectionRef string `json:"connectionRef"`
	Provider      string `json:"provider"`
	AccountRef    string `json:"accountRef"`
	AccountEmail  string `json:"accountEmail"`
	Status        string `json:"status"`
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
	mu          sync.Mutex
	repo        Repository
	connections map[int][]CalendarConnection
	catalog     map[int][]CatalogCalendar
	selected    map[int][]SelectedCalendar
	destination map[int]string
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
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
		s.connections[userID] = fixtureCalendarConnections(userID)
		s.catalog[userID] = fixtureCatalogCalendars(userID)
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
