create table if not exists integration_app_installations (
	app_installation_ref text primary key,
	install_completion_ref text not null references integration_app_install_completions(install_completion_ref) on delete cascade,
	install_intent_ref text not null references integration_app_install_intents(install_intent_ref) on delete cascade,
	user_id integer not null,
	app_slug text not null,
	provider text not null,
	activation_status text not null check (activation_status = 'active'),
	activation_mode text not null default 'simulated' check (activation_mode = 'simulated'),
	activated_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (install_completion_ref),
	unique (install_intent_ref)
);

create index if not exists integration_app_installations_user_idx
	on integration_app_installations (user_id, activated_at);
