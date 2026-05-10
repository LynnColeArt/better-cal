package apps

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var ErrInvalidAppMetadata = errors.New("invalid app metadata")
var ErrInvalidInstallIntent = errors.New("invalid app install intent")
var ErrInstallIntentNotFound = errors.New("app install intent not found")
var ErrInstallIntentNotReady = errors.New("app install intent is not ready for external auth")
var ErrExternalAuthSessionNotFound = errors.New("external auth handoff session not found")
var ErrExternalAuthSessionConsumed = errors.New("external auth handoff session already consumed")
var ErrExternalAuthSessionExpired = errors.New("external auth handoff session expired")
var ErrInvalidExternalAuthCallbackPreflight = errors.New("invalid external auth callback preflight")
var ErrExternalAuthCallbackPreflightNotFound = errors.New("external auth callback preflight not found")
var ErrExternalAuthCallbackPreflightConsumed = errors.New("external auth callback preflight already recorded")
var ErrInvalidExternalAuthProviderExchange = errors.New("invalid external auth provider exchange")
var ErrExternalAuthProviderExchangeNotFound = errors.New("external auth provider exchange not found")
var ErrExternalAuthProviderExchangeRecorded = errors.New("external auth provider exchange already recorded")
var ErrInvalidAppInstallCompletion = errors.New("invalid app install completion")
var ErrAppInstallCompletionNotFound = errors.New("app install completion not found")
var ErrAppInstallCompletionNotReady = errors.New("app install completion is not ready for activation")
var ErrAppInstallCompletionRecorded = errors.New("app install completion already recorded")
var ErrInvalidAppInstallation = errors.New("invalid app installation")
var ErrAppInstallationRecorded = errors.New("app installation already recorded")
var ErrAppNotFound = errors.New("app not found")

const (
	InstallIntentStatusPending                   = "pending"
	InstallIntentStatusRequiresExternalAuth      = "requires_external_auth"
	InstallIntentStatusInstalled                 = "installed"
	InstallIntentStatusBlocked                   = "blocked"
	ExternalAuthAuthorizationPreviewStatus       = "preview_only"
	ExternalAuthCallbackStatusAuthorized         = "provider_authorized"
	ExternalAuthCallbackStatusDenied             = "provider_denied"
	ExternalAuthCallbackPreflightAccepted        = "accepted"
	ExternalAuthProviderExchangeQueued           = "queued"
	ExternalAuthProviderExchangeBlocked          = "blocked"
	ExternalAuthProviderExchangeModeSimulated    = "simulated"
	AppInstallCompletionStatusInstalled          = "installed"
	AppInstallCompletionStatusBlocked            = "blocked"
	AppInstallCompletionModeSimulated            = "simulated"
	AppInstallationStatusActive                  = "active"
	AppInstallationModeSimulated                 = "simulated"
	InstallProgressStatusPending                 = "pending"
	InstallProgressStatusExternalAuthReady       = "external_auth_ready"
	InstallProgressStatusHandoffReady            = "handoff_ready"
	InstallProgressStatusHandoffConsumed         = "handoff_consumed"
	InstallProgressStatusHandoffExpired          = "handoff_expired"
	InstallProgressStatusPreviewRecorded         = "authorization_preview_recorded"
	InstallProgressStatusCallbackPreflight       = "callback_preflight_recorded"
	InstallProgressStatusProviderExchangeQueued  = "provider_exchange_queued"
	InstallProgressStatusProviderExchangeBlocked = "provider_exchange_blocked"
	InstallProgressStatusInstalled               = "installed"
	InstallProgressStatusBlocked                 = "blocked"
	InstallProgressStatusActive                  = "active"
	InstallProgressActionStartExternalAuth       = "start_external_auth"
	InstallProgressActionReadDescriptor          = "read_external_auth_descriptor"
	InstallProgressActionPreviewAuth             = "preview_authorization"
	InstallProgressActionRecordCallback          = "record_callback_preflight"
	InstallProgressActionAwaitExchange           = "await_provider_exchange"
	InstallProgressActionNone                    = "none"
	InstallProgressActionCreateNewIntent         = "create_new_install_intent"
	InstallProgressActionRestartExternalAuth     = "restart_external_auth"
)

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

type AppInstallIntent struct {
	InstallIntentRef string `json:"installIntentRef"`
	UserID           int    `json:"-"`
	AppSlug          string `json:"appSlug"`
	Status           string `json:"status"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type CreateInstallIntentRequest struct {
	AppSlug string `json:"appSlug"`
}

type ConsumeExternalAuthRequest struct {
	HandoffStateRef string `json:"handoffStateRef"`
}

type ExternalAuthCallbackPreflightRequest struct {
	StateRef       string `json:"stateRef"`
	CallbackStatus string `json:"callbackStatus"`
}

type ExternalAuthProviderExchangeRequest struct {
	StateRef             string `json:"stateRef"`
	CallbackPreflightRef string `json:"callbackPreflightRef"`
}

type AppInstallCompletionRequest struct {
	ProviderExchangeRef string `json:"providerExchangeRef"`
}

type AppInstallationActivationRequest struct {
	InstallCompletionRef string `json:"installCompletionRef"`
}

type ExternalAuthDescriptor struct {
	InstallIntentRef string   `json:"installIntentRef"`
	AppSlug          string   `json:"appSlug"`
	Provider         string   `json:"provider"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	AuthType         string   `json:"authType"`
	Scopes           []string `json:"scopes"`
	Status           string   `json:"status"`
	HandoffStateRef  string   `json:"handoffStateRef"`
	ExpiresAt        string   `json:"expiresAt"`
}

type ExternalAuthSession struct {
	HandoffStateRef  string `json:"handoffStateRef"`
	InstallIntentRef string `json:"installIntentRef"`
	UserID           int    `json:"-"`
	ExpiresAt        string `json:"expiresAt"`
	ConsumedAt       string `json:"consumedAt,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type ExternalAuthAuthorizationPreview struct {
	AuthorizationPreviewRef string   `json:"authorizationPreviewRef"`
	InstallIntentRef        string   `json:"installIntentRef"`
	AppSlug                 string   `json:"appSlug"`
	ProviderSlug            string   `json:"providerSlug"`
	AuthType                string   `json:"authType"`
	RequestedScopes         []string `json:"requestedScopes"`
	StateRef                string   `json:"stateRef"`
	AuthorizationEndpointID string   `json:"authorizationEndpointId"`
	Status                  string   `json:"status"`
	ExpiresAt               string   `json:"expiresAt"`
	RequestedAt             string   `json:"requestedAt"`
	CreatedAt               string   `json:"createdAt"`
	UpdatedAt               string   `json:"updatedAt"`
	UserID                  int      `json:"-"`
}

type ExternalAuthCallbackPreflight struct {
	CallbackPreflightRef string `json:"callbackPreflightRef"`
	InstallIntentRef     string `json:"installIntentRef"`
	AppSlug              string `json:"appSlug"`
	ProviderSlug         string `json:"providerSlug"`
	StateRef             string `json:"stateRef"`
	CallbackStatus       string `json:"callbackStatus"`
	PreflightStatus      string `json:"preflightStatus"`
	ReceivedAt           string `json:"receivedAt"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	UserID               int    `json:"-"`
}

