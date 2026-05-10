create table if not exists integration_app_install_callback_preflights (
	callback_preflight_ref text primary key,
	handoff_state_ref text not null references integration_app_install_handoff_sessions(handoff_state_ref) on delete cascade,
	install_intent_ref text not null references integration_app_install_intents(install_intent_ref) on delete cascade,
	user_id integer not null,
	app_slug text not null,
	provider text not null,
	callback_status text not null check (callback_status in ('provider_authorized', 'provider_denied')),
	preflight_status text not null default 'accepted',
	received_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (handoff_state_ref)
);

create index if not exists integration_app_install_callback_preflights_intent_idx
	on integration_app_install_callback_preflights (install_intent_ref, user_id, received_at);
