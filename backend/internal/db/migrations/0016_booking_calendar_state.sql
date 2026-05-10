alter table bookings
	add column if not exists selected_calendar_ref text,
	add column if not exists destination_calendar_ref text,
	add column if not exists external_calendar_event_id text;
