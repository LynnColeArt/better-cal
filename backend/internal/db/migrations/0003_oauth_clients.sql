create table if not exists oauth_clients (
	client_id text primary key check (client_id <> ''),
	name text not null check (name <> ''),
	redirect_uris text[] not null default '{}',
	client_created_at timestamptz not null,
	client_updated_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);
