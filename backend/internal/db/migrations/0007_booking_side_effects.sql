create table if not exists booking_planned_side_effects (
	id bigserial primary key,
	booking_uid text not null references bookings(uid) on delete cascade,
	name text not null,
	request_id text not null,
	status text not null default 'planned',
	created_at timestamptz not null default now(),
	unique (booking_uid, name, request_id)
);
