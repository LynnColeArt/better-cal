alter table booking_webhook_delivery_attempts
	add column if not exists attempt_count integer not null default 0,
	add column if not exists last_attempted_at timestamptz,
	add column if not exists delivered_at timestamptz,
	add column if not exists response_status integer,
	add column if not exists last_error text;

create index if not exists booking_webhook_delivery_attempts_pending_idx
	on booking_webhook_delivery_attempts (side_effect_id, delivered_at, id);
