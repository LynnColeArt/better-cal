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
	SaveCreated(ctx context.Context, booking Booking, idempotencyKey string) error
	Save(ctx context.Context, bookings ...Booking) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadByUID(ctx context.Context, uid string) (Booking, bool, error) {
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

func (r *PostgresRepository) ReadByIdempotencyKey(ctx context.Context, key string) (Booking, bool, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		select f.payload
		from booking_idempotency_keys i
		join booking_fixtures f on f.uid = i.booking_uid
		where i.idempotency_key = $1
	`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, nil
	}
	if err != nil {
		return Booking{}, false, fmt.Errorf("read idempotency booking fixture: %w", err)
	}

	bookingValue, err := decodeBooking(raw)
	if err != nil {
		return Booking{}, false, err
	}
	return bookingValue, true, nil
}

func (r *PostgresRepository) SaveCreated(ctx context.Context, booking Booking, idempotencyKey string) error {
	return db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		if err := saveBooking(ctx, tx, booking); err != nil {
			return err
		}
		if idempotencyKey == "" {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			insert into booking_idempotency_keys (idempotency_key, booking_uid)
			values ($1, $2)
			on conflict (idempotency_key) do nothing
		`, idempotencyKey, booking.UID); err != nil {
			return fmt.Errorf("save idempotency key: %w", err)
		}
		return nil
	})
}

func (r *PostgresRepository) Save(ctx context.Context, bookings ...Booking) error {
	return db.WithTx(ctx, r.pool, func(tx db.Tx) error {
		for _, bookingValue := range bookings {
			if err := saveBooking(ctx, tx, bookingValue); err != nil {
				return err
			}
		}
		return nil
	})
}

func saveBooking(ctx context.Context, tx db.Tx, booking Booking) error {
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

func decodeBooking(raw []byte) (Booking, error) {
	var bookingValue Booking
	if err := json.Unmarshal(raw, &bookingValue); err != nil {
		return Booking{}, fmt.Errorf("decode booking fixture: %w", err)
	}
	return bookingValue, nil
}
