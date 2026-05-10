alter table calendar_connections
	add column if not exists status_code text,
	add column if not exists status_checked_at timestamptz;

alter table integration_credential_metadata
	add column if not exists status_code text,
	add column if not exists status_checked_at timestamptz;
