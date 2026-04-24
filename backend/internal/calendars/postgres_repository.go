package calendars

import (
	"context"
	"errors"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadCalendarConnections(ctx context.Context, userID int) ([]CalendarConnection, error) {
	rows, err := r.pool.Query(ctx, `
		select connection_ref, provider, account_ref, account_email, status
		from calendar_connections
		where user_id = $1
		order by connection_ref
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("read calendar connections: %w", err)
	}
	defer rows.Close()

	connections := []CalendarConnection{}
	for rows.Next() {
		var connection CalendarConnection
		if err := rows.Scan(&connection.ConnectionRef, &connection.Provider, &connection.AccountRef, &connection.AccountEmail, &connection.Status); err != nil {
			return nil, fmt.Errorf("scan calendar connection: %w", err)
		}
		connections = append(connections, connection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read calendar connection rows: %w", err)
	}
	return connections, nil
}

func (r *PostgresRepository) SaveCalendarConnection(ctx context.Context, userID int, connection CalendarConnection) (CalendarConnection, error) {
	if connection.ConnectionRef == "" || connection.Provider == "" || connection.AccountRef == "" || connection.AccountEmail == "" || connection.Status == "" {
		return CalendarConnection{}, ErrInvalidSelectedCalendar
	}

	if _, err := r.pool.Exec(ctx, `
		insert into calendar_connections (
			user_id,
			connection_ref,
			provider,
			account_ref,
			account_email,
			status
		)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (user_id, connection_ref) do update set
			provider = excluded.provider,
			account_ref = excluded.account_ref,
			account_email = excluded.account_email,
			status = excluded.status,
			updated_at = now()
	`, userID, connection.ConnectionRef, connection.Provider, connection.AccountRef, connection.AccountEmail, connection.Status); err != nil {
		return CalendarConnection{}, fmt.Errorf("save calendar connection: %w", err)
	}
	return connection, nil
}

func (r *PostgresRepository) ReadCatalogCalendars(ctx context.Context, userID int) ([]CatalogCalendar, error) {
	rows, err := r.pool.Query(ctx, `
		select calendar_ref, connection_ref, provider, external_id, name, is_primary, writable
		from calendar_catalog
		where user_id = $1
		order by calendar_ref
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("read catalog calendars: %w", err)
	}
	defer rows.Close()

	calendars := []CatalogCalendar{}
	for rows.Next() {
		var calendar CatalogCalendar
		if err := rows.Scan(&calendar.CalendarRef, &calendar.ConnectionRef, &calendar.Provider, &calendar.ExternalID, &calendar.Name, &calendar.Primary, &calendar.Writable); err != nil {
			return nil, fmt.Errorf("scan catalog calendar: %w", err)
		}
		calendars = append(calendars, calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read catalog calendar rows: %w", err)
	}
	return calendars, nil
}

func (r *PostgresRepository) SaveCatalogCalendar(ctx context.Context, userID int, calendar CatalogCalendar) (CatalogCalendar, error) {
	if calendar.CalendarRef == "" || calendar.ConnectionRef == "" || calendar.Provider == "" || calendar.ExternalID == "" || calendar.Name == "" {
		return CatalogCalendar{}, ErrInvalidSelectedCalendar
	}

	if _, err := r.pool.Exec(ctx, `
		insert into calendar_catalog (
			user_id,
			calendar_ref,
			connection_ref,
			provider,
			external_id,
			name,
			is_primary,
			writable
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (user_id, calendar_ref) do update set
			connection_ref = excluded.connection_ref,
			provider = excluded.provider,
			external_id = excluded.external_id,
			name = excluded.name,
			is_primary = excluded.is_primary,
			writable = excluded.writable,
			updated_at = now()
	`, userID, calendar.CalendarRef, calendar.ConnectionRef, calendar.Provider, calendar.ExternalID, calendar.Name, calendar.Primary, calendar.Writable); err != nil {
		return CatalogCalendar{}, fmt.Errorf("save catalog calendar: %w", err)
	}
	return calendar, nil
}

func (r *PostgresRepository) ReadCatalogCalendar(ctx context.Context, userID int, calendarRef string) (CatalogCalendar, bool, error) {
	var calendar CatalogCalendar
	err := r.pool.QueryRow(ctx, `
		select calendar_ref, connection_ref, provider, external_id, name, is_primary, writable
		from calendar_catalog
		where user_id = $1 and calendar_ref = $2
	`, userID, calendarRef).Scan(&calendar.CalendarRef, &calendar.ConnectionRef, &calendar.Provider, &calendar.ExternalID, &calendar.Name, &calendar.Primary, &calendar.Writable)
	if errors.Is(err, pgx.ErrNoRows) {
		return CatalogCalendar{}, false, nil
	}
	if err != nil {
		return CatalogCalendar{}, false, fmt.Errorf("read catalog calendar by ref: %w", err)
	}
	return calendar, true, nil
}

func (r *PostgresRepository) ReadSelectedCalendars(ctx context.Context, userID int) ([]SelectedCalendar, error) {
	rows, err := r.pool.Query(ctx, `
		select calendar_ref, provider, external_id, name
		from selected_calendars
		where user_id = $1
		order by calendar_ref
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("read selected calendars: %w", err)
	}
	defer rows.Close()

	calendars := []SelectedCalendar{}
	for rows.Next() {
		var calendar SelectedCalendar
		if err := rows.Scan(&calendar.CalendarRef, &calendar.Provider, &calendar.ExternalID, &calendar.Name); err != nil {
			return nil, fmt.Errorf("scan selected calendar: %w", err)
		}
		calendars = append(calendars, calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read selected calendar rows: %w", err)
	}
	return calendars, nil
}

func (r *PostgresRepository) SaveSelectedCalendar(ctx context.Context, userID int, calendar SelectedCalendar) (SelectedCalendar, error) {
	if calendar.CalendarRef == "" || calendar.Provider == "" || calendar.ExternalID == "" || calendar.Name == "" {
		return SelectedCalendar{}, ErrInvalidSelectedCalendar
	}

	if _, err := r.pool.Exec(ctx, `
		insert into selected_calendars (
			user_id,
			calendar_ref,
			provider,
			external_id,
			name
		)
		values ($1, $2, $3, $4, $5)
		on conflict (user_id, calendar_ref) do update set
			provider = excluded.provider,
			external_id = excluded.external_id,
			name = excluded.name,
			updated_at = now()
	`, userID, calendar.CalendarRef, calendar.Provider, calendar.ExternalID, calendar.Name); err != nil {
		return SelectedCalendar{}, fmt.Errorf("save selected calendar: %w", err)
	}
	return calendar, nil
}

func (r *PostgresRepository) DeleteSelectedCalendar(ctx context.Context, userID int, calendarRef string) (DeleteSelectedCalendarResult, error) {
	if calendarRef == "" {
		return DeleteSelectedCalendarResult{}, ErrInvalidSelectedCalendarRef
	}

	result := DeleteSelectedCalendarResult{}
	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		var destinationRef string
		err := tx.QueryRow(ctx, `
			select calendar_ref
			from destination_calendars
			where user_id = $1
		`, userID).Scan(&destinationRef)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read destination calendar ref: %w", err)
		}
		result.ClearedDestination = destinationRef == calendarRef

		tag, err := tx.Exec(ctx, `
			delete from selected_calendars
			where user_id = $1 and calendar_ref = $2
		`, userID, calendarRef)
		if err != nil {
			return fmt.Errorf("delete selected calendar: %w", err)
		}
		result.Removed = tag.RowsAffected() > 0
		return nil
	}); err != nil {
		return DeleteSelectedCalendarResult{}, err
	}
	return result, nil
}

func (r *PostgresRepository) ReadDestinationCalendar(ctx context.Context, userID int) (SelectedCalendar, bool, error) {
	var calendar SelectedCalendar
	err := r.pool.QueryRow(ctx, `
		select selected.calendar_ref, selected.provider, selected.external_id, selected.name
		from destination_calendars destination
		join selected_calendars selected
			on selected.user_id = destination.user_id
			and selected.calendar_ref = destination.calendar_ref
		where destination.user_id = $1
	`, userID).Scan(&calendar.CalendarRef, &calendar.Provider, &calendar.ExternalID, &calendar.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return SelectedCalendar{}, false, nil
	}
	if err != nil {
		return SelectedCalendar{}, false, fmt.Errorf("read destination calendar: %w", err)
	}
	return calendar, true, nil
}

func (r *PostgresRepository) SetDestinationCalendar(ctx context.Context, userID int, calendarRef string) (SelectedCalendar, bool, error) {
	if calendarRef == "" {
		return SelectedCalendar{}, false, ErrInvalidDestinationCalendarRef
	}

	calendar, ok, err := r.readSelectedCalendar(ctx, userID, calendarRef)
	if err != nil {
		return SelectedCalendar{}, false, err
	}
	if !ok {
		return SelectedCalendar{}, false, nil
	}

	if _, err := r.pool.Exec(ctx, `
		insert into destination_calendars (
			user_id,
			calendar_ref
		)
		values ($1, $2)
		on conflict (user_id) do update set
			calendar_ref = excluded.calendar_ref,
			updated_at = now()
	`, userID, calendarRef); err != nil {
		return SelectedCalendar{}, false, fmt.Errorf("set destination calendar: %w", err)
	}
	return calendar, true, nil
}

func (r *PostgresRepository) readSelectedCalendar(ctx context.Context, userID int, calendarRef string) (SelectedCalendar, bool, error) {
	var calendar SelectedCalendar
	err := r.pool.QueryRow(ctx, `
		select calendar_ref, provider, external_id, name
		from selected_calendars
		where user_id = $1 and calendar_ref = $2
	`, userID, calendarRef).Scan(&calendar.CalendarRef, &calendar.Provider, &calendar.ExternalID, &calendar.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return SelectedCalendar{}, false, nil
	}
	if err != nil {
		return SelectedCalendar{}, false, fmt.Errorf("read selected calendar by ref: %w", err)
	}
	return calendar, true, nil
}
