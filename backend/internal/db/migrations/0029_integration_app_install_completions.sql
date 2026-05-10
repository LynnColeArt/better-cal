create table if not exists integration_app_install_completions (
	install_completion_ref text primary key,
	provider_exchange_ref text not null references integration_app_install_provider_exchanges(provider_exchange_ref) on delete cascade,
	install_intent_ref text not null references integration_app_install_intents(install_intent_ref) on delete cascade,
	user_id integer not null,
	app_slug text not null,
	provider text not null,
	completion_status text not null check (completion_status in ('installed', 'blocked')),
	completion_mode text not null default 'simulated' check (completion_mode = 'simulated'),
	completed_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (provider_exchange_ref),
	unique (install_intent_ref)
);

create index if not exists integration_app_install_completions_intent_idx
	on integration_app_install_completions (install_intent_ref, user_id, completed_at);