type ExternalAuthProviderExchange struct {
	ProviderExchangeRef  string `json:"providerExchangeRef"`
	InstallIntentRef     string `json:"installIntentRef"`
	AppSlug              string `json:"appSlug"`
	ProviderSlug         string `json:"providerSlug"`
	StateRef             string `json:"stateRef"`
	CallbackPreflightRef string `json:"callbackPreflightRef"`
	ExchangeStatus       string `json:"exchangeStatus"`
	ExchangeMode         string `json:"exchangeMode"`
	RecordedAt           string `json:"recordedAt"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	UserID               int    `json:"-"`
}

type AppInstallCompletion struct {
	InstallCompletionRef string `json:"installCompletionRef"`
	InstallIntentRef     string `json:"installIntentRef"`
	AppSlug              string `json:"appSlug"`
	ProviderSlug         string `json:"providerSlug"`
	ProviderExchangeRef  string `json:"providerExchangeRef"`
	CompletionStatus     string `json:"completionStatus"`
	CompletionMode       string `json:"completionMode"`
	CompletedAt          string `json:"completedAt"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	UserID               int    `json:"-"`
}

type AppInstallation struct {
	AppInstallationRef   string   `json:"appInstallationRef"`
	InstallIntentRef     string   `json:"installIntentRef"`
	InstallCompletionRef string   `json:"installCompletionRef"`
	AppSlug              string   `json:"appSlug"`
	AppName              string   `json:"appName"`
	AppCategory          string   `json:"appCategory"`
	ProviderSlug         string   `json:"providerSlug"`
	AuthType             string   `json:"authType"`
	Capabilities         []string `json:"capabilities"`
	ActivationStatus     string   `json:"activationStatus"`
	ActivationMode       string   `json:"activationMode"`
	ActivatedAt          string   `json:"activatedAt"`
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
	UserID               int      `json:"-"`
}

type AppInstallProgress struct {
	InstallIntentRef                string `json:"installIntentRef"`
	AppSlug                         string `json:"appSlug"`
	AppName                         string `json:"appName"`
	ProviderSlug                    string `json:"providerSlug"`
	AuthType                        string `json:"authType"`
	InstallStatus                   string `json:"installStatus"`
	ProgressStatus                  string `json:"progressStatus"`
	ExternalAuthDescriptorAvailable bool   `json:"externalAuthDescriptorAvailable"`
	LatestHandoffStateRef           string `json:"latestHandoffStateRef,omitempty"`
	LatestHandoffStateExpiresAt     string `json:"latestHandoffStateExpiresAt,omitempty"`
	LatestHandoffStateConsumedAt    string `json:"latestHandoffStateConsumedAt,omitempty"`
	AuthorizationPreviewRecorded    bool   `json:"authorizationPreviewRecorded"`
	AuthorizationPreviewRef         string `json:"authorizationPreviewRef,omitempty"`
	AuthorizationPreviewStatus      string `json:"authorizationPreviewStatus,omitempty"`
	AuthorizationPreviewRequestedAt string `json:"authorizationPreviewRequestedAt,omitempty"`
	CallbackPreflightRecorded       bool   `json:"callbackPreflightRecorded"`
	CallbackPreflightRef            string `json:"callbackPreflightRef,omitempty"`
	CallbackStatus                  string `json:"callbackStatus,omitempty"`
	CallbackPreflightStatus         string `json:"callbackPreflightStatus,omitempty"`
	CallbackPreflightReceivedAt     string `json:"callbackPreflightReceivedAt,omitempty"`
	ProviderExchangeRecorded        bool   `json:"providerExchangeRecorded"`
	ProviderExchangeRef             string `json:"providerExchangeRef,omitempty"`
	ProviderExchangeStatus          string `json:"providerExchangeStatus,omitempty"`
	ProviderExchangeMode            string `json:"providerExchangeMode,omitempty"`
	ProviderExchangeRecordedAt      string `json:"providerExchangeRecordedAt,omitempty"`
	InstallCompletionRecorded       bool   `json:"installCompletionRecorded"`
	InstallCompletionRef            string `json:"installCompletionRef,omitempty"`
	InstallCompletionStatus         string `json:"installCompletionStatus,omitempty"`
	InstallCompletionMode           string `json:"installCompletionMode,omitempty"`
	InstallCompletedAt              string `json:"installCompletedAt,omitempty"`
	AppInstallationRecorded         bool   `json:"appInstallationRecorded"`
	AppInstallationRef              string `json:"appInstallationRef,omitempty"`
	AppInstallationStatus           string `json:"appInstallationStatus,omitempty"`
	AppInstallationMode             string `json:"appInstallationMode,omitempty"`
	AppInstallationActivatedAt      string `json:"appInstallationActivatedAt,omitempty"`
	NextAction                      string `json:"nextAction"`
	UpdatedAt                       string `json:"updatedAt"`
	UserID                          int    `json:"-"`
}

type Repository interface {
	ReadAppCatalog(ctx context.Context) ([]AppMetadata, error)
	ReadInstallIntents(ctx context.Context, userID int) ([]AppInstallIntent, error)
	ReadAppInstallations(ctx context.Context, userID int) ([]AppInstallation, error)
	ReadInstallProgress(ctx context.Context, userID int, installIntentRef string, observedAt time.Time) (AppInstallProgress, error)
	SaveAppMetadata(ctx context.Context, app AppMetadata) (AppMetadata, error)
	SaveInstallIntent(ctx context.Context, intent AppInstallIntent) (AppInstallIntent, error)
	MarkInstallIntentRequiresExternalAuth(ctx context.Context, userID int, installIntentRef string) (AppInstallIntent, error)
	SaveExternalAuthSession(ctx context.Context, session ExternalAuthSession) (ExternalAuthSession, error)
	ConsumeExternalAuthSession(ctx context.Context, userID int, installIntentRef string, handoffStateRef string, consumedAt time.Time) (ExternalAuthSession, error)
	SaveExternalAuthAuthorizationPreview(ctx context.Context, intent AppInstallIntent, app AppMetadata, handoffStateRef string, requestedAt time.Time) (ExternalAuthAuthorizationPreview, error)
	SaveExternalAuthCallbackPreflight(ctx context.Context, preflight ExternalAuthCallbackPreflight, receivedAt time.Time) (ExternalAuthCallbackPreflight, error)
	SaveExternalAuthProviderExchange(ctx context.Context, exchange ExternalAuthProviderExchange, recordedAt time.Time) (ExternalAuthProviderExchange, error)
	SaveAppInstallCompletion(ctx context.Context, completion AppInstallCompletion, completedAt time.Time) (AppInstallCompletion, error)
	SaveAppInstallation(ctx context.Context, installation AppInstallation, activatedAt time.Time) (AppInstallation, error)
}

