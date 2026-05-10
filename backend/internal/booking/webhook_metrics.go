package booking

import (
	"context"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookDeliveryMetrics struct {
	PendingAttempts       int
	DeliveredAttempts     int
	FailedPendingAttempts int
	DeadLetteredAttempts  int
	DisabledSubscribers   int
}

func ReadWebhookDeliveryMetrics(ctx context.Context, pool *pgxpool.Pool) (WebhookDeliveryMetrics, error) {
	if pool == nil {
		return WebhookDeliveryMetrics{}, db.ErrNilPool
	}

	var metrics WebhookDeliveryMetrics
	if err := pool.QueryRow(ctx, `
		select
			(select count(*) from booking_webhook_delivery_attempts where delivered_at is null and dead_lettered_at is null),
			(select count(*) from booking_webhook_delivery_attempts where delivered_at is not null),
			(select count(*) from booking_webhook_delivery_attempts where delivered_at is null and dead_lettered_at is null and last_error is not null),
			(select count(*) from booking_webhook_delivery_attempts where dead_lettered_at is not null),
			(select count(*) from booking_webhook_subscriptions where active = false and disabled_at is not null)
	`).Scan(
		&metrics.PendingAttempts,
		&metrics.DeliveredAttempts,
		&metrics.FailedPendingAttempts,
		&metrics.DeadLetteredAttempts,
		&metrics.DisabledSubscribers,
	); err != nil {
		return WebhookDeliveryMetrics{}, fmt.Errorf("read webhook delivery metrics: %w", err)
	}
	return metrics, nil
}
