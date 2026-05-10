alter table booking_webhook_subscriptions
	add column if not exists failure_count integer not null default 0,
	add column if not exists disabled_at timestamptz,
	add column if not exists disabled_reason text;

alter table booking_webhook_delivery_attempts
	add column if not exists dead_lettered_at timestamptz;

create index if not exists booking_webhook_delivery_attempts_actionable_idx
	on booking_webhook_delivery_attempts (side_effect_id, id)
	where delivered_at is null
		and dead_lettered_at is null;

create index if not exists booking_webhook_subscriptions_disabled_idx
	on booking_webhook_subscriptions (active, disabled_at);
