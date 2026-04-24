package credentials

import (
	"context"
	"errors"
	"slices"
	"sync"
)

const fixtureUserID = 123

var ErrInvalidCredentialMetadata = errors.New("invalid credential metadata")

type CredentialMetadata struct {
	CredentialRef string   `json:"credentialRef"`
	AppSlug       string   `json:"appSlug"`
	AppCategory   string   `json:"appCategory"`
	Provider      string   `json:"provider"`
	AccountRef    string   `json:"accountRef"`
	AccountLabel  string   `json:"accountLabel"`
	Status        string   `json:"status"`
	Scopes        []string `json:"scopes"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

type Repository interface {
	ReadCredentialMetadata(ctx context.Context, userID int) ([]CredentialMetadata, error)
	SaveCredentialMetadata(ctx context.Context, userID int, credential CredentialMetadata) (CredentialMetadata, error)
}

type Store struct {
	mu          sync.Mutex
	repo        Repository
	credentials map[int][]CredentialMetadata
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
	}
}

func NewStore(opts ...StoreOption) *Store {
	store := &Store{
		credentials: map[int][]CredentialMetadata{},
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

func NewStoreWithRepository(repo Repository, opts ...StoreOption) *Store {
	return NewStore(append([]StoreOption{WithRepository(repo)}, opts...)...)
}

func SeedFixtureMetadata(ctx context.Context, repo Repository) error {
	for _, credential := range fixtureCredentialMetadata(fixtureUserID) {
		if _, err := repo.SaveCredentialMetadata(ctx, fixtureUserID, credential); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ReadCredentialMetadata(ctx context.Context, userID int) ([]CredentialMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoadedLocked(ctx, userID); err != nil {
		return nil, err
	}
	items := append([]CredentialMetadata(nil), s.credentials[userID]...)
	for index := range items {
		items[index].Scopes = append([]string(nil), items[index].Scopes...)
	}
	return items, nil
}

func (s *Store) ensureLoadedLocked(ctx context.Context, userID int) error {
	if _, ok := s.credentials[userID]; ok {
		return nil
	}
	if s.repo == nil {
		s.credentials[userID] = fixtureCredentialMetadata(userID)
		return nil
	}
	items, err := s.repo.ReadCredentialMetadata(ctx, userID)
	if err != nil {
		return err
	}
	if len(items) == 0 && userID == fixtureUserID {
		if err := SeedFixtureMetadata(ctx, s.repo); err != nil {
			return err
		}
		items, err = s.repo.ReadCredentialMetadata(ctx, userID)
		if err != nil {
			return err
		}
	}
	s.credentials[userID] = cloneCredentialMetadata(items)
	return nil
}

func ValidateCredentialMetadata(credential CredentialMetadata) error {
	if credential.CredentialRef == "" ||
		credential.AppSlug == "" ||
		credential.AppCategory == "" ||
		credential.Provider == "" ||
		credential.AccountRef == "" ||
		credential.AccountLabel == "" ||
		credential.Status == "" {
		return ErrInvalidCredentialMetadata
	}
	return nil
}

func sortCredentialMetadata(items []CredentialMetadata) {
	slices.SortFunc(items, func(left CredentialMetadata, right CredentialMetadata) int {
		switch {
		case left.CredentialRef < right.CredentialRef:
			return -1
		case left.CredentialRef > right.CredentialRef:
			return 1
		default:
			return 0
		}
	})
}

func cloneCredentialMetadata(items []CredentialMetadata) []CredentialMetadata {
	cloned := make([]CredentialMetadata, 0, len(items))
	for _, item := range items {
		item.Scopes = append([]string(nil), item.Scopes...)
		cloned = append(cloned, item)
	}
	sortCredentialMetadata(cloned)
	return cloned
}

func fixtureCredentialMetadata(userID int) []CredentialMetadata {
	if userID != fixtureUserID {
		return []CredentialMetadata{}
	}
	return []CredentialMetadata{
		{
			CredentialRef: "google-calendar-credential-fixture",
			AppSlug:       "google-calendar",
			AppCategory:   "calendar",
			Provider:      "google-calendar-fixture",
			AccountRef:    "google-account-fixture",
			AccountLabel:  "fixture-user@example.test",
			Status:        "active",
			Scopes:        []string{"calendar.read", "calendar.write"},
			CreatedAt:     "2026-01-01T00:00:00.000Z",
			UpdatedAt:     "2026-01-01T00:00:00.000Z",
		},
	}
}
