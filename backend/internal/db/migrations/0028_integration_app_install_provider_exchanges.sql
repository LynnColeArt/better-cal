create table if not exists integration_app_install_provider_exchanges (
	provider_exchange_ref text primary key,
	callback_preflight_ref text not null references integration_app_install_callback_preflights(callback_preflight_ref) on delete cascade,
	handoff_state_ref text not null references integration_app_install_handoff_sessions(handoff_state_ref) on delete cascade,
	install_intent_ref text not null references integration_app_install_intents(install_intent_ref) on delete cascade,
	user_id integer not null,
	app_slug text not null,
	provider text not null,
	exchange_status text not null check (exchange_status in ('queued', 'blocked')),
	exchange_mode text not null default 'simulated' check (exchange_mode = 'simulated'),
	recorded_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (callback_preflight_ref),
	unique (handoff_state_ref)
);

create index if not exists integration_app_install_provider_exchanges_intent_idx
	on integration_app_install_provider_exchanges (install_intent_ref, user_id, recorded_at);
