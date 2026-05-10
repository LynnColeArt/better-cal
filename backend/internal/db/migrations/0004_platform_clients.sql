create table if not exists platform_clients (
	client_id text primary key check (client_id <> ''),
	secret_sha256 text not null check (secret_sha256 ~ '^[0-9a-f]{64}$'),
	name text not null check (name <> ''),
	organization_id integer not null,
	permissions text[] not null default '{}',
	policy_permissions text[] not null default '{}',
	client_created_at timestamptz not null,
	client_updated_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);
