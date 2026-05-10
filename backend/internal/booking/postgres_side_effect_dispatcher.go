package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/calendar"
	"github.com/LynnColeArt/better-cal/backend/internal/db"
	emailprovider "github.com/LynnColeArt/better-cal/backend/internal/email"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultWebhookMaxAttempts = 3

type BookingReader interface {
	ReadByUID(context.Context, string) (Booking, bool, error)
}

type PostgresSideEffectDispatcher struct {
	pool                *pgxpool.Pool
	bookings            BookingReader
	subscriptions       WebhookSubscriptionStore
	secretResolver      WebhookSigningSecretResolver
	webhookTransport    WebhookAttemptTransport
	emailProvider       emailprovider.ProviderAdapter
	emailTransport      EmailDeliveryTransport
	emailDispatchURL    string
	calendarProvider    calendar.ProviderAdapter
	calendarTransport   CalendarDispatchTransport
	calendarDispatchURL string
	maxAttempts         int
}

type PostgresSideEffectDispatcherOption func(*PostgresSideEffectDispatcher)

func WithWebhookMaxAttempts(maxAttempts int) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		if maxAttempts > 0 {
			d.maxAttempts = maxAttempts
		}
	}
}

func WithCalendarTransport(transport CalendarDispatchTransport) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		if transport != nil {
			d.calendarTransport = transport
		}
	}
}

func WithCalendarProvider(provider calendar.ProviderAdapter) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		if provider != nil {
			d.calendarProvider = provider
		}
	}
}

func WithEmailTransport(transport EmailDeliveryTransport) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		if transport != nil {
			d.emailTransport = transport
		}
	}
}

func WithEmailProvider(provider emailprovider.ProviderAdapter) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		if provider != nil {
			d.emailProvider = provider
		}
	}
}

func WithEmailDispatchURL(targetURL string) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		d.emailDispatchURL = targetURL
	}
}

func WithCalendarDispatchURL(targetURL string) PostgresSideEffectDispatcherOption {
	return func(d *PostgresSideEffectDispatcher) {
		d.calendarDispatchURL = targetURL
	}
}

func NewPostgresSideEffectDispatcher(pool *pgxpool.Pool, bookings BookingReader, subscriptions WebhookSubscriptionStore, secretResolver WebhookSigningSecretResolver, transport WebhookAttemptTransport, opts ...PostgresSideEffectDispatcherOption) PostgresSideEffectDispatcher {
	dispatcher := PostgresSideEffectDispatcher{
		pool:             pool,
		bookings:         bookings,
		subscriptions:    subscriptions,
		secretResolver:   secretResolver,
		webhookTransport: transport,
		emailProvider:    emailprovider.NewResendFixtureProvider(),
		calendarProvider: calendar.NewGoogleFixtureProvider(),
		maxAttempts:      defaultWebhookMaxAttempts,
	}
	for _, opt := range opts {
		opt(&dispatcher)
	}
	return dispatcher
}