type Store struct {
	mu                    sync.Mutex
	repo                  Repository
	catalog               []AppMetadata
	installIntents        map[string]AppInstallIntent
	externalAuthSessions  map[string]ExternalAuthSession
	authorizationPreviews map[string]ExternalAuthAuthorizationPreview
	callbackPreflights    map[string]ExternalAuthCallbackPreflight
	providerExchanges     map[string]ExternalAuthProviderExchange
	installCompletions    map[string]AppInstallCompletion
	appInstallations      map[string]AppInstallation
	now                   func() time.Time
	loaded                bool
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
	}
}

func WithClock(now func() time.Time) StoreOption {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

func NewStore(opts ...StoreOption) *Store {
	store := &Store{
		installIntents:        make(map[string]AppInstallIntent),
		externalAuthSessions:  make(map[string]ExternalAuthSession),
		authorizationPreviews: make(map[string]ExternalAuthAuthorizationPreview),
		callbackPreflights:    make(map[string]ExternalAuthCallbackPreflight),
		providerExchanges:     make(map[string]ExternalAuthProviderExchange),
		installCompletions:    make(map[string]AppInstallCompletion),
		appInstallations:      make(map[string]AppInstallation),
		now:                   time.Now,
	}
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

func (s *Store) CreateInstallIntent(ctx context.Context, userID int, appSlug string) (AppInstallIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 || strings.TrimSpace(appSlug) == "" {
		return AppInstallIntent{}, ErrInvalidInstallIntent
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return AppInstallIntent{}, err
	}
	if !s.hasAppLocked(appSlug) {
		return AppInstallIntent{}, ErrAppNotFound
	}

	now := wireTime(time.Now())
	intent := AppInstallIntent{
		InstallIntentRef: newInstallIntentRef(),
		UserID:           userID,
		AppSlug:          appSlug,
		Status:           InstallIntentStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if s.repo != nil {
		return s.repo.SaveInstallIntent(ctx, intent)
	}
	s.installIntents[intent.InstallIntentRef] = intent
	return intent, nil
}

func (s *Store) ReadInstallIntents(ctx context.Context, userID int) ([]AppInstallIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 {
		return nil, ErrInvalidInstallIntent
	}
	if s.repo != nil {
		return s.repo.ReadInstallIntents(ctx, userID)
	}
	items := []AppInstallIntent{}
	for _, intent := range s.installIntents {
		if intent.UserID == userID {
			items = append(items, intent)
		}
	}
	return cloneInstallIntents(items), nil
}

func (s *Store) ReadAppInstallations(ctx context.Context, userID int) ([]AppInstallation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 {
		return nil, ErrInvalidAppInstallation
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return nil, err
	}
	if s.repo != nil {
		return s.repo.ReadAppInstallations(ctx, userID)
	}
	items := []AppInstallation{}
	for _, installation := range s.appInstallations {
		if installation.UserID == userID {
			items = append(items, installation)
		}
	}
	return cloneAppInstallations(items), nil
}

func (s *Store) ReadInstallProgress(ctx context.Context, userID int, installIntentRef string) (AppInstallProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" {
		return AppInstallProgress{}, ErrInvalidInstallIntent
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return AppInstallProgress{}, err
	}
	now := s.now()
	if s.repo != nil {
		return s.repo.ReadInstallProgress(ctx, userID, installIntentRef, now)
	}

	intent, ok := s.installIntents[installIntentRef]
	if !ok || intent.UserID != userID {
		return AppInstallProgress{}, ErrInstallIntentNotFound
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return AppInstallProgress{}, ErrAppNotFound
	}
	session, hasSession := s.latestExternalAuthSessionLocked(userID, installIntentRef)
	preview, hasPreview := s.latestExternalAuthAuthorizationPreviewLocked(userID, installIntentRef)
	preflight, hasPreflight := s.latestExternalAuthCallbackPreflightLocked(userID, installIntentRef)
	exchange, hasExchange := s.latestExternalAuthProviderExchangeLocked(userID, installIntentRef)
	completion, hasCompletion := s.latestAppInstallCompletionLocked(userID, installIntentRef)
	installation, hasInstallation := s.latestAppInstallationLocked(userID, installIntentRef)
	return newAppInstallProgress(
		intent,
		app,
		optionalExternalAuthSession(session, hasSession),
		optionalExternalAuthAuthorizationPreview(preview, hasPreview),
		optionalExternalAuthCallbackPreflight(preflight, hasPreflight),
		optionalExternalAuthProviderExchange(exchange, hasExchange),
		optionalAppInstallCompletion(completion, hasCompletion),
		optionalAppInstallation(installation, hasInstallation),
		now,
	), nil
}

func (s *Store) MarkInstallIntentRequiresExternalAuth(ctx context.Context, userID int, installIntentRef string) (AppInstallIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" {
		return AppInstallIntent{}, ErrInvalidInstallIntent
	}
	if s.repo != nil {
		return s.repo.MarkInstallIntentRequiresExternalAuth(ctx, userID, installIntentRef)
	}
	intent, ok := s.installIntents[installIntentRef]
	if !ok || intent.UserID != userID {
		return AppInstallIntent{}, ErrInstallIntentNotFound
	}
	if intent.Status != InstallIntentStatusPending && intent.Status != InstallIntentStatusRequiresExternalAuth {
		return AppInstallIntent{}, ErrInstallIntentNotReady
	}
	intent.Status = InstallIntentStatusRequiresExternalAuth
	intent.UpdatedAt = wireTime(time.Now())
	s.installIntents[installIntentRef] = intent
	return intent, nil
}

func (s *Store) ReadExternalAuthDescriptor(ctx context.Context, userID int, installIntentRef string) (ExternalAuthDescriptor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" {
		return ExternalAuthDescriptor{}, ErrInvalidInstallIntent
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return ExternalAuthDescriptor{}, err
	}

	intent, ok, err := s.findInstallIntentLocked(ctx, userID, installIntentRef)
	if err != nil {
		return ExternalAuthDescriptor{}, err
	}
	if !ok {
		return ExternalAuthDescriptor{}, ErrInstallIntentNotFound
	}
	if intent.Status != InstallIntentStatusRequiresExternalAuth {
		return ExternalAuthDescriptor{}, ErrInstallIntentNotReady
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return ExternalAuthDescriptor{}, ErrAppNotFound
	}
	session, err := s.createExternalAuthSessionLocked(ctx, intent)
	if err != nil {
		return ExternalAuthDescriptor{}, err
	}
	return newExternalAuthDescriptor(intent, app, session), nil
}

func (s *Store) ConsumeExternalAuthSession(ctx context.Context, userID int, installIntentRef string, handoffStateRef string) (ExternalAuthSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.consumeExternalAuthSessionLocked(ctx, userID, installIntentRef, handoffStateRef)
}

