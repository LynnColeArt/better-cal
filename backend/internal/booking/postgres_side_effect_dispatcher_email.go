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

func (d PostgresSideEffectDispatcher) prepareEmailAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord) ([]EmailDeliveryAttempt, error) {
	if _, ok := emailDeliveryActionForSideEffect(effect.Name); !ok {
		return nil, nil
	}

	deliveryRecord, found, err := readEmailDelivery(ctx, tx, effect.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		deliveryRecord, err = d.buildEmailDelivery(ctx, effect)
		if err != nil {
			return nil, err
		}
		if deliveryRecord == nil {
			return nil, nil
		}
		deliveryID, err := upsertEmailDelivery(ctx, tx, effect, deliveryRecord)
		if err != nil {
			return nil, err
		}
		deliveryRecord.ID = deliveryID
	}

	attemptCount, err := countEmailDeliveryAttempts(ctx, tx, deliveryRecord.ID)
	if err != nil {
		return nil, err
	}
	if attemptCount == 0 {
		attempts, err := d.emailAttemptTemplates(deliveryRecord)
		if err != nil {
			return nil, err
		}
		if err := recordEmailDeliveryAttempts(ctx, tx, effect, deliveryRecord.ID, deliveryRecord, attempts); err != nil {
			return nil, err
		}
	}

	return readPendingEmailDeliveryAttempts(ctx, tx, effect.ID)
}

type emailDeliveryRecord struct {
	ID          int64
	Action      string
	ContentType string
	CreatedAt   string
	Body        string
}

func readEmailDelivery(ctx context.Context, tx db.Tx, sideEffectID int64) (*emailDeliveryRecord, bool, error) {
	var delivery emailDeliveryRecord
	err := tx.QueryRow(ctx, `
		select id, action, content_type, created_at_wire, body
		from booking_email_deliveries
		where side_effect_id = $1
	`, sideEffectID).Scan(&delivery.ID, &delivery.Action, &delivery.ContentType, &delivery.CreatedAt, &delivery.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read email delivery: %w", err)
	}
	return &delivery, true, nil
}

func upsertEmailDelivery(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, deliveryRecord *emailDeliveryRecord) (int64, error) {
	var deliveryID int64
	err := tx.QueryRow(ctx, `
		with inserted as (
			insert into booking_email_deliveries (
				side_effect_id,
				booking_uid,
				request_id,
				action,
				content_type,
				created_at_wire,
				body
			)
			values ($1, $2, $3, $4, $5, $6, $7)
			on conflict (side_effect_id) do nothing
			returning id
		)
		select id from inserted
		union all
		select id
		from booking_email_deliveries
		where side_effect_id = $1
		limit 1
	`, effect.ID, effect.BookingUID, effect.RequestID, deliveryRecord.Action, deliveryRecord.ContentType, deliveryRecord.CreatedAt, deliveryRecord.Body).Scan(&deliveryID)
	if err != nil {
		return 0, fmt.Errorf("record email delivery: %w", err)
	}
	return deliveryID, nil
}

func countEmailDeliveryAttempts(ctx context.Context, tx db.Tx, deliveryID int64) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `
		select count(*)
		from booking_email_delivery_attempts
		where delivery_id = $1
	`, deliveryID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count email delivery attempts: %w", err)
	}
	return count, nil
}

func recordEmailDeliveryAttempts(ctx context.Context, tx db.Tx, effect PlannedSideEffectRecord, deliveryID int64, deliveryRecord *emailDeliveryRecord, attempts []EmailDeliveryAttempt) error {
	for _, attempt := range attempts {
		if _, err := tx.Exec(ctx, `
			insert into booking_email_delivery_attempts (
				delivery_id,
				side_effect_id,
				target_url,
				action,
				content_type,
				body
			)
			values ($1, $2, $3, $4, $5, $6)
			on conflict (delivery_id, target_url) do nothing
		`, deliveryID, effect.ID, attempt.TargetURL, deliveryRecord.Action, deliveryRecord.ContentType, deliveryRecord.Body); err != nil {
			return fmt.Errorf("record email delivery attempt: %w", err)
		}
	}
	return nil
}

