create table if not exists booking_fixtures (
	uid text primary key,
	payload jsonb not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists booking_idempotency_keys (
	idempotency_key text primary key,
	booking_uid text not null references booking_fixtures(uid) on delete cascade,
	created_at timestamptz not null default now()
);