func (s *Store) PreviewExternalAuthAuthorization(ctx context.Context, userID int, installIntentRef string, handoffStateRef string) (ExternalAuthAuthorizationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" || strings.TrimSpace(handoffStateRef) == "" {
		return ExternalAuthAuthorizationPreview{}, ErrInvalidInstallIntent
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return ExternalAuthAuthorizationPreview{}, err
	}

	intent, ok, err := s.findInstallIntentLocked(ctx, userID, installIntentRef)
	if err != nil {
		return ExternalAuthAuthorizationPreview{}, err
	}
	if !ok {
		return ExternalAuthAuthorizationPreview{}, ErrInstallIntentNotFound
	}
	if intent.Status != InstallIntentStatusRequiresExternalAuth {
		return ExternalAuthAuthorizationPreview{}, ErrInstallIntentNotReady
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return ExternalAuthAuthorizationPreview{}, ErrAppNotFound
	}

	if s.repo != nil {
		return s.repo.SaveExternalAuthAuthorizationPreview(ctx, intent, app, handoffStateRef, s.now())
	}
	session, err := s.consumeExternalAuthSessionLocked(ctx, userID, installIntentRef, handoffStateRef)
	if err != nil {
		return ExternalAuthAuthorizationPreview{}, err
	}
	preview := newExternalAuthAuthorizationPreview(intent, app, session, s.now())
	s.authorizationPreviews[preview.StateRef] = preview
	return preview, nil
}

func (s *Store) PreflightExternalAuthCallback(ctx context.Context, userID int, installIntentRef string, request ExternalAuthCallbackPreflightRequest) (ExternalAuthCallbackPreflight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateRef := strings.TrimSpace(request.StateRef)
	if userID <= 0 ||
		strings.TrimSpace(installIntentRef) == "" ||
		stateRef == "" ||
		!isValidExternalAuthCallbackStatus(request.CallbackStatus) {
		return ExternalAuthCallbackPreflight{}, ErrInvalidExternalAuthCallbackPreflight
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return ExternalAuthCallbackPreflight{}, err
	}

	intent, ok, err := s.findInstallIntentLocked(ctx, userID, installIntentRef)
	if err != nil {
		return ExternalAuthCallbackPreflight{}, err
	}
	if !ok {
		return ExternalAuthCallbackPreflight{}, ErrInstallIntentNotFound
	}
	if intent.Status != InstallIntentStatusRequiresExternalAuth {
		return ExternalAuthCallbackPreflight{}, ErrInstallIntentNotReady
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return ExternalAuthCallbackPreflight{}, ErrAppNotFound
	}

	now := s.now()
	preflight := newExternalAuthCallbackPreflight(intent, app, stateRef, request.CallbackStatus, now)
	if s.repo != nil {
		return s.repo.SaveExternalAuthCallbackPreflight(ctx, preflight, now)
	}

	session, ok := s.externalAuthSessions[stateRef]
	if !ok || session.UserID != userID || session.InstallIntentRef != installIntentRef {
		return ExternalAuthCallbackPreflight{}, ErrExternalAuthSessionNotFound
	}
	expiresAt, err := parseWireTime(session.ExpiresAt)
	if err != nil {
		return ExternalAuthCallbackPreflight{}, ErrInvalidExternalAuthCallbackPreflight
	}
	if !expiresAt.After(now) {
		return ExternalAuthCallbackPreflight{}, ErrExternalAuthSessionExpired
	}
	if _, ok := s.callbackPreflights[stateRef]; ok {
		return ExternalAuthCallbackPreflight{}, ErrExternalAuthCallbackPreflightConsumed
	}
	if session.ConsumedAt == "" {
		session.ConsumedAt = wireTime(now)
	}
	session.UpdatedAt = wireTime(now)
	s.externalAuthSessions[stateRef] = session
	s.callbackPreflights[stateRef] = preflight
	return preflight, nil
}

func (s *Store) ExchangeExternalAuthProvider(ctx context.Context, userID int, installIntentRef string, request ExternalAuthProviderExchangeRequest) (ExternalAuthProviderExchange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateRef := strings.TrimSpace(request.StateRef)
	callbackPreflightRef := strings.TrimSpace(request.CallbackPreflightRef)
	if userID <= 0 ||
		strings.TrimSpace(installIntentRef) == "" ||
		stateRef == "" ||
		callbackPreflightRef == "" {
		return ExternalAuthProviderExchange{}, ErrInvalidExternalAuthProviderExchange
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return ExternalAuthProviderExchange{}, err
	}

	intent, ok, err := s.findInstallIntentLocked(ctx, userID, installIntentRef)
	if err != nil {
		return ExternalAuthProviderExchange{}, err
	}
	if !ok {
		return ExternalAuthProviderExchange{}, ErrInstallIntentNotFound
	}
	if intent.Status != InstallIntentStatusRequiresExternalAuth {
		return ExternalAuthProviderExchange{}, ErrInstallIntentNotReady
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return ExternalAuthProviderExchange{}, ErrAppNotFound
	}

	now := s.now()
	if s.repo != nil {
		exchange := newExternalAuthProviderExchangeRequest(intent, app, stateRef, callbackPreflightRef, now)
		return s.repo.SaveExternalAuthProviderExchange(ctx, exchange, now)
	}
	preflight, ok := s.findExternalAuthCallbackPreflightLocked(userID, installIntentRef, stateRef, callbackPreflightRef)
	if !ok {
		return ExternalAuthProviderExchange{}, ErrExternalAuthCallbackPreflightNotFound
	}
	exchange := newExternalAuthProviderExchange(intent, app, preflight, now)
	if _, ok := s.providerExchanges[callbackPreflightRef]; ok {
		return ExternalAuthProviderExchange{}, ErrExternalAuthProviderExchangeRecorded
	}
	s.providerExchanges[callbackPreflightRef] = exchange
	return exchange, nil
}

func (s *Store) CompleteAppInstall(ctx context.Context, userID int, installIntentRef string, request AppInstallCompletionRequest) (AppInstallCompletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	providerExchangeRef := strings.TrimSpace(request.ProviderExchangeRef)
	if userID <= 0 ||
		strings.TrimSpace(installIntentRef) == "" ||
		providerExchangeRef == "" {
		return AppInstallCompletion{}, ErrInvalidAppInstallCompletion
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return AppInstallCompletion{}, err
	}

	intent, ok, err := s.findInstallIntentLocked(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallCompletion{}, err
	}
	if !ok {
		return AppInstallCompletion{}, ErrInstallIntentNotFound
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return AppInstallCompletion{}, ErrAppNotFound
	}

	now := s.now()
	if s.repo != nil {
		completion := newAppInstallCompletionRequest(intent, app, providerExchangeRef, now)
		return s.repo.SaveAppInstallCompletion(ctx, completion, now)
	}
	exchange, ok := s.findExternalAuthProviderExchangeLocked(userID, installIntentRef, providerExchangeRef)
	if !ok {
		return AppInstallCompletion{}, ErrExternalAuthProviderExchangeNotFound
	}
	if _, ok := s.installCompletions[providerExchangeRef]; ok {
		return AppInstallCompletion{}, ErrAppInstallCompletionRecorded
	}
	if intent.Status != InstallIntentStatusRequiresExternalAuth {
		return AppInstallCompletion{}, ErrInstallIntentNotReady
	}
	completion := newAppInstallCompletion(intent, app, exchange, now)
	s.installCompletions[providerExchangeRef] = completion
	intent.Status = completion.CompletionStatus
	intent.UpdatedAt = completion.CompletedAt
	s.installIntents[installIntentRef] = intent
	return completion, nil
}

