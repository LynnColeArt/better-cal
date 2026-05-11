create table if not exists integration_provider_token_secrets (
	user_id integer not null,
	credential_ref text not null,
	app_slug text not null,
	provider text not null,
	account_ref text not null,
	key_ref text not null,
	sealed_payload bytea not null check (length(sealed_payload) > 0),
	sealed_payload_sha256 text not null check (sealed_payload_sha256 ~ '^[0-9a-f]{64}$'),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (user_id, credential_ref),
	unique (user_id, provider, account_ref)
);

create index if not exists integration_provider_token_secrets_provider_idx
	on integration_provider_token_secrets (provider, updated_at);
