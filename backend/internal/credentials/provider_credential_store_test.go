package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/apps"
)

func TestProviderCredentialStoreWritesOnlyMetadataThroughCredentialRepository(t *testing.T) {
	repo := newProviderCredentialMemoryRepository()
	secretStore := NewInMemoryProviderTokenSecretStore()
	store := NewProviderCredentialStore(repo, secretStore)

	receipt, err := store.StoreProviderCredentialSecret(context.Background(), apps.ProviderCredentialSecret{
		UserID:       123,
		AppSlug:      "google-calendar",
		AppCategory:  "calendar",
		ProviderSlug: "google-calendar-fixture",
		AccountRef:   "google-account-oauth-fixture",
		AccountLabel: "oauth-user@example.test",
		Scopes:       []string{"calendar.read", "calendar.write"},
		TokenPayload: apps.ProviderTokenPayload{
			AccessToken:         "provider-access-token-secret-fixture",
			RefreshToken:        "provider-refresh-token-secret-fixture",
			TokenType:           "Bearer",
			RawProviderResponse: []byte(`{"access_token":"provider-access-token-secret-fixture"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.CredentialRef == "" {
		t.Fatalf("credential receipt ref was empty: %#v", receipt)
	}
	if receipt.AccountRef != "google-account-oauth-fixture" {
		t.Fatalf("receipt account ref = %q", receipt.AccountRef)
	}
	if len(repo.items[123]) != 1 {
		t.Fatalf("metadata count = %d", len(repo.items[123]))
	}
	metadata := repo.items[123][0]
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"provider-access-token-secret-fixture",
		"provider-refresh-token-secret-fixture",
		"access_token",
		"refresh_token",
		"rawprovider",
		"providerresponse",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("credential metadata exposed forbidden term %q: %s", forbidden, body)
		}
	}
	if len(secretStore.secrets) != 1 {
		t.Fatalf("secret store count = %d", len(secretStore.secrets))
	}
	for _, secret := range secretStore.secrets {
		if secret.TokenPayload.AccessToken != "provider-access-token-secret-fixture" {
			t.Fatalf("secret store access token = %q", secret.TokenPayload.AccessToken)
		}
	}
}

func TestProviderCredentialStoreSecretFailureDoesNotWriteMetadata(t *testing.T) {
	repo := newProviderCredentialMemoryRepository()
	store := NewProviderCredentialStore(repo, failingProviderTokenSecretStore{err: apps.ErrInvalidProviderCredentialSecret})

	_, err := store.StoreProviderCredentialSecret(context.Background(), apps.ProviderCredentialSecret{
		UserID:       123,
		AppSlug:      "google-calendar",
		AppCategory:  "calendar",
		ProviderSlug: "google-calendar-fixture",
		AccountRef:   "google-account-oauth-fixture",
		AccountLabel: "oauth-user@example.test",
		Scopes:       []string{"calendar.read"},
		TokenPayload: apps.ProviderTokenPayload{
			AccessToken:  "provider-access-token-secret-fixture",
			RefreshToken: "provider-refresh-token-secret-fixture",
		},
	})
	if !errors.Is(err, apps.ErrInvalidProviderCredentialSecret) {
		t.Fatalf("err = %v", err)
	}
	if len(repo.items[123]) != 0 {
		t.Fatalf("metadata was written after secret failure: %#v", repo.items[123])
	}
}

type providerCredentialMemoryRepository struct {
	items map[int][]CredentialMetadata
}

func newProviderCredentialMemoryRepository() *providerCredentialMemoryRepository {
	return &providerCredentialMemoryRepository{
		items: map[int][]CredentialMetadata{},
	}
}

func (r *providerCredentialMemoryRepository) ReadCredentialMetadata(_ context.Context, userID int) ([]CredentialMetadata, error) {
	return cloneCredentialMetadata(r.items[userID]), nil
}

func (r *providerCredentialMemoryRepository) SaveCredentialMetadata(_ context.Context, userID int, credential CredentialMetadata) (CredentialMetadata, error) {
	if err := ValidateCredentialMetadata(credential); err != nil {
		return CredentialMetadata{}, err
	}
	items := r.items[userID]
	for index, item := range items {
		if item.CredentialRef == credential.CredentialRef {
			items[index] = credential
			r.items[userID] = items
			return credential, nil
		}
	}
	r.items[userID] = append(items, credential)
	return credential, nil
}

func (r *providerCredentialMemoryRepository) RefreshCredentialStatuses(_ context.Context, userID int, _ []CredentialStatusUpdate, _ string) ([]CredentialMetadata, error) {
	return cloneCredentialMetadata(r.items[userID]), nil
}

type failingProviderTokenSecretStore struct {
	err error
}

func (s failingProviderTokenSecretStore) StoreProviderTokenPayload(context.Context, string, apps.ProviderCredentialSecret) error {
	return s.err
}
