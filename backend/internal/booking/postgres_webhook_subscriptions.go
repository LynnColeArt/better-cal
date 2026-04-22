package booking

import (
	"context"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresWebhookSubscriptionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresWebhookSubscriptionStore(pool *pgxpool.Pool) *PostgresWebhookSubscriptionStore {
	return &PostgresWebhookSubscriptionStore{pool: pool}
}

func (s *PostgresWebhookSubscriptionStore) SaveWebhookSubscription(ctx context.Context, subscription WebhookSubscription) error {
	if s.pool == nil {
		return db.ErrNilPool
	}
	if subscription.SubscriberURL == "" || subscription.TriggerEvent == "" || subscription.SigningKeyRef == "" {
		return fmt.Errorf("invalid webhook subscription")
	}
	active := subscription.Active
	if !subscription.Active {
		active = false
	}
	if _, err := s.pool.Exec(ctx, `
		insert into booking_webhook_subscriptions (subscriber_url, trigger_event, signing_key_ref, active)
		values ($1, $2, $3, $4)
		on conflict (subscriber_url, trigger_event, signing_key_ref) do update set
			active = excluded.active,
			updated_at = now()
	`, subscription.SubscriberURL, string(subscription.TriggerEvent), subscription.SigningKeyRef, active); err != nil {
		return fmt.Errorf("save webhook subscription: %w", err)
	}
	return nil
}

func (s *PostgresWebhookSubscriptionStore) ReadWebhookSubscriptionsByTrigger(ctx context.Context, triggerEvent WebhookTriggerEvent) ([]WebhookSubscription, error) {
	if s.pool == nil {
		return nil, db.ErrNilPool
	}
	rows, err := s.pool.Query(ctx, `
		select id, subscriber_url, trigger_event, signing_key_ref, active
		from booking_webhook_subscriptions
		where trigger_event = $1
			and active = true
		order by id
	`, string(triggerEvent))
	if err != nil {
		return nil, fmt.Errorf("read webhook subscriptions: %w", err)
	}
	defer rows.Close()

	subscriptions := []WebhookSubscription{}
	for rows.Next() {
		var subscription WebhookSubscription
		var trigger string
		if err := rows.Scan(&subscription.ID, &subscription.SubscriberURL, &trigger, &subscription.SigningKeyRef, &subscription.Active); err != nil {
			return nil, fmt.Errorf("scan webhook subscription: %w", err)
		}
		subscription.TriggerEvent = WebhookTriggerEvent(trigger)
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read webhook subscription rows: %w", err)
	}
	return subscriptions, nil
}
