package apps

import (
	"context"
	"errors"
	"slices"
	"sync"
)

var ErrInvalidAppMetadata = errors.New("invalid app metadata")

type AppMetadata struct {
	AppSlug      string   `json:"appSlug"`
	Category     string   `json:"category"`
	Provider     string   `json:"provider"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	AuthType     string   `json:"authType"`
	Capabilities []string `json:"capabilities"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

type Repository interface {
	ReadAppCatalog(ctx context.Context) ([]AppMetadata, error)
	SaveAppMetadata(ctx context.Context, app AppMetadata) (AppMetadata, error)
}

type Store struct {
	mu      sync.Mutex
	repo    Repository
	catalog []AppMetadata
	loaded  bool
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
	}
}

func NewStore(opts ...StoreOption) *Store {
	store := &Store{}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

func NewStoreWithRepository(repo Repository, opts ...StoreOption) *Store {
	return NewStore(append([]StoreOption{WithRepository(repo)}, opts...)...)
}

func SeedFixtureCatalog(ctx context.Context, repo Repository) error {
	for _, app := range fixtureAppCatalog() {
		if _, err := repo.SaveAppMetadata(ctx, app); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ReadAppCatalog(ctx context.Context) ([]AppMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx); err != nil {
		return nil, err
	}
	return cloneAppMetadata(s.catalog), nil
}

func (s *Store) ensureLoadedLocked(ctx context.Context) error {
	if s.loaded {
		return nil
	}
	if s.repo == nil {
		s.catalog = fixtureAppCatalog()
		s.loaded = true
		return nil
	}
	items, err := s.repo.ReadAppCatalog(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if err := SeedFixtureCatalog(ctx, s.repo); err != nil {
			return err
		}
		items, err = s.repo.ReadAppCatalog(ctx)
		if err != nil {
			return err
		}
	}
	s.catalog = cloneAppMetadata(items)
	s.loaded = true
	return nil
}

func ValidateAppMetadata(app AppMetadata) error {
	if app.AppSlug == "" ||
		app.Category == "" ||
		app.Provider == "" ||
		app.Name == "" ||
		app.Description == "" ||
		app.AuthType == "" {
		return ErrInvalidAppMetadata
	}
	return nil
}

func cloneAppMetadata(items []AppMetadata) []AppMetadata {
	cloned := make([]AppMetadata, 0, len(items))
	for _, item := range items {
		item.Capabilities = append([]string(nil), item.Capabilities...)
		cloned = append(cloned, item)
	}
	sortAppMetadata(cloned)
	return cloned
}

func sortAppMetadata(items []AppMetadata) {
	slices.SortFunc(items, func(left AppMetadata, right AppMetadata) int {
		switch {
		case left.AppSlug < right.AppSlug:
			return -1
		case left.AppSlug > right.AppSlug:
			return 1
		default:
			return 0
		}
	})
}

func fixtureAppCatalog() []AppMetadata {
	return []AppMetadata{
		{
			AppSlug:      "google-calendar",
			Category:     "calendar",
			Provider:     "google-calendar-fixture",
			Name:         "Google Calendar",
			Description:  "Calendar availability, selected calendar, and booking event sync.",
			AuthType:     "oauth",
			Capabilities: []string{"calendar.read", "calendar.write", "booking.calendar-dispatch"},
			CreatedAt:    "2026-01-01T00:00:00.000Z",
			UpdatedAt:    "2026-01-01T00:00:00.000Z",
		},
		{
			AppSlug:      "resend-email",
			Category:     "email",
			Provider:     "resend-fixture",
			Name:         "Resend",
			Description:  "Transactional booking email delivery.",
			AuthType:     "api-key",
			Capabilities: []string{"booking.email-dispatch"},
			CreatedAt:    "2026-01-01T00:00:00.000Z",
			UpdatedAt:    "2026-01-01T00:00:00.000Z",
		},
	}
}