func (d PostgresSideEffectDispatcher) Dispatch(ctx context.Context, effect PlannedSideEffectRecord) error {
	if d.pool == nil {
		return db.ErrNilPool
	}
	if effect.ID == 0 || effect.Name == "" || effect.BookingUID == "" || effect.RequestID == "" {
		return fmt.Errorf("invalid planned side effect dispatch record")
	}

	pendingWebhookAttempts := []WebhookDeliveryAttempt{}
	pendingEmailAttempts := []EmailDeliveryAttempt{}
	pendingCalendarAttempts := []CalendarDispatchAttempt{}
	if err := db.WithTx(ctx, d.pool, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			insert into booking_side_effect_dispatch_log (side_effect_id, booking_uid, name, request_id)
			values ($1, $2, $3, $4)
			on conflict (side_effect_id) do nothing
		`, effect.ID, effect.BookingUID, string(effect.Name), effect.RequestID); err != nil {
			return fmt.Errorf("record side effect dispatch: %w", err)
		}

		var err error
		pendingWebhookAttempts, err = d.prepareWebhookAttempts(ctx, tx, effect)
		if err != nil {
			return err
		}
		pendingEmailAttempts, err = d.prepareEmailAttempts(ctx, tx, effect)
		if err != nil {
			return err
		}
		pendingCalendarAttempts, err = d.prepareCalendarAttempts(ctx, tx, effect)
		return err
	}); err != nil {
		return err
	}

	var dispatchErr error
	if len(pendingWebhookAttempts) > 0 {
		if d.webhookTransport == nil {
			return errors.New("webhook dispatch requires a transport")
		}
		for _, attempt := range pendingWebhookAttempts {
			receipt, err := d.webhookTransport.DeliverWebhookAttempt(ctx, attempt)
			statusCode := webhookAttemptStatusCode(receipt, err)
			if err != nil {
				result, markErr := markWebhookAttemptFailed(ctx, d.pool, attempt.ID, statusCode, err, d.maxAttempts)
				if markErr != nil {
					return fmt.Errorf("mark webhook attempt failed: %w", markErr)
				}
				if dispatchErr == nil && !result.DeadLettered {
					dispatchErr = err
				}
				continue
			}
			if err := markWebhookAttemptDelivered(ctx, d.pool, attempt.ID, statusCode); err != nil {
				return fmt.Errorf("mark webhook attempt delivered: %w", err)
			}
		}
	}

	if len(pendingEmailAttempts) > 0 {
		if d.emailTransport == nil {
			return errors.New("email delivery requires a transport")
		}
		for _, attempt := range pendingEmailAttempts {
			prepared, err := d.prepareEmailProviderDispatch(ctx, attempt)
			if err != nil {
				return err
			}
			receipt, err := d.emailTransport.DeliverEmailDelivery(ctx, prepared)
			statusCode := emailDeliveryStatusCode(receipt, err)
			if err != nil {
				if markErr := markEmailDeliveryFailed(ctx, d.pool, attempt.ID, statusCode, err); markErr != nil {
					return fmt.Errorf("mark email delivery failed: %w", markErr)
				}
				if dispatchErr == nil {
					dispatchErr = err
				}
				continue
			}
			if err := markEmailDeliveryDelivered(ctx, d.pool, attempt.ID, statusCode); err != nil {
				return fmt.Errorf("mark email delivery delivered: %w", err)
			}
		}
	}

	if len(pendingCalendarAttempts) > 0 {
		if d.calendarTransport == nil {
			return errors.New("calendar dispatch requires a transport")
		}
		for _, attempt := range pendingCalendarAttempts {
			prepared, err := d.prepareCalendarProviderDispatch(ctx, attempt)
			if err != nil {
				return err
			}
			receipt, err := d.calendarTransport.DeliverCalendarDispatch(ctx, prepared)
			statusCode := calendarDispatchStatusCode(receipt, err)
			if err != nil {
				if markErr := markCalendarDispatchFailed(ctx, d.pool, attempt.ID, statusCode, err); markErr != nil {
					return fmt.Errorf("mark calendar dispatch failed: %w", markErr)
				}
				if dispatchErr == nil {
					dispatchErr = err
				}
				continue
			}
			if err := markCalendarDispatchDelivered(ctx, d.pool, attempt.ID, statusCode); err != nil {
				return fmt.Errorf("mark calendar dispatch delivered: %w", err)
			}
		}
	}
	return dispatchErr
}

func (d PostgresSideEffectDispatcher) prepareWebhookAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord) ([]WebhookDeliveryAttempt, error) {
	if _, ok := webhookTriggerEventForSideEffect(effect.Name); !ok {
		return nil, nil
	}

	webhookDelivery, found, err := readWebhookDelivery(ctx, tx, effect.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		webhookDelivery, err = d.buildWebhookDelivery(ctx, effect)
		if err != nil {
			return nil, err
		}
		if webhookDelivery == nil {
			return nil, nil
		}
		deliveryID, err := upsertWebhookDelivery(ctx, tx, effect, webhookDelivery)
		if err != nil {
			return nil, err
		}
		webhookDelivery.ID = deliveryID
	}

	attemptCount, err := countWebhookAttempts(ctx, tx, webhookDelivery.ID)
	if err != nil {
		return nil, err
	}
	if attemptCount == 0 {
		attempts, err := d.webhookAttemptTemplates(ctx, WebhookTriggerEvent(webhookDelivery.TriggerEvent), webhookDelivery.Body)
		if err != nil {
			return nil, err
		}
		if err := recordWebhookAttempts(ctx, tx, effect, webhookDelivery.ID, webhookDelivery, attempts); err != nil {
			return nil, err
		}
	}

	return readPendingWebhookAttempts(ctx, tx, effect.ID)
}

type webhookDeliveryRecord struct {
	ID           int64
	TriggerEvent string
	ContentType  string
	CreatedAt    string
	Body         string
}

func readWebhookDelivery(ctx context.Context, tx db.Tx, sideEffectID int64) (*webhookDeliveryRecord, bool, error) {
	var delivery webhookDeliveryRecord
	err := tx.QueryRow(ctx, `
		select id, trigger_event, content_type, created_at_wire, body
		from booking_webhook_deliveries
		where side_effect_id = $1
	`, sideEffectID).Scan(&delivery.ID, &delivery.TriggerEvent, &delivery.ContentType, &delivery.CreatedAt, &delivery.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read webhook delivery: %w", err)
	}
	return &delivery, true, nil
}

func upsertWebhookDelivery(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, webhookDelivery *webhookDeliveryRecord) (int64, error) {
	var deliveryID int64
	err := tx.QueryRow(ctx, `
		with inserted as (
			insert into booking_webhook_deliveries (
				side_effect_id,
				booking_uid,
				request_id,
				trigger_event,
				content_type,
				created_at_wire,
				body
			)
			values ($1, $2, $3, $4, $5, $6, $7)
			on conflict (side_effect_id) do nothing
			returning id
		)
		select id from inserted
		union all
		select id
		from booking_webhook_deliveries
		where side_effect_id = $1
		limit 1
	`, effect.ID, effect.BookingUID, effect.RequestID, webhookDelivery.TriggerEvent, webhookDelivery.ContentType, webhookDelivery.CreatedAt, webhookDelivery.Body).Scan(&deliveryID)
	if err != nil {
		return 0, fmt.Errorf("record webhook delivery: %w", err)
	}
	return deliveryID, nil
}

func countWebhookAttempts(ctx context.Context, tx db.Tx, deliveryID int64) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `
		select count(*)
		from booking_webhook_delivery_attempts
		where delivery_id = $1
	`, deliveryID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count webhook delivery attempts: %w", err)
	}
	return count, nil
}

func recordWebhookAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, deliveryID int64, webhookDelivery *webhookDeliveryRecord, attempts []WebhookDeliveryAttempt) error {
	for _, attempt := range attempts {
		if _, err := tx.Exec(ctx, `
			insert into booking_webhook_delivery_attempts (
				delivery_id,
				side_effect_id,
				subscriber_id,
				subscriber_url,
				trigger_event,
				content_type,
				signature_header_name,
				signature_header_value,
				body
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			on conflict (delivery_id, subscriber_id) do nothing
		`, deliveryID, effect.ID, attempt.SubscriberID, attempt.SubscriberURL, webhookDelivery.TriggerEvent, webhookDelivery.ContentType, attempt.SignatureHeaderName, attempt.SignatureHeaderValue, webhookDelivery.Body); err != nil {
			return fmt.Errorf("record webhook delivery attempt: %w", err)
		}
	}
	return nil
}

func readPendingWebhookAttempts(ctx context.Context, tx db.Tx, sideEffectID int64) ([]WebhookDeliveryAttempt, error) {
	rows, err := tx.Query(ctx, `
		select id, delivery_id, side_effect_id, subscriber_id, subscriber_url, trigger_event, content_type, signature_header_name, signature_header_value, body
		from booking_webhook_delivery_attempts
		where side_effect_id = $1
			and delivered_at is null
			and dead_lettered_at is null
		order by id
	`, sideEffectID)
	if err != nil {
		return nil, fmt.Errorf("read pending webhook attempts: %w", err)
	}
	defer rows.Close()

	attempts := []WebhookDeliveryAttempt{}
	for rows.Next() {
		var attempt WebhookDeliveryAttempt
		if err := rows.Scan(
			&attempt.ID,
			&attempt.DeliveryID,
			&attempt.SideEffectID,
			&attempt.SubscriberID,
			&attempt.SubscriberURL,
			&attempt.TriggerEvent,
			&attempt.ContentType,
			&attempt.SignatureHeaderName,
			&attempt.SignatureHeaderValue,
			&attempt.Body,
		); err != nil {
			return nil, fmt.Errorf("scan pending webhook attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending webhook attempt rows: %w", err)
	}
	return attempts, nil
}

func (d PostgresSideEffectDispatcher) prepareCalendarAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord) ([]CalendarDispatchAttempt, error) {
	if _, ok := calendarDispatchActionForSideEffect(effect.Name); !ok {
		return nil, nil
	}

	dispatchRecord, found, err := readCalendarDispatch(ctx, tx, effect.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		dispatchRecord, err = d.buildCalendarDispatch(ctx, effect)
		if err != nil {
			return nil, err
		}
		if dispatchRecord == nil {
			return nil, nil
		}
		dispatchID, err := upsertCalendarDispatch(ctx, tx, effect, dispatchRecord)
		if err != nil {
			return nil, err
		}
		dispatchRecord.ID = dispatchID
	}

	attemptCount, err := countCalendarDispatchAttempts(ctx, tx, dispatchRecord.ID)
	if err != nil {
		return nil, err
	}
	if attemptCount == 0 {
		attempts, err := d.calendarAttemptTemplates(dispatchRecord)
		if err != nil {
			return nil, err
		}
		if err := recordCalendarDispatchAttempts(ctx, tx, effect, dispatchRecord.ID, dispatchRecord, attempts); err != nil {
			return nil, err
		}
	}

	return readPendingCalendarDispatchAttempts(ctx, tx, effect.ID)
}

type calendarDispatchRecord struct {
	ID          int64
	Action      string
	ContentType string
	CreatedAt   string
	Body        string
}

func readCalendarDispatch(ctx context.Context, tx db.Tx, sideEffectID int64) (*calendarDispatchRecord, bool, error) {
	var dispatchRecord calendarDispatchRecord
	err := tx.QueryRow(ctx, `
		select id, action, content_type, created_at_wire, body
		from booking_calendar_dispatches
		where side_effect_id = $1
	`, sideEffectID).Scan(&dispatchRecord.ID, &dispatchRecord.Action, &dispatchRecord.ContentType, &dispatchRecord.CreatedAt, &dispatchRecord.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read calendar dispatch: %w", err)
	}
	return &dispatchRecord, true, nil
}

func upsertCalendarDispatch(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, dispatchRecord *calendarDispatchRecord) (int64, error) {
	var dispatchID int64
	err := tx.QueryRow(ctx, `
		with inserted as (
			insert into booking_calendar_dispatches (
				side_effect_id,
				booking_uid,
				request_id,
				action,
				content_type,
				created_at_wire,
				body
			)
			values ($1, $2, $3, $4, $5, $6, $7)
			on conflict (side_effect_id) do nothing
			returning id
		)
		select id from inserted
		union all
		select id
		from booking_calendar_dispatches
		where side_effect_id = $1
		limit 1
	`, effect.ID, effect.BookingUID, effect.RequestID, dispatchRecord.Action, dispatchRecord.ContentType, dispatchRecord.CreatedAt, dispatchRecord.Body).Scan(&dispatchID)
	if err != nil {
		return 0, fmt.Errorf("record calendar dispatch: %w", err)
	}
	return dispatchID, nil
}

func countCalendarDispatchAttempts(ctx context.Context, tx db.Tx, dispatchID int64) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `
		select count(*)
		from booking_calendar_dispatch_attempts
		where dispatch_id = $1
	`, dispatchID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count calendar dispatch attempts: %w", err)
	}
	return count, nil
}

func recordCalendarDispatchAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, dispatchID int64, dispatchRecord *calendarDispatchRecord, attempts []CalendarDispatchAttempt) error {
	for _, attempt := range attempts {
		if _, err := tx.Exec(ctx, `
			insert into booking_calendar_dispatch_attempts (
				dispatch_id,
				side_effect_id,
				target_url,
				action,
				content_type,
				body
			)
			values ($1, $2, $3, $4, $5, $6)
			on conflict (dispatch_id, target_url) do nothing
		`, dispatchID, effect.ID, attempt.TargetURL, dispatchRecord.Action, dispatchRecord.ContentType, dispatchRecord.Body); err != nil {
			return fmt.Errorf("record calendar dispatch attempt: %w", err)
		}
	}
	return nil
}

func readPendingCalendarDispatchAttempts(ctx context.Context, tx db.Tx, sideEffectID int64) ([]CalendarDispatchAttempt, error) {
	rows, err := tx.Query(ctx, `
		select id, dispatch_id, side_effect_id, target_url, action, content_type, body
		from booking_calendar_dispatch_attempts
		where side_effect_id = $1
			and delivered_at is null
		order by id
	`, sideEffectID)
	if err != nil {
		return nil, fmt.Errorf("read pending calendar dispatch attempts: %w", err)
	}
	defer rows.Close()

	attempts := []CalendarDispatchAttempt{}
	for rows.Next() {
		var attempt CalendarDispatchAttempt
		if err := rows.Scan(
			&attempt.ID,
			&attempt.DispatchID,
			&attempt.SideEffectID,
			&attempt.TargetURL,
			&attempt.Action,
			&attempt.ContentType,
			&attempt.Body,
		); err != nil {
			return nil, fmt.Errorf("scan pending calendar dispatch attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending calendar dispatch attempt rows: %w", err)
	}
	return attempts, nil
}

func markCalendarDispatchDelivered(ctx context.Context, pool *pgxpool.Pool, attemptID int64, statusCode int) error {
	if pool == nil {
		return db.ErrNilPool
	}
	var updatedID int64
	err := pool.QueryRow(ctx, `
		update booking_calendar_dispatch_attempts
		set delivered_at = now(),
			response_status = $2,
			last_error = null,
			last_attempted_at = now(),
			attempt_count = booking_calendar_dispatch_attempts.attempt_count + 1
		where id = $1
			and delivered_at is null
		returning id
	`, attemptID, nullableStatusCode(statusCode)).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("calendar dispatch attempt %d was not pending", attemptID)
	}
	if err != nil {
		return fmt.Errorf("update delivered calendar dispatch attempt: %w", err)
	}
	return nil
}

func markCalendarDispatchFailed(ctx context.Context, pool *pgxpool.Pool, attemptID int64, statusCode int, dispatchErr error) error {
	if pool == nil {
		return db.ErrNilPool
	}
	var updatedID int64
	err := pool.QueryRow(ctx, `
		update booking_calendar_dispatch_attempts
		set response_status = $2,
			last_error = $3,
			last_attempted_at = now(),
			attempt_count = booking_calendar_dispatch_attempts.attempt_count + 1
		where id = $1
			and delivered_at is null
		returning id
	`, attemptID, nullableStatusCode(statusCode), safeCalendarDispatchError(dispatchErr)).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("calendar dispatch attempt %d was not pending", attemptID)
	}
	if err != nil {
		return fmt.Errorf("update failed calendar dispatch attempt: %w", err)
	}
	return nil
}

func markWebhookAttemptDelivered(ctx context.Context, pool *pgxpool.Pool, attemptID int64, statusCode int) error {
	if pool == nil {
		return db.ErrNilPool
	}
	return db.WithTx(ctx, pool, func(tx db.Tx) error {
		var subscriberID int64
		err := tx.QueryRow(ctx, `
			update booking_webhook_delivery_attempts
			set delivered_at = now(),
				response_status = $2,
				last_error = null,
				last_attempted_at = now(),
				attempt_count = booking_webhook_delivery_attempts.attempt_count + 1
			where id = $1
				and delivered_at is null
				and dead_lettered_at is null
			returning subscriber_id
		`, attemptID, nullableStatusCode(statusCode)).Scan(&subscriberID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("webhook attempt %d was not pending", attemptID)
		}
		if err != nil {
			return fmt.Errorf("update delivered webhook attempt: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			update booking_webhook_subscriptions
			set failure_count = 0,
				updated_at = now()
			where id = $1
		`, subscriberID); err != nil {
			return fmt.Errorf("reset webhook subscription failure count: %w", err)
		}
		return nil
	})
}

