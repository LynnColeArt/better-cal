package calendars

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
		select connection_ref, provider, account_ref, account_email, status, status_code, status_checked_at
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
		var statusCode sql.NullString
		var statusCheckedAt sql.NullTime
		if err := rows.Scan(&connection.ConnectionRef, &connection.Provider, &connection.AccountRef, &connection.AccountEmail, &connection.Status, &statusCode, &statusCheckedAt); err != nil {
			return nil, fmt.Errorf("scan calendar connection: %w", err)
		}
		if statusCode.Valid {
			connection.StatusCode = statusCode.String
		}
		if statusCheckedAt.Valid {
			connection.StatusCheckedAt = formatCalendarWireTime(statusCheckedAt.Time)
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

func (r *PostgresRepository) RecordCalendarConnectionStatusTransition(ctx context.Context, userID int, transition CalendarConnectionStatusTransition) error {
	if transition.ConnectionRef == "" || transition.Provider == "" || transition.PreviousStatus == "" || transition.NextStatus == "" || transition.Reason == "" {
		return ErrInvalidCalendarCatalogSnapshot
	}

	if _, err := r.pool.Exec(ctx, `
		insert into calendar_connection_status_history (
			user_id,
			connection_ref,
			provider,
			previous_status,
			next_status,
			reason
		)
		values ($1, $2, $3, $4, $5, $6)
	`, userID, transition.ConnectionRef, transition.Provider, transition.PreviousStatus, transition.NextStatus, transition.Reason); err != nil {
		return fmt.Errorf("record calendar connection status transition: %w", err)
	}
	return nil
}

func (r *PostgresRepository) SyncCalendarCatalog(ctx context.Context, userID int, connections []CalendarConnection, catalog []CatalogCalendar, transitionReason string) ([]CalendarConnection, []CatalogCalendar, error) {
	if transitionReason == "" {
		return nil, nil, ErrInvalidCalendarCatalogSnapshot
	}
	if err := validateProviderCatalog(connections, catalog); err != nil {
		return nil, nil, err
	}

	var syncedConnections []CalendarConnection
	var syncedCatalog []CatalogCalendar
	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		existingConnections, err := readCalendarConnectionsTx(ctx, tx, userID, true)
		if err != nil {
			return err
		}
		existingCatalog, err := readCatalogCalendarsTx(ctx, tx, userID, true)
		if err != nil {
			return err
		}

		connectionByRef := map[string]CalendarConnection{}
		for _, connection := range connections {
			connectionByRef[connection.ConnectionRef] = connection
		}
		existingConnectionByRef := map[string]CalendarConnection{}
		for _, connection := range existingConnections {
			existingConnectionByRef[connection.ConnectionRef] = connection
		}
		catalogByRef := map[string]CatalogCalendar{}
		for _, calendar := range catalog {
			catalogByRef[calendar.CalendarRef] = calendar
		}

		for _, existing := range existingCatalog {
			if _, ok := catalogByRef[existing.CalendarRef]; ok {
				continue
			}
			if _, err := tx.Exec(ctx, `
				delete from selected_calendars
				where user_id = $1 and calendar_ref = $2
			`, userID, existing.CalendarRef); err != nil {
				return fmt.Errorf("delete stale selected calendar: %w", err)
			}
			if _, err := tx.Exec(ctx, `
				delete from calendar_catalog
				where user_id = $1 and calendar_ref = $2
			`, userID, existing.CalendarRef); err != nil {
				return fmt.Errorf("delete stale catalog calendar: %w", err)
			}
		}

		for _, connection := range connections {
			if err := upsertCalendarConnectionTx(ctx, tx, userID, connection); err != nil {
				return err
			}
			if previous, ok := existingConnectionByRef[connection.ConnectionRef]; ok && previous.Status != connection.Status {
				if err := recordCalendarConnectionStatusTransitionTx(ctx, tx, userID, CalendarConnectionStatusTransition{
					ConnectionRef:  connection.ConnectionRef,
					Provider:       connection.Provider,
					PreviousStatus: previous.Status,
					NextStatus:     connection.Status,
					Reason:         transitionReason,
				}); err != nil {
					return err
				}
			}
		}

		for _, calendar := range catalog {
			if err := upsertCatalogCalendarTx(ctx, tx, userID, calendar); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				update selected_calendars
				set provider = $3,
					external_id = $4,
					name = $5,
					updated_at = now()
				where user_id = $1 and calendar_ref = $2
			`, userID, calendar.CalendarRef, calendar.Provider, calendar.ExternalID, calendar.Name); err != nil {
				return fmt.Errorf("refresh selected calendar snapshot: %w", err)
			}
		}

		for _, existing := range existingConnections {
			if _, ok := connectionByRef[existing.ConnectionRef]; ok {
				continue
			}
			if _, err := tx.Exec(ctx, `
				delete from calendar_connections
				where user_id = $1 and connection_ref = $2
			`, userID, existing.ConnectionRef); err != nil {
				return fmt.Errorf("delete stale calendar connection: %w", err)
			}
		}

		syncedConnections, err = readCalendarConnectionsTx(ctx, tx, userID, false)
		if err != nil {
			return err
		}
		syncedCatalog, err = readCatalogCalendarsTx(ctx, tx, userID, false)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return syncedConnections, syncedCatalog, nil
}

func (r *PostgresRepository) RefreshCalendarConnectionStatuses(ctx context.Context, userID int, updates []CalendarConnectionStatusUpdate, checkedAt string, transitionReason string) ([]CalendarConnection, error) {
	if checkedAt == "" {
		checkedAt = currentConnectionStatusWireTime()
	}
	if transitionReason == "" {
		return nil, ErrInvalidCalendarStatusSnapshot
	}
	if err := validateCalendarConnectionStatusUpdatesForRepository(updates); err != nil {
		return nil, err
	}

	var refreshed []CalendarConnection
	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		existing, err := readCalendarConnectionsTx(ctx, tx, userID, true)
		if err != nil {
			return err
		}
		existingByRef := map[string]CalendarConnection{}
		for _, connection := range existing {
			existingByRef[connection.ConnectionRef] = connection
		}

		for _, update := range updates {
			previous, ok := existingByRef[update.ConnectionRef]
			if !ok || previous.Provider != update.Provider || previous.AccountRef != update.AccountRef {
				return ErrInvalidCalendarStatusSnapshot
			}
			if _, err := tx.Exec(ctx, `
				update calendar_connections
				set status = $5,
					status_code = nullif($6, ''),
					status_checked_at = $7::timestamptz,
					updated_at = now()
				where user_id = $1
					and connection_ref = $2
					and provider = $3
					and account_ref = $4
			`, userID, update.ConnectionRef, update.Provider, update.AccountRef, update.Status, update.StatusCode, checkedAt); err != nil {
				return fmt.Errorf("refresh calendar connection status: %w", err)
			}
			if previous.Status != update.Status {
				if err := recordCalendarConnectionStatusTransitionTx(ctx, tx, userID, CalendarConnectionStatusTransition{
					ConnectionRef:  update.ConnectionRef,
					Provider:       update.Provider,
					PreviousStatus: previous.Status,
					NextStatus:     update.Status,
					Reason:         transitionReason,
				}); err != nil {
					return err
				}
			}
		}
		refreshed, err = readCalendarConnectionsTx(ctx, tx, userID, false)
		return err
	}); err != nil {
		return nil, err
	}
	return refreshed, nil
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

func readCalendarConnectionsTx(ctx context.Context, tx db.Tx, userID int, lockRows bool) ([]CalendarConnection, error) {
	query := `
		select connection_ref, provider, account_ref, account_email, status, status_code, status_checked_at
		from calendar_connections
		where user_id = $1
		order by connection_ref
	`
	if lockRows {
		query += ` for update`
	}
	rows, err := tx.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("read calendar connections: %w", err)
	}
	defer rows.Close()

	connections := []CalendarConnection{}
	for rows.Next() {
		var connection CalendarConnection
		var statusCode sql.NullString
		var statusCheckedAt sql.NullTime
		if err := rows.Scan(&connection.ConnectionRef, &connection.Provider, &connection.AccountRef, &connection.AccountEmail, &connection.Status, &statusCode, &statusCheckedAt); err != nil {
			return nil, fmt.Errorf("scan calendar connection: %w", err)
		}
		if statusCode.Valid {
			connection.StatusCode = statusCode.String
		}
		if statusCheckedAt.Valid {
			connection.StatusCheckedAt = formatCalendarWireTime(statusCheckedAt.Time)
		}
		connections = append(connections, connection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read calendar connection rows: %w", err)
	}
	return connections, nil
}

func readCatalogCalendarsTx(ctx context.Context, tx db.Tx, userID int, lockRows bool) ([]CatalogCalendar, error) {
	query := `
		select calendar_ref, connection_ref, provider, external_id, name, is_primary, writable
		from calendar_catalog
		where user_id = $1
		order by calendar_ref
	`
	if lockRows {
		query += ` for update`
	}
	rows, err := tx.Query(ctx, query, userID)
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

func upsertCalendarConnectionTx(ctx context.Context, tx db.Tx, userID int, connection CalendarConnection) error {
	if _, err := tx.Exec(ctx, `
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
		return fmt.Errorf("save calendar connection: %w", err)
	}
	return nil
}

func validateCalendarConnectionStatusUpdatesForRepository(updates []CalendarConnectionStatusUpdate) error {
	seen := map[string]struct{}{}
	for _, update := range updates {
		if update.ConnectionRef == "" || update.Provider == "" || update.AccountRef == "" || update.Status == "" {
			return ErrInvalidCalendarStatusSnapshot
		}
		if _, ok := seen[update.ConnectionRef]; ok {
			return ErrInvalidCalendarStatusSnapshot
		}
		seen[update.ConnectionRef] = struct{}{}
	}
	return nil
}

func formatCalendarWireTime(value time.Time) string {
	return value.UTC().Format(statusWireTimeLayout)
}

func upsertCatalogCalendarTx(ctx context.Context, tx db.Tx, userID int, calendar CatalogCalendar) error {
	if _, err := tx.Exec(ctx, `
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
		return fmt.Errorf("save catalog calendar: %w", err)
	}
	return nil
}

func recordCalendarConnectionStatusTransitionTx(ctx context.Context, tx db.Tx, userID int, transition CalendarConnectionStatusTransition) error {
	if _, err := tx.Exec(ctx, `
		insert into calendar_connection_status_history (
			user_id,
			connection_ref,
			provider,
			previous_status,
			next_status,
			reason
		)
		values ($1, $2, $3, $4, $5, $6)
	`, userID, transition.ConnectionRef, transition.Provider, transition.PreviousStatus, transition.NextStatus, transition.Reason); err != nil {
		return fmt.Errorf("record calendar connection status transition: %w", err)
	}
	return nil
}
