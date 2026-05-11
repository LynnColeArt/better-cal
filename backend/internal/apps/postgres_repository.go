package apps

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const wireTimeLayout = "2006-01-02T15:04:05.000Z"

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadAppCatalog(ctx context.Context) ([]AppMetadata, error) {
	rows, err := r.pool.Query(ctx, `
		select app_slug, app_category, provider, name, description, auth_type, capabilities, created_at, updated_at
		from integration_app_catalog
		order by app_slug
	`)
	if err != nil {
		return nil, fmt.Errorf("read app catalog: %w", err)
	}
	defer rows.Close()

	catalog := []AppMetadata{}
	for rows.Next() {
		var app AppMetadata
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(
			&app.AppSlug,
			&app.Category,
			&app.Provider,
			&app.Name,
			&app.Description,
			&app.AuthType,
			&app.Capabilities,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan app catalog: %w", err)
		}
		app.CreatedAt = formatWireTime(createdAt)
		app.UpdatedAt = formatWireTime(updatedAt)
		catalog = append(catalog, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read app catalog rows: %w", err)
	}
	return catalog, nil
}

func (r *PostgresRepository) ReadInstallIntents(ctx context.Context, userID int) ([]AppInstallIntent, error) {
	if userID <= 0 {
		return nil, ErrInvalidInstallIntent
	}
	rows, err := r.pool.Query(ctx, `
		select install_intent_ref, user_id, app_slug, status, created_at, updated_at
		from integration_app_install_intents
		where user_id = $1
		order by created_at, install_intent_ref
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("read app install intents: %w", err)
	}
	defer rows.Close()

	intents := []AppInstallIntent{}
	for rows.Next() {
		intent, err := scanInstallIntentRow(rows)
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read app install intent rows: %w", err)
	}
	return intents, nil
}

func (r *PostgresRepository) ReadAppInstallations(ctx context.Context, userID int) ([]AppInstallation, error) {
	if userID <= 0 {
		return nil, ErrInvalidAppInstallation
	}
	rows, err := r.pool.Query(ctx, `
		select i.app_installation_ref, i.install_intent_ref, i.user_id, i.app_slug, c.name, c.app_category,
			i.provider, c.auth_type, c.capabilities, i.install_completion_ref, i.activation_status,
			i.activation_mode, i.activated_at, i.created_at, i.updated_at
		from integration_app_installations i
		join integration_app_catalog c on c.app_slug = i.app_slug
		where i.user_id = $1
		order by i.activated_at desc, i.app_installation_ref
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("read app installations: %w", err)
	}
	defer rows.Close()

	installations := []AppInstallation{}
	for rows.Next() {
		installation, err := scanAppInstallationRow(rows)
		if err != nil {
			return nil, err
		}
		installations = append(installations, installation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read app installation rows: %w", err)
	}
	return installations, nil
}

func (r *PostgresRepository) ReadInstallProgress(ctx context.Context, userID int, installIntentRef string, observedAt time.Time) (AppInstallProgress, error) {
	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" {
		return AppInstallProgress{}, ErrInvalidInstallIntent
	}

	var intent AppInstallIntent
	var app AppMetadata
	var intentCreatedAt time.Time
	var intentUpdatedAt time.Time
	err := r.pool.QueryRow(ctx, `
		select i.install_intent_ref, i.user_id, i.app_slug, i.status, i.created_at, i.updated_at,
			c.provider, c.name, c.auth_type
		from integration_app_install_intents i
		join integration_app_catalog c on c.app_slug = i.app_slug
		where i.install_intent_ref = $1
			and i.user_id = $2
	`, installIntentRef, userID).Scan(
		&intent.InstallIntentRef,
		&intent.UserID,
		&intent.AppSlug,
		&intent.Status,
		&intentCreatedAt,
		&intentUpdatedAt,
		&app.Provider,
		&app.Name,
		&app.AuthType,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallProgress{}, ErrInstallIntentNotFound
		}
		return AppInstallProgress{}, fmt.Errorf("read app install progress intent: %w", err)
	}
	intent.CreatedAt = formatWireTime(intentCreatedAt)
	intent.UpdatedAt = formatWireTime(intentUpdatedAt)
	app.AppSlug = intent.AppSlug

	session, hasSession, err := r.readLatestExternalAuthSession(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallProgress{}, err
	}
	preview, hasPreview, err := r.readLatestExternalAuthAuthorizationPreview(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallProgress{}, err
	}
	preflight, hasPreflight, err := r.readLatestExternalAuthCallbackPreflight(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallProgress{}, err
	}
	exchange, hasExchange, err := r.readLatestExternalAuthProviderExchange(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallProgress{}, err
	}
	completion, hasCompletion, err := r.readLatestAppInstallCompletion(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallProgress{}, err
	}
	installation, hasInstallation, err := r.readLatestAppInstallation(ctx, userID, installIntentRef)
	if err != nil {
		return AppInstallProgress{}, err
	}
	return newAppInstallProgress(
		intent,
		app,
		optionalExternalAuthSession(session, hasSession),
		optionalExternalAuthAuthorizationPreview(preview, hasPreview),
		optionalExternalAuthCallbackPreflight(preflight, hasPreflight),
		optionalExternalAuthProviderExchange(exchange, hasExchange),
		optionalAppInstallCompletion(completion, hasCompletion),
		optionalAppInstallation(installation, hasInstallation),
		observedAt,
	), nil
}

func (r *PostgresRepository) ReadExternalAuthCallbackPreflight(ctx context.Context, userID int, installIntentRef string, stateRef string, callbackPreflightRef string) (ExternalAuthCallbackPreflight, error) {
	if userID <= 0 ||
		strings.TrimSpace(installIntentRef) == "" ||
		strings.TrimSpace(stateRef) == "" ||
		strings.TrimSpace(callbackPreflightRef) == "" {
		return ExternalAuthCallbackPreflight{}, ErrInvalidExternalAuthCallbackPreflight
	}
	preflight, err := scanExternalAuthCallbackPreflightRow(r.pool.QueryRow(ctx, `
		select callback_preflight_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_status, preflight_status, received_at, created_at, updated_at
		from integration_app_install_callback_preflights
		where callback_preflight_ref = $1
			and handoff_state_ref = $2
			and install_intent_ref = $3
			and user_id = $4
	`, callbackPreflightRef, stateRef, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthCallbackPreflight{}, ErrExternalAuthCallbackPreflightNotFound
		}
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("read external auth callback preflight: %w", err)
	}
	return preflight, nil
}

