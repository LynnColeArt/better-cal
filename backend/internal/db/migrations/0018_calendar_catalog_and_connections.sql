create table if not exists calendar_connections (
	user_id integer not null,
	connection_ref text not null,
	provider text not null,
	account_ref text not null,
	account_email text not null,
	status text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (user_id, connection_ref)
);

create table if not exists calendar_catalog (
	user_id integer not null,
	calendar_ref text not null,
	connection_ref text not null,
	provider text not null,
	external_id text not null,
	name text not null,
	is_primary boolean not null default false,
	writable boolean not null default false,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (user_id, calendar_ref),
	constraint calendar_catalog_connection_fkey
		foreign key (user_id, connection_ref)
		references calendar_connections (user_id, connection_ref)
		on delete cascade
);

create unique index if not exists calendar_catalog_external_idx
	on calendar_catalog (user_id, provider, external_id);
