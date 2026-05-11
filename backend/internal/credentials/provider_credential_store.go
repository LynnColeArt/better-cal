package credentials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/LynnColeArt/better-cal/backend/internal/apps"
)

var ErrProviderTokenSecretStoreUnset = errors.New("provider token secret store is not configured")

type ProviderTokenSecretStore interface {
	StoreProviderTokenPayload(ctx context.Context, secret apps.ProviderCredentialSecret) error
}

type ProviderCredentialStore struct {
	repo        Repository
	secretStore ProviderTokenSecretStore
}

func NewProviderCredentialStore(repo Repository, secretStore ProviderTokenSecretStore) *ProviderCredentialStore {
	return &ProviderCredentialStore{
		repo:        repo,
		secretStore: secretStore,
	}
}

func (s *ProviderCredentialStore) StoreProviderCredentialSecret(ctx context.Context, secret apps.ProviderCredentialSecret) (apps.ProviderCredentialReceipt, error) {
	if s == nil || s.repo == nil {
		return apps.ProviderCredentialReceipt{}, ErrInvalidCredentialMetadata
	}
	if s.secretStore == nil {
		return apps.ProviderCredentialReceipt{}, ErrProviderTokenSecretStoreUnset
	}
	if err := validateProviderCredentialSecret(secret); err != nil {
		return apps.ProviderCredentialReceipt{}, err
	}

	credentialRef := providerCredentialRef(secret)
	existing, err := s.repo.ReadCredentialMetadata(ctx, secret.UserID)
	if err != nil {
		return apps.ProviderCredentialReceipt{}, err
	}
	for _, credential := range existing {
		if credential.Provider == secret.ProviderSlug && credential.AccountRef == secret.AccountRef {
			credentialRef = credential.CredentialRef
			break
		}
	}

	if err := s.secretStore.StoreProviderTokenPayload(ctx, secret); err != nil {
		return apps.ProviderCredentialReceipt{}, err
	}

	saved, err := s.repo.SaveCredentialMetadata(ctx, secret.UserID, CredentialMetadata{
		CredentialRef: credentialRef,
		AppSlug:       secret.AppSlug,
		AppCategory:   secret.AppCategory,
		Provider:      secret.ProviderSlug,
		AccountRef:    secret.AccountRef,
		AccountLabel:  secret.AccountLabel,
		Status:        "active",
		Scopes:        append([]string(nil), secret.Scopes...),
	})
	if err != nil {
		return apps.ProviderCredentialReceipt{}, err
	}
	return apps.ProviderCredentialReceipt{
		CredentialRef: saved.CredentialRef,
		AccountRef:    saved.AccountRef,
		AccountLabel:  saved.AccountLabel,
		Status:        saved.Status,
		Scopes:        append([]string(nil), saved.Scopes...),
	}, nil
}

type InMemoryProviderTokenSecretStore struct {
	mu      sync.Mutex
	secrets map[string]apps.ProviderCredentialSecret
}

func NewInMemoryProviderTokenSecretStore() *InMemoryProviderTokenSecretStore {
	return &InMemoryProviderTokenSecretStore{
		secrets: map[string]apps.ProviderCredentialSecret{},
	}
}

func (s *InMemoryProviderTokenSecretStore) StoreProviderTokenPayload(_ context.Context, secret apps.ProviderCredentialSecret) error {
	if s == nil {
		return ErrProviderTokenSecretStoreUnset
	}
	if err := validateProviderCredentialSecret(secret); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	secret.Scopes = append([]string(nil), secret.Scopes...)
	secret.TokenPayload.RawProviderResponse = append([]byte(nil), secret.TokenPayload.RawProviderResponse...)
	s.secrets[providerCredentialSecretKey(secret)] = secret
	return nil
}

func validateProviderCredentialSecret(secret apps.ProviderCredentialSecret) error {
	if secret.UserID <= 0 ||
		strings.TrimSpace(secret.AppSlug) == "" ||
		strings.TrimSpace(secret.AppCategory) == "" ||
		strings.TrimSpace(secret.ProviderSlug) == "" ||
		strings.TrimSpace(secret.AccountRef) == "" ||
		strings.TrimSpace(secret.AccountLabel) == "" ||
		len(secret.Scopes) == 0 ||
		strings.TrimSpace(secret.TokenPayload.AccessToken) == "" ||
		strings.TrimSpace(secret.TokenPayload.RefreshToken) == "" {
		return apps.ErrInvalidProviderCredentialSecret
	}
	return nil
}

func providerCredentialRef(secret apps.ProviderCredentialSecret) string {
	return "provider-credential-" + providerCredentialFingerprint(secret.UserID, secret.ProviderSlug, secret.AccountRef)[:24]
}

func providerCredentialSecretKey(secret apps.ProviderCredentialSecret) string {
	return providerCredentialFingerprint(secret.UserID, secret.ProviderSlug, secret.AccountRef)
}

func providerCredentialFingerprint(userID int, parts ...string) string {
	builder := strings.Builder{}
	builder.WriteString(strconv.Itoa(userID))
	for _, part := range parts {
		builder.WriteByte(0)
		builder.WriteString(part)
	}
	hash := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(hash[:])
}
