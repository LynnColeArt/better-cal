package slots

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReadAvailable(ctx context.Context, requestID string, req Request) (Response, bool, error) {
	var eventTypeExists bool
	if err := r.pool.QueryRow(ctx, `
		select exists(select 1 from event_types where event_type_id = $1)
	`, req.EventTypeID).Scan(&eventTypeExists); err != nil {
		return Response{}, false, fmt.Errorf("read event type existence: %w", err)
	}
	if !eventTypeExists {
		return Response{}, false, nil
	}

	startAt, err := time.Parse(time.RFC3339Nano, req.Start)
	if err != nil {
		return Response{}, false, fmt.Errorf("parse start time: %w", err)
	}
	endAt, err := time.Parse(time.RFC3339Nano, req.End)
	if err != nil {
		return Response{}, false, fmt.Errorf("parse end time: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		select slot_time_wire, duration_minutes
		from availability_slots
		where event_type_id = $1
			and time_zone = $2
			and slot_time_at >= $3
			and slot_time_at < $4
		order by slot_time_at
	`, req.EventTypeID, req.TimeZone, startAt, endAt)
	if err != nil {
		return Response{}, false, fmt.Errorf("read availability slots: %w", err)
	}
	defer rows.Close()

	slotsByDay := map[string][]Slot{}
	for rows.Next() {
		var slot Slot
		if err := rows.Scan(&slot.Time, &slot.Duration); err != nil {
			return Response{}, false, fmt.Errorf("scan availability slot: %w", err)
		}
		day, err := slotDay(slot.Time, req.TimeZone)
		if err != nil {
			return Response{}, false, err
		}
		slotsByDay[day] = append(slotsByDay[day], slot)
	}
	if err := rows.Err(); err != nil {
		return Response{}, false, fmt.Errorf("read availability slot rows: %w", err)
	}

	return Response{
		EventTypeID: req.EventTypeID,
		TimeZone:    req.TimeZone,
		Start:       req.Start,
		End:         req.End,
		Slots:       slotsByDay,
		RequestID:   requestID,
	}, true, nil
}

func (r *PostgresRepository) BusyTimes(ctx context.Context, req Request) ([]BusyTime, error) {
	startAt, err := time.Parse(time.RFC3339Nano, req.Start)
	if err != nil {
		return nil, fmt.Errorf("parse busy start boundary: %w", err)
	}
	endAt, err := time.Parse(time.RFC3339Nano, req.End)
	if err != nil {
		return nil, fmt.Errorf("parse busy end boundary: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		select start_time, end_time
		from bookings
		where event_type_id = $1
			and status = 'accepted'
			and start_time::timestamptz < $3
			and end_time::timestamptz > $2
		order by start_time::timestamptz
	`, req.EventTypeID, startAt, endAt)
	if err != nil {
		return nil, fmt.Errorf("read internal busy times: %w", err)
	}
	defer rows.Close()

	busyTimes := []BusyTime{}
	for rows.Next() {
		var busyTime BusyTime
		if err := rows.Scan(&busyTime.Start, &busyTime.End); err != nil {
			return nil, fmt.Errorf("scan internal busy time: %w", err)
		}
		busyTimes = append(busyTimes, busyTime)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read internal busy time rows: %w", err)
	}
	return busyTimes, nil
}

func (r *PostgresRepository) SaveEventType(ctx context.Context, eventType EventType) error {
	_, err := r.pool.Exec(ctx, `
		insert into event_types (event_type_id, title, duration_minutes, time_zone, updated_at)
		values ($1, $2, $3, $4, now())
		on conflict (event_type_id) do update set
			title = excluded.title,
			duration_minutes = excluded.duration_minutes,
			time_zone = excluded.time_zone,
			updated_at = now()
	`, eventType.ID, eventType.Title, eventType.Duration, eventType.TimeZone)
	if err != nil {
		return fmt.Errorf("save event type: %w", err)
	}
	return nil
}

func (r *PostgresRepository) SaveAvailabilitySlot(ctx context.Context, slot AvailabilitySlot) error {
	slotAt, err := time.Parse(time.RFC3339Nano, slot.Time)
	if err != nil {
		return fmt.Errorf("parse slot time: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		insert into availability_slots (event_type_id, time_zone, slot_time_wire, slot_time_at, duration_minutes, updated_at)
		values ($1, $2, $3, $4, $5, now())
		on conflict (event_type_id, time_zone, slot_time_wire) do update set
			slot_time_at = excluded.slot_time_at,
			duration_minutes = excluded.duration_minutes,
			updated_at = now()
	`, slot.EventTypeID, slot.TimeZone, slot.Time, slotAt, slot.Duration)
	if err != nil {
		return fmt.Errorf("save availability slot: %w", err)
	}
	return nil
}

func SeedFixtureAvailability(ctx context.Context, repo Repository) error {
	if repo == nil {
		return errors.New("nil slots repository")
	}
	if err := repo.SaveEventType(ctx, FixtureEventType()); err != nil {
		return err
	}
	for _, slot := range FixtureAvailabilitySlots() {
		if err := repo.SaveAvailabilitySlot(ctx, slot); err != nil {
			return err
		}
	}
	return nil
}

func slotDay(slotTime string, timeZone string) (string, error) {
	parsed, err := time.Parse(time.RFC3339Nano, slotTime)
	if err != nil {
		return "", fmt.Errorf("parse slot day: %w", err)
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return "", validationError("INVALID_TIME_ZONE", "Time zone is invalid")
	}
	return parsed.In(location).Format("2006-01-02"), nil
}
