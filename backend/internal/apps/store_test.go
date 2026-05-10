package apps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStoreReadsFixtureAppCatalog(t *testing.T) {
	store := NewStore()

	items, err := store.ReadAppCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("app catalog count = %d", len(items))
	}
	if items[0].AppSlug != "google-calendar" {
		t.Fatalf("first app slug = %q", items[0].AppSlug)
	}
	if items[0].Provider != "google-calendar-fixture" {
		t.Fatalf("provider = %q", items[0].Provider)
	}
	if len(items[0].Capabilities) == 0 {
		t.Fatalf("capabilities = %#v", items[0].Capabilities)
	}
}

func TestAppCatalogJSONDoesNotExposeSecrets(t *testing.T) {
	items := fixtureAppCatalog()
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("app catalog exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestStoreRejectsInvalidAppMetadata(t *testing.T) {
	if err := ValidateAppMetadata(AppMetadata{}); !errors.Is(err, ErrInvalidAppMetadata) {
		t.Fatalf("err = %v", err)
	}
}

func TestStoreClonesAppCapabilities(t *testing.T) {
	store := NewStore()

	items, err := store.ReadAppCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items[0].Capabilities[0] = "mutated"

	items, err = store.ReadAppCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Capabilities[0] == "mutated" {
		t.Fatal("app capabilities were mutated through read result")
	}
}

func TestStoreCreatesInstallIntentForCatalogApp(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if intent.InstallIntentRef == "" {
		t.Fatal("install intent ref was empty")
	}
	if intent.AppSlug != "google-calendar" {
		t.Fatalf("app slug = %q", intent.AppSlug)
	}
	if intent.UserID != 123 {
		t.Fatalf("user id = %d", intent.UserID)
	}
	if intent.Status != InstallIntentStatusPending {
		t.Fatalf("status = %q", intent.Status)
	}
	if intent.CreatedAt == "" || intent.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", intent)
	}
}

func TestStoreReadsInstallIntentsForCurrentUser(t *testing.T) {
	store := NewStore()

	if _, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateInstallIntent(context.Background(), 999, "resend-email"); err != nil {
		t.Fatal(err)
	}

	items, err := store.ReadInstallIntents(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("install intent count = %d", len(items))
	}
	if items[0].AppSlug != "google-calendar" {
		t.Fatalf("app slug = %q", items[0].AppSlug)
	}
}

func TestStoreMarksInstallIntentRequiresExternalAuth(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != InstallIntentStatusRequiresExternalAuth {
		t.Fatalf("status = %q", updated.Status)
	}

	items, err := store.ReadInstallIntents(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != InstallIntentStatusRequiresExternalAuth {
		t.Fatalf("read intents = %#v", items)
	}
}

func TestStoreReadsExternalAuthDescriptor(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef); !errors.Is(err, ErrInstallIntentNotReady) {
		t.Fatalf("pending descriptor err = %v", err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}

	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.InstallIntentRef != intent.InstallIntentRef {
		t.Fatalf("install intent ref = %q", descriptor.InstallIntentRef)
	}
	if descriptor.AppSlug != "google-calendar" {
		t.Fatalf("app slug = %q", descriptor.AppSlug)
	}
	if descriptor.Provider != "google-calendar-fixture" {
		t.Fatalf("provider = %q", descriptor.Provider)
	}
	if descriptor.AuthType != "oauth" {
		t.Fatalf("auth type = %q", descriptor.AuthType)
	}
	if descriptor.Status != InstallIntentStatusRequiresExternalAuth {
		t.Fatalf("status = %q", descriptor.Status)
	}
	if descriptor.HandoffStateRef == "" || descriptor.HandoffStateRef == intent.InstallIntentRef {
		t.Fatalf("handoff state ref = %q", descriptor.HandoffStateRef)
	}
	if descriptor.ExpiresAt == "" {
		t.Fatalf("expires at was empty: %#v", descriptor)
	}
	if len(descriptor.Scopes) != 3 {
		t.Fatalf("scopes = %#v", descriptor.Scopes)
	}
	descriptor.Scopes[0] = "mutated"
	descriptor, err = store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Scopes[0] == "mutated" {
		t.Fatal("descriptor scopes were mutated through read result")
	}
}

func TestStoreRejectsExternalAuthDescriptorForWrongOwner(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadExternalAuthDescriptor(context.Background(), 999, intent.InstallIntentRef); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner descriptor err = %v", err)
	}
}