type webhookAttemptFailureResult struct {
	SubscriberID  int64
	AttemptCount  int
	DeadLettered  bool
	DisabledSubID int64
}

func markWebhookAttemptFailed(ctx context.Context, pool *pgxpool.Pool, attemptID int64, statusCode int, dispatchErr error, maxAttempts int) (webhookAttemptFailureResult, error) {
	if pool == nil {
		return webhookAttemptFailureResult{}, db.ErrNilPool
	}
	if maxAttempts <= 0 {
		maxAttempts = defaultWebhookMaxAttempts
	}
	result := webhookAttemptFailureResult{}
	return result, db.WithTx(ctx, pool, func(tx db.Tx) error {
		err := tx.QueryRow(ctx, `
			update booking_webhook_delivery_attempts
			set response_status = $2,
				last_error = $3,
				last_attempted_at = now(),
				attempt_count = booking_webhook_delivery_attempts.attempt_count + 1,
				dead_lettered_at = case
					when booking_webhook_delivery_attempts.attempt_count + 1 >= $4 then coalesce(booking_webhook_delivery_attempts.dead_lettered_at, now())
					else booking_webhook_delivery_attempts.dead_lettered_at
				end
			where id = $1
				and delivered_at is null
				and dead_lettered_at is null
			returning subscriber_id, attempt_count, dead_lettered_at is not null
		`, attemptID, nullableStatusCode(statusCode), safeWebhookAttemptError(dispatchErr), maxAttempts).Scan(&result.SubscriberID, &result.AttemptCount, &result.DeadLettered)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("webhook attempt %d was not pending", attemptID)
		}
		if err != nil {
			return fmt.Errorf("update failed webhook attempt: %w", err)
		}
		if !result.DeadLettered {
			if _, err := tx.Exec(ctx, `
				update booking_webhook_subscriptions
				set failure_count = failure_count + 1,
					updated_at = now()
				where id = $1
			`, result.SubscriberID); err != nil {
				return fmt.Errorf("record webhook subscription failure: %w", err)
			}
			return nil
		}
		if _, err := tx.Exec(ctx, `
			update booking_webhook_subscriptions
			set active = false,
				failure_count = failure_count + 1,
				disabled_at = coalesce(disabled_at, now()),
				disabled_reason = 'delivery attempts exhausted',
				updated_at = now()
			where id = $1
		`, result.SubscriberID); err != nil {
			return fmt.Errorf("disable webhook subscription: %w", err)
		}
		result.DisabledSubID = result.SubscriberID
		return nil
	})
}

