package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/LynnColeArt/better-cal/backend/internal/apps"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrProviderTokenSealerUnset      = errors.New("provider token sealer is not configured")
	ErrInvalidProviderTokenEnvelope  = errors.New("invalid provider token envelope")
	ErrInvalidProviderTokenSealerKey = errors.New("invalid provider token sealer key")
)

type ProviderTokenEnvelope struct {
	KeyRef           string
	SealedPayload    []byte
	SealedPayloadSHA string
}

type ProviderTokenSealer interface {
	SealProviderTokenPayload(ctx context.Context, credentialRef string, secret apps.ProviderCredentialSecret) (ProviderTokenEnvelope, error)
}

type PostgresProviderTokenSecretStore struct {
	pool   *pgxpool.Pool
	sealer ProviderTokenSealer
}

func NewPostgresProviderTokenSecretStore(pool *pgxpool.Pool, sealer ProviderTokenSealer) *PostgresProviderTokenSecretStore {
	return &PostgresProviderTokenSecretStore{
		pool:   pool,
		sealer: sealer,
	}
}

func (s *PostgresProviderTokenSecretStore) StoreProviderTokenPayload(ctx context.Context, credentialRef string, secret apps.ProviderCredentialSecret) error {
	if s == nil || s.pool == nil {
		return ErrProviderTokenSecretStoreUnset
	}
	if s.sealer == nil {
		return ErrProviderTokenSealerUnset
	}
	if strings.TrimSpace(credentialRef) == "" {
		return ErrInvalidCredentialMetadata
	}
	if err := validateProviderCredentialSecret(secret); err != nil {
		return err
	}

	envelope, err := s.sealer.SealProviderTokenPayload(ctx, credentialRef, secret)
	if err != nil {
		return err
	}
	if err := validateProviderTokenEnvelope(envelope); err != nil {
		return err
	}

	if _, err := s.pool.Exec(ctx, `
		insert into integration_provider_token_secrets (
			user_id,
			credential_ref,
			app_slug,
			provider,
			account_ref,
			key_ref,
			sealed_payload,
			sealed_payload_sha256
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (user_id, credential_ref) do update set
			app_slug = excluded.app_slug,
			provider = excluded.provider,
			account_ref = excluded.account_ref,
			key_ref = excluded.key_ref,
			sealed_payload = excluded.sealed_payload,
			sealed_payload_sha256 = excluded.sealed_payload_sha256,
			updated_at = now()
	`, secret.UserID, credentialRef, secret.AppSlug, secret.ProviderSlug, secret.AccountRef, envelope.KeyRef, envelope.SealedPayload, envelope.SealedPayloadSHA); err != nil {
		return fmt.Errorf("store provider token secret: %w", err)
	}
	return nil
}

type AESGCMProviderTokenSealer struct {
	keyRef string
	key    []byte
	rand   io.Reader
}

func NewAESGCMProviderTokenSealer(keyRef string, key []byte) (*AESGCMProviderTokenSealer, error) {
	return newAESGCMProviderTokenSealer(keyRef, key, rand.Reader)
}

func NewFixtureProviderTokenSealer() *AESGCMProviderTokenSealer {
	key := sha256.Sum256([]byte("better-cal-fixture-provider-token-key-v1"))
	sealer, err := newAESGCMProviderTokenSealer("fixture-provider-token-key-v1", key[:], rand.Reader)
	if err != nil {
		panic(err)
	}
	return sealer
}

func newAESGCMProviderTokenSealer(keyRef string, key []byte, random io.Reader) (*AESGCMProviderTokenSealer, error) {
	if strings.TrimSpace(keyRef) == "" || random == nil {
		return nil, ErrInvalidProviderTokenSealerKey
	}
	switch len(key) {
	case 16, 24, 32:
	default:
		return nil, ErrInvalidProviderTokenSealerKey
	}
	copiedKey := append([]byte(nil), key...)
	return &AESGCMProviderTokenSealer{
		keyRef: strings.TrimSpace(keyRef),
		key:    copiedKey,
		rand:   random,
	}, nil
}

func (s *AESGCMProviderTokenSealer) SealProviderTokenPayload(_ context.Context, credentialRef string, secret apps.ProviderCredentialSecret) (ProviderTokenEnvelope, error) {
	if s == nil {
		return ProviderTokenEnvelope{}, ErrProviderTokenSealerUnset
	}
	if strings.TrimSpace(credentialRef) == "" {
		return ProviderTokenEnvelope{}, ErrInvalidCredentialMetadata
	}
	if err := validateProviderCredentialSecret(secret); err != nil {
		return ProviderTokenEnvelope{}, err
	}

	payload, err := json.Marshal(providerTokenSealedPayload{
		CredentialRef:       credentialRef,
		AppSlug:             secret.AppSlug,
		AppCategory:         secret.AppCategory,
		ProviderSlug:        secret.ProviderSlug,
		AccountRef:          secret.AccountRef,
		AccountLabel:        secret.AccountLabel,
		Scopes:              append([]string(nil), secret.Scopes...),
		AccessToken:         secret.TokenPayload.AccessToken,
		RefreshToken:        secret.TokenPayload.RefreshToken,
		TokenType:           secret.TokenPayload.TokenType,
		ExpiresAt:           secret.TokenPayload.ExpiresAt,
		RawProviderResponse: append([]byte(nil), secret.TokenPayload.RawProviderResponse...),
	})
	if err != nil {
		return ProviderTokenEnvelope{}, fmt.Errorf("marshal provider token payload: %w", err)
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return ProviderTokenEnvelope{}, fmt.Errorf("create provider token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ProviderTokenEnvelope{}, fmt.Errorf("create provider token aead: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(s.rand, nonce); err != nil {
		return ProviderTokenEnvelope{}, fmt.Errorf("read provider token nonce: %w", err)
	}
	sealed := append([]byte(nil), nonce...)
	sealed = gcm.Seal(sealed, nonce, payload, []byte(credentialRef))
	hash := sha256.Sum256(sealed)
	return ProviderTokenEnvelope{
		KeyRef:           s.keyRef,
		SealedPayload:    sealed,
		SealedPayloadSHA: hex.EncodeToString(hash[:]),
	}, nil
}

type providerTokenSealedPayload struct {
	CredentialRef       string   `json:"credentialRef"`
	AppSlug             string   `json:"appSlug"`
	AppCategory         string   `json:"appCategory"`
	ProviderSlug        string   `json:"providerSlug"`
	AccountRef          string   `json:"accountRef"`
	AccountLabel        string   `json:"accountLabel"`
	Scopes              []string `json:"scopes"`
	AccessToken         string   `json:"accessToken"`
	RefreshToken        string   `json:"refreshToken"`
	TokenType           string   `json:"tokenType,omitempty"`
	ExpiresAt           string   `json:"expiresAt,omitempty"`
	RawProviderResponse []byte   `json:"rawProviderResponse,omitempty"`
}

func validateProviderTokenEnvelope(envelope ProviderTokenEnvelope) error {
	if strings.TrimSpace(envelope.KeyRef) == "" ||
		len(envelope.SealedPayload) == 0 ||
		!isSHA256Hex(envelope.SealedPayloadSHA) {
		return ErrInvalidProviderTokenEnvelope
	}
	hash := sha256.Sum256(envelope.SealedPayload)
	if envelope.SealedPayloadSHA != hex.EncodeToString(hash[:]) {
		return ErrInvalidProviderTokenEnvelope
	}
	return nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
