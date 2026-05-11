package apps

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryRoundTripAppCatalog(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	slug := "app-catalog-repository-fixture"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_catalog where app_slug = $1`, slug)
	})

	app := AppMetadata{
		AppSlug:      slug,
		Category:     "calendar",
		Provider:     "repository-provider-fixture",
		Name:         "Repository Fixture",
		Description:  "Repository app catalog fixture.",
		AuthType:     "oauth",
		Capabilities: []string{"calendar.read", "calendar.write"},
	}
	if _, err := repo.SaveAppMetadata(ctx, app); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ReadAppCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found AppMetadata
	for _, item := range items {
		if item.AppSlug == slug {
			found = item
			break
		}
	}
	if found.AppSlug == "" {
		t.Fatalf("saved app %q was not found in catalog: %#v", slug, items)
	}
	if found.Name != app.Name {
		t.Fatalf("name = %q", found.Name)
	}
	if len(found.Capabilities) != 2 {
		t.Fatalf("capabilities = %#v", found.Capabilities)
	}
	if found.CreatedAt == "" || found.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", found)
	}
}

func TestPostgresRepositoryRoundTripInstallIntent(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	slug := "app-install-intent-repository-fixture"
	intentRef := "app-intent-repository-fixture"
	otherIntentRef := "app-intent-repository-other-user-fixture"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_install_intents where install_intent_ref in ($1, $2)`, intentRef, otherIntentRef)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_catalog where app_slug = $1`, slug)
	})

	if _, err := repo.SaveAppMetadata(ctx, AppMetadata{
		AppSlug:      slug,
		Category:     "calendar",
		Provider:     "install-intent-provider-fixture",
		Name:         "Install Intent Fixture",
		Description:  "Install intent app catalog fixture.",
		AuthType:     "oauth",
		Capabilities: []string{"calendar.read"},
	}); err != nil {
		t.Fatal(err)
	}

	intent, err := repo.SaveInstallIntent(ctx, AppInstallIntent{
		InstallIntentRef: intentRef,
		UserID:           123,
		AppSlug:          slug,
		Status:           InstallIntentStatusPending,
	})
	if err != nil {
		t.Fatal(err)
	}
	if intent.InstallIntentRef != intentRef {
		t.Fatalf("install intent ref = %q", intent.InstallIntentRef)
	}
	if intent.UserID != 123 {
		t.Fatalf("user id = %d", intent.UserID)
	}
	if intent.AppSlug != slug {
		t.Fatalf("app slug = %q", intent.AppSlug)
	}
	if intent.Status != InstallIntentStatusPending {
		t.Fatalf("status = %q", intent.Status)
	}
	if intent.CreatedAt == "" || intent.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", intent)
	}

	if _, err := repo.SaveInstallIntent(ctx, AppInstallIntent{
		InstallIntentRef: otherIntentRef,
		UserID:           999,
		AppSlug:          slug,
		Status:           InstallIntentStatusPending,
	}); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ReadInstallIntents(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	found, ok := findInstallIntent(items, intentRef)
	if !ok {
		t.Fatalf("saved install intent was not found: %#v", items)
	}
	if _, ok := findInstallIntent(items, otherIntentRef); ok {
		t.Fatalf("wrong-user install intent leaked into current-user read: %#v", items)
	}
	if found.Status != InstallIntentStatusPending {
		t.Fatalf("read status = %q", found.Status)
	}

	updated, err := repo.MarkInstallIntentRequiresExternalAuth(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != InstallIntentStatusRequiresExternalAuth {
		t.Fatalf("updated status = %q", updated.Status)
	}

	items, err = repo.ReadInstallIntents(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	found, ok = findInstallIntent(items, intentRef)
	if !ok || found.Status != InstallIntentStatusRequiresExternalAuth {
		t.Fatalf("read updated intents = %#v", items)
	}
	if _, err := repo.MarkInstallIntentRequiresExternalAuth(ctx, 123, otherIntentRef); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-user transition err = %v", err)
	}

	store := NewStoreWithRepository(repo)
	descriptor, err := store.ReadExternalAuthDescriptor(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.InstallIntentRef != intentRef {
		t.Fatalf("descriptor install intent ref = %q", descriptor.InstallIntentRef)
	}
	if descriptor.Provider != "install-intent-provider-fixture" {
		t.Fatalf("descriptor provider = %q", descriptor.Provider)
	}
	if len(descriptor.Scopes) != 1 || descriptor.Scopes[0] != "calendar.read" {
		t.Fatalf("descriptor scopes = %#v", descriptor.Scopes)
	}
	session, err := store.ConsumeExternalAuthSession(ctx, 123, intentRef, descriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	if session.HandoffStateRef != descriptor.HandoffStateRef {
		t.Fatalf("session state ref = %q", session.HandoffStateRef)
	}
	if session.ConsumedAt == "" {
		t.Fatalf("session was not consumed: %#v", session)
	}
	if _, err := store.ConsumeExternalAuthSession(ctx, 123, intentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionConsumed) {
		t.Fatalf("replay consume err = %v", err)
	}
	if _, err := store.ConsumeExternalAuthSession(ctx, 999, intentRef, descriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionNotFound) {
		t.Fatalf("wrong-user consume err = %v", err)
	}

	previewDescriptor, err := store.ReadExternalAuthDescriptor(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.PreviewExternalAuthAuthorization(ctx, 123, intentRef, previewDescriptor.HandoffStateRef)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ProviderSlug != "install-intent-provider-fixture" {
		t.Fatalf("preview provider slug = %q", preview.ProviderSlug)
	}
	if preview.AuthorizationPreviewRef == "" {
		t.Fatalf("preview ref was empty: %#v", preview)
	}
	if len(preview.RequestedScopes) != 1 || preview.RequestedScopes[0] != "calendar.read" {
		t.Fatalf("preview requested scopes = %#v", preview.RequestedScopes)
	}
	if preview.StateRef != previewDescriptor.HandoffStateRef {
		t.Fatalf("preview state ref = %q", preview.StateRef)
	}
	if preview.AuthorizationEndpointID != "install-intent-provider-fixture.authorization.preview" {
		t.Fatalf("preview authorization endpoint id = %q", preview.AuthorizationEndpointID)
	}
	progress, err := store.ReadInstallProgress(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusPreviewRecorded {
		t.Fatalf("preview progress status = %q", progress.ProgressStatus)
	}
	if !progress.AuthorizationPreviewRecorded || progress.AuthorizationPreviewRef != preview.AuthorizationPreviewRef {
		t.Fatalf("preview progress = %#v, preview = %#v", progress, preview)
	}
	if _, err := store.PreviewExternalAuthAuthorization(ctx, 123, intentRef, previewDescriptor.HandoffStateRef); !errors.Is(err, ErrExternalAuthSessionConsumed) {
		t.Fatalf("preview replay err = %v", err)
	}
	callbackPreflight, err := store.PreflightExternalAuthCallback(ctx, 123, intentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	if callbackPreflight.StateRef != preview.StateRef {
		t.Fatalf("callback preflight state ref = %q", callbackPreflight.StateRef)
	}
	if callbackPreflight.ProviderSlug != "install-intent-provider-fixture" {
		t.Fatalf("callback preflight provider slug = %q", callbackPreflight.ProviderSlug)
	}
	if callbackPreflight.CallbackStatus != ExternalAuthCallbackStatusAuthorized {
		t.Fatalf("callback preflight status = %q", callbackPreflight.CallbackStatus)
	}
	if callbackPreflight.PreflightStatus != ExternalAuthCallbackPreflightAccepted {
		t.Fatalf("callback preflight preflight status = %q", callbackPreflight.PreflightStatus)
	}
	if callbackPreflight.ReceivedAt == "" {
		t.Fatalf("callback preflight did not record received time: %#v", callbackPreflight)
	}
	progress, err = store.ReadInstallProgress(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusCallbackPreflight {
		t.Fatalf("callback progress status = %q", progress.ProgressStatus)
	}
	if !progress.CallbackPreflightRecorded || progress.CallbackPreflightRef != callbackPreflight.CallbackPreflightRef {
		t.Fatalf("callback progress = %#v, preflight = %#v", progress, callbackPreflight)
	}
	if progress.NextAction != InstallProgressActionAwaitExchange {
		t.Fatalf("callback progress next action = %q", progress.NextAction)
	}
	exchange, err := store.ExchangeExternalAuthProvider(ctx, 123, intentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             callbackPreflight.StateRef,
		CallbackPreflightRef: callbackPreflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exchange.ProviderExchangeRef == "" {
		t.Fatalf("provider exchange ref was empty: %#v", exchange)
	}
	if exchange.ExchangeStatus != ExternalAuthProviderExchangeQueued {
		t.Fatalf("provider exchange status = %q", exchange.ExchangeStatus)
	}
	if exchange.ExchangeMode != ExternalAuthProviderExchangeModeSimulated {
		t.Fatalf("provider exchange mode = %q", exchange.ExchangeMode)
	}
	progress, err = store.ReadInstallProgress(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusProviderExchangeQueued {
		t.Fatalf("provider exchange progress status = %q", progress.ProgressStatus)
	}
	if !progress.ProviderExchangeRecorded || progress.ProviderExchangeRef != exchange.ProviderExchangeRef {
		t.Fatalf("provider exchange progress = %#v, exchange = %#v", progress, exchange)
	}
	if _, err := store.ExchangeExternalAuthProvider(ctx, 123, intentRef, ExternalAuthProviderExchangeRequest{
		StateRef:             callbackPreflight.StateRef,
		CallbackPreflightRef: callbackPreflight.CallbackPreflightRef,
	}); !errors.Is(err, ErrExternalAuthProviderExchangeRecorded) {
		t.Fatalf("provider exchange replay err = %v", err)
	}
	if _, err := store.PreflightExternalAuthCallback(ctx, 123, intentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       preview.StateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	}); !errors.Is(err, ErrExternalAuthCallbackPreflightConsumed) {
		t.Fatalf("callback preflight replay err = %v", err)
	}
	expiredStateRef := "handoff-state-repository-expired-fixture"
	if _, err := repo.SaveExternalAuthSession(ctx, ExternalAuthSession{
		HandoffStateRef:  expiredStateRef,
		InstallIntentRef: intentRef,
		UserID:           123,
		ExpiresAt:        wireTime(time.Now().Add(-time.Minute)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeExternalAuthSession(ctx, 123, intentRef, expiredStateRef); !errors.Is(err, ErrExternalAuthSessionExpired) {
		t.Fatalf("expired consume err = %v", err)
	}
	if _, err := store.PreflightExternalAuthCallback(ctx, 123, intentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       expiredStateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	}); !errors.Is(err, ErrExternalAuthSessionExpired) {
		t.Fatalf("expired callback preflight err = %v", err)
	}
	completion, err := store.CompleteAppInstall(ctx, 123, intentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if completion.InstallCompletionRef == "" {
		t.Fatalf("install completion ref was empty: %#v", completion)
	}
	if completion.CompletionStatus != AppInstallCompletionStatusInstalled {
		t.Fatalf("install completion status = %q", completion.CompletionStatus)
	}
	if completion.CompletionMode != AppInstallCompletionModeSimulated {
		t.Fatalf("install completion mode = %q", completion.CompletionMode)
	}
	progress, err = store.ReadInstallProgress(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.InstallStatus != InstallIntentStatusInstalled {
		t.Fatalf("completed install status = %q", progress.InstallStatus)
	}
	if progress.ProgressStatus != InstallProgressStatusInstalled {
		t.Fatalf("completed progress status = %q", progress.ProgressStatus)
	}
	if !progress.InstallCompletionRecorded || progress.InstallCompletionRef != completion.InstallCompletionRef {
		t.Fatalf("install completion progress = %#v, completion = %#v", progress, completion)
	}
	if _, err := store.CompleteAppInstall(ctx, 123, intentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: exchange.ProviderExchangeRef,
	}); !errors.Is(err, ErrAppInstallCompletionRecorded) {
		t.Fatalf("install completion replay err = %v", err)
	}
	if _, err := store.ActivateAppInstallation(ctx, 999, intentRef, AppInstallationActivationRequest{
		InstallCompletionRef: completion.InstallCompletionRef,
	}); !errors.Is(err, ErrInstallIntentNotFound) {
		t.Fatalf("wrong-user app installation activation err = %v", err)
	}
	installation, err := store.ActivateAppInstallation(ctx, 123, intentRef, AppInstallationActivationRequest{
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
	if installation.ActivationStatus != AppInstallationStatusActive {
		t.Fatalf("activation status = %q", installation.ActivationStatus)
	}
	if installation.ActivationMode != AppInstallationModeSimulated {
		t.Fatalf("activation mode = %q", installation.ActivationMode)
	}
	installations, err := store.ReadAppInstallations(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(installations) != 1 || installations[0].AppInstallationRef != installation.AppInstallationRef {
		t.Fatalf("app installations = %#v", installations)
	}
	otherInstallations, err := store.ReadAppInstallations(ctx, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherInstallations) != 0 {
		t.Fatalf("wrong-user app installations leaked: %#v", otherInstallations)
	}
	progress, err = store.ReadInstallProgress(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusActive {
		t.Fatalf("active progress status = %q", progress.ProgressStatus)
	}
	if !progress.AppInstallationRecorded || progress.AppInstallationRef != installation.AppInstallationRef {
		t.Fatalf("app installation progress = %#v, installation = %#v", progress, installation)
	}
	if _, err := store.ActivateAppInstallation(ctx, 123, intentRef, AppInstallationActivationRequest{
		InstallCompletionRef: completion.InstallCompletionRef,
	}); !errors.Is(err, ErrAppInstallationRecorded) {
		t.Fatalf("app installation replay err = %v", err)
	}
}

func TestPostgresRepositoryRoundTripProviderOAuthCallbackModes(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	slug := "app-install-provider-oauth-repository-fixture"
	intentRef := "app-intent-provider-oauth-repository-fixture"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_install_intents where install_intent_ref = $1`, intentRef)
		_, _ = pool.Exec(cleanupCtx, `delete from integration_app_catalog where app_slug = $1`, slug)
	})

	if _, err := repo.SaveAppMetadata(ctx, AppMetadata{
		AppSlug:      slug,
		Category:     "calendar",
		Provider:     "provider-oauth-provider-fixture",
		Name:         "Provider OAuth Fixture",
		Description:  "Provider OAuth app catalog fixture.",
		AuthType:     "oauth",
		Capabilities: []string{"calendar.read"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveInstallIntent(ctx, AppInstallIntent{
		InstallIntentRef: intentRef,
		UserID:           123,
		AppSlug:          slug,
		Status:           InstallIntentStatusPending,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.MarkInstallIntentRequiresExternalAuth(ctx, 123, intentRef); err != nil {
		t.Fatal(err)
	}

	oauthPort := &fakeProviderOAuthExchangePort{
		result: ProviderOAuthExchangeResult{
			AccountRef:   "provider-oauth-account-repository-fixture",
			AccountLabel: "provider-oauth@example.test",
			Scopes:       []string{"calendar.read"},
			TokenPayload: ProviderTokenPayload{
				AccessToken:  "provider-access-token-secret-fixture",
				RefreshToken: "provider-refresh-token-secret-fixture",
			},
		},
	}
	store := NewStoreWithRepository(
		repo,
		WithProviderOAuthExchangePort(oauthPort),
		WithProviderCredentialStore(&fakeProviderCredentialStore{
			receipt: ProviderCredentialReceipt{
				CredentialRef: "credential-provider-oauth-repository-fixture",
				AccountRef:    "provider-oauth-account-repository-fixture",
				AccountLabel:  "provider-oauth@example.test",
				Status:        "active",
				Scopes:        []string{"calendar.read"},
			},
		}),
	)
	descriptor, err := store.ReadExternalAuthDescriptor(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := store.PreflightExternalAuthCallback(ctx, 123, intentRef, ExternalAuthCallbackPreflightRequest{
		StateRef:       descriptor.HandoffStateRef,
		CallbackStatus: ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.HandleProviderOAuthCallback(ctx, ProviderOAuthCallbackRequest{
		StateRef:          descriptor.HandoffStateRef,
		AuthorizationCode: "provider-code-secret-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CallbackPreflightRef != preflight.CallbackPreflightRef {
		t.Fatalf("callback preflight ref = %q", result.CallbackPreflightRef)
	}
	if result.ExchangeStatus != ExternalAuthProviderExchangeCredentialReady {
		t.Fatalf("provider exchange status = %q", result.ExchangeStatus)
	}
	if result.ExchangeMode != ExternalAuthProviderExchangeModeProviderOAuth {
		t.Fatalf("provider exchange mode = %q", result.ExchangeMode)
	}
	progress, err := store.ReadInstallProgress(ctx, 123, intentRef)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ProgressStatus != InstallProgressStatusProviderCredentialReady {
		t.Fatalf("provider credential progress status = %q", progress.ProgressStatus)
	}

	completion, err := store.CompleteAppInstall(ctx, 123, intentRef, AppInstallCompletionRequest{
		ProviderExchangeRef: result.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if completion.CompletionMode != AppInstallCompletionModeProviderOAuth {
		t.Fatalf("completion mode = %q", completion.CompletionMode)
	}
	installation, err := store.ActivateAppInstallation(ctx, 123, intentRef, AppInstallationActivationRequest{
		InstallCompletionRef: completion.InstallCompletionRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if installation.ActivationMode != AppInstallationModeProviderOAuth {
		t.Fatalf("activation mode = %q", installation.ActivationMode)
	}
}

func findInstallIntent(items []AppInstallIntent, installIntentRef string) (AppInstallIntent, bool) {
	for _, item := range items {
		if item.InstallIntentRef == installIntentRef {
			return item, true
		}
	}
	return AppInstallIntent{}, false
}

func TestPostgresAppCatalogTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_catalog'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw_response", "error_body"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app catalog table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallIntentTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_intents'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw_response", "error_body"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install intent table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallHandoffSessionTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_handoff_sessions'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw_response", "error_body", "redirect", "callback"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install handoff session table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallAuthorizationPreviewTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_authorization_previews'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw", "query", "response", "error", "code", "redirect", "account", "url"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install authorization preview table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallCallbackPreflightTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_callback_preflights'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw", "query", "response", "error", "code", "redirect", "account"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install callback preflight table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallProviderExchangeTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_provider_exchanges'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw", "query", "response", "error", "code", "redirect", "account"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install provider exchange table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallCompletionTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_install_completions'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw", "query", "response", "error", "code", "redirect", "account"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app install completion table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppInstallationTableHasNoSecretColumns(t *testing.T) {
	pool := testPostgresPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		select column_name
		from information_schema.columns
		where table_name = 'integration_app_installations'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		lowerColumn := strings.ToLower(column)
		for _, forbidden := range []string{"secret", "token", "encrypted", "credential", "payload", "raw", "query", "response", "error", "code", "redirect", "account"} {
			if strings.Contains(lowerColumn, forbidden) {
				t.Fatalf("app installation table has secret-like column %q", column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func testPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("CALDIY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("CALDIY_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set CALDIY_TEST_DATABASE_URL or CALDIY_DATABASE_URL to run Postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
