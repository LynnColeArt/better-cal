create table if not exists integration_app_install_authorization_previews (
	authorization_preview_ref text primary key,
	handoff_state_ref text not null references integration_app_install_handoff_sessions(handoff_state_ref) on delete cascade,
	install_intent_ref text not null references integration_app_install_intents(install_intent_ref) on delete cascade,
	user_id integer not null,
	app_slug text not null,
	provider text not null,
	auth_type text not null,
	requested_scopes text[] not null default '{}',
	preview_status text not null default 'preview_only' check (preview_status = 'preview_only'),
	requested_at timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (handoff_state_ref)
);

create index if not exists integration_app_install_authorization_previews_intent_idx
	on integration_app_install_authorization_previews (install_intent_ref, user_id, requested_at);
