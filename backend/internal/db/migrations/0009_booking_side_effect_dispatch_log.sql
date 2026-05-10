create table if not exists booking_side_effect_dispatch_log (
	id bigserial primary key,
	side_effect_id bigint not null references booking_planned_side_effects(id) on delete cascade,
	booking_uid text not null references bookings(uid) on delete cascade,
	name text not null,
	request_id text not null,
	dispatched_at timestamptz not null default now(),
	unique (side_effect_id)
);

create index if not exists booking_side_effect_dispatch_log_booking_idx
	on booking_side_effect_dispatch_log (booking_uid, dispatched_at);
