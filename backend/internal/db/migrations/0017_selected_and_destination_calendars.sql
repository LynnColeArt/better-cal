create table if not exists selected_calendars (
	user_id integer not null,
	calendar_ref text not null,
	provider text not null,
	external_id text not null,
	name text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (user_id, calendar_ref)
);

create unique index if not exists selected_calendars_external_idx
	on selected_calendars (user_id, provider, external_id);

create table if not exists destination_calendars (
	user_id integer primary key,
	calendar_ref text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint destination_calendars_selected_fkey
		foreign key (user_id, calendar_ref)
		references selected_calendars (user_id, calendar_ref)
		on delete cascade
);
