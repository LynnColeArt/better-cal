create table if not exists integration_app_catalog (
	app_slug text primary key,
	app_category text not null,
	provider text not null,
	name text not null,
	description text not null,
	auth_type text not null,
	capabilities text[] not null default '{}',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists integration_app_catalog_provider_idx
	on integration_app_catalog (provider);