func readPendingEmailDeliveryAttempts(ctx context.Context, tx db.Tx, sideEffectID int64) ([]EmailDeliveryAttempt, error) {
	rows, err := tx.Query(ctx, `
		select id, delivery_id, side_effect_id, target_url, action, content_type, body
		from booking_email_delivery_attempts
		where side_effect_id = $1
			and delivered_at is null
		order by id
	`, sideEffectID)
	if err != nil {
		return nil, fmt.Errorf("read pending email delivery attempts: %w", err)
	}
	defer rows.Close()

	attempts := []EmailDeliveryAttempt{}
	for rows.Next() {
		var attempt EmailDeliveryAttempt
		if err := rows.Scan(
			&attempt.ID,
			&attempt.DeliveryID,
			&attempt.SideEffectID,
			&attempt.TargetURL,
			&attempt.Action,
			&attempt.ContentType,
			&attempt.Body,
		); err != nil {
			return nil, fmt.Errorf("scan pending email delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending email delivery attempt rows: %w", err)
	}
	return attempts, nil
}

func markEmailDeliveryDelivered(ctx context.Context, pool *pgxpool.Pool, attemptID int64, statusCode int) error {
	if pool == nil {
		return db.ErrNilPool
	}
	var updatedID int64
	err := pool.QueryRow(ctx, `
		update booking_email_delivery_attempts
		set delivered_at = now(),
			response_status = $2,
			last_error = null,
			last_attempted_at = now(),
			attempt_count = booking_email_delivery_attempts.attempt_count + 1
		where id = $1
			and delivered_at is null
		returning id
	`, attemptID, nullableStatusCode(statusCode)).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("email delivery attempt %d was not pending", attemptID)
	}
	if err != nil {
		return fmt.Errorf("update delivered email delivery attempt: %w", err)
	}
	return nil
}

func markEmailDeliveryFailed(ctx context.Context, pool *pgxpool.Pool, attemptID int64, statusCode int, deliveryErr error) error {
	if pool == nil {
		return db.ErrNilPool
	}
	var updatedID int64
	err := pool.QueryRow(ctx, `
		update booking_email_delivery_attempts
		set response_status = $2,
			last_error = $3,
			last_attempted_at = now(),
			attempt_count = booking_email_delivery_attempts.attempt_count + 1
		where id = $1
			and delivered_at is null
		returning id
	`, attemptID, nullableStatusCode(statusCode), safeEmailDeliveryError(deliveryErr)).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("email delivery attempt %d was not pending", attemptID)
	}
	if err != nil {
		return fmt.Errorf("update failed email delivery attempt: %w", err)
	}
	return nil
}

func (d PostgresSideEffectDispatcher) buildEmailDelivery(ctx context.Context, effect PlannedSideEffectRecord) (*emailDeliveryRecord, error) {
	envelope, ok, err := d.emailDeliveryEnvelope(ctx, effect)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	bodyRaw, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode email delivery body: %w", err)
	}
	return &emailDeliveryRecord{
		Action:      string(envelope.Action),
		ContentType: "application/json",
		CreatedAt:   envelope.CreatedAt,
		Body:        string(bodyRaw),
	}, nil
}

func (d PostgresSideEffectDispatcher) emailDeliveryEnvelope(ctx context.Context, effect PlannedSideEffectRecord) (EmailDeliveryEnvelope, bool, error) {
	if _, ok := emailDeliveryActionForSideEffect(effect.Name); !ok {
		return EmailDeliveryEnvelope{}, false, nil
	}
	if d.bookings == nil {
		return EmailDeliveryEnvelope{}, false, errors.New("email delivery requires a booking reader")
	}

	bookingValue, found, err := d.bookings.ReadByUID(ctx, effect.BookingUID)
	if err != nil {
		return EmailDeliveryEnvelope{}, false, fmt.Errorf("read booking for email delivery: %w", err)
	}
	if !found {
		return EmailDeliveryEnvelope{}, false, fmt.Errorf("read booking for email delivery %q: not found", effect.BookingUID)
	}

	envelope, ok := emailDeliveryEnvelopeForBooking(effect, bookingValue)
	return envelope, ok, nil
}

func (d PostgresSideEffectDispatcher) emailAttemptTemplates(deliveryRecord *emailDeliveryRecord) ([]EmailDeliveryAttempt, error) {
	if deliveryRecord == nil {
		return nil, nil
	}
	if d.emailDispatchURL == "" {
		return nil, errors.New("email delivery requires a target url")
	}
	return []EmailDeliveryAttempt{
		{
			TargetURL:   d.emailDispatchURL,
			Action:      deliveryRecord.Action,
			ContentType: deliveryRecord.ContentType,
			Body:        deliveryRecord.Body,
		},
	}, nil
}
