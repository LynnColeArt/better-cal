package booking

import (
	"context"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSideEffectDispatcher struct {
	pool *pgxpool.Pool
}

func NewPostgresSideEffectDispatcher(pool *pgxpool.Pool) PostgresSideEffectDispatcher {
	return PostgresSideEffectDispatcher{pool: pool}
}

func (d PostgresSideEffectDispatcher) Dispatch(ctx context.Context, effect PlannedSideEffectRecord) error {
	if d.pool == nil {
		return db.ErrNilPool
	}
	if effect.ID == 0 || effect.Name == "" || effect.BookingUID == "" || effect.RequestID == "" {
		return fmt.Errorf("invalid planned side effect dispatch record")
	}

	if _, err := d.pool.Exec(ctx, `
		insert into booking_side_effect_dispatch_log (side_effect_id, booking_uid, name, request_id)
		values ($1, $2, $3, $4)
		on conflict (side_effect_id) do nothing
	`, effect.ID, effect.BookingUID, string(effect.Name), effect.RequestID); err != nil {
		return fmt.Errorf("record side effect dispatch: %w", err)
	}
	return nil
}
