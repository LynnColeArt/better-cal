create table if not exists event_types (
	event_type_id integer primary key,
	title text not null check (title <> ''),
	duration_minutes integer not null check (duration_minutes > 0),
	time_zone text not null check (time_zone <> ''),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists availability_slots (
	event_type_id integer not null references event_types(event_type_id) on delete cascade,
	time_zone text not null check (time_zone <> ''),
	slot_time_wire text not null check (slot_time_wire <> ''),
	slot_time_at timestamptz not null,
	duration_minutes integer not null check (duration_minutes > 0),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (event_type_id, time_zone, slot_time_wire)
);

create index if not exists availability_slots_lookup_idx
	on availability_slots (event_type_id, time_zone, slot_time_at);