func (r *PostgresRepository) ReadExternalAuthCallbackPreflightByState(ctx context.Context, stateRef string) (ExternalAuthCallbackPreflight, error) {
	if strings.TrimSpace(stateRef) == "" {
		return ExternalAuthCallbackPreflight{}, ErrInvalidExternalAuthCallbackPreflight
	}
	preflight, err := scanExternalAuthCallbackPreflightRow(r.pool.QueryRow(ctx, `
		select callback_preflight_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_status, preflight_status, received_at, created_at, updated_at
		from integration_app_install_callback_preflights
		where handoff_state_ref = $1
	`, stateRef))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthCallbackPreflight{}, ErrExternalAuthCallbackPreflightNotFound
		}
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("read external auth callback preflight by state: %w", err)
	}
	return preflight, nil
}

func (r *PostgresRepository) ReadExternalAuthProviderExchangeByPreflight(ctx context.Context, userID int, installIntentRef string, callbackPreflightRef string) (ExternalAuthProviderExchange, error) {
	if userID <= 0 ||
		strings.TrimSpace(installIntentRef) == "" ||
		strings.TrimSpace(callbackPreflightRef) == "" {
		return ExternalAuthProviderExchange{}, ErrInvalidExternalAuthProviderExchange
	}
	exchange, err := scanExternalAuthProviderExchangeRow(r.pool.QueryRow(ctx, `
		select provider_exchange_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_preflight_ref, exchange_status, exchange_mode, recorded_at, created_at, updated_at
		from integration_app_install_provider_exchanges
		where callback_preflight_ref = $1
			and install_intent_ref = $2
			and user_id = $3
	`, callbackPreflightRef, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthProviderExchange{}, ErrExternalAuthProviderExchangeNotFound
		}
		return ExternalAuthProviderExchange{}, fmt.Errorf("read external auth provider exchange by preflight: %w", err)
	}
	return exchange, nil
}

func (r *PostgresRepository) SaveAppMetadata(ctx context.Context, app AppMetadata) (AppMetadata, error) {
	if err := ValidateAppMetadata(app); err != nil {
		return AppMetadata{}, err
	}
	capabilities := app.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}

	if _, err := r.pool.Exec(ctx, `
		insert into integration_app_catalog (
			app_slug,
			app_category,
			provider,
			name,
			description,
			auth_type,
			capabilities
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (app_slug) do update set
			app_category = excluded.app_category,
			provider = excluded.provider,
			name = excluded.name,
			description = excluded.description,
			auth_type = excluded.auth_type,
			capabilities = excluded.capabilities,
			updated_at = now()
	`, app.AppSlug, app.Category, app.Provider, app.Name, app.Description, app.AuthType, capabilities); err != nil {
		return AppMetadata{}, fmt.Errorf("save app metadata: %w", err)
	}

	items, err := r.ReadAppCatalog(ctx)
	if err != nil {
		return AppMetadata{}, err
	}
	for _, item := range items {
		if item.AppSlug == app.AppSlug {
			return item, nil
		}
	}
	return AppMetadata{}, fmt.Errorf("saved app metadata %q was not found", app.AppSlug)
}

func (r *PostgresRepository) SaveInstallIntent(ctx context.Context, intent AppInstallIntent) (AppInstallIntent, error) {
	if err := ValidateInstallIntent(intent); err != nil {
		return AppInstallIntent{}, err
	}

	saved, err := scanInstallIntentRow(r.pool.QueryRow(ctx, `
		insert into integration_app_install_intents (
			install_intent_ref,
			user_id,
			app_slug,
			status
		)
		values ($1, $2, $3, $4)
		returning install_intent_ref, user_id, app_slug, status, created_at, updated_at
	`, intent.InstallIntentRef, intent.UserID, intent.AppSlug, intent.Status))
	if err != nil {
		return AppInstallIntent{}, fmt.Errorf("save app install intent: %w", err)
	}
	return saved, nil
}

func (r *PostgresRepository) MarkInstallIntentRequiresExternalAuth(ctx context.Context, userID int, installIntentRef string) (AppInstallIntent, error) {
	if userID <= 0 || installIntentRef == "" {
		return AppInstallIntent{}, ErrInvalidInstallIntent
	}
	intent, err := scanInstallIntentRow(r.pool.QueryRow(ctx, `
		update integration_app_install_intents
		set status = $3,
			updated_at = now()
		where install_intent_ref = $1
			and user_id = $2
			and status in ('pending', 'requires_external_auth')
		returning install_intent_ref, user_id, app_slug, status, created_at, updated_at
	`, installIntentRef, userID, InstallIntentStatusRequiresExternalAuth))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, readErr := scanInstallIntentRow(r.pool.QueryRow(ctx, `
				select install_intent_ref, user_id, app_slug, status, created_at, updated_at
				from integration_app_install_intents
				where install_intent_ref = $1
					and user_id = $2
			`, installIntentRef, userID))
			if readErr == nil && (existing.Status == InstallIntentStatusInstalled || existing.Status == InstallIntentStatusBlocked) {
				return AppInstallIntent{}, ErrInstallIntentNotReady
			}
			if readErr != nil && !errors.Is(readErr, pgx.ErrNoRows) {
				return AppInstallIntent{}, fmt.Errorf("read app install intent after external auth transition miss: %w", readErr)
			}
			return AppInstallIntent{}, ErrInstallIntentNotFound
		}
		return AppInstallIntent{}, fmt.Errorf("mark app install intent external auth: %w", err)
	}
	return intent, nil
}

