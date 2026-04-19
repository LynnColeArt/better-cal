package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	ReadByUID(ctx context.Context, uid string) (Booking, bool, error)
	ReadByIdempotencyKey(ctx context.Context, key string) (Booking, bool, error)
	SaveCreated(ctx context.Context, booking Booking, idempotencyKey string, effects []PlannedSideEffect) (Booking, bool, error)
	Save(ctx context.Context, effects []PlannedSideEffect, bookings ...Booking) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadByUID(ctx context.Context, uid string) (Booking, bool, error) {
	bookingValue, ok, err := r.readStructuredBooking(ctx, uid)
	if err != nil {
		return Booking{}, false, err
	}
	if ok {
		return bookingValue, true, nil
	}
	return r.readFixtureBooking(ctx, uid)
}

func (r *PostgresRepository) ReadByIdempotencyKey(ctx context.Context, key string) (Booking, bool, error) {
	var uid string
	err := r.pool.QueryRow(ctx, `
		select booking_uid
		from booking_idempotency_keys
		where idempotency_key = $1
	`, key).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, nil
	}
	if err != nil {
		return Booking{}, false, fmt.Errorf("read idempotency booking uid: %w", err)
	}
	return r.ReadByUID(ctx, uid)
}

func (r *PostgresRepository) readStructuredBooking(ctx context.Context, uid string) (Booking, bool, error) {
	var bookingValue Booking
	var responsesRaw []byte
	var metadataRaw []byte
	err := r.pool.QueryRow(ctx, `
		select uid, booking_id, title, status, start_time, end_time, event_type_id, responses, metadata, created_at_wire, updated_at_wire, request_id
		from bookings
		where uid = $1
	`, uid).Scan(
		&bookingValue.UID,
		&bookingValue.ID,
		&bookingValue.Title,
		&bookingValue.Status,
		&bookingValue.Start,
		&bookingValue.End,
		&bookingValue.EventTypeID,
		&responsesRaw,
		&metadataRaw,
		&bookingValue.CreatedAt,
		&bookingValue.UpdatedAt,
		&bookingValue.RequestID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, nil
	}
	if err != nil {
		return Booking{}, false, fmt.Errorf("read booking row: %w", err)
	}

	bookingValue.Responses, err = decodeObject(responsesRaw, "booking responses")
	if err != nil {
		return Booking{}, false, err
	}
	bookingValue.Metadata, err = decodeObject(metadataRaw, "booking metadata")
	if err != nil {
		return Booking{}, false, err
	}

	rows, err := r.pool.Query(ctx, `
		select attendee_id, name, email, time_zone
		from booking_attendees
		where booking_uid = $1
		order by position
	`, uid)
	if err != nil {
		return Booking{}, false, fmt.Errorf("read booking attendees: %w", err)
	}
	defer rows.Close()

	attendees := []Attendee{}
	for rows.Next() {
		var attendee Attendee
		if err := rows.Scan(&attendee.ID, &attendee.Name, &attendee.Email, &attendee.TimeZone); err != nil {
			return Booking{}, false, fmt.Errorf("scan booking attendee: %w", err)
		}
		attendees = append(attendees, attendee)
	}
	if err := rows.Err(); err != nil {
		return Booking{}, false, fmt.Errorf("read booking attendee rows: %w", err)
	}
	bookingValue.Attendees = attendees
	return bookingValue, true, nil
}

func (r *PostgresRepository) readFixtureBooking(ctx context.Context, uid string) (Booking, bool, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `select payload from booking_fixtures where uid = $1`, uid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, nil
	}
	if err != nil {
		return Booking{}, false, fmt.Errorf("read booking fixture: %w", err)
	}

	bookingValue, err := decodeBooking(raw)
	if err != nil {
		return Booking{}, false, err
	}
	return bookingValue, true, nil
}