func TestStoreConsumesExternalAuthSessionOnce(t *testing.T) {
	current := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	session, err := store.ConsumeExternalAuthSession(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	if session.HandoffStateRef != descriptor.HandoffStateRef {
		t.Fatalf("handoff state ref = %q", session.HandoffStateRef)
	}
	if session.ConsumedAt != wireTime(current) {
		t.Fatalf("consumed at = %q", session.ConsumedAt)
	}
	if _, err := store.ConsumeExternalAuthSession(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionConsumed) {
		t.Fatalf("replay err = %v", err)
	}
	if _, err := store.ConsumeExternalAuthSession(context.Background(), 999, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionNotFound) {
		t.Fatalf("wrong-owner consume err = %v", err)
	}
}

func TestStoreCreatesExternalAuthAuthorizationPreviewOnce(t *testing.T) {
	current := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	if preview.InstallIntentRef != intent.InstallIntentRef {
		t.Fatalf("install intent ref = %q", preview.InstallIntentRef)
	}
	if preview.AuthorizationPreviewRef == "" {
		t.Fatalf("authorization preview ref was empty: %#v", preview)
	}
	if preview.AppSlug != "google-calendar" {
		t.Fatalf("app slug = %q", preview.AppSlug)
	}
	if preview.ProviderSlug != "google-calendar-fixture" {
		t.Fatalf("provider slug = %q", preview.ProviderSlug)
	}
	if preview.AuthType != "oauth" {
		t.Fatalf("auth type = %q", preview.AuthType)
	}
	if len(preview.RequestedScopes) != 3 {
		t.Fatalf("requested scopes = %#v", preview.RequestedScopes)
	}
	if preview.StateRef != descriptor.HandoffStateRef {
		t.Fatalf("state ref = %q", preview.StateRef)
	}
	if preview.AuthorizationEndpointID != "google-calendar-fixture.authorization.preview" {
		t.Fatalf("authorization endpoint id = %q", preview.AuthorizationEndpointID)
	}
	if preview.Status != "preview_only" {
		t.Fatalf("status = %q", preview.Status)
	}
	if preview.ExpiresAt != descriptor.ExpiresAt {
		t.Fatalf("expires at = %q", preview.ExpiresAt)
	}
	if preview.RequestedAt != wireTime(current) {
		t.Fatalf("requested at = %q", preview.RequestedAt)
	}
	preview.RequestedScopes[0] = "mutated"

	if _, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionConsumed) {
		t.Fatalf("preview replay err = %v", err)
	}
	if _, err := store.ConsumeExternalAuthSession(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionConsumed) {
		t.Fatalf("consume after preview err = %v", err)
	}

	descriptor, err = store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preview, err = store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	if preview.RequestedScopes[0] == "mutated" {
		t.Fatal("preview requested scopes were mutated through read result")
	}
}

func TestStoreCreatesExternalAuthCallbackPreflightFromHandoffStateOnce(t *testing.T) {
	current := time.Date(2026, 4, 30, 12, 30, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.CallbackPreflightRef == "" {
		t.Fatalf("callback preflight ref was empty: %#v", preflight)
	}
	if preflight.InstallIntentRef != intent.InstallIntentRef {
		t.Fatalf("install intent ref = %q", preflight.InstallIntentRef)
	}
	if preflight.AppSlug != "google-calendar" {
		t.Fatalf("app slug = %q", preflight.AppSlug)
	}
	if preflight.ProviderSlug != "google-calendar-fixture" {
		t.Fatalf("provider slug = %q", preflight.ProviderSlug)
	}
	if preflight.StateRef != descriptor.HandoffStateRef {
		t.Fatalf("state ref = %q", preflight.StateRef)
	}
	if preflight.CallbackStatus != ExternalAuthCallbackStatusAuthorized {
		t.Fatalf("callback status = %q", preflight.CallbackStatus)
	}
	if preflight.PreflightStatus != ExternalAuthCallbackPreflightAccepted {
		t.Fatalf("preflight status = %q", preflight.PreflightStatus)
	}
	if preflight.ReceivedAt != wireTime(current) {
		t.Fatalf("received at = %q", preflight.ReceivedAt)
	}

	if _, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	}); !errors.Is(err, ErrExternalAuthCallbackPreflightConsumed) {
		t.Fatalf("callback preflight replay err = %v", err)
	}
	if _, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionConsumed) {
		t.Fatalf("preview after callback preflight err = %v", err)
	}
}

func TestStoreCreatesExternalAuthCallbackPreflightFromPreviewState(t *testing.T) {
	current := time.Date(2026, 4, 30, 12, 45, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}

	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusDenied,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.StateRef != preview.StateRef {
		t.Fatalf("state ref = %q", preflight.StateRef)
	}
	if preflight.CallbackStatus != ExternalAuthCallbackStatusDenied {
		t.Fatalf("callback status = %q", preflight.CallbackStatus)
	}
}