func (r *PostgresRepository) SaveExternalAuthSession(ctx context.Context, session ExternalAuthSession) (ExternalAuthSession, error) {
	if strings.TrimSpace(session.HandoffStateRef) == "" ||
		strings.TrimSpace(session.InstallIntentRef) == "" ||
		session.UserID <= 0 ||
		strings.TrimSpace(session.ExpiresAt) == "" {
		return ExternalAuthSession{}, ErrInvalidInstallIntent
	}
	expiresAt, err := parseWireTime(session.ExpiresAt)
	if err != nil {
		return ExternalAuthSession{}, ErrInvalidInstallIntent
	}

	saved, err := scanExternalAuthSessionRow(r.pool.QueryRow(ctx, `
		insert into integration_app_install_handoff_sessions (
			handoff_state_ref,
			install_intent_ref,
			user_id,
			expires_at
		)
		values ($1, $2, $3, $4)
		returning handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
	`, session.HandoffStateRef, session.InstallIntentRef, session.UserID, expiresAt))
	if err != nil {
		return ExternalAuthSession{}, fmt.Errorf("save external auth handoff session: %w", err)
	}
	return saved, nil
}

func (r *PostgresRepository) ConsumeExternalAuthSession(ctx context.Context, userID int, installIntentRef string, handoffStateRef string, consumedAt time.Time) (ExternalAuthSession, error) {
	if userID <= 0 || strings.TrimSpace(installIntentRef) == "" || strings.TrimSpace(handoffStateRef) == "" {
		return ExternalAuthSession{}, ErrInvalidInstallIntent
	}
	consumedAt = consumedAt.UTC()
	session, err := scanExternalAuthSessionRow(r.pool.QueryRow(ctx, `
		update integration_app_install_handoff_sessions
		set consumed_at = $4,
			updated_at = now()
		where handoff_state_ref = $1
			and install_intent_ref = $2
			and user_id = $3
			and consumed_at is null
			and expires_at > $4
		returning handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
	`, handoffStateRef, installIntentRef, userID, consumedAt))
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ExternalAuthSession{}, fmt.Errorf("consume external auth handoff session: %w", err)
	}

	existing, readErr := scanExternalAuthSessionRow(r.pool.QueryRow(ctx, `
		select handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
		from integration_app_install_handoff_sessions
		where handoff_state_ref = $1
			and install_intent_ref = $2
			and user_id = $3
	`, handoffStateRef, installIntentRef, userID))
	if readErr != nil {
		if errors.Is(readErr, pgx.ErrNoRows) {
			return ExternalAuthSession{}, ErrExternalAuthSessionNotFound
		}
		return ExternalAuthSession{}, fmt.Errorf("read external auth handoff session: %w", readErr)
	}
	if existing.ConsumedAt != "" {
		return ExternalAuthSession{}, ErrExternalAuthSessionConsumed
	}
	expiresAt, err := parseWireTime(existing.ExpiresAt)
	if err != nil {
		return ExternalAuthSession{}, fmt.Errorf("parse external auth handoff session expiry: %w", err)
	}
	if !expiresAt.After(consumedAt) {
		return ExternalAuthSession{}, ErrExternalAuthSessionExpired
	}
	return ExternalAuthSession{}, ErrExternalAuthSessionNotFound
}