func (s *Store) ActivateAppInstallation(ctx context.Context, userID int, installIntentRef string, request AppInstallationActivationRequest) (AppInstallation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	installCompletionRef := strings.TrimSpace(request.InstallCompletionRef)
	if userID <= 0 ||
		strings.TrimSpace(installIntentRef) == "" ||
		installCompletionRef == "" {
		return AppInstallation{}, ErrInvalidAppInstallation
	}
	if err := s.ensureLoadedLocked(ctx); err != nil {
		return AppInstallation{}, err
	}

	intent, ok, err := s.findInstallIntentLocked(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallation{}, err
	}
	if !ok {
		return AppInstallation{}, ErrInstallIntentNotFound
	}
	app, ok := s.findAppLocked(intent.AppSlug)
	if !ok {
		return AppInstallation{}, ErrAppNotFound
	}

	now := s.now()
	if s.repo != nil {
		installation := newAppInstallationRequest(intent, app, installCompletionRef, now)
		return s.repo.SaveAppInstallation(ctx, installation, now)
	}
	completion, ok := s.findAppInstallCompletionLocked(userID, installIntentRef, installCompletionRef)
	if !ok {
		return AppInstallation{}, ErrAppInstallCompletionNotFound
	}
	if completion.CompletionStatus != AppInstallCompletionStatusInstalled || intent.Status != InstallIntentStatusInstalled {
		return AppInstallation{}, ErrAppInstallCompletionNotReady
	}
	if _, ok := s.appInstallations[installCompletionRef]; ok {
		return AppInstallation{}, ErrAppInstallationRecorded
	}
	for _, installation := range s.appInstallations {
		if installation.UserID == userID && installation.InstallIntentRef == installIntentRef {
			return AppInstallation{}, ErrAppInstallationRecorded
		}
	}
	installation := newAppInstallation(intent, app, completion, now)
	s.appInstallations[installCompletionRef] = installation
	return installation, nil
}

func (s *Store) consumeExternalAuthSessionLocked(ctx context.Context, userID int, installIntentRef string, handoffStateRef string) (ExternalAuthSession, error) {
	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" || strings.TrimSpace(handoffStateRef) == "" {
		return ExternalAuthSession{}, ErrInvalidInstallIntent
	}
	if s.repo != nil {
		return s.repo.ConsumeExternalAuthSession(ctx, userID, installIntentRef, handoffStateRef, s.now())
	}
	session, ok := s.externalAuthSessions[handoffStateRef]
	if !ok || session.UserID != userID || session.InstallIntentRef != installIntentRef {
		return ExternalAuthSession{}, ErrExternalAuthSessionNotFound
	}
	if session.ConsumedAt != "" {
		return ExternalAuthSession{}, ErrExternalAuthSessionConsumed
	}
	expiresAt, err := parseWireTime(session.ExpiresAt)
	if err != nil {
		return ExternalAuthSession{}, ErrInvalidInstallIntent
	}
	if !expiresAt.After(s.now()) {
		return ExternalAuthSession{}, ErrExternalAuthSessionExpired
	}
	now := wireTime(s.now())
	session.ConsumedAt = now
	session.UpdatedAt = now
	s.externalAuthSessions[handoffStateRef] = session
	return session, nil
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

func (s *Store) hasAppLocked(appSlug string) bool {
	_, ok := s.findAppLocked(appSlug)
	return ok
}

func (s *Store) findAppLocked(appSlug string) (AppMetadata, bool) {
	for _, app := range s.catalog {
		if app.AppSlug == appSlug {
			return app, true
		}
	}
	return AppMetadata{}, false
}

func (s *Store) findInstallIntentLocked(ctx context.Context, userID int, installIntentRef string) (AppInstallIntent, bool, error) {
	if s.repo != nil {
		items, err := s.repo.ReadInstallIntents(ctx, userID)
		if err != nil {
			return AppInstallIntent{}, false, err
		}
		for _, intent := range items {
			if intent.InstallIntentRef == installIntentRef {
				return intent, true, nil
			}
		}
		return AppInstallIntent{}, false, nil
	}
	intent, ok := s.installIntents[installIntentRef]
	if !ok || intent.UserID != userID {
		return AppInstallIntent{}, false, nil
	}
	return intent, true, nil
}

func (s *Store) createExternalAuthSessionLocked(ctx context.Context, intent AppInstallIntent) (ExternalAuthSession, error) {
	now := s.now()
	session := ExternalAuthSession{
		HandoffStateRef:  newHandoffStateRef(),
		InstallIntentRef: intent.InstallIntentRef,
		UserID:           intent.UserID,
		ExpiresAt:        wireTime(now.Add(10 * time.Minute)),
		CreatedAt:        wireTime(now),
		UpdatedAt:        wireTime(now),
	}
	if s.repo != nil {
		return s.repo.SaveExternalAuthSession(ctx, session)
	}
	s.externalAuthSessions[session.HandoffStateRef] = session
	return session, nil
}

func (s *Store) latestExternalAuthSessionLocked(userID int, installIntentRef string) (ExternalAuthSession, bool) {
	var latest ExternalAuthSession
	found := false
	for _, session := range s.externalAuthSessions {
		if session.UserID != userID || session.InstallIntentRef != installIntentRef {
			continue
		}
		if !found || session.CreatedAt > latest.CreatedAt || (session.CreatedAt == latest.CreatedAt && session.HandoffStateRef > latest.HandoffStateRef) {
			latest = session
			found = true
		}
	}
	return latest, found
}

func (s *Store) latestExternalAuthAuthorizationPreviewLocked(userID int, installIntentRef string) (ExternalAuthAuthorizationPreview, bool) {
	var latest ExternalAuthAuthorizationPreview
	found := false
	for _, preview := range s.authorizationPreviews {
		if preview.UserID != userID || preview.InstallIntentRef != installIntentRef {
			continue
		}
		if !found || preview.RequestedAt > latest.RequestedAt || (preview.RequestedAt == latest.RequestedAt && preview.AuthorizationPreviewRef > latest.AuthorizationPreviewRef) {
			latest = preview
			found = true
		}
	}
	return latest, found
}

