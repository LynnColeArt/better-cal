create table if not exists bookings (
	uid text primary key,
	booking_id integer not null,
	title text not null,
	status text not null,
	start_time text not null,
	end_time text not null,
	event_type_id integer not null,
	responses jsonb not null default '{}',
	metadata jsonb not null default '{}',
	created_at_wire text not null,
	updated_at_wire text not null,
	request_id text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists booking_attendees (
	booking_uid text not null references bookings(uid) on delete cascade,
	position integer not null,
	attendee_id integer,
	name text not null,
	email text not null,
	time_zone text not null,
	primary key (booking_uid, position)
);
