create table if not exists booking_webhook_subscriptions (
	id bigserial primary key,
	subscriber_url text not null,
	trigger_event text not null,
	signing_key_ref text not null,
	active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (subscriber_url, trigger_event, signing_key_ref)
);

create index if not exists booking_webhook_subscriptions_trigger_idx
	on booking_webhook_subscriptions (trigger_event, active);

create table if not exists booking_webhook_delivery_attempts (
	id bigserial primary key,
	delivery_id bigint not null references booking_webhook_deliveries(id) on delete cascade,
	side_effect_id bigint not null references booking_planned_side_effects(id) on delete cascade,
	subscriber_id bigint not null references booking_webhook_subscriptions(id) on delete cascade,
	subscriber_url text not null,
	trigger_event text not null,
	content_type text not null default 'application/json',
	signature_header_name text not null default 'X-Cal-Signature-256',
	signature_header_value text not null,
	body text not null,
	attempted_at timestamptz not null default now(),
	unique (delivery_id, subscriber_id)
);

create index if not exists booking_webhook_delivery_attempts_side_effect_idx
	on booking_webhook_delivery_attempts (side_effect_id, attempted_at);