func (s *Store) latestExternalAuthCallbackPreflightLocked(userID int, installIntentRef string) (ExternalAuthCallbackPreflight, bool) {
	var latest ExternalAuthCallbackPreflight
	found := false
	for _, preflight := range s.callbackPreflights {
		if preflight.UserID != userID || preflight.InstallIntentRef != installIntentRef {
			continue
		}
		if !found || preflight.ReceivedAt > latest.ReceivedAt || (preflight.ReceivedAt == latest.ReceivedAt && preflight.CallbackPreflightRef > latest.CallbackPreflightRef) {
			latest = preflight
			found = true
		}
	}
	return latest, found
}

func (s *Store) latestExternalAuthProviderExchangeLocked(userID int, installIntentRef string) (ExternalAuthProviderExchange, bool) {
	var latest ExternalAuthProviderExchange
	found := false
	for _, exchange := range s.providerExchanges {
		if exchange.UserID != userID || exchange.InstallIntentRef != installIntentRef {
			continue
		}
		if !found || exchange.RecordedAt > latest.RecordedAt || (exchange.RecordedAt == latest.RecordedAt && exchange.ProviderExchangeRef > latest.ProviderExchangeRef) {
			latest = exchange
			found = true
		}
	}
	return latest, found
}

func (s *Store) latestAppInstallCompletionLocked(userID int, installIntentRef string) (AppInstallCompletion, bool) {
	var latest AppInstallCompletion
	found := false
	for _, completion := range s.installCompletions {
		if completion.UserID != userID || completion.InstallIntentRef != installIntentRef {
			continue
		}
		if !found || completion.CompletedAt > latest.CompletedAt || (completion.CompletedAt == latest.CompletedAt && completion.InstallCompletionRef > latest.InstallCompletionRef) {
			latest = completion
			found = true
		}
	}
	return latest, found
}

func (s *Store) latestAppInstallationLocked(userID int, installIntentRef string) (AppInstallation, bool) {
	var latest AppInstallation
	found := false
	for _, installation := range s.appInstallations {
		if installation.UserID != userID || installation.InstallIntentRef != installIntentRef {
			continue
		}
		if !found || installation.ActivatedAt > latest.ActivatedAt || (installation.ActivatedAt == latest.ActivatedAt && installation.AppInstallationRef > latest.AppInstallationRef) {
			latest = installation
			found = true
		}
	}
	return latest, found
}

func (s *Store) findExternalAuthCallbackPreflightLocked(userID int, installIntentRef string, stateRef string, callbackPreflightRef string) (ExternalAuthCallbackPreflight, bool) {
	for _, preflight := range s.callbackPreflights {
		if preflight.UserID == userID &&
			preflight.InstallIntentRef == installIntentRef &&
			preflight.StateRef == stateRef &&
			preflight.CallbackPreflightRef == callbackPreflightRef {
			return preflight, true
		}
	}
	return ExternalAuthCallbackPreflight{}, false
}

func (s *Store) findExternalAuthProviderExchangeLocked(userID int, installIntentRef string, providerExchangeRef string) (ExternalAuthProviderExchange, bool) {
	for _, exchange := range s.providerExchanges {
		if exchange.UserID == userID &&
			exchange.InstallIntentRef == installIntentRef &&
			exchange.ProviderExchangeRef == providerExchangeRef {
			return exchange, true
		}
	}
	return ExternalAuthProviderExchange{}, false
}

