package credentials

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/integrations"
)

const fixtureUserID = 123

var (
	ErrInvalidCredentialMetadata       = errors.New("invalid credential metadata")
	ErrCredentialStatusProviderUnset   = errors.New("credential status provider is not configured")
	ErrInvalidCredentialStatusSnapshot = errors.New("invalid credential status snapshot")
)

type CredentialMetadata struct {
	CredentialRef   string   `json:"credentialRef"`
	AppSlug         string   `json:"appSlug"`
	AppCategory     string   `json:"appCategory"`
	Provider        string   `json:"provider"`
	AccountRef      string   `json:"accountRef"`
	AccountLabel    string   `json:"accountLabel"`
	Status          string   `json:"status"`
	StatusCode      string   `json:"statusCode,omitempty"`
	StatusCheckedAt string   `json:"statusCheckedAt,omitempty"`
	Scopes          []string `json:"scopes"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

type CredentialStatusUpdate struct {
	CredentialRef string
	Provider      string
	AccountRef    string
	Status        string
	StatusCode    string
}

type Repository interface {
	ReadCredentialMetadata(ctx context.Context, userID int) ([]CredentialMetadata, error)
	SaveCredentialMetadata(ctx context.Context, userID int, credential CredentialMetadata) (CredentialMetadata, error)
	RefreshCredentialStatuses(ctx context.Context, userID int, updates []CredentialStatusUpdate, checkedAt string) ([]CredentialMetadata, error)
}

type Store struct {
	mu             sync.Mutex
	repo           Repository
	statusProvider integrations.StatusProviderAdapter
	credentials    map[int][]CredentialMetadata
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
	}
}

func WithStatusProvider(provider integrations.StatusProviderAdapter) StoreOption {
	return func(s *Store) {
		s.statusProvider = provider
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

func (s *Store) RefreshProviderStatus(ctx context.Context, userID int) error {
	if s.statusProvider == nil {
		return ErrCredentialStatusProviderUnset
	}
	snapshot, err := s.statusProvider.ReadStatus(ctx, integrations.StatusInput{UserID: userID})
	if err != nil {
		return err
	}
	updates, err := credentialStatusUpdatesFromProvider(snapshot.Credentials)
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
	checkedAt := currentWireTime()
	if err := validateCredentialStatusUpdates(s.credentials[userID], updates); err != nil {
		return err
	}
	if s.repo != nil {
		refreshed, err := s.repo.RefreshCredentialStatuses(ctx, userID, updates, checkedAt)
		if err != nil {
			return err
		}
		s.credentials[userID] = cloneCredentialMetadata(refreshed)
		return nil
	}
	s.credentials[userID] = applyCredentialStatusUpdates(s.credentials[userID], updates, checkedAt)
	return nil
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

func credentialStatusUpdatesFromProvider(items []integrations.CredentialStatus) ([]CredentialStatusUpdate, error) {
	updates := make([]CredentialStatusUpdate, 0, len(items))
	for _, item := range items {
		if item.CredentialRef == "" || item.Provider == "" || item.AccountRef == "" || item.Status == "" {
			return nil, ErrInvalidCredentialStatusSnapshot
		}
		updates = append(updates, CredentialStatusUpdate{
			CredentialRef: item.CredentialRef,
			Provider:      item.Provider,
			AccountRef:    item.AccountRef,
			Status:        item.Status,
			StatusCode:    item.StatusCode,
		})
	}
	return updates, nil
}

func validateCredentialStatusUpdates(existing []CredentialMetadata, updates []CredentialStatusUpdate) error {
	existingByRef := map[string]CredentialMetadata{}
	for _, credential := range existing {
		existingByRef[credential.CredentialRef] = credential
	}
	seen := map[string]struct{}{}
	for _, update := range updates {
		if update.CredentialRef == "" || update.Provider == "" || update.AccountRef == "" || update.Status == "" {
			return ErrInvalidCredentialStatusSnapshot
		}
		if _, ok := seen[update.CredentialRef]; ok {
			return ErrInvalidCredentialStatusSnapshot
		}
		seen[update.CredentialRef] = struct{}{}
		credential, ok := existingByRef[update.CredentialRef]
		if !ok || credential.Provider != update.Provider || credential.AccountRef != update.AccountRef {
			return ErrInvalidCredentialStatusSnapshot
		}
	}
	return nil
}

func applyCredentialStatusUpdates(existing []CredentialMetadata, updates []CredentialStatusUpdate, checkedAt string) []CredentialMetadata {
	updatedByRef := map[string]CredentialStatusUpdate{}
	for _, update := range updates {
		updatedByRef[update.CredentialRef] = update
	}
	items := cloneCredentialMetadata(existing)
	for index := range items {
		update, ok := updatedByRef[items[index].CredentialRef]
		if !ok {
			continue
		}
		items[index].Status = update.Status
		items[index].StatusCode = update.StatusCode
		items[index].StatusCheckedAt = checkedAt
		items[index].UpdatedAt = checkedAt
	}
	return items
}

func currentWireTime() string {
	return time.Now().UTC().Format(wireTimeLayout)
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