func TestStoreRecordsExternalAuthProviderExchangeFromCallbackPreflight(t *testing.T) {
	current := time.Date(2026, 4, 30, 13, 15, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}

	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             preview.StateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exchange.ProviderExchangeRef == "" {
		t.Fatalf("provider exchange ref was empty: %#v", exchange)
	}
	if exchange.InstallIntentRef != intent.InstallIntentRef {
		t.Fatalf("install intent ref = %q", exchange.InstallIntentRef)
	}
	if exchange.StateRef != preview.StateRef {
		t.Fatalf("state ref = %q", exchange.StateRef)
	}
	if exchange.CallbackPreflightRef != preflight.CallbackPreflightRef {
		t.Fatalf("callback preflight ref = %q", exchange.CallbackPreflightRef)
	}
	if exchange.ExchangeStatus != ExternalAuthProviderExchangeQueued {
		t.Fatalf("exchange status = %q", exchange.ExchangeStatus)
	}
	if exchange.ExchangeMode != ExternalAuthProviderExchangeModeSimulated {
		t.Fatalf("exchange mode = %q", exchange.ExchangeMode)
	}
	if exchange.RecordedAt != wireTime(current) {
		t.Fatalf("recorded at = %q", exchange.RecordedAt)
	}
	if _, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             preview.StateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	}); !errors.Is(err, ErrExternalAuthProviderExchangeRecorded) {
		t.Fatalf("provider exchange replay err = %v", err)
	}
}

func TestStoreRecordsBlockedExternalAuthProviderExchangeForDeniedCallback(t *testing.T) {
	current := time.Date(2026, 4, 30, 13, 30, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusDenied,
	})
	if err != nil {
		t.Fatal(err)
	}

	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             descriptor.HandoffStateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exchange.ExchangeStatus != ExternalAuthProviderExchangeBlocked {
		t.Fatalf("exchange status = %q", exchange.ExchangeStatus)
	}

	progress, err := store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusProviderExchangeBlocked {
		t.Fatalf("blocked progress status = %q", progress.ProgressStatus)
	}
	if progress.NextAction != InstallProgressActionRestartExternalAuth {
		t.Fatalf("blocked next action = %q", progress.NextAction)
	}
}

func TestStoreCompletesAppInstallFromProviderExchange(t *testing.T) {
	current := time.Date(2026, 4, 30, 13, 45, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             preview.StateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	current = current.Add(time.Minute)
	completion, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if completion.InstallCompletionRef == "" {
		t.Fatalf("install completion ref was empty: %#v", completion)
	}
	if completion.InstallIntentRef != intent.InstallIntentRef {
		t.Fatalf("install intent ref = %q", completion.InstallIntentRef)
	}
	if completion.ProviderExchangeRef != exchange.ProviderExchangeRef {
		t.Fatalf("provider exchange ref = %q", completion.ProviderExchangeRef)
	}
	if completion.CompletionStatus != AppInstallCompletionStatusInstalled {
		t.Fatalf("completion status = %q", completion.CompletionStatus)
	}
	if completion.CompletionMode != AppInstallCompletionModeSimulated {
		t.Fatalf("completion mode = %q", completion.CompletionMode)
	}
	if completion.CompletedAt != wireTime(current) {
		t.Fatalf("completed at = %q", completion.CompletedAt)
	}

	progress, err := store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.InstallStatus != InstallIntentStatusInstalled {
		t.Fatalf("install status = %q", progress.InstallStatus)
	}
	if progress.ProgressStatus != InstallProgressStatusInstalled {
		t.Fatalf("progress status = %q", progress.ProgressStatus)
	}
	if progress.NextAction != InstallProgressActionNone {
		t.Fatalf("next action = %q", progress.NextAction)
	}
	if !progress.InstallCompletionRecorded || progress.InstallCompletionRef != completion.InstallCompletionRef {
		t.Fatalf("completion progress = %#v, completion = %#v", progress, completion)
	}
	if _, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	}); !errors.Is(err, ErrAppInstallCompletionRecorded) {
		t.Fatalf("install completion replay err = %v", err)
	}
}