func (r *PostgresRepository) SaveExternalAuthAuthorizationPreview(ctx context.Context, intent AppInstallIntent, app AppMetadata, handoffStateRef string, requestedAt time.Time) (ExternalAuthAuthorizationPreview, error) {
	if intent.UserID <= 0 ||
		strings.TrimSpace(intent.InstallIntentRef) == "" ||
		strings.TrimSpace(intent.AppSlug) == "" ||
		strings.TrimSpace(app.Provider) == "" ||
		strings.TrimSpace(app.AuthType) == "" ||
		strings.TrimSpace(handoffStateRef) == "" {
		return ExternalAuthAuthorizationPreview{}, ErrInvalidInstallIntent
	}

	requestedAt = requestedAt.UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ExternalAuthAuthorizationPreview{}, fmt.Errorf("begin external auth authorization preview: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	session, err := scanExternalAuthSessionRow(tx.QueryRow(ctx, `
		update integration_app_install_handoff_sessions
		set consumed_at = $4,
			updated_at = now()
		where handoff_state_ref = $1
			and install_intent_ref = $2
			and user_id = $3
			and consumed_at is null
			and expires_at > $4
		returning handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
	`, handoffStateRef, intent.InstallIntentRef, intent.UserID, requestedAt))
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthAuthorizationPreview{}, fmt.Errorf("consume external auth authorization preview session: %w", err)
		}
		existing, readErr := scanExternalAuthSessionRow(tx.QueryRow(ctx, `
			select handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
			from integration_app_install_handoff_sessions
			where handoff_state_ref = $1
				and install_intent_ref = $2
				and user_id = $3
		`, handoffStateRef, intent.InstallIntentRef, intent.UserID))
		if readErr != nil {
			if errors.Is(readErr, pgx.ErrNoRows) {
				return ExternalAuthAuthorizationPreview{}, ErrExternalAuthSessionNotFound
			}
			return ExternalAuthAuthorizationPreview{}, fmt.Errorf("read external auth authorization preview session: %w", readErr)
		}
		if existing.ConsumedAt != "" {
			return ExternalAuthAuthorizationPreview{}, ErrExternalAuthSessionConsumed
		}
		expiresAt, parseErr := parseWireTime(existing.ExpiresAt)
		if parseErr != nil {
			return ExternalAuthAuthorizationPreview{}, fmt.Errorf("parse external auth authorization preview expiry: %w", parseErr)
		}
		if !expiresAt.After(requestedAt) {
			return ExternalAuthAuthorizationPreview{}, ErrExternalAuthSessionExpired
		}
		return ExternalAuthAuthorizationPreview{}, ErrExternalAuthSessionNotFound
	}

	preview := newExternalAuthAuthorizationPreview(intent, app, session, requestedAt)
	var createdAt time.Time
	var updatedAt time.Time
	if err := tx.QueryRow(ctx, `
		insert into integration_app_install_authorization_previews (
			authorization_preview_ref,
			handoff_state_ref,
			install_intent_ref,
			user_id,
			app_slug,
			provider,
			auth_type,
			requested_scopes,
			preview_status,
			requested_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		on conflict (handoff_state_ref) do nothing
		returning created_at, updated_at
	`,
		preview.AuthorizationPreviewRef,
		preview.StateRef,
		preview.InstallIntentRef,
		preview.UserID,
		preview.AppSlug,
		preview.ProviderSlug,
		preview.AuthType,
		preview.RequestedScopes,
		preview.Status,
		requestedAt,
	).Scan(&createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthAuthorizationPreview{}, ErrExternalAuthSessionConsumed
		}
		return ExternalAuthAuthorizationPreview{}, fmt.Errorf("save external auth authorization preview: %w", err)
	}
	preview.CreatedAt = formatWireTime(createdAt)
	preview.UpdatedAt = formatWireTime(updatedAt)

	if err := tx.Commit(ctx); err != nil {
		return ExternalAuthAuthorizationPreview{}, fmt.Errorf("commit external auth authorization preview: %w", err)
	}
	return preview, nil
}

func (r *PostgresRepository) SaveExternalAuthCallbackPreflight(ctx context.Context, preflight ExternalAuthCallbackPreflight, receivedAt time.Time) (ExternalAuthCallbackPreflight, error) {
	if strings.TrimSpace(preflight.CallbackPreflightRef) == "" ||
		strings.TrimSpace(preflight.InstallIntentRef) == "" ||
		strings.TrimSpace(preflight.StateRef) == "" ||
		strings.TrimSpace(preflight.AppSlug) == "" ||
		strings.TrimSpace(preflight.ProviderSlug) == "" ||
		preflight.UserID <= 0 ||
		!isValidExternalAuthCallbackStatus(preflight.CallbackStatus) {
		return ExternalAuthCallbackPreflight{}, ErrInvalidExternalAuthCallbackPreflight
	}

	receivedAt = receivedAt.UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("begin external auth callback preflight: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	session, err := scanExternalAuthSessionRow(tx.QueryRow(ctx, `
		select handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
		from integration_app_install_handoff_sessions
		where handoff_state_ref = $1
			and install_intent_ref = $2
			and user_id = $3
		for update
	`, preflight.StateRef, preflight.InstallIntentRef, preflight.UserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthCallbackPreflight{}, ErrExternalAuthSessionNotFound
		}
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("read external auth callback preflight session: %w", err)
	}
	expiresAt, err := parseWireTime(session.ExpiresAt)
	if err != nil {
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("parse external auth callback preflight expiry: %w", err)
	}
	if !expiresAt.After(receivedAt) {
		return ExternalAuthCallbackPreflight{}, ErrExternalAuthSessionExpired
	}

	saved, err := scanExternalAuthCallbackPreflightRow(tx.QueryRow(ctx, `
		insert into integration_app_install_callback_preflights (
			callback_preflight_ref,
			handoff_state_ref,
			install_intent_ref,
			user_id,
			app_slug,
			provider,
			callback_status,
			preflight_status,
			received_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (handoff_state_ref) do nothing
		returning callback_preflight_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref, callback_status, preflight_status, received_at, created_at, updated_at
	`,
		preflight.CallbackPreflightRef,
		preflight.StateRef,
		preflight.InstallIntentRef,
		preflight.UserID,
		preflight.AppSlug,
		preflight.ProviderSlug,
		preflight.CallbackStatus,
		preflight.PreflightStatus,
		receivedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthCallbackPreflight{}, ErrExternalAuthCallbackPreflightConsumed
		}
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("save external auth callback preflight: %w", err)
	}

	if session.ConsumedAt == "" {
		if _, err := tx.Exec(ctx, `
			update integration_app_install_handoff_sessions
			set consumed_at = $4,
				updated_at = now()
			where handoff_state_ref = $1
				and install_intent_ref = $2
				and user_id = $3
		`, preflight.StateRef, preflight.InstallIntentRef, preflight.UserID, receivedAt); err != nil {
			return ExternalAuthCallbackPreflight{}, fmt.Errorf("mark external auth callback preflight session consumed: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("commit external auth callback preflight: %w", err)
	}
	return saved, nil
}

func (r *PostgresRepository) SaveExternalAuthProviderExchange(ctx context.Context, exchange ExternalAuthProviderExchange, recordedAt time.Time) (ExternalAuthProviderExchange, error) {
	if strings.TrimSpace(exchange.ProviderExchangeRef) == "" ||
		strings.TrimSpace(exchange.InstallIntentRef) == "" ||
		strings.TrimSpace(exchange.StateRef) == "" ||
		strings.TrimSpace(exchange.CallbackPreflightRef) == "" ||
		strings.TrimSpace(exchange.AppSlug) == "" ||
		strings.TrimSpace(exchange.ProviderSlug) == "" ||
		exchange.UserID <= 0 ||
		!isValidExternalAuthProviderExchangeMode(exchange.ExchangeMode) ||
		(exchange.ExchangeStatus != "" && !isValidExternalAuthProviderExchangeStatus(exchange.ExchangeStatus)) {
		return ExternalAuthProviderExchange{}, ErrInvalidExternalAuthProviderExchange
	}

	recordedAt = recordedAt.UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ExternalAuthProviderExchange{}, fmt.Errorf("begin external auth provider exchange: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	preflight, err := scanExternalAuthCallbackPreflightRow(tx.QueryRow(ctx, `
		select callback_preflight_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_status, preflight_status, received_at, created_at, updated_at
		from integration_app_install_callback_preflights
		where callback_preflight_ref = $1
			and handoff_state_ref = $2
			and install_intent_ref = $3
			and user_id = $4
		for update
	`, exchange.CallbackPreflightRef, exchange.StateRef, exchange.InstallIntentRef, exchange.UserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthProviderExchange{}, ErrExternalAuthCallbackPreflightNotFound
		}
		return ExternalAuthProviderExchange{}, fmt.Errorf("read external auth provider exchange preflight: %w", err)
	}
	if preflight.AppSlug != exchange.AppSlug || preflight.ProviderSlug != exchange.ProviderSlug {
		return ExternalAuthProviderExchange{}, ErrInvalidExternalAuthProviderExchange
	}

	expectedStatus := ExternalAuthProviderExchangeQueued
	if preflight.CallbackStatus == ExternalAuthCallbackStatusDenied {
		expectedStatus = ExternalAuthProviderExchangeBlocked
	}
	if exchange.ExchangeMode == ExternalAuthProviderExchangeModeProviderOAuth {
		if preflight.CallbackStatus != ExternalAuthCallbackStatusAuthorized {
			return ExternalAuthProviderExchange{}, ErrProviderOAuthExchangeDenied
		}
		expectedStatus = ExternalAuthProviderExchangeCredentialReady
	}
	if exchange.ExchangeStatus != "" && exchange.ExchangeStatus != expectedStatus {
		return ExternalAuthProviderExchange{}, ErrInvalidExternalAuthProviderExchange
	}
	exchange.ExchangeStatus = expectedStatus

	saved, err := scanExternalAuthProviderExchangeRow(tx.QueryRow(ctx, `
		insert into integration_app_install_provider_exchanges (
			provider_exchange_ref,
			callback_preflight_ref,
			handoff_state_ref,
			install_intent_ref,
			user_id,
			app_slug,
			provider,
			exchange_status,
			exchange_mode,
			recorded_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		on conflict (callback_preflight_ref) do nothing
		returning provider_exchange_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_preflight_ref, exchange_status, exchange_mode, recorded_at, created_at, updated_at
	`,
		exchange.ProviderExchangeRef,
		exchange.CallbackPreflightRef,
		exchange.StateRef,
		exchange.InstallIntentRef,
		exchange.UserID,
		exchange.AppSlug,
		exchange.ProviderSlug,
		exchange.ExchangeStatus,
		exchange.ExchangeMode,
		recordedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthProviderExchange{}, ErrExternalAuthProviderExchangeRecorded
		}
		return ExternalAuthProviderExchange{}, fmt.Errorf("save external auth provider exchange: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ExternalAuthProviderExchange{}, fmt.Errorf("commit external auth provider exchange: %w", err)
	}
	return saved, nil
}

func (r *PostgresRepository) SaveAppInstallCompletion(ctx context.Context, completion AppInstallCompletion, completedAt time.Time) (AppInstallCompletion, error) {
	if strings.TrimSpace(completion.InstallCompletionRef) == "" ||
		strings.TrimSpace(completion.InstallIntentRef) == "" ||
		strings.TrimSpace(completion.ProviderExchangeRef) == "" ||
		strings.TrimSpace(completion.AppSlug) == "" ||
		strings.TrimSpace(completion.ProviderSlug) == "" ||
		completion.UserID <= 0 ||
		(completion.CompletionMode != "" && !isValidAppInstallCompletionMode(completion.CompletionMode)) ||
		(completion.CompletionStatus != "" && completion.CompletionStatus != AppInstallCompletionStatusInstalled && completion.CompletionStatus != AppInstallCompletionStatusBlocked) {
		return AppInstallCompletion{}, ErrInvalidAppInstallCompletion
	}

	completedAt = completedAt.UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AppInstallCompletion{}, fmt.Errorf("begin app install completion: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var intent AppInstallIntent
	var exchange ExternalAuthProviderExchange
	var intentCreatedAt time.Time
	var intentUpdatedAt time.Time
	var exchangeRecordedAt time.Time
	var exchangeCreatedAt time.Time
	var exchangeUpdatedAt time.Time
	err = tx.QueryRow(ctx, `
		select i.install_intent_ref, i.user_id, i.app_slug, i.status, i.created_at, i.updated_at,
			e.provider_exchange_ref, e.provider, e.handoff_state_ref, e.callback_preflight_ref,
			e.exchange_status, e.exchange_mode, e.recorded_at, e.created_at, e.updated_at
		from integration_app_install_provider_exchanges e
		join integration_app_install_intents i on i.install_intent_ref = e.install_intent_ref
		where e.provider_exchange_ref = $1
			and e.install_intent_ref = $2
			and e.user_id = $3
		for update of e, i
	`, completion.ProviderExchangeRef, completion.InstallIntentRef, completion.UserID).Scan(
		&intent.InstallIntentRef,
		&intent.UserID,
		&intent.AppSlug,
		&intent.Status,
		&intentCreatedAt,
		&intentUpdatedAt,
		&exchange.ProviderExchangeRef,
		&exchange.ProviderSlug,
		&exchange.StateRef,
		&exchange.CallbackPreflightRef,
		&exchange.ExchangeStatus,
		&exchange.ExchangeMode,
		&exchangeRecordedAt,
		&exchangeCreatedAt,
		&exchangeUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallCompletion{}, ErrExternalAuthProviderExchangeNotFound
		}
		return AppInstallCompletion{}, fmt.Errorf("read app install completion provider exchange: %w", err)
	}
	intent.CreatedAt = formatWireTime(intentCreatedAt)
	intent.UpdatedAt = formatWireTime(intentUpdatedAt)
	exchange.InstallIntentRef = intent.InstallIntentRef
	exchange.UserID = intent.UserID
	exchange.AppSlug = intent.AppSlug
	exchange.RecordedAt = formatWireTime(exchangeRecordedAt)
	exchange.CreatedAt = formatWireTime(exchangeCreatedAt)
	exchange.UpdatedAt = formatWireTime(exchangeUpdatedAt)
	if intent.AppSlug != completion.AppSlug || exchange.ProviderSlug != completion.ProviderSlug {
		return AppInstallCompletion{}, ErrInvalidAppInstallCompletion
	}

	existingCompletion, err := scanAppInstallCompletionRow(tx.QueryRow(ctx, `
		select install_completion_ref, install_intent_ref, user_id, app_slug, provider, provider_exchange_ref,
			completion_status, completion_mode, completed_at, created_at, updated_at
		from integration_app_install_completions
		where provider_exchange_ref = $1
			or (install_intent_ref = $2 and user_id = $3)
		limit 1
	`, completion.ProviderExchangeRef, completion.InstallIntentRef, completion.UserID))
	if err == nil && existingCompletion.InstallCompletionRef != "" {
		return AppInstallCompletion{}, ErrAppInstallCompletionRecorded
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return AppInstallCompletion{}, fmt.Errorf("read existing app install completion: %w", err)
	}
	if intent.Status != InstallIntentStatusRequiresExternalAuth {
		return AppInstallCompletion{}, ErrInstallIntentNotReady
	}

	expectedStatus := AppInstallCompletionStatusInstalled
	if exchange.ExchangeStatus == ExternalAuthProviderExchangeBlocked {
		expectedStatus = AppInstallCompletionStatusBlocked
	}
	if completion.CompletionStatus != "" && completion.CompletionStatus != expectedStatus {
		return AppInstallCompletion{}, ErrInvalidAppInstallCompletion
	}
	completion.CompletionStatus = expectedStatus
	expectedMode := AppInstallCompletionModeSimulated
	if exchange.ExchangeMode == ExternalAuthProviderExchangeModeProviderOAuth {
		expectedMode = AppInstallCompletionModeProviderOAuth
	}
	if completion.CompletionMode != "" && completion.CompletionMode != expectedMode {
		return AppInstallCompletion{}, ErrInvalidAppInstallCompletion
	}
	completion.CompletionMode = expectedMode

	saved, err := scanAppInstallCompletionRow(tx.QueryRow(ctx, `
		insert into integration_app_install_completions (
			install_completion_ref,
			provider_exchange_ref,
			install_intent_ref,
			user_id,
			app_slug,
			provider,
			completion_status,
			completion_mode,
			completed_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (provider_exchange_ref) do nothing
		returning install_completion_ref, install_intent_ref, user_id, app_slug, provider, provider_exchange_ref,
			completion_status, completion_mode, completed_at, created_at, updated_at
	`,
		completion.InstallCompletionRef,
		completion.ProviderExchangeRef,
		completion.InstallIntentRef,
		completion.UserID,
		completion.AppSlug,
		completion.ProviderSlug,
		completion.CompletionStatus,
		completion.CompletionMode,
		completedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallCompletion{}, ErrAppInstallCompletionRecorded
		}
		return AppInstallCompletion{}, fmt.Errorf("save app install completion: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		update integration_app_install_intents
		set status = $3,
			updated_at = $4
		where install_intent_ref = $1
			and user_id = $2
	`, completion.InstallIntentRef, completion.UserID, completion.CompletionStatus, completedAt); err != nil {
		return AppInstallCompletion{}, fmt.Errorf("mark app install intent complete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AppInstallCompletion{}, fmt.Errorf("commit app install completion: %w", err)
	}
	return saved, nil
}

func (r *PostgresRepository) SaveAppInstallation(ctx context.Context, installation AppInstallation, activatedAt time.Time) (AppInstallation, error) {
	if strings.TrimSpace(installation.AppInstallationRef) == "" ||
		strings.TrimSpace(installation.InstallIntentRef) == "" ||
		strings.TrimSpace(installation.InstallCompletionRef) == "" ||
		strings.TrimSpace(installation.AppSlug) == "" ||
		strings.TrimSpace(installation.ProviderSlug) == "" ||
		installation.UserID <= 0 ||
		strings.TrimSpace(installation.ActivationStatus) != AppInstallationStatusActive ||
		(installation.ActivationMode != "" && !isValidAppInstallationMode(installation.ActivationMode)) {
		return AppInstallation{}, ErrInvalidAppInstallation
	}

	activatedAt = activatedAt.UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AppInstallation{}, fmt.Errorf("begin app installation activation: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var intent AppInstallIntent
	var completion AppInstallCompletion
	var app AppMetadata
	var intentCreatedAt time.Time
	var intentUpdatedAt time.Time
	var completionCompletedAt time.Time
	var completionCreatedAt time.Time
	var completionUpdatedAt time.Time
	err = tx.QueryRow(ctx, `
		select i.install_intent_ref, i.user_id, i.app_slug, i.status, i.created_at, i.updated_at,
			c.app_category, c.provider, c.name, c.auth_type, c.capabilities,
			m.install_completion_ref, m.provider_exchange_ref, m.completion_status, m.completion_mode,
			m.completed_at, m.created_at, m.updated_at
		from integration_app_install_completions m
		join integration_app_install_intents i on i.install_intent_ref = m.install_intent_ref
		join integration_app_catalog c on c.app_slug = i.app_slug
		where m.install_completion_ref = $1
			and m.install_intent_ref = $2
			and m.user_id = $3
		for update of m, i
	`, installation.InstallCompletionRef, installation.InstallIntentRef, installation.UserID).Scan(
		&intent.InstallIntentRef,
		&intent.UserID,
		&intent.AppSlug,
		&intent.Status,
		&intentCreatedAt,
		&intentUpdatedAt,
		&app.Category,
		&app.Provider,
		&app.Name,
		&app.AuthType,
		&app.Capabilities,
		&completion.InstallCompletionRef,
		&completion.ProviderExchangeRef,
		&completion.CompletionStatus,
		&completion.CompletionMode,
		&completionCompletedAt,
		&completionCreatedAt,
		&completionUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallation{}, ErrAppInstallCompletionNotFound
		}
		return AppInstallation{}, fmt.Errorf("read app installation completion: %w", err)
	}
	intent.CreatedAt = formatWireTime(intentCreatedAt)
	intent.UpdatedAt = formatWireTime(intentUpdatedAt)
	completion.InstallIntentRef = intent.InstallIntentRef
	completion.UserID = intent.UserID
	completion.AppSlug = intent.AppSlug
	completion.ProviderSlug = app.Provider
	completion.CompletedAt = formatWireTime(completionCompletedAt)
	completion.CreatedAt = formatWireTime(completionCreatedAt)
	completion.UpdatedAt = formatWireTime(completionUpdatedAt)
	app.AppSlug = intent.AppSlug
	if intent.AppSlug != installation.AppSlug || app.Provider != installation.ProviderSlug {
		return AppInstallation{}, ErrInvalidAppInstallation
	}
	if intent.Status != InstallIntentStatusInstalled || completion.CompletionStatus != AppInstallCompletionStatusInstalled {
		return AppInstallation{}, ErrAppInstallCompletionNotReady
	}
	expectedMode := AppInstallationModeSimulated
	if completion.CompletionMode == AppInstallCompletionModeProviderOAuth {
		expectedMode = AppInstallationModeProviderOAuth
	}
	if installation.ActivationMode != "" && installation.ActivationMode != expectedMode {
		return AppInstallation{}, ErrInvalidAppInstallation
	}
	installation.ActivationMode = expectedMode

	existingInstallation, err := scanAppInstallationRow(tx.QueryRow(ctx, `
		select i.app_installation_ref, i.install_intent_ref, i.user_id, i.app_slug, c.name, c.app_category,
			i.provider, c.auth_type, c.capabilities, i.install_completion_ref, i.activation_status,
			i.activation_mode, i.activated_at, i.created_at, i.updated_at
		from integration_app_installations i
		join integration_app_catalog c on c.app_slug = i.app_slug
		where i.install_completion_ref = $1
			or (i.install_intent_ref = $2 and i.user_id = $3)
		limit 1
	`, installation.InstallCompletionRef, installation.InstallIntentRef, installation.UserID))
	if err == nil && existingInstallation.AppInstallationRef != "" {
		return AppInstallation{}, ErrAppInstallationRecorded
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return AppInstallation{}, fmt.Errorf("read existing app installation: %w", err)
	}

	saved, err := scanAppInstallationRow(tx.QueryRow(ctx, `
		with inserted as (
			insert into integration_app_installations (
				app_installation_ref,
				install_completion_ref,
				install_intent_ref,
				user_id,
				app_slug,
				provider,
				activation_status,
				activation_mode,
				activated_at
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			on conflict (install_completion_ref) do nothing
			returning app_installation_ref, install_intent_ref, user_id, app_slug, provider, install_completion_ref,
				activation_status, activation_mode, activated_at, created_at, updated_at
		)
		select i.app_installation_ref, i.install_intent_ref, i.user_id, i.app_slug, c.name, c.app_category,
			i.provider, c.auth_type, c.capabilities, i.install_completion_ref, i.activation_status,
			i.activation_mode, i.activated_at, i.created_at, i.updated_at
		from inserted i
		join integration_app_catalog c on c.app_slug = i.app_slug
	`,
		installation.AppInstallationRef,
		installation.InstallCompletionRef,
		installation.InstallIntentRef,
		installation.UserID,
		installation.AppSlug,
		installation.ProviderSlug,
		installation.ActivationStatus,
		installation.ActivationMode,
		activatedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallation{}, ErrAppInstallationRecorded
		}
		return AppInstallation{}, fmt.Errorf("save app installation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AppInstallation{}, fmt.Errorf("commit app installation activation: %w", err)
	}
	return saved, nil
}

func (r *PostgresRepository) readLatestExternalAuthSession(ctx context.Context, userID int, installIntentRef string) (ExternalAuthSession, bool, error) {
	session, err := scanExternalAuthSessionRow(r.pool.QueryRow(ctx, `
		select handoff_state_ref, install_intent_ref, user_id, expires_at, consumed_at, created_at, updated_at
		from integration_app_install_handoff_sessions
		where install_intent_ref = $1
			and user_id = $2
		order by created_at desc, handoff_state_ref desc
		limit 1
	`, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthSession{}, false, nil
		}
		return ExternalAuthSession{}, false, fmt.Errorf("read latest external auth handoff session: %w", err)
	}
	return session, true, nil
}

func (r *PostgresRepository) readLatestExternalAuthAuthorizationPreview(ctx context.Context, userID int, installIntentRef string) (ExternalAuthAuthorizationPreview, bool, error) {
	preview, err := scanExternalAuthAuthorizationPreviewRow(r.pool.QueryRow(ctx, `
		select p.authorization_preview_ref, p.install_intent_ref, p.user_id, p.app_slug, p.provider,
			p.auth_type, p.requested_scopes, p.handoff_state_ref, s.expires_at, p.preview_status,
			p.requested_at, p.created_at, p.updated_at
		from integration_app_install_authorization_previews p
		join integration_app_install_handoff_sessions s on s.handoff_state_ref = p.handoff_state_ref
		where p.install_intent_ref = $1
			and p.user_id = $2
		order by p.requested_at desc, p.authorization_preview_ref desc
		limit 1
	`, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthAuthorizationPreview{}, false, nil
		}
		return ExternalAuthAuthorizationPreview{}, false, fmt.Errorf("read latest external auth authorization preview: %w", err)
	}
	return preview, true, nil
}

func (r *PostgresRepository) readLatestExternalAuthCallbackPreflight(ctx context.Context, userID int, installIntentRef string) (ExternalAuthCallbackPreflight, bool, error) {
	preflight, err := scanExternalAuthCallbackPreflightRow(r.pool.QueryRow(ctx, `
		select callback_preflight_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_status, preflight_status, received_at, created_at, updated_at
		from integration_app_install_callback_preflights
		where install_intent_ref = $1
			and user_id = $2
		order by received_at desc, callback_preflight_ref desc
		limit 1
	`, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthCallbackPreflight{}, false, nil
		}
		return ExternalAuthCallbackPreflight{}, false, fmt.Errorf("read latest external auth callback preflight: %w", err)
	}
	return preflight, true, nil
}

func (r *PostgresRepository) readLatestExternalAuthProviderExchange(ctx context.Context, userID int, installIntentRef string) (ExternalAuthProviderExchange, bool, error) {
	exchange, err := scanExternalAuthProviderExchangeRow(r.pool.QueryRow(ctx, `
		select provider_exchange_ref, install_intent_ref, user_id, app_slug, provider, handoff_state_ref,
			callback_preflight_ref, exchange_status, exchange_mode, recorded_at, created_at, updated_at
		from integration_app_install_provider_exchanges
		where install_intent_ref = $1
			and user_id = $2
		order by recorded_at desc, provider_exchange_ref desc
		limit 1
	`, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalAuthProviderExchange{}, false, nil
		}
		return ExternalAuthProviderExchange{}, false, fmt.Errorf("read latest external auth provider exchange: %w", err)
	}
	return exchange, true, nil
}

func (r *PostgresRepository) readLatestAppInstallCompletion(ctx context.Context, userID int, installIntentRef string) (AppInstallCompletion, bool, error) {
	completion, err := scanAppInstallCompletionRow(r.pool.QueryRow(ctx, `
		select install_completion_ref, install_intent_ref, user_id, app_slug, provider, provider_exchange_ref,
			completion_status, completion_mode, completed_at, created_at, updated_at
		from integration_app_install_completions
		where install_intent_ref = $1
			and user_id = $2
		order by completed_at desc, install_completion_ref desc
		limit 1
	`, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallCompletion{}, false, nil
		}
		return AppInstallCompletion{}, false, fmt.Errorf("read latest app install completion: %w", err)
	}
	return completion, true, nil
}

func (r *PostgresRepository) readLatestAppInstallation(ctx context.Context, userID int, installIntentRef string) (AppInstallation, bool, error) {
	installation, err := scanAppInstallationRow(r.pool.QueryRow(ctx, `
		select i.app_installation_ref, i.install_intent_ref, i.user_id, i.app_slug, c.name, c.app_category,
			i.provider, c.auth_type, c.capabilities, i.install_completion_ref, i.activation_status,
			i.activation_mode, i.activated_at, i.created_at, i.updated_at
		from integration_app_installations i
		join integration_app_catalog c on c.app_slug = i.app_slug
		where i.install_intent_ref = $1
			and i.user_id = $2
		order by i.activated_at desc, i.app_installation_ref desc
		limit 1
	`, installIntentRef, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AppInstallation{}, false, nil
		}
		return AppInstallation{}, false, fmt.Errorf("read latest app installation: %w", err)
	}
	return installation, true, nil
}

type installIntentScanner interface {
	Scan(dest ...any) error
}

func scanInstallIntentRow(row installIntentScanner) (AppInstallIntent, error) {
	var intent AppInstallIntent
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&intent.InstallIntentRef,
		&intent.UserID,
		&intent.AppSlug,
		&intent.Status,
		&createdAt,
		&updatedAt,
	); err != nil {
		return AppInstallIntent{}, fmt.Errorf("scan app install intent: %w", err)
	}
	intent.CreatedAt = formatWireTime(createdAt)
	intent.UpdatedAt = formatWireTime(updatedAt)
	return intent, nil
}

func scanExternalAuthSessionRow(row installIntentScanner) (ExternalAuthSession, error) {
	var session ExternalAuthSession
	var expiresAt time.Time
	var consumedAt sql.NullTime
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&session.HandoffStateRef,
		&session.InstallIntentRef,
		&session.UserID,
		&expiresAt,
		&consumedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return ExternalAuthSession{}, fmt.Errorf("scan external auth handoff session: %w", err)
	}
	session.ExpiresAt = formatWireTime(expiresAt)
	if consumedAt.Valid {
		session.ConsumedAt = formatWireTime(consumedAt.Time)
	}
	session.CreatedAt = formatWireTime(createdAt)
	session.UpdatedAt = formatWireTime(updatedAt)
	return session, nil
}

func scanExternalAuthAuthorizationPreviewRow(row installIntentScanner) (ExternalAuthAuthorizationPreview, error) {
	var preview ExternalAuthAuthorizationPreview
	var expiresAt time.Time
	var requestedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&preview.AuthorizationPreviewRef,
		&preview.InstallIntentRef,
		&preview.UserID,
		&preview.AppSlug,
		&preview.ProviderSlug,
		&preview.AuthType,
		&preview.RequestedScopes,
		&preview.StateRef,
		&expiresAt,
		&preview.Status,
		&requestedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return ExternalAuthAuthorizationPreview{}, fmt.Errorf("scan external auth authorization preview: %w", err)
	}
	preview.AuthorizationEndpointID = preview.ProviderSlug + ".authorization.preview"
	preview.ExpiresAt = formatWireTime(expiresAt)
	preview.RequestedAt = formatWireTime(requestedAt)
	preview.CreatedAt = formatWireTime(createdAt)
	preview.UpdatedAt = formatWireTime(updatedAt)
	return preview, nil
}

func scanExternalAuthCallbackPreflightRow(row installIntentScanner) (ExternalAuthCallbackPreflight, error) {
	var preflight ExternalAuthCallbackPreflight
	var receivedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&preflight.CallbackPreflightRef,
		&preflight.InstallIntentRef,
		&preflight.UserID,
		&preflight.AppSlug,
		&preflight.ProviderSlug,
		&preflight.StateRef,
		&preflight.CallbackStatus,
		&preflight.PreflightStatus,
		&receivedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return ExternalAuthCallbackPreflight{}, fmt.Errorf("scan external auth callback preflight: %w", err)
	}
	preflight.ReceivedAt = formatWireTime(receivedAt)
	preflight.CreatedAt = formatWireTime(createdAt)
	preflight.UpdatedAt = formatWireTime(updatedAt)
	return preflight, nil
}

func scanExternalAuthProviderExchangeRow(row installIntentScanner) (ExternalAuthProviderExchange, error) {
	var exchange ExternalAuthProviderExchange
	var recordedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&exchange.ProviderExchangeRef,
		&exchange.InstallIntentRef,
		&exchange.UserID,
		&exchange.AppSlug,
		&exchange.ProviderSlug,
		&exchange.StateRef,
		&exchange.CallbackPreflightRef,
		&exchange.ExchangeStatus,
		&exchange.ExchangeMode,
		&recordedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return ExternalAuthProviderExchange{}, fmt.Errorf("scan external auth provider exchange: %w", err)
	}
	exchange.RecordedAt = formatWireTime(recordedAt)
	exchange.CreatedAt = formatWireTime(createdAt)
	exchange.UpdatedAt = formatWireTime(updatedAt)
	return exchange, nil
}

func scanAppInstallCompletionRow(row installIntentScanner) (AppInstallCompletion, error) {
	var completion AppInstallCompletion
	var completedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&completion.InstallCompletionRef,
		&completion.InstallIntentRef,
		&completion.UserID,
		&completion.AppSlug,
		&completion.ProviderSlug,
		&completion.ProviderExchangeRef,
		&completion.CompletionStatus,
		&completion.CompletionMode,
		&completedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return AppInstallCompletion{}, fmt.Errorf("scan app install completion: %w", err)
	}
	completion.CompletedAt = formatWireTime(completedAt)
	completion.CreatedAt = formatWireTime(createdAt)
	completion.UpdatedAt = formatWireTime(updatedAt)
	return completion, nil
}

func scanAppInstallationRow(row installIntentScanner) (AppInstallation, error) {
	var installation AppInstallation
	var activatedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&installation.AppInstallationRef,
		&installation.InstallIntentRef,
		&installation.UserID,
		&installation.AppSlug,
		&installation.AppName,
		&installation.AppCategory,
		&installation.ProviderSlug,
		&installation.AuthType,
		&installation.Capabilities,
		&installation.InstallCompletionRef,
		&installation.ActivationStatus,
		&installation.ActivationMode,
		&activatedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return AppInstallation{}, fmt.Errorf("scan app installation: %w", err)
	}
	installation.ActivatedAt = formatWireTime(activatedAt)
	installation.CreatedAt = formatWireTime(createdAt)
	installation.UpdatedAt = formatWireTime(updatedAt)
	return installation, nil
}

func formatWireTime(value time.Time) string {
	return value.UTC().Format(wireTimeLayout)
}
