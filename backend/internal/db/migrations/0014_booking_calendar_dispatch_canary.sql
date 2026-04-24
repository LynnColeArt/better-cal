create table if not exists booking_calendar_dispatches (
	id bigserial primary key,
	side_effect_id bigint not null references booking_planned_side_effects(id) on delete cascade,
	booking_uid text not null references bookings(uid) on delete cascade,
	request_id text not null,
	action text not null,
	content_type text not null default 'application/json',
	created_at_wire text not null,
	body jsonb not null,
	dispatched_at timestamptz not null default now(),
	unique (side_effect_id)
);

create index if not exists booking_calendar_dispatches_booking_idx
	on booking_calendar_dispatches (booking_uid, action, dispatched_at);

create table if not exists booking_calendar_dispatch_attempts (
	id bigserial primary key,
	dispatch_id bigint not null references booking_calendar_dispatches(id) on delete cascade,
	side_effect_id bigint not null references booking_planned_side_effects(id) on delete cascade,
	target_url text not null,
	action text not null,
	content_type text not null default 'application/json',
	body text not null,
	response_status integer,
	last_error text,
	attempt_count integer not null default 0,
	last_attempted_at timestamptz,
	delivered_at timestamptz,
	unique (dispatch_id, target_url)
);

create index if not exists booking_calendar_dispatch_attempts_pending_idx
	on booking_calendar_dispatch_attempts (side_effect_id, delivered_at, id);
