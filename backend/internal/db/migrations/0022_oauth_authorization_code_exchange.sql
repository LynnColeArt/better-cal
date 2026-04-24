create table if not exists oauth_authorization_codes (
	code_sha256 text primary key check (code_sha256 ~ '^[0-9a-f]{64}$'),
	client_id text not null references oauth_clients (client_id) on delete cascade,
	redirect_uri text not null check (redirect_uri <> ''),
	user_id integer not null,
	user_uuid text not null,
	principal_type text not null,
	username text not null,
	email text not null,
	permissions text[] not null default '{}',
	scopes text[] not null default '{}',
	expires_at timestamptz not null,
	code_created_at timestamptz not null,
	consumed_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists oauth_authorization_codes_client_idx
	on oauth_authorization_codes (client_id, expires_at);

create table if not exists oauth_tokens (
	access_token_sha256 text primary key check (access_token_sha256 ~ '^[0-9a-f]{64}$'),
	refresh_token_sha256 text not null unique check (refresh_token_sha256 ~ '^[0-9a-f]{64}$'),
	client_id text not null references oauth_clients (client_id) on delete cascade,
	user_id integer not null,
	user_uuid text not null,
	principal_type text not null,
	username text not null,
	email text not null,
	permissions text[] not null default '{}',
	scopes text[] not null default '{}',
	access_expires_at timestamptz not null,
	refresh_expires_at timestamptz not null,
	revoked_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists oauth_tokens_client_user_idx
	on oauth_tokens (client_id, user_id, created_at desc);
