package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingReader interface {
	ReadByUID(context.Context, string) (Booking, bool, error)
}

type PostgresSideEffectDispatcher struct {
	pool           *pgxpool.Pool
	bookings       BookingReader
	subscriptions  WebhookSubscriptionStore
	secretResolver WebhookSigningSecretResolver
}

func NewPostgresSideEffectDispatcher(pool *pgxpool.Pool, bookings BookingReader, subscriptions WebhookSubscriptionStore, secretResolver WebhookSigningSecretResolver) PostgresSideEffectDispatcher {
	return PostgresSideEffectDispatcher{
		pool:           pool,
		bookings:       bookings,
		subscriptions:  subscriptions,
		secretResolver: secretResolver,
	}
}

func (d PostgresSideEffectDispatcher) Dispatch(ctx context.Context, effect PlannedSideEffectRecord) error {
	if d.pool == nil {
		return db.ErrNilPool
	}
	if effect.ID == 0 || effect.Name == "" || effect.BookingUID == "" || effect.RequestID == "" {
		return fmt.Errorf("invalid planned side effect dispatch record")
	}

	webhookDelivery, err := d.buildWebhookDelivery(ctx, effect)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, d.pool, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			insert into booking_side_effect_dispatch_log (side_effect_id, booking_uid, name, request_id)
			values ($1, $2, $3, $4)
			on conflict (side_effect_id) do nothing
		`, effect.ID, effect.BookingUID, string(effect.Name), effect.RequestID); err != nil {
			return fmt.Errorf("record side effect dispatch: %w", err)
		}
		if webhookDelivery == nil {
			return nil
		}
		deliveryID, err := upsertWebhookDelivery(ctx, tx, effect, webhookDelivery)
		if err != nil {
			return err
		}
		if err := recordWebhookAttempts(ctx, tx, effect, deliveryID, webhookDelivery); err != nil {
			return err
		}
		return nil
	})
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

func recordWebhookAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, deliveryID int64, webhookDelivery *webhookDeliveryRecord) error {
	for _, attempt := range webhookDelivery.Attempts {
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

type webhookDeliveryRecord struct {
	TriggerEvent string
	ContentType  string
	CreatedAt    string
	Body         string
	Attempts     []signedWebhookAttempt
}

type signedWebhookAttempt struct {
	SubscriberID         int64
	SubscriberURL        string
	SignatureHeaderName  string
	SignatureHeaderValue string
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
	attempts, err := d.webhookAttempts(ctx, envelope.TriggerEvent, string(bodyRaw))
	if err != nil {
		return nil, err
	}
	return &webhookDeliveryRecord{
		TriggerEvent: string(envelope.TriggerEvent),
		ContentType:  "application/json",
		CreatedAt:    envelope.CreatedAt,
		Body:         string(bodyRaw),
		Attempts:     attempts,
	}, nil
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

func (d PostgresSideEffectDispatcher) webhookAttempts(ctx context.Context, triggerEvent WebhookTriggerEvent, body string) ([]signedWebhookAttempt, error) {
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

	attempts := make([]signedWebhookAttempt, 0, len(subscriptions))
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
		attempts = append(attempts, signedWebhookAttempt{
			SubscriberID:         subscription.ID,
			SubscriberURL:        subscription.SubscriberURL,
			SignatureHeaderName:  webhookSignatureHeaderName,
			SignatureHeaderValue: signatureValue,
		})
	}
	return attempts, nil
}
