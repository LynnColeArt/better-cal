package booking

import (
	"context"
	"errors"
	"fmt"
)

const defaultSideEffectClaimLimit = 25

type PlannedSideEffectRecord struct {
	ID         int64
	Name       SideEffectName
	BookingUID string
	RequestID  string
	Attempts   int
	Status     string
}

type PlannedSideEffectQueue interface {
	ClaimPlannedSideEffects(context.Context, int) ([]PlannedSideEffectRecord, error)
	MarkPlannedSideEffectDelivered(context.Context, int64) error
	MarkPlannedSideEffectFailed(context.Context, int64, error) error
}

type SideEffectDispatcher interface {
	Dispatch(context.Context, PlannedSideEffectRecord) error
}

type NoopSideEffectDispatcher struct{}

func (NoopSideEffectDispatcher) Dispatch(context.Context, PlannedSideEffectRecord) error {
	return nil
}

type SideEffectWorker struct {
	queue      PlannedSideEffectQueue
	dispatcher SideEffectDispatcher
}

type SideEffectWorkerResult struct {
	Claimed   int
	Delivered int
	Failed    int
}

func NewSideEffectWorker(queue PlannedSideEffectQueue, dispatcher SideEffectDispatcher) *SideEffectWorker {
	if dispatcher == nil {
		dispatcher = NoopSideEffectDispatcher{}
	}
	return &SideEffectWorker{queue: queue, dispatcher: dispatcher}
}

func (w *SideEffectWorker) RunOnce(ctx context.Context, limit int) (SideEffectWorkerResult, error) {
	if w == nil || w.queue == nil {
		return SideEffectWorkerResult{}, errors.New("side-effect worker requires a queue")
	}
	if limit <= 0 {
		limit = defaultSideEffectClaimLimit
	}

	claimed, err := w.queue.ClaimPlannedSideEffects(ctx, limit)
	if err != nil {
		return SideEffectWorkerResult{}, err
	}

	result := SideEffectWorkerResult{Claimed: len(claimed)}
	for _, effect := range claimed {
		if err := w.dispatcher.Dispatch(ctx, effect); err != nil {
			if markErr := w.queue.MarkPlannedSideEffectFailed(ctx, effect.ID, err); markErr != nil {
				return result, fmt.Errorf("mark planned side effect failed: %w", markErr)
			}
			result.Failed++
			continue
		}
		if err := w.queue.MarkPlannedSideEffectDelivered(ctx, effect.ID); err != nil {
			return result, fmt.Errorf("mark planned side effect delivered: %w", err)
		}
		result.Delivered++
	}
	return result, nil
}