func (s *Store) findAppInstallCompletionLocked(userID int, installIntentRef string, installCompletionRef string) (AppInstallCompletion, bool) {
	for _, completion := range s.installCompletions {
		if completion.UserID == userID &&
			completion.InstallIntentRef == installIntentRef &&
			completion.InstallCompletionRef == installCompletionRef {
			return completion, true
		}
	}
	return AppInstallCompletion{}, false
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

func ValidateInstallIntent(intent AppInstallIntent) error {
	if strings.TrimSpace(intent.InstallIntentRef) == "" ||
		intent.UserID <= 0 ||
		strings.TrimSpace(intent.AppSlug) == "" ||
		!isValidInstallIntentStatus(intent.Status) {
		return ErrInvalidInstallIntent
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

func cloneInstallIntents(items []AppInstallIntent) []AppInstallIntent {
	cloned := append([]AppInstallIntent(nil), items...)
	slices.SortFunc(cloned, func(left AppInstallIntent, right AppInstallIntent) int {
		switch {
		case left.CreatedAt < right.CreatedAt:
			return -1
		case left.CreatedAt > right.CreatedAt:
			return 1
		case left.InstallIntentRef < right.InstallIntentRef:
			return -1
		case left.InstallIntentRef > right.InstallIntentRef:
			return 1
		default:
			return 0
		}
	})
	return cloned
}

func cloneAppInstallations(items []AppInstallation) []AppInstallation {
	cloned := make([]AppInstallation, 0, len(items))
	for _, item := range items {
		item.Capabilities = append([]string(nil), item.Capabilities...)
		cloned = append(cloned, item)
	}
	slices.SortFunc(cloned, func(left AppInstallation, right AppInstallation) int {
		switch {
		case left.ActivatedAt > right.ActivatedAt:
			return -1
		case left.ActivatedAt < right.ActivatedAt:
			return 1
		case left.AppInstallationRef < right.AppInstallationRef:
			return -1
		case left.AppInstallationRef > right.AppInstallationRef:
			return 1
		default:
			return 0
		}
	})
	return cloned
}

func isValidInstallIntentStatus(status string) bool {
	switch status {
	case InstallIntentStatusPending, InstallIntentStatusRequiresExternalAuth, InstallIntentStatusInstalled, InstallIntentStatusBlocked:
		return true
	default:
		return false
	}
}

func isValidExternalAuthCallbackStatus(status string) bool {
	switch status {
	case ExternalAuthCallbackStatusAuthorized, ExternalAuthCallbackStatusDenied:
		return true
	default:
		return false
	}
}

func optionalExternalAuthSession(session ExternalAuthSession, ok bool) *ExternalAuthSession {
	if !ok {
		return nil
	}
	return &session
}

func optionalExternalAuthAuthorizationPreview(preview ExternalAuthAuthorizationPreview, ok bool) *ExternalAuthAuthorizationPreview {
	if !ok {
		return nil
	}
	return &preview
}

func optionalExternalAuthCallbackPreflight(preflight ExternalAuthCallbackPreflight, ok bool) *ExternalAuthCallbackPreflight {
	if !ok {
		return nil
	}
	return &preflight
}

func optionalExternalAuthProviderExchange(exchange ExternalAuthProviderExchange, ok bool) *ExternalAuthProviderExchange {
	if !ok {
		return nil
	}
	return &exchange
}

func optionalAppInstallCompletion(completion AppInstallCompletion, ok bool) *AppInstallCompletion {
	if !ok {
		return nil
	}
	return &completion
}

func optionalAppInstallation(installation AppInstallation, ok bool) *AppInstallation {
	if !ok {
		return nil
	}
	return &installation
}

func newExternalAuthDescriptor(intent AppInstallIntent, app AppMetadata, session ExternalAuthSession) ExternalAuthDescriptor {
	return ExternalAuthDescriptor{
		InstallIntentRef: intent.InstallIntentRef,
		AppSlug:          intent.AppSlug,
		Provider:         app.Provider,
		Name:             app.Name,
		Description:      app.Description,
		AuthType:         app.AuthType,
		Scopes:           append([]string(nil), app.Capabilities...),
		Status:           intent.Status,
		HandoffStateRef:  session.HandoffStateRef,
		ExpiresAt:        session.ExpiresAt,
	}
}

func newExternalAuthAuthorizationPreview(intent AppInstallIntent, app AppMetadata, session ExternalAuthSession, requestedAt time.Time) ExternalAuthAuthorizationPreview {
	now := wireTime(requestedAt)
	return ExternalAuthAuthorizationPreview{
		AuthorizationPreviewRef: newAuthorizationPreviewRef(),
		InstallIntentRef:        intent.InstallIntentRef,
		AppSlug:                 intent.AppSlug,
		ProviderSlug:            app.Provider,
		AuthType:                app.AuthType,
		RequestedScopes:         append([]string(nil), app.Capabilities...),
		StateRef:                session.HandoffStateRef,
		AuthorizationEndpointID: app.Provider + ".authorization.preview",
		Status:                  ExternalAuthAuthorizationPreviewStatus,
		ExpiresAt:               session.ExpiresAt,
		RequestedAt:             now,
		CreatedAt:               now,
		UpdatedAt:               now,
		UserID:                  intent.UserID,
	}
}

func newAppInstallProgress(intent AppInstallIntent, app AppMetadata, session *ExternalAuthSession, preview *ExternalAuthAuthorizationPreview, preflight *ExternalAuthCallbackPreflight, exchange *ExternalAuthProviderExchange, completion *AppInstallCompletion, installation *AppInstallation, observedAt time.Time) AppInstallProgress {
	progress := AppInstallProgress{
		InstallIntentRef:                intent.InstallIntentRef,
		AppSlug:                         intent.AppSlug,
		AppName:                         app.Name,
		ProviderSlug:                    app.Provider,
		AuthType:                        app.AuthType,
		InstallStatus:                   intent.Status,
		ProgressStatus:                  InstallProgressStatusPending,
		ExternalAuthDescriptorAvailable: false,
		NextAction:                      InstallProgressActionStartExternalAuth,
		UpdatedAt:                       intent.UpdatedAt,
		UserID:                          intent.UserID,
	}
	if intent.Status == InstallIntentStatusInstalled {
		progress.ProgressStatus = InstallProgressStatusInstalled
		progress.NextAction = InstallProgressActionNone
	}
	if intent.Status == InstallIntentStatusBlocked {
		progress.ProgressStatus = InstallProgressStatusBlocked
		progress.NextAction = InstallProgressActionCreateNewIntent
	}
	if intent.Status == InstallIntentStatusRequiresExternalAuth {
		progress.ProgressStatus = InstallProgressStatusExternalAuthReady
		progress.ExternalAuthDescriptorAvailable = true
		progress.NextAction = InstallProgressActionReadDescriptor
	}
	if session != nil {
		progress.LatestHandoffStateRef = session.HandoffStateRef
		progress.LatestHandoffStateExpiresAt = session.ExpiresAt
		progress.LatestHandoffStateConsumedAt = session.ConsumedAt
		progress.ProgressStatus = InstallProgressStatusHandoffReady
		progress.NextAction = InstallProgressActionPreviewAuth
		progress.UpdatedAt = laterWireTime(progress.UpdatedAt, session.UpdatedAt)
		if session.ConsumedAt != "" {
			progress.ProgressStatus = InstallProgressStatusHandoffConsumed
			progress.NextAction = InstallProgressActionRecordCallback
		} else if handoffSessionExpired(session.ExpiresAt, observedAt) {
			progress.ProgressStatus = InstallProgressStatusHandoffExpired
			progress.NextAction = InstallProgressActionReadDescriptor
		}
	}
	if preview != nil {
		progress.AuthorizationPreviewRecorded = true
		progress.AuthorizationPreviewRef = preview.AuthorizationPreviewRef
		progress.AuthorizationPreviewStatus = preview.Status
		progress.AuthorizationPreviewRequestedAt = preview.RequestedAt
		progress.ProgressStatus = InstallProgressStatusPreviewRecorded
		progress.NextAction = InstallProgressActionRecordCallback
		progress.UpdatedAt = laterWireTime(progress.UpdatedAt, preview.UpdatedAt)
	}
	if preflight != nil {
		progress.CallbackPreflightRecorded = true
		progress.CallbackPreflightRef = preflight.CallbackPreflightRef
		progress.CallbackStatus = preflight.CallbackStatus
		progress.CallbackPreflightStatus = preflight.PreflightStatus
		progress.CallbackPreflightReceivedAt = preflight.ReceivedAt
		progress.ProgressStatus = InstallProgressStatusCallbackPreflight
		progress.NextAction = InstallProgressActionAwaitExchange
		if preflight.CallbackStatus == ExternalAuthCallbackStatusDenied {
			progress.NextAction = InstallProgressActionRestartExternalAuth
		}
		progress.UpdatedAt = laterWireTime(progress.UpdatedAt, preflight.UpdatedAt)
	}
	if exchange != nil {
		progress.ProviderExchangeRecorded = true
		progress.ProviderExchangeRef = exchange.ProviderExchangeRef
		progress.ProviderExchangeStatus = exchange.ExchangeStatus
		progress.ProviderExchangeMode = exchange.ExchangeMode
		progress.ProviderExchangeRecordedAt = exchange.RecordedAt
		progress.ProgressStatus = InstallProgressStatusProviderExchangeQueued
		progress.NextAction = InstallProgressActionAwaitExchange
		if exchange.ExchangeStatus == ExternalAuthProviderExchangeBlocked {
			progress.ProgressStatus = InstallProgressStatusProviderExchangeBlocked
			progress.NextAction = InstallProgressActionRestartExternalAuth
		}
		progress.UpdatedAt = laterWireTime(progress.UpdatedAt, exchange.UpdatedAt)
	}
	if completion != nil {
		progress.InstallCompletionRecorded = true
		progress.InstallCompletionRef = completion.InstallCompletionRef
		progress.InstallCompletionStatus = completion.CompletionStatus
		progress.InstallCompletionMode = completion.CompletionMode
		progress.InstallCompletedAt = completion.CompletedAt
		progress.ProgressStatus = InstallProgressStatusInstalled
		progress.NextAction = InstallProgressActionNone
		if completion.CompletionStatus == AppInstallCompletionStatusBlocked {
			progress.ProgressStatus = InstallProgressStatusBlocked
			progress.NextAction = InstallProgressActionCreateNewIntent
		}
		progress.UpdatedAt = laterWireTime(progress.UpdatedAt, completion.UpdatedAt)
	}
	if completion == nil {
		if intent.Status == InstallIntentStatusInstalled {
			progress.ProgressStatus = InstallProgressStatusInstalled
			progress.NextAction = InstallProgressActionNone
		}
		if intent.Status == InstallIntentStatusBlocked {
			progress.ProgressStatus = InstallProgressStatusBlocked
			progress.NextAction = InstallProgressActionCreateNewIntent
		}
	}
	if installation != nil {
		progress.AppInstallationRecorded = true
		progress.AppInstallationRef = installation.AppInstallationRef
		progress.AppInstallationStatus = installation.ActivationStatus
		progress.AppInstallationMode = installation.ActivationMode
		progress.AppInstallationActivatedAt = installation.ActivatedAt
		progress.ProgressStatus = InstallProgressStatusActive
		progress.NextAction = InstallProgressActionNone
		progress.UpdatedAt = laterWireTime(progress.UpdatedAt, installation.UpdatedAt)
	}
	return progress
}

func handoffSessionExpired(expiresAt string, observedAt time.Time) bool {
	parsed, err := parseWireTime(expiresAt)
	if err != nil {
		return false
	}
	return !parsed.After(observedAt)
}

func laterWireTime(left string, right string) string {
	if right > left {
		return right
	}
	return left
}

func newExternalAuthCallbackPreflight(intent AppInstallIntent, app AppMetadata, stateRef string, callbackStatus string, receivedAt time.Time) ExternalAuthCallbackPreflight {
	now := wireTime(receivedAt)
	return ExternalAuthCallbackPreflight{
		CallbackPreflightRef: newCallbackPreflightRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		AppSlug:              intent.AppSlug,
		ProviderSlug:         app.Provider,
		StateRef:             stateRef,
		CallbackStatus:       callbackStatus,
		PreflightStatus:      ExternalAuthCallbackPreflightAccepted,
		ReceivedAt:           now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newExternalAuthProviderExchange(intent AppInstallIntent, app AppMetadata, preflight ExternalAuthCallbackPreflight, recordedAt time.Time) ExternalAuthProviderExchange {
	now := wireTime(recordedAt)
	status := ExternalAuthProviderExchangeQueued
	if preflight.CallbackStatus == ExternalAuthCallbackStatusDenied {
		status = ExternalAuthProviderExchangeBlocked
	}
	return ExternalAuthProviderExchange{
		ProviderExchangeRef:  newProviderExchangeRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		AppSlug:              intent.AppSlug,
		ProviderSlug:         app.Provider,
		StateRef:             preflight.StateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
		ExchangeStatus:       status,
		ExchangeMode:         ExternalAuthProviderExchangeModeSimulated,
		RecordedAt:           now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newExternalAuthProviderExchangeRequest(intent AppInstallIntent, app AppMetadata, stateRef string, callbackPreflightRef string, recordedAt time.Time) ExternalAuthProviderExchange {
	now := wireTime(recordedAt)
	return ExternalAuthProviderExchange{
		ProviderExchangeRef:  newProviderExchangeRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		AppSlug:              intent.AppSlug,
		ProviderSlug:         app.Provider,
		StateRef:             stateRef,
		CallbackPreflightRef: callbackPreflightRef,
		ExchangeMode:         ExternalAuthProviderExchangeModeSimulated,
		RecordedAt:           now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newAppInstallCompletion(intent AppInstallIntent, app AppMetadata, exchange ExternalAuthProviderExchange, completedAt time.Time) AppInstallCompletion {
	now := wireTime(completedAt)
	status := AppInstallCompletionStatusInstalled
	if exchange.ExchangeStatus == ExternalAuthProviderExchangeBlocked {
		status = AppInstallCompletionStatusBlocked
	}
	return AppInstallCompletion{
		InstallCompletionRef: newInstallCompletionRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		AppSlug:              intent.AppSlug,
		ProviderSlug:         app.Provider,
		ProviderExchangeRef:  exchange.ProviderExchangeRef,
		CompletionStatus:     status,
		CompletionMode:       AppInstallCompletionModeSimulated,
		CompletedAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newAppInstallCompletionRequest(intent AppInstallIntent, app AppMetadata, providerExchangeRef string, completedAt time.Time) AppInstallCompletion {
	now := wireTime(completedAt)
	return AppInstallCompletion{
		InstallCompletionRef: newInstallCompletionRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		AppSlug:              intent.AppSlug,
		ProviderSlug:         app.Provider,
		ProviderExchangeRef:  providerExchangeRef,
		CompletionMode:       AppInstallCompletionModeSimulated,
		CompletedAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newAppInstallation(intent AppInstallIntent, app AppMetadata, completion AppInstallCompletion, activatedAt time.Time) AppInstallation {
	now := wireTime(activatedAt)
	return AppInstallation{
		AppInstallationRef:   newAppInstallationRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		InstallCompletionRef: completion.InstallCompletionRef,
		AppSlug:              intent.AppSlug,
		AppName:              app.Name,
		AppCategory:          app.Category,
		ProviderSlug:         app.Provider,
		AuthType:             app.AuthType,
		Capabilities:         append([]string(nil), app.Capabilities...),
		ActivationStatus:     AppInstallationStatusActive,
		ActivationMode:       AppInstallationModeSimulated,
		ActivatedAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newAppInstallationRequest(intent AppInstallIntent, app AppMetadata, installCompletionRef string, activatedAt time.Time) AppInstallation {
	now := wireTime(activatedAt)
	return AppInstallation{
		AppInstallationRef:   newAppInstallationRef(),
		InstallIntentRef:     intent.InstallIntentRef,
		InstallCompletionRef: installCompletionRef,
		AppSlug:              intent.AppSlug,
		AppName:              app.Name,
		AppCategory:          app.Category,
		ProviderSlug:         app.Provider,
		AuthType:             app.AuthType,
		Capabilities:         append([]string(nil), app.Capabilities...),
		ActivationStatus:     AppInstallationStatusActive,
		ActivationMode:       AppInstallationModeSimulated,
		ActivatedAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
		UserID:               intent.UserID,
	}
}

func newProviderExchangeRef() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "provider-exchange-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("provider-exchange-%d", time.Now().UnixNano())
}

func newInstallCompletionRef() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "install-completion-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("install-completion-%d", time.Now().UnixNano())
}

func newAppInstallationRef() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "app-installation-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("app-installation-%d", time.Now().UnixNano())
}

func newAuthorizationPreviewRef() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "authorization-preview-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("authorization-preview-%d", time.Now().UnixNano())
}

func newCallbackPreflightRef() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "callback-preflight-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("callback-preflight-%d", time.Now().UnixNano())
}

func newHandoffStateRef() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "handoff-state-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("handoff-state-%d", time.Now().UnixNano())
}

func newInstallIntentRef() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "app-intent-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("app-intent-%d", time.Now().UnixNano())
}

func wireTime(value time.Time) string {
	return value.UTC().Format(wireTimeLayout)
}

func parseWireTime(value string) (time.Time, error) {
	parsed, err := time.Parse(wireTimeLayout, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
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
