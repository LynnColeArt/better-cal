alter table booking_planned_side_effects
	add column if not exists payload jsonb not null default '{}';

create table if not exists booking_webhook_deliveries (
	id bigserial primary key,
	side_effect_id bigint not null references booking_planned_side_effects(id) on delete cascade,
	booking_uid text not null references bookings(uid) on delete cascade,
	request_id text not null,
	trigger_event text not null,
	content_type text not null default 'application/json',
	created_at_wire text not null,
	body jsonb not null,
	delivered_at timestamptz not null default now(),
	unique (side_effect_id)
);

create index if not exists booking_webhook_deliveries_booking_idx
	on booking_webhook_deliveries (booking_uid, trigger_event, delivered_at);