func (r *PostgresRepository) SaveCreated(ctx context.Context, booking Booking, idempotencyKey string, effects []PlannedSideEffect) (Booking, bool, error) {
	var duplicateUID string
	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		if idempotencyKey != "" {
			if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
				return fmt.Errorf("lock idempotency key: %w", err)
			}
			err := tx.QueryRow(ctx, `
				select booking_uid
				from booking_idempotency_keys
				where idempotency_key = $1
				for update
			`, idempotencyKey).Scan(&duplicateUID)
			if err == nil {
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("read locked idempotency booking uid: %w", err)
			}
		}
		if err := saveBooking(ctx, tx, booking); err != nil {
			return err
		}
		if err := savePlannedSideEffects(ctx, tx, effects); err != nil {
			return err
		}
		if idempotencyKey == "" {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			insert into booking_idempotency_keys (idempotency_key, booking_uid)
			values ($1, $2)
		`, idempotencyKey, booking.UID); err != nil {
			return fmt.Errorf("save idempotency key: %w", err)
		}
		return nil
	}); err != nil {
		return Booking{}, false, err
	}
	if duplicateUID == "" {
		return booking, false, nil
	}
	duplicate, ok, err := r.ReadByUID(ctx, duplicateUID)
	if err != nil {
		return Booking{}, false, err
	}
	if !ok {
		return Booking{}, false, fmt.Errorf("read duplicate idempotency booking %q: not found", duplicateUID)
	}
	return duplicate, true, nil
}

func (r *PostgresRepository) Save(ctx context.Context, effects []PlannedSideEffect, bookings ...Booking) error {
	return db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		for _, bookingValue := range bookings {
			if err := saveBooking(ctx, tx, bookingValue); err != nil {
				return err
			}
		}
		return savePlannedSideEffects(ctx, tx, effects)
	})
}

func (r *PostgresRepository) ClaimPlannedSideEffects(ctx context.Context, limit int) ([]PlannedSideEffectRecord, error) {
	if limit <= 0 {
		limit = defaultSideEffectClaimLimit
	}

	records := []PlannedSideEffectRecord{}
	if err := db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			with candidate as (
				select id
				from booking_planned_side_effects
				where status in ('planned', 'failed')
					and next_attempt_at <= now()
				order by created_at, id
				limit $1
				for update skip locked
			)
			update booking_planned_side_effects side_effect
			set status = 'processing',
				locked_at = now(),
				attempts = side_effect.attempts + 1,
				last_error = null
			from candidate
			where side_effect.id = candidate.id
			returning side_effect.id,
				side_effect.name,
				side_effect.booking_uid,
				side_effect.request_id,
				side_effect.attempts,
				side_effect.status
		`, limit)
		if err != nil {
			return fmt.Errorf("claim planned side effects: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var record PlannedSideEffectRecord
			var name string
			if err := rows.Scan(&record.ID, &name, &record.BookingUID, &record.RequestID, &record.Attempts, &record.Status); err != nil {
				return fmt.Errorf("scan planned side effect: %w", err)
			}
			record.Name = SideEffectName(name)
			records = append(records, record)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read planned side effects: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *PostgresRepository) MarkPlannedSideEffectDelivered(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		update booking_planned_side_effects
		set status = 'delivered',
			delivered_at = now(),
			locked_at = null,
			last_error = null
		where id = $1
			and status = 'processing'
	`, id)
	if err != nil {
		return fmt.Errorf("mark planned side effect delivered: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark planned side effect delivered: side effect %d was not processing", id)
	}
	return nil
}

func (r *PostgresRepository) MarkPlannedSideEffectFailed(ctx context.Context, id int64, dispatchErr error) error {
	tag, err := r.pool.Exec(ctx, `
		update booking_planned_side_effects
		set status = 'failed',
			locked_at = null,
			last_error = $2,
			next_attempt_at = now() + (least(60, greatest(attempts, 1) * 5) * interval '1 second')
		where id = $1
			and status = 'processing'
	`, id, safeDispatchError(dispatchErr))
	if err != nil {
		return fmt.Errorf("mark planned side effect failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark planned side effect failed: side effect %d was not processing", id)
	}
	return nil
}

func saveBooking(ctx context.Context, tx db.Tx, booking Booking) error {
	if err := saveStructuredBooking(ctx, tx, booking); err != nil {
		return err
	}
	return saveFixtureBooking(ctx, tx, booking)
}

func saveStructuredBooking(ctx context.Context, tx db.Tx, booking Booking) error {
	responsesRaw, err := json.Marshal(objectOrEmpty(booking.Responses))
	if err != nil {
		return fmt.Errorf("encode booking responses: %w", err)
	}
	metadataRaw, err := json.Marshal(objectOrEmpty(booking.Metadata))
	if err != nil {
		return fmt.Errorf("encode booking metadata: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into bookings (
			uid,
			booking_id,
			title,
			status,
			start_time,
			end_time,
			event_type_id,
			responses,
			metadata,
			created_at_wire,
			updated_at_wire,
			request_id
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		on conflict (uid) do update set
			booking_id = excluded.booking_id,
			title = excluded.title,
			status = excluded.status,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			event_type_id = excluded.event_type_id,
			responses = excluded.responses,
			metadata = excluded.metadata,
			created_at_wire = excluded.created_at_wire,
			updated_at_wire = excluded.updated_at_wire,
			request_id = excluded.request_id,
			updated_at = now()
	`, booking.UID, booking.ID, booking.Title, booking.Status, booking.Start, booking.End, booking.EventTypeID, string(responsesRaw), string(metadataRaw), booking.CreatedAt, booking.UpdatedAt, booking.RequestID); err != nil {
		return fmt.Errorf("save booking row: %w", err)
	}

	if _, err := tx.Exec(ctx, `delete from booking_attendees where booking_uid = $1`, booking.UID); err != nil {
		return fmt.Errorf("delete booking attendees: %w", err)
	}
	for position, attendee := range booking.Attendees {
		if _, err := tx.Exec(ctx, `
			insert into booking_attendees (booking_uid, position, attendee_id, name, email, time_zone)
			values ($1, $2, $3, $4, $5, $6)
		`, booking.UID, position, attendee.ID, attendee.Name, attendee.Email, attendee.TimeZone); err != nil {
			return fmt.Errorf("save booking attendee: %w", err)
		}
	}
	return nil
}