func nullableStatusCode(statusCode int) any {
	if statusCode <= 0 {
		return nil
	}
	return statusCode
}

func (d PostgresSideEffectDispatcher) buildWebhookDelivery(ctx context.Context, effect PlannedSideEffectRecord) (*webhookDeliveryRecord, error) {
	envelope, ok, err := d.webhookEnvelope(ctx, effect)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	bodyRaw, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode webhook delivery body: %w", err)
	}
	return &webhookDeliveryRecord{
		TriggerEvent: string(envelope.TriggerEvent),
		ContentType:  "application/json",
		CreatedAt:    envelope.CreatedAt,
		Body:         string(bodyRaw),
	}, nil
}

func (d PostgresSideEffectDispatcher) buildCalendarDispatch(ctx context.Context, effect PlannedSideEffectRecord) (*calendarDispatchRecord, error) {
	envelope, ok, err := d.calendarDispatchEnvelope(ctx, effect)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	bodyRaw, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode calendar dispatch body: %w", err)
	}
	return &calendarDispatchRecord{
		Action:      string(envelope.Action),
		ContentType: "application/json",
		CreatedAt:   envelope.CreatedAt,
		Body:        string(bodyRaw),
	}, nil
}

func (d PostgresSideEffectDispatcher) calendarDispatchEnvelope(ctx context.Context, effect PlannedSideEffectRecord) (CalendarDispatchEnvelope, bool, error) {
	if _, ok := calendarDispatchActionForSideEffect(effect.Name); !ok {
		return CalendarDispatchEnvelope{}, false, nil
	}
	if d.bookings == nil {
		return CalendarDispatchEnvelope{}, false, errors.New("calendar dispatch requires a booking reader")
	}

	bookingValue, found, err := d.bookings.ReadByUID(ctx, effect.BookingUID)
	if err != nil {
		return CalendarDispatchEnvelope{}, false, fmt.Errorf("read booking for calendar dispatch: %w", err)
	}
	if !found {
		return CalendarDispatchEnvelope{}, false, fmt.Errorf("read booking for calendar dispatch %q: not found", effect.BookingUID)
	}

	envelope, ok := calendarDispatchEnvelopeForBooking(effect, bookingValue)
	return envelope, ok, nil
}

