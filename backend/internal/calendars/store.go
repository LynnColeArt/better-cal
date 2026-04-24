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
	FixtureSelectedCalendarRef    = "selected-calendar-fixture"
	FixtureDestinationCalendarRef = "destination-calendar-fixture"
)

var (
	ErrInvalidSelectedCalendar       = errors.New("invalid selected calendar")
	ErrInvalidSelectedCalendarRef    = errors.New("invalid selected calendar ref")
	ErrInvalidDestinationCalendarRef = errors.New("invalid destination calendar ref")
)

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
	ReadSelectedCalendars(ctx context.Context, userID int) ([]SelectedCalendar, error)
	SaveSelectedCalendar(ctx context.Context, userID int, calendar SelectedCalendar) (SelectedCalendar, error)
	DeleteSelectedCalendar(ctx context.Context, userID int, calendarRef string) (DeleteSelectedCalendarResult, error)
	ReadDestinationCalendar(ctx context.Context, userID int) (SelectedCalendar, bool, error)
	SetDestinationCalendar(ctx context.Context, userID int, calendarRef string) (SelectedCalendar, bool, error)
}

type Store struct {
	mu          sync.Mutex
	repo        Repository
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

func (s *Store) ReadSelectedCalendars(ctx context.Context, userID int) ([]SelectedCalendar, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return nil, err
	}
	return append([]SelectedCalendar(nil), s.selected[userID]...), nil
}

func (s *Store) SaveSelectedCalendar(ctx context.Context, userID int, req SaveSelectedCalendarRequest) (SelectedCalendar, error) {
	if req.CalendarRef == "" || req.Provider == "" || req.ExternalID == "" || req.Name == "" {
		return SelectedCalendar{}, ErrInvalidSelectedCalendar
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return SelectedCalendar{}, err
	}

	calendar := SelectedCalendar{
		CalendarRef: req.CalendarRef,
		Provider:    req.Provider,
		ExternalID:  req.ExternalID,
		Name:        req.Name,
	}
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
	slices.SortFunc(items, func(left SelectedCalendar, right SelectedCalendar) int {
		if left.CalendarRef < right.CalendarRef {
			return -1
		}
		if left.CalendarRef > right.CalendarRef {
			return 1
		}
		return 0
	})
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
		s.selected[userID] = fixtureSelectedCalendars(userID)
		if userID == fixtureUserID {
			s.destination[userID] = FixtureDestinationCalendarRef
		}
		return nil
	}

	selected, err := s.repo.ReadSelectedCalendars(ctx, userID)
	if err != nil {
		return err
	}
	destination, ok, err := s.repo.ReadDestinationCalendar(ctx, userID)
	if err != nil {
		return err
	}
	if len(selected) == 0 && !ok && userID == fixtureUserID {
		for _, fixture := range fixtureSelectedCalendars(userID) {
			if _, err := s.repo.SaveSelectedCalendar(ctx, userID, fixture); err != nil {
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

	s.selected[userID] = append([]SelectedCalendar(nil), selected...)
	if ok {
		s.destination[userID] = destination.CalendarRef
	}
	return nil
}

func fixtureSelectedCalendars(userID int) []SelectedCalendar {
	if userID != fixtureUserID {
		return []SelectedCalendar{}
	}
	return []SelectedCalendar{
		{
			CalendarRef: FixtureDestinationCalendarRef,
			Provider:    "google-calendar-fixture",
			ExternalID:  "google-calendar-destination",
			Name:        "Fixture Destination Calendar",
		},
		{
			CalendarRef: FixtureSelectedCalendarRef,
			Provider:    "google-calendar-fixture",
			ExternalID:  "google-calendar-selected",
			Name:        "Fixture Selected Calendar",
		},
	}
}
