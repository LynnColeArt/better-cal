create table if not exists integration_app_install_handoff_sessions (
	handoff_state_ref text primary key,
	install_intent_ref text not null references integration_app_install_intents(install_intent_ref) on delete cascade,
	user_id integer not null,
	expires_at timestamptz not null,
	consumed_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists integration_app_install_handoff_sessions_intent_idx
	on integration_app_install_handoff_sessions (install_intent_ref, user_id, expires_at);

create index if not exists integration_app_install_handoff_sessions_consumable_idx
	on integration_app_install_handoff_sessions (user_id, handoff_state_ref, expires_at)
	where consumed_at is null;
