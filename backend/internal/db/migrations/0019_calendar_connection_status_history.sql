create table if not exists calendar_connection_status_history (
	id bigserial primary key,
	user_id integer not null,
	connection_ref text not null,
	provider text not null,
	previous_status text not null,
	next_status text not null,
	reason text not null,
	created_at timestamptz not null default now(),
	constraint calendar_connection_status_history_connection_fkey
		foreign key (user_id, connection_ref)
		references calendar_connections (user_id, connection_ref)
		on delete cascade
);

create index if not exists calendar_connection_status_history_user_connection_idx
	on calendar_connection_status_history (user_id, connection_ref, created_at desc);