func TestStoreActivatesAppInstallationFromCompletion(t *testing.T) {
	current := time.Date(2026, 5, 2, 9, 15, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             preview.StateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	current = current.Add(time.Minute)
	installation, err := store.ActivateAppInstallation(context.Background(), 123, intent.InstallIntentRef, AppInstallationActivationRequest{
		InstallCompletionRef: completion.InstallCompletionRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if installation.AppInstallationRef == "" {
		t.Fatalf("app installation ref was empty: %#v", installation)
	}
	if installation.InstallCompletionRef != completion.InstallCompletionRef {
		t.Fatalf("install completion ref = %q", installation.InstallCompletionRef)
	}
	if installation.AppName != "Google Calendar" {
		t.Fatalf("app name = %q", installation.AppName)
	}
	if installation.ActivationStatus != AppInstallationStatusActive {
		t.Fatalf("activation status = %q", installation.ActivationStatus)
	}
	if installation.ActivationMode != AppInstallationModeSimulated {
		t.Fatalf("activation mode = %q", installation.ActivationMode)
	}
	if installation.ActivatedAt != wireTime(current) {
		t.Fatalf("activated at = %q", installation.ActivatedAt)
	}

	items, err := store.ReadAppInstallations(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].AppInstallationRef != installation.AppInstallationRef {
		t.Fatalf("app installations = %#v", items)
	}
	progress, err := store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusActive {
		t.Fatalf("active progress status = %q", progress.ProgressStatus)
	}
	if !progress.AppInstallationRecorded || progress.AppInstallationRef != installation.AppInstallationRef {
		t.Fatalf("app installation progress = %#v, installation = %#v", progress, installation)
	}
	if _, err := store.ActivateAppInstallation(context.Background(), 123, intent.InstallIntentRef, AppInstallationActivationRequest{
		InstallCompletionRef: completion.InstallCompletionRef,
	}); !errors.Is(err, ErrAppInstallationRecorded) {
		t.Fatalf("app installation replay err = %v", err)
	}
}

func TestStoreCompletesBlockedAppInstallFromBlockedProviderExchange(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusDenied,
	})
	if err != nil {
		t.Fatal(err)
	}
	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             descriptor.HandoffStateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	completion, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if completion.CompletionStatus != AppInstallCompletionStatusBlocked {
		t.Fatalf("completion status = %q", completion.CompletionStatus)
	}
	progress, err := store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.InstallStatus != InstallIntentStatusBlocked {
		t.Fatalf("install status = %q", progress.InstallStatus)
	}
	if progress.ProgressStatus != InstallProgressStatusBlocked {
		t.Fatalf("blocked completion progress status = %q", progress.ProgressStatus)
	}
	if progress.NextAction != InstallProgressActionCreateNewIntent {
		t.Fatalf("blocked completion next action = %q", progress.NextAction)
	}
}

func TestStoreRejectsAppInstallationForWrongOwnerMissingAndBlockedCompletion(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusDenied,
	})
	if err != nil {
		t.Fatal(err)
	}
	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             descriptor.HandoffStateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := AppInstallationActivationRequest{InstallCompletionRef: completion.InstallCompletionRef}

	if _, err := store.ActivateAppInstallation(context.Background(), 999, intent.InstallIntentRef, request); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner app installation err = %v", err)
	}
	if _, err := store.ActivateAppInstallation(context.Background(), 123, intent.InstallIntentRef, AppInstallationActivationRequest{
		InstallCompletionRef: "install-completion-missing",
	}); !errors.Is(err, ErrAppInstallCompletionNotFound) {
		t.Fatalf("missing completion app installation err = %v", err)
	}
	if _, err := store.ActivateAppInstallation(context.Background(), 123, intent.InstallIntentRef, request); !errors.Is(err, ErrAppInstallCompletionNotReady) {
		t.Fatalf("blocked completion app installation err = %v", err)
	}
	if _, err := store.ActivateAppInstallation(context.Background(), 123, intent.InstallIntentRef, AppInstallationActivationRequest{}); !errors.Is(err, ErrInvalidAppInstallation) {
		t.Fatalf("invalid app installation err = %v", err)
	}
}

func TestStoreRejectsAppInstallCompletionForWrongOwnerMissingExchangeAndInvalidRequest(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	request := AppInstallCompletionRequest{ProviderExchangeRef: "provider-exchange-missing"}

	if _, err := store.CompleteAppInstall(context.Background(), 999, intent.InstallIntentRef, request); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner completion err = %v", err)
	}
	if _, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, request); !errors.Is(err, ErrExternalAuthProviderExchangeNotFound) {
		t.Fatalf("missing provider exchange completion err = %v", err)
	}
	if _, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{}); !errors.Is(err, ErrInvalidAppInstallCompletion) {
		t.Fatalf("invalid completion err = %v", err)
	}
}

func TestStoreRejectsExternalAuthProviderExchangeForWrongOwnerAndMissingPreflight(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	request := ExternalAuthProviderExchangeRequest{
		StateRef:             descriptor.HandoffStateRef,
		CallbackPreflightRef: "callback-preflight-missing",
	}

	if _, err := store.ExchangeExternalAuthProvider(context.Background(), 999, intent.InstallIntentRef, request); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner provider exchange err = %v", err)
	}
	if _, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, request); !errors.Is(err, ErrExternalAuthCallbackPreflightNotFound) {
		t.Fatalf("missing preflight provider exchange err = %v", err)
	}
	if _, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef: descriptor.HandoffStateRef,
	}); !errors.Is(err, ErrInvalidExternalAuthProviderExchange) {
		t.Fatalf("invalid provider exchange err = %v", err)
	}
}

