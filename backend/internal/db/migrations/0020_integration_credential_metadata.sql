create table if not exists integration_credential_metadata (
	user_id integer not null,
	credential_ref text not null,
	app_slug text not null,
	app_category text not null,
	provider text not null,
	account_ref text not null,
	account_label text not null,
	status text not null,
	scopes text[] not null default '{}',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (user_id, credential_ref)
);

create unique index if not exists integration_credential_metadata_provider_account_idx
	on integration_credential_metadata (user_id, provider, account_ref);
