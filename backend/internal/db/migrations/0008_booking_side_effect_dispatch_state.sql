alter table booking_planned_side_effects
	add column if not exists attempts integer not null default 0,
	add column if not exists locked_at timestamptz,
	add column if not exists delivered_at timestamptz,
	add column if not exists last_error text,
	add column if not exists next_attempt_at timestamptz not null default now();

create index if not exists booking_planned_side_effects_claim_idx
	on booking_planned_side_effects (status, next_attempt_at, created_at, id);