func TestStoreRejectsExternalAuthCallbackPreflightForWrongOwnerAndExpiredState(t *testing.T) {
	current := time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	request := ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	}
	if _, err := store.PreflightExternalAuthCallback(context.Background(), 999, intent.InstallIntentRef, request); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner callback preflight err = %v", err)
	}
	if _, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: "unexpected",
	}); !errors.Is(err, ErrInvalidExternalAuthCallbackPreflight) {
		t.Fatalf("invalid callback preflight err = %v", err)
	}

	current = current.Add(11 * time.Minute)
	if _, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, request); !errors.Is(err, ErrExternalAuthSessionExpired) {
		t.Fatalf("expired callback preflight err = %v", err)
	}
}

func TestStoreReadsInstallProgressThroughExternalAuthFlow(t *testing.T) {
	current := time.Date(2026, 4, 30, 14, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	progress, err := store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusPending {
		t.Fatalf("pending progress status = %q", progress.ProgressStatus)
	}
	if progress.ExternalAuthDescriptorAvailable {
		t.Fatalf("pending descriptor availability = true: %#v", progress)
	}
	if progress.NextAction != InstallProgressActionStartExternalAuth {
		t.Fatalf("pending next action = %q", progress.NextAction)
	}

	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	progress, err = store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusExternalAuthReady {
		t.Fatalf("ready progress status = %q", progress.ProgressStatus)
	}
	if !progress.ExternalAuthDescriptorAvailable {
		t.Fatalf("descriptor availability = false: %#v", progress)
	}
	if progress.NextAction != InstallProgressActionReadDescriptor {
		t.Fatalf("ready next action = %q", progress.NextAction)
	}

	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	progress, err = store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusHandoffReady {
		t.Fatalf("handoff progress status = %q", progress.ProgressStatus)
	}
	if progress.LatestHandoffStateRef != descriptor.HandoffStateRef {
		t.Fatalf("latest handoff state ref = %q", progress.LatestHandoffStateRef)
	}
	if progress.LatestHandoffStateExpiresAt != descriptor.ExpiresAt {
		t.Fatalf("latest handoff expires at = %q", progress.LatestHandoffStateExpiresAt)
	}
	if progress.NextAction != InstallProgressActionPreviewAuth {
		t.Fatalf("handoff next action = %q", progress.NextAction)
	}

	preview, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	progress, err = store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusPreviewRecorded {
		t.Fatalf("preview progress status = %q", progress.ProgressStatus)
	}
	if !progress.AuthorizationPreviewRecorded {
		t.Fatalf("authorization preview recorded = false: %#v", progress)
	}
	if progress.AuthorizationPreviewRef != preview.AuthorizationPreviewRef {
		t.Fatalf("authorization preview ref = %q", progress.AuthorizationPreviewRef)
	}
	if progress.AuthorizationPreviewStatus != ExternalAuthAuthorizationPreviewStatus {
		t.Fatalf("authorization preview status = %q", progress.AuthorizationPreviewStatus)
	}
	if progress.NextAction != InstallProgressActionRecordCallback {
		t.Fatalf("preview next action = %q", progress.NextAction)
	}

	preflight, err := store.PreflightExternalAuthCallback(context.Background(), 123, intent.InstallIntentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	progress, err = store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusCallbackPreflight {
		t.Fatalf("callback progress status = %q", progress.ProgressStatus)
	}
	if !progress.CallbackPreflightRecorded {
		t.Fatalf("callback preflight recorded = false: %#v", progress)
	}
	if progress.CallbackPreflightRef != preflight.CallbackPreflightRef {
		t.Fatalf("callback preflight ref = %q", progress.CallbackPreflightRef)
	}
	if progress.CallbackStatus != ExternalAuthCallbackStatusAuthorized {
		t.Fatalf("callback status = %q", progress.CallbackStatus)
	}
	if progress.NextAction != InstallProgressActionAwaitExchange {
		t.Fatalf("callback next action = %q", progress.NextAction)
	}
	exchange, err := store.ExchangeExternalAuthProvider(context.Background(), 123, intent.InstallIntentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             preflight.StateRef,
		CallbackPreflightRef: preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	progress, err = store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusProviderExchangeQueued {
		t.Fatalf("provider exchange progress status = %q", progress.ProgressStatus)
	}
	if !progress.ProviderExchangeRecorded {
		t.Fatalf("provider exchange recorded = false: %#v", progress)
	}
	if progress.ProviderExchangeRef != exchange.ProviderExchangeRef {
		t.Fatalf("provider exchange ref = %q", progress.ProviderExchangeRef)
	}
	if progress.ProviderExchangeStatus != ExternalAuthProviderExchangeQueued {
		t.Fatalf("provider exchange status = %q", progress.ProviderExchangeStatus)
	}
	completion, err := store.CompleteAppInstall(context.Background(), 123, intent.InstallIntentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	progress, err = store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.InstallStatus != InstallIntentStatusInstalled {
		t.Fatalf("completed install status = %q", progress.InstallStatus)
	}
	if progress.ProgressStatus != InstallProgressStatusInstalled {
		t.Fatalf("completed progress status = %q", progress.ProgressStatus)
	}
	if !progress.InstallCompletionRecorded {
		t.Fatalf("install completion recorded = false: %#v", progress)
	}
	if progress.InstallCompletionRef != completion.InstallCompletionRef {
		t.Fatalf("install completion ref = %q", progress.InstallCompletionRef)
	}
	if progress.InstallCompletionStatus != AppInstallCompletionStatusInstalled {
		t.Fatalf("install completion status = %q", progress.InstallCompletionStatus)
	}
	if progress.NextAction != InstallProgressActionNone {
		t.Fatalf("completed next action = %q", progress.NextAction)
	}
	if _, err := store.ReadInstallProgress(context.Background(), 999, intent.InstallIntentRef); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner progress err = %v", err)
	}
}

func TestStoreReadsExpiredInstallProgress(t *testing.T) {
	current := time.Date(2026, 4, 30, 15, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	current = current.Add(11 * time.Minute)

	progress, err := store.ReadInstallProgress(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusHandoffExpired {
		t.Fatalf("expired progress status = %q", progress.ProgressStatus)
	}
	if progress.NextAction != InstallProgressActionReadDescriptor {
		t.Fatalf("expired next action = %q", progress.NextAction)
	}
}

func TestStoreRejectsExternalAuthAuthorizationPreviewForWrongOwner(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.PreviewExternalAuthAuthorization(context.Background(), 999, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-owner preview err = %v", err)
	}
}

func TestStoreRejectsExpiredExternalAuthAuthorizationPreview(t *testing.T) {
	current := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	current = current.Add(11 * time.Minute)
	if _, err := store.PreviewExternalAuthAuthorization(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionExpired) {
		t.Fatalf("expired preview err = %v", err)
	}
}

func TestStoreRejectsExpiredExternalAuthSession(t *testing.T) {
	current := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	store := NewStore(WithClock(func() time.Time { return current }))

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, intent.InstallIntentRef); err != nil {
		t.Fatal(err)
	}
	descriptor, err := store.ReadExternalAuthDescriptor(context.Background(), 123, intent.InstallIntentRef)
	if err != nil {
		t.Fatal(err)
	}

	current = current.Add(11 * time.Minute)
	if _, err := store.ConsumeExternalAuthSession(context.Background(), 123, intent.InstallIntentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionExpired) {
		t.Fatalf("expired consume err = %v", err)
	}
}

func TestStoreRejectsUnknownInstallIntentTransition(t *testing.T) {
	store := NewStore()

	if _, err := store.MarkInstallIntentRequiresExternalAuth(context.Background(), 123, "missing-intent"); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestStoreRejectsInstallIntentForUnknownApp(t *testing.T) {
	store := NewStore()

	if _, err := store.CreateInstallIntent(context.Background(), 123, "unknown-app"); !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestStoreRejectsInvalidInstallIntentInputs(t *testing.T) {
	store := NewStore()

	if _, err := store.CreateInstallIntent(context.Background(), 0, "google-calendar"); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("zero user err = %v", err)
	}
	if _, err := store.CreateInstallIntent(context.Background(), 123, " "); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("blank app slug err = %v", err)
	}
	if err := ValidateInstallIntent(AppInstallIntent{}); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("validate err = %v", err)
	}
	if err := ValidateInstallIntent(AppInstallIntent{
		InstallIntentRef: "app-intent-invalid-status",
		UserID:           123,
		AppSlug:          "google-calendar",
		Status:           "connected",
	}); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("invalid status err = %v", err)
	}
}

func TestAppInstallIntentJSONDoesNotExposeSecrets(t *testing.T) {
	intent := AppInstallIntent{
		InstallIntentRef: "app-intent-fixture",
		UserID:           123,
		AppSlug:          "google-calendar",
		Status:           InstallIntentStatusPending,
		CreatedAt:        "2026-01-01T00:00:00.000Z",
		UpdatedAt:        "2026-01-01T00:00:00.000Z",
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("install intent exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("install intent exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestExternalAuthDescriptorJSONDoesNotExposeSecrets(t *testing.T) {
	descriptor := ExternalAuthDescriptor{
		InstallIntentRef: "app-intent-fixture",
		AppSlug:          "google-calendar",
		Provider:         "google-calendar-fixture",
		Name:             "Google Calendar",
		Description:      "Calendar fixture.",
		AuthType:         "oauth",
		Scopes:           []string{"calendar.read"},
		Status:           InstallIntentStatusRequiresExternalAuth,
		HandoffStateRef:  "handoff-state-fixture",
		ExpiresAt:        "2026-01-01T00:10:00.000Z",
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("external auth descriptor exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("external auth descriptor exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestExternalAuthSessionJSONDoesNotExposeSecrets(t *testing.T) {
	session := ExternalAuthSession{
		HandoffStateRef:  "handoff-state-fixture",
		InstallIntentRef: "app-intent-fixture",
		UserID:           123,
		ExpiresAt:        "2026-01-01T00:10:00.000Z",
		ConsumedAt:       "2026-01-01T00:01:00.000Z",
		CreatedAt:        "2026-01-01T00:00:00.000Z",
		UpdatedAt:        "2026-01-01T00:01:00.000Z",
	}
	raw, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("external auth session exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("external auth session exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestExternalAuthAuthorizationPreviewJSONDoesNotExposeSecrets(t *testing.T) {
	preview := ExternalAuthAuthorizationPreview{
		AuthorizationPreviewRef: "authorization-preview-fixture",
		InstallIntentRef:        "app-intent-fixture",
		AppSlug:                 "google-calendar",
		ProviderSlug:            "google-calendar-fixture",
		AuthType:                "oauth",
		RequestedScopes:         []string{"calendar.read"},
		StateRef:                "handoff-state-fixture",
		AuthorizationEndpointID: "google-calendar-fixture.authorization.preview",
		Status:                  "preview_only",
		ExpiresAt:               "2026-01-01T00:10:00.000Z",
		RequestedAt:             "2026-01-01T00:01:00.000Z",
		CreatedAt:               "2026-01-01T00:01:00.000Z",
		UpdatedAt:               "2026-01-01T00:01:00.000Z",
		UserID:                  123,
	}
	raw, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("external auth authorization preview exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("external auth authorization preview exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestAppInstallProgressJSONDoesNotExposeSecrets(t *testing.T) {
	progress := AppInstallProgress{
		InstallIntentRef:                "app-intent-fixture",
		AppSlug:                         "google-calendar",
		AppName:                         "Google Calendar",
		ProviderSlug:                    "google-calendar-fixture",
		AuthType:                        "oauth",
		InstallStatus:                   InstallIntentStatusRequiresExternalAuth,
		ProgressStatus:                  InstallProgressStatusCallbackPreflight,
		ExternalAuthDescriptorAvailable: true,
		LatestHandoffStateRef:           "handoff-state-fixture",
		LatestHandoffStateExpiresAt:     "2026-01-01T00:10:00.000Z",
		LatestHandoffStateConsumedAt:    "2026-01-01T00:01:00.000Z",
		AuthorizationPreviewRecorded:    true,
		AuthorizationPreviewRef:         "authorization-preview-fixture",
		AuthorizationPreviewStatus:      ExternalAuthAuthorizationPreviewStatus,
		AuthorizationPreviewRequestedAt: "2026-01-01T00:01:00.000Z",
		CallbackPreflightRecorded:       true,
		CallbackPreflightRef:            "callback-preflight-fixture",
		CallbackStatus:                  ExternalAuthCallbackStatusAuthorized,
		CallbackPreflightStatus:         ExternalAuthCallbackPreflightAccepted,
		CallbackPreflightReceivedAt:     "2026-01-01T00:02:00.000Z",
		ProviderExchangeRecorded:        true,
		ProviderExchangeRef:             "provider-exchange-fixture",
		ProviderExchangeStatus:          ExternalAuthProviderExchangeQueued,
		ProviderExchangeMode:            ExternalAuthProviderExchangeModeSimulated,
		ProviderExchangeRecordedAt:      "2026-01-01T00:03:00.000Z",
		InstallCompletionRecorded:       true,
		InstallCompletionRef:            "install-completion-fixture",
		InstallCompletionStatus:         AppInstallCompletionStatusInstalled,
		InstallCompletionMode:           AppInstallCompletionModeSimulated,
		InstallCompletedAt:              "2026-01-01T00:04:00.000Z",
		AppInstallationRecorded:         true,
		AppInstallationRef:              "app-installation-fixture",
		AppInstallationStatus:           AppInstallationStatusActive,
		AppInstallationMode:             AppInstallationModeSimulated,
		AppInstallationActivatedAt:      "2026-01-01T00:05:00.000Z",
		NextAction:                      InstallProgressActionNone,
		UpdatedAt:                       "2026-01-01T00:02:00.000Z",
		UserID:                          123,
	}
	raw, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("app install progress exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
		"authorizationcode",
		"providercode",
		"rawquery",
		"querystring",
		"providerresponse",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("app install progress exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestAppInstallationJSONDoesNotExposeSecrets(t *testing.T) {
	installation := AppInstallation{
		AppInstallationRef:   "app-installation-fixture",
		InstallIntentRef:     "app-intent-fixture",
		InstallCompletionRef: "install-completion-fixture",
		AppSlug:              "google-calendar",
		AppName:              "Google Calendar",
		AppCategory:          "calendar",
		ProviderSlug:         "google-calendar-fixture",
		AuthType:             "oauth",
		Capabilities:         []string{"calendar.read"},
		ActivationStatus:     AppInstallationStatusActive,
		ActivationMode:       AppInstallationModeSimulated,
		ActivatedAt:          "2026-01-01T00:05:00.000Z",
		CreatedAt:            "2026-01-01T00:05:00.000Z",
		UpdatedAt:            "2026-01-01T00:05:00.000Z",
		UserID:               123,
	}
	raw, err := json.Marshal(installation)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("app installation exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
		"authorizationcode",
		"providercode",
		"rawquery",
		"querystring",
		"providerresponse",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("app installation exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestAppInstallCompletionJSONDoesNotExposeSecrets(t *testing.T) {
	completion := AppInstallCompletion{
		InstallCompletionRef: "install-completion-fixture",
		InstallIntentRef:     "app-intent-fixture",
		AppSlug:              "google-calendar",
		ProviderSlug:         "google-calendar-fixture",
		ProviderExchangeRef:  "provider-exchange-fixture",
		CompletionStatus:     AppInstallCompletionStatusInstalled,
		CompletionMode:       AppInstallCompletionModeSimulated,
		CompletedAt:          "2026-01-01T00:04:00.000Z",
		CreatedAt:            "2026-01-01T00:04:00.000Z",
		UpdatedAt:            "2026-01-01T00:04:00.000Z",
		UserID:               123,
	}
	raw, err := json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("app install completion exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
		"authorizationcode",
		"providercode",
		"rawquery",
		"querystring",
		"providerresponse",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("app install completion exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestExternalAuthProviderExchangeJSONDoesNotExposeSecrets(t *testing.T) {
	exchange := ExternalAuthProviderExchange{
		ProviderExchangeRef:  "provider-exchange-fixture",
		InstallIntentRef:     "app-intent-fixture",
		AppSlug:              "google-calendar",
		ProviderSlug:         "google-calendar-fixture",
		StateRef:             "handoff-state-fixture",
		CallbackPreflightRef: "callback-preflight-fixture",
		ExchangeStatus:       ExternalAuthProviderExchangeQueued,
		ExchangeMode:         ExternalAuthProviderExchangeModeSimulated,
		RecordedAt:           "2026-01-01T00:03:00.000Z",
		CreatedAt:            "2026-01-01T00:03:00.000Z",
		UpdatedAt:            "2026-01-01T00:03:00.000Z",
		UserID:               123,
	}
	raw, err := json.Marshal(exchange)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("external auth provider exchange exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
		"authorizationcode",
		"providercode",
		"rawquery",
		"querystring",
		"providerresponse",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("external auth provider exchange exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestExternalAuthCallbackPreflightJSONDoesNotExposeSecrets(t *testing.T) {
	preflight := ExternalAuthCallbackPreflight{
		CallbackPreflightRef: "callback-preflight-fixture",
		InstallIntentRef:     "app-intent-fixture",
		AppSlug:              "google-calendar",
		ProviderSlug:         "google-calendar-fixture",
		StateRef:             "handoff-state-fixture",
		CallbackStatus:       ExternalAuthCallbackStatusAuthorized,
		PreflightStatus:      ExternalAuthCallbackPreflightAccepted,
		ReceivedAt:           "2026-01-01T00:01:00.000Z",
		CreatedAt:            "2026-01-01T00:01:00.000Z",
		UpdatedAt:            "2026-01-01T00:01:00.000Z",
		UserID:               123,
	}
	raw, err := json.Marshal(preflight)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("external auth callback preflight exposed internal user id: %s", body)
	}

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"credentialref",
		"providerpayload",
		"rawprovider",
		"accountref",
		"accountlabel",
		"redirecturl",
		"callbackurl",
		"authorizationcode",
		"providercode",
		"rawquery",
		"querystring",
		"providerresponse",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("external auth callback preflight exposed forbidden term %q: %s", forbidden, body)
		}
	}
}
