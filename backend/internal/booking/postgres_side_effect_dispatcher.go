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
	pool     *pgxpool.Pool
	bookings BookingReader
}

func NewPostgresSideEffectDispatcher(pool *pgxpool.Pool, bookings BookingReader) PostgresSideEffectDispatcher {
	return PostgresSideEffectDispatcher{pool: pool, bookings: bookings}
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
		if _, err := tx.Exec(ctx, `
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
		`, effect.ID, effect.BookingUID, effect.RequestID, webhookDelivery.TriggerEvent, webhookDelivery.ContentType, webhookDelivery.CreatedAt, webhookDelivery.Body); err != nil {
			return fmt.Errorf("record webhook delivery: %w", err)
		}
		return nil
	})
}

type webhookDeliveryRecord struct {
	TriggerEvent string
	ContentType  string
	CreatedAt    string
	Body         string
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