func saveFixtureBooking(ctx context.Context, tx db.Tx, booking Booking) error {
	raw, err := json.Marshal(booking)
	if err != nil {
		return fmt.Errorf("encode booking fixture: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into booking_fixtures (uid, payload)
		values ($1, $2)
		on conflict (uid) do update set
			payload = excluded.payload,
			updated_at = now()
	`, booking.UID, string(raw)); err != nil {
		return fmt.Errorf("save booking fixture: %w", err)
	}
	return nil
}

func savePlannedSideEffects(ctx context.Context, tx db.Tx, effects []PlannedSideEffect) error {
	for _, effect := range effects {
		if effect.Name == "" || effect.BookingUID == "" || effect.RequestID == "" {
			return fmt.Errorf("invalid planned side effect")
		}
		if _, err := tx.Exec(ctx, `
			insert into booking_planned_side_effects (booking_uid, name, request_id)
			values ($1, $2, $3)
			on conflict (booking_uid, name, request_id) do nothing
		`, effect.BookingUID, string(effect.Name), effect.RequestID); err != nil {
			return fmt.Errorf("save planned side effect: %w", err)
		}
	}
	return nil
}

func objectOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func decodeObject(raw []byte, label string) (map[string]any, error) {
	value := map[string]any{}
	if len(raw) == 0 {
		return value, nil
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return value, nil
}

func decodeBooking(raw []byte) (Booking, error) {
	var bookingValue Booking
	if err := json.Unmarshal(raw, &bookingValue); err != nil {
		return Booking{}, fmt.Errorf("decode booking fixture: %w", err)
	}
	return bookingValue, nil
}

func safeDispatchError(error) string {
	return "dispatch failed"
}
