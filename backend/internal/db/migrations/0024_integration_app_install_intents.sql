create table if not exists integration_app_install_intents (
	install_intent_ref text primary key,
	user_id integer not null,
	app_slug text not null references integration_app_catalog(app_slug) on delete restrict,
	status text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists integration_app_install_intents_user_status_idx
	on integration_app_install_intents (user_id, status, created_at);

create index if not exists integration_app_install_intents_app_slug_idx
	on integration_app_install_intents (app_slug);
