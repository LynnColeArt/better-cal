package booking

import (
	"context"
	"errors"
	"testing"
)

func TestSideEffectWorkerDispatchesClaimedEffects(t *testing.T) {
	queue := &fakeSideEffectQueue{
		claimed: []PlannedSideEffectRecord{
			{ID: 1, Name: SideEffectEmailCancelled, BookingUID: PrimaryFixtureUID, RequestID: "request-id"},
			{ID: 2, Name: SideEffectWebhookBookingCancelled, BookingUID: PrimaryFixtureUID, RequestID: "request-id"},
		},
	}
	dispatcher := &fakeDispatcher{}
	worker := NewSideEffectWorker(queue, dispatcher)

	result, err := worker.RunOnce(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 2 || result.Delivered != 2 || result.Failed != 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(dispatcher.dispatched) != 2 {
		t.Fatalf("dispatched = %d", len(dispatcher.dispatched))
	}
	if len(queue.delivered) != 2 {
		t.Fatalf("delivered = %v", queue.delivered)
	}
	if len(queue.failed) != 0 {
		t.Fatalf("failed = %v", queue.failed)
	}
}

func TestSideEffectWorkerMarksDispatchFailuresRetryable(t *testing.T) {
	queue := &fakeSideEffectQueue{
		claimed: []PlannedSideEffectRecord{
			{ID: 3, Name: SideEffectEmailCancelled, BookingUID: PrimaryFixtureUID, RequestID: "request-id"},
		},
	}
	dispatcher := &fakeDispatcher{err: errors.New("provider unavailable")}
	worker := NewSideEffectWorker(queue, dispatcher)

	result, err := worker.RunOnce(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 || result.Delivered != 0 || result.Failed != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(queue.failed) != 1 || queue.failed[0] != 3 {
		t.Fatalf("failed = %v", queue.failed)
	}
	if len(queue.delivered) != 0 {
		t.Fatalf("delivered = %v", queue.delivered)
	}
	if queue.limit != defaultSideEffectClaimLimit {
		t.Fatalf("claim limit = %d", queue.limit)
	}
}

type fakeSideEffectQueue struct {
	claimed   []PlannedSideEffectRecord
	delivered []int64
	failed    []int64
	limit     int
}

func (q *fakeSideEffectQueue) ClaimPlannedSideEffects(_ context.Context, limit int) ([]PlannedSideEffectRecord, error) {
	q.limit = limit
	return q.claimed, nil
}

func (q *fakeSideEffectQueue) MarkPlannedSideEffectDelivered(_ context.Context, id int64) error {
	q.delivered = append(q.delivered, id)
	return nil
}

func (q *fakeSideEffectQueue) MarkPlannedSideEffectFailed(_ context.Context, id int64, _ error) error {
	q.failed = append(q.failed, id)
	return nil
}

type fakeDispatcher struct {
	dispatched []PlannedSideEffectRecord
	err        error
}

func (d *fakeDispatcher) Dispatch(_ context.Context, effect PlannedSideEffectRecord) error {
	d.dispatched = append(d.dispatched, effect)
	return d.err
}
