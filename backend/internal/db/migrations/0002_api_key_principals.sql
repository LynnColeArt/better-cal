create table if not exists api_key_principals (
	token_sha256 text primary key check (token_sha256 ~ '^[0-9a-f]{64}$'),
	user_id integer not null,
	user_uuid text not null,
	principal_type text not null,
	username text not null,
	email text not null,
	permissions text[] not null default '{}',
	principal_created_at timestamptz not null,
	principal_updated_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);