func (d PostgresSideEffectDispatcher) webhookEnvelope(ctx context.Context, effect PlannedSideEffectRecord) (WebhookDeliveryEnvelope, bool, error) {
	if _, ok := webhookTriggerEventForSideEffect(effect.Name); !ok {
		return WebhookDeliveryEnvelope{}, false, nil
	}
	if d.bookings == nil {
		return WebhookDeliveryEnvelope{}, false, errors.New("webhook dispatch requires a booking reader")
	}

	bookingValue, found, err := d.bookings.ReadByUID(ctx, effect.BookingUID)
	if err != nil {
		return WebhookDeliveryEnvelope{}, false, fmt.Errorf("read booking for webhook dispatch: %w", err)
	}
	if !found {
		return WebhookDeliveryEnvelope{}, false, fmt.Errorf("read booking for webhook dispatch %q: not found", effect.BookingUID)
	}

	envelope, ok := webhookEnvelopeForBooking(effect, bookingValue)
	return envelope, ok, nil
}

func (d PostgresSideEffectDispatcher) webhookAttemptTemplates(ctx context.Context, triggerEvent WebhookTriggerEvent, body string) ([]WebhookDeliveryAttempt, error) {
	if d.subscriptions == nil {
		return nil, errors.New("webhook dispatch requires a subscription store")
	}
	if d.secretResolver == nil {
		return nil, errors.New("webhook dispatch requires a signing secret resolver")
	}

	subscriptions, err := d.subscriptions.ReadWebhookSubscriptionsByTrigger(ctx, triggerEvent)
	if err != nil {
		return nil, fmt.Errorf("read webhook subscriptions: %w", err)
	}

	attempts := make([]WebhookDeliveryAttempt, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		secret, ok, err := d.secretResolver.ResolveWebhookSigningSecret(ctx, subscription.SigningKeyRef)
		if err != nil {
			return nil, fmt.Errorf("resolve webhook signing secret: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("resolve webhook signing secret %q: not found", subscription.SigningKeyRef)
		}
		signatureValue, err := signWebhookBody(body, secret)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, WebhookDeliveryAttempt{
			SubscriberID:         subscription.ID,
			SubscriberURL:        subscription.SubscriberURL,
			TriggerEvent:         string(subscription.TriggerEvent),
			ContentType:          "application/json",
			SignatureHeaderName:  webhookSignatureHeaderName,
			SignatureHeaderValue: signatureValue,
			Body:                 body,
		})
	}
	return attempts, nil
}

func (d PostgresSideEffectDispatcher) calendarAttemptTemplates(dispatchRecord *calendarDispatchRecord) ([]CalendarDispatchAttempt, error) {
	if dispatchRecord == nil {
		return nil, nil
	}
	if d.calendarDispatchURL == "" {
		return nil, errors.New("calendar dispatch requires a target url")
	}
	return []CalendarDispatchAttempt{
		{
			TargetURL:   d.calendarDispatchURL,
			Action:      dispatchRecord.Action,
			ContentType: dispatchRecord.ContentType,
			Body:        dispatchRecord.Body,
		},
	}, nil
}
