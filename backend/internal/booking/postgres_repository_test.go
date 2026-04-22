package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryPersistsBookingFixture(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-booking-%d", time.Now().UnixNano())
	idempotencyKey := uid + "-idempotency"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	bookingValue := Booking{
		UID:         uid,
		ID:          654,
		Title:       "Repository Fixture",
		Status:      "accepted",
		Start:       "2026-05-03T15:00:00.000Z",
		End:         "2026-05-03T15:30:00.000Z",
		EventTypeID: 1001,
		Attendees: []Attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: "America/Chicago",
			},
		},
		Responses: map[string]any{
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "postgres-repository",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
		RequestID: "repo-test-request",
	}

	persisted, duplicate, err := repo.SaveCreated(ctx, bookingValue, idempotencyKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("initial save reported duplicate")
	}
	if persisted.UID != uid {
		t.Fatalf("persisted uid = %q", persisted.UID)
	}

	assertExplicitBookingRows(t, ctx, pool, uid, "accepted", 1)

	staleFixture := bookingValue
	staleFixture.Status = "stale-jsonb-status"
	staleFixture.RequestID = "stale-jsonb-request"
	rawStaleFixture, err := json.Marshal(staleFixture)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update booking_fixtures set payload = $2 where uid = $1`, uid, string(rawStaleFixture)); err != nil {
		t.Fatal(err)
	}

	found, ok, err := repo.ReadByUID(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking fixture was not found by uid")
	}
	if found.UID != uid {
		t.Fatalf("uid = %q", found.UID)
	}
	if found.Metadata["fixture"] != "postgres-repository" {
		t.Fatalf("metadata = %#v", found.Metadata)
	}
	if found.Status != "accepted" {
		t.Fatalf("status = %q", found.Status)
	}
	if found.RequestID != "repo-test-request" {
		t.Fatalf("request id = %q", found.RequestID)
	}

	replayed, ok, err := repo.ReadByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("booking fixture was not found by idempotency key")
	}
	if replayed.RequestID != "repo-test-request" {
		t.Fatalf("request id = %q", replayed.RequestID)
	}

	bookingValue.Status = "cancelled"
	effects := []PlannedSideEffect{
		{Name: SideEffectCalendarCancelled, BookingUID: uid, RequestID: "repo-test-request"},
		{Name: SideEffectEmailCancelled, BookingUID: uid, RequestID: "repo-test-request"},
	}
	if err := repo.Save(ctx, effects, bookingValue); err != nil {
		t.Fatal(err)
	}
	assertExplicitBookingRows(t, ctx, pool, uid, "cancelled", 1)
	assertPlannedSideEffectRows(t, ctx, pool, uid, 2)
	found, ok, err = repo.ReadByUID(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("updated booking fixture was not found")
	}
	if found.Status != "cancelled" {
		t.Fatalf("status = %q", found.Status)
	}
}

func TestPostgresRepositoryReplaysIdempotencyConflictWithoutOverwriting(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	originalUID := fmt.Sprintf("repo-idempotent-original-%d", time.Now().UnixNano())
	conflictingUID := originalUID + "-conflict"
	idempotencyKey := originalUID + "-idempotency"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid in ($1, $2)`, originalUID, conflictingUID)
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid in ($1, $2)`, originalUID, conflictingUID)
	})

	original := repositoryTestBooking(originalUID, "original-request")
	if _, duplicate, err := repo.SaveCreated(ctx, original, idempotencyKey, nil); err != nil {
		t.Fatal(err)
	} else if duplicate {
		t.Fatal("initial save reported duplicate")
	}

	conflicting := repositoryTestBooking(conflictingUID, "conflicting-request")
	conflicting.Status = "cancelled"
	replayed, duplicate, err := repo.SaveCreated(ctx, conflicting, idempotencyKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate {
		t.Fatal("conflicting idempotency key was not reported as duplicate")
	}
	if replayed.UID != originalUID {
		t.Fatalf("replayed uid = %q, want %q", replayed.UID, originalUID)
	}
	if replayed.RequestID != "original-request" {
		t.Fatalf("replayed request id = %q", replayed.RequestID)
	}
	assertBookingRowCount(t, ctx, pool, conflictingUID, 0)

	found, ok, err := repo.ReadByUID(ctx, originalUID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("original booking was not found")
	}
	if found.Status != "accepted" {
		t.Fatalf("original status = %q", found.Status)
	}
}

func TestPostgresRepositoryRollsBackBookingWhenSideEffectWriteFails(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-side-effect-rollback-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
	})

	err := repo.Save(ctx, []PlannedSideEffect{
		{Name: SideEffectEmailCancelled, BookingUID: "missing-side-effect-booking", RequestID: "rollback-request"},
	}, repositoryTestBooking(uid, "rollback-request"))
	if err == nil {
		t.Fatal("expected side-effect persistence error")
	}
	assertBookingRowCount(t, ctx, pool, uid, 0)
}

func TestPostgresRepositoryClaimsAndMarksPlannedSideEffects(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-side-effect-claim-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	if err := repo.Save(ctx, []PlannedSideEffect{
		{Name: SideEffectEmailCancelled, BookingUID: uid, RequestID: "claim-request"},
		{
			Name:       SideEffectWebhookBookingCancelled,
			BookingUID: uid,
			RequestID:  "claim-request",
			Payload: map[string]any{
				"cancellationReason": "claim fixture cancellation",
			},
		},
	}, repositoryTestBooking(uid, "claim-request")); err != nil {
		t.Fatal(err)
	}

	claimed, err := repo.ClaimPlannedSideEffects(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed side effects = %d, want 2", len(claimed))
	}
	for _, record := range claimed {
		if record.BookingUID != uid {
			t.Fatalf("claimed booking uid = %q, want %q", record.BookingUID, uid)
		}
		if record.Status != "processing" {
			t.Fatalf("claimed status = %q, want processing", record.Status)
		}
		if record.Attempts != 1 {
			t.Fatalf("claimed attempts = %d, want 1", record.Attempts)
		}
		if record.Name == SideEffectWebhookBookingCancelled {
			assertPayloadField(t, record.Payload, "cancellationReason", "claim fixture cancellation")
		}
	}

	claimedAgain, err := repo.ClaimPlannedSideEffects(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimedAgain) != 0 {
		t.Fatalf("claimed processing side effects = %d, want 0", len(claimedAgain))
	}

	if err := repo.MarkPlannedSideEffectDelivered(ctx, claimed[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPlannedSideEffectFailed(ctx, claimed[1].ID, errors.New("provider exploded: api-key-secret")); err != nil {
		t.Fatal(err)
	}

	var deliveredStatus string
	if err := pool.QueryRow(ctx, `
		select status
		from booking_planned_side_effects
		where id = $1
	`, claimed[0].ID).Scan(&deliveredStatus); err != nil {
		t.Fatal(err)
	}
	if deliveredStatus != "delivered" {
		t.Fatalf("delivered status = %q", deliveredStatus)
	}

	var failedStatus string
	var failedLastError string
	var failedAttempts int
	if err := pool.QueryRow(ctx, `
		select status, last_error, attempts
		from booking_planned_side_effects
		where id = $1
	`, claimed[1].ID).Scan(&failedStatus, &failedLastError, &failedAttempts); err != nil {
		t.Fatal(err)
	}
	if failedStatus != "failed" {
		t.Fatalf("failed status = %q", failedStatus)
	}
	if failedLastError != "dispatch failed" {
		t.Fatalf("failed last_error = %q", failedLastError)
	}
	if failedAttempts != 1 {
		t.Fatalf("failed attempts = %d, want 1", failedAttempts)
	}
}

func TestPostgresRepositoryClaimLimit(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-side-effect-limit-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	if err := repo.Save(ctx, []PlannedSideEffect{
		{Name: SideEffectEmailCancelled, BookingUID: uid, RequestID: "limit-request"},
		{Name: SideEffectWebhookBookingCancelled, BookingUID: uid, RequestID: "limit-request"},
	}, repositoryTestBooking(uid, "limit-request")); err != nil {
		t.Fatal(err)
	}

	claimed, err := repo.ClaimPlannedSideEffects(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 {
		t.Fatalf("claimed side effects = %d, want 1", len(claimed))
	}

	var planned int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from booking_planned_side_effects
		where booking_uid = $1
			and status = 'planned'
	`, uid).Scan(&planned); err != nil {
		t.Fatal(err)
	}
	if planned != 1 {
		t.Fatalf("planned side effects = %d, want 1", planned)
	}
}

func TestPostgresSideEffectDispatcherRecordsDeliveryOnce(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	subscriptionStore := NewPostgresWebhookSubscriptionStore(pool)
	uid := fmt.Sprintf("repo-side-effect-dispatch-%d", time.Now().UnixNano())
	subscriberURL := "https://example.invalid/" + uid
	signingKeyRef := "dispatch-key-ref"
	signingSecret := "dispatch-signing-secret"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from booking_webhook_delivery_attempts where subscriber_url = $1`, subscriberURL)
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_webhook_subscriptions where trigger_event = $1`, string(WebhookTriggerBookingCancelled))
	})
	if _, err := pool.Exec(ctx, `delete from booking_webhook_subscriptions where trigger_event = $1`, string(WebhookTriggerBookingCancelled)); err != nil {
		t.Fatal(err)
	}
	if err := SeedWebhookSubscriptions(ctx, subscriptionStore, FixtureWebhookSubscriptions(subscriberURL, signingKeyRef)); err != nil {
		t.Fatal(err)
	}

	bookingValue := repositoryTestBooking(uid, "dispatch-request")
	bookingValue.Status = "cancelled"
	bookingValue.UpdatedAt = "2026-01-01T00:05:00.000Z"
	if err := repo.Save(ctx, []PlannedSideEffect{
		{
			Name:       SideEffectWebhookBookingCancelled,
			BookingUID: uid,
			RequestID:  "dispatch-request",
			Payload: map[string]any{
				"cancellationReason": "dispatch fixture cancellation",
			},
		},
	}, bookingValue); err != nil {
		t.Fatal(err)
	}

	var sideEffectID int64
	if err := pool.QueryRow(ctx, `
		select id
		from booking_planned_side_effects
		where booking_uid = $1
	`, uid).Scan(&sideEffectID); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewPostgresSideEffectDispatcher(
		pool,
		repo,
		subscriptionStore,
		NewFixtureWebhookSigningSecretResolver(map[string]string{
			signingKeyRef: signingSecret,
		}),
	)
	record := PlannedSideEffectRecord{
		ID:         sideEffectID,
		Name:       SideEffectWebhookBookingCancelled,
		BookingUID: uid,
		RequestID:  "dispatch-request",
		Payload: map[string]any{
			"cancellationReason": "dispatch fixture cancellation",
		},
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}

	var dispatchRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from booking_side_effect_dispatch_log
		where side_effect_id = $1
			and booking_uid = $2
			and name = $3
			and request_id = $4
	`, sideEffectID, uid, string(SideEffectWebhookBookingCancelled), "dispatch-request").Scan(&dispatchRows); err != nil {
		t.Fatal(err)
	}
	if dispatchRows != 1 {
		t.Fatalf("dispatch rows = %d, want 1", dispatchRows)
	}

	var triggerEvent string
	var contentType string
	var createdAtWire string
	var bodyRaw []byte
	if err := pool.QueryRow(ctx, `
		select trigger_event, content_type, created_at_wire, body
		from booking_webhook_deliveries
		where side_effect_id = $1
	`, sideEffectID).Scan(&triggerEvent, &contentType, &createdAtWire, &bodyRaw); err != nil {
		t.Fatal(err)
	}
	if triggerEvent != string(WebhookTriggerBookingCancelled) {
		t.Fatalf("trigger event = %q", triggerEvent)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	if createdAtWire != bookingValue.UpdatedAt {
		t.Fatalf("created_at_wire = %q, want %q", createdAtWire, bookingValue.UpdatedAt)
	}

	var envelope WebhookDeliveryEnvelope
	if err := json.Unmarshal(bodyRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.TriggerEvent != WebhookTriggerBookingCancelled {
		t.Fatalf("envelope trigger event = %q", envelope.TriggerEvent)
	}
	if envelope.Payload.UID != uid {
		t.Fatalf("envelope payload uid = %q", envelope.Payload.UID)
	}
	if envelope.Payload.CancellationReason != "dispatch fixture cancellation" {
		t.Fatalf("envelope cancellation reason = %q", envelope.Payload.CancellationReason)
	}

	var rawBody map[string]any
	if err := json.Unmarshal(bodyRaw, &rawBody); err != nil {
		t.Fatal(err)
	}
	payloadValue, _ := rawBody["payload"].(map[string]any)
	attendeesValue, _ := payloadValue["attendees"].([]any)
	if len(attendeesValue) == 0 {
		t.Fatal("expected webhook attendees")
	}
	firstAttendee, _ := attendeesValue[0].(map[string]any)
	if _, ok := firstAttendee["id"]; ok {
		t.Fatal("webhook attendee exposed internal id")
	}

	var attemptRows int
	var attemptSubscriberURL string
	var attemptTriggerEvent string
	var attemptContentType string
	var signatureHeaderName string
	var signatureHeaderValue string
	var attemptBody string
	if err := pool.QueryRow(ctx, `
		select count(*), min(subscriber_url), min(trigger_event), min(content_type), min(signature_header_name), min(signature_header_value), min(body)
		from booking_webhook_delivery_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&attemptRows, &attemptSubscriberURL, &attemptTriggerEvent, &attemptContentType, &signatureHeaderName, &signatureHeaderValue, &attemptBody); err != nil {
		t.Fatal(err)
	}
	if attemptRows != 1 {
		t.Fatalf("attempt rows = %d, want 1", attemptRows)
	}
	if attemptSubscriberURL != subscriberURL {
		t.Fatalf("attempt subscriber url = %q", attemptSubscriberURL)
	}
	if attemptTriggerEvent != string(WebhookTriggerBookingCancelled) {
		t.Fatalf("attempt trigger event = %q", attemptTriggerEvent)
	}
	if attemptContentType != "application/json" {
		t.Fatalf("attempt content type = %q", attemptContentType)
	}
	if signatureHeaderName != webhookSignatureHeaderName {
		t.Fatalf("signature header name = %q", signatureHeaderName)
	}
	expectedSignature, err := signWebhookBody(attemptBody, signingSecret)
	if err != nil {
		t.Fatal(err)
	}
	if signatureHeaderValue != expectedSignature {
		t.Fatalf("signature header value = %q, want %q", signatureHeaderValue, expectedSignature)
	}

	var attemptEnvelope WebhookDeliveryEnvelope
	if err := json.Unmarshal([]byte(attemptBody), &attemptEnvelope); err != nil {
		t.Fatal(err)
	}
	if attemptEnvelope.TriggerEvent != envelope.TriggerEvent {
		t.Fatalf("attempt trigger event = %q", attemptEnvelope.TriggerEvent)
	}
	if attemptEnvelope.Payload.UID != envelope.Payload.UID {
		t.Fatalf("attempt payload uid = %q", attemptEnvelope.Payload.UID)
	}
	if attemptEnvelope.Payload.CancellationReason != envelope.Payload.CancellationReason {
		t.Fatalf("attempt cancellation reason = %q", attemptEnvelope.Payload.CancellationReason)
	}
}

func TestPostgresWebhookSubscriptionStoreReadsActiveSubscribersByTrigger(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := NewPostgresWebhookSubscriptionStore(pool)
	prefix := fmt.Sprintf("https://example.invalid/subscription-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from booking_webhook_subscriptions where trigger_event = $1`, string(WebhookTriggerBookingCancelled))
		_, _ = pool.Exec(cleanupCtx, `delete from booking_webhook_subscriptions where trigger_event = $1`, string(WebhookTriggerBookingConfirmed))
	})
	if _, err := pool.Exec(ctx, `delete from booking_webhook_subscriptions where trigger_event = $1`, string(WebhookTriggerBookingCancelled)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `delete from booking_webhook_subscriptions where trigger_event = $1`, string(WebhookTriggerBookingConfirmed)); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveWebhookSubscription(ctx, WebhookSubscription{
		SubscriberURL: prefix + "-cancelled-active",
		TriggerEvent:  WebhookTriggerBookingCancelled,
		SigningKeyRef: "cancelled-active",
		Active:        true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWebhookSubscription(ctx, WebhookSubscription{
		SubscriberURL: prefix + "-cancelled-inactive",
		TriggerEvent:  WebhookTriggerBookingCancelled,
		SigningKeyRef: "cancelled-inactive",
		Active:        false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWebhookSubscription(ctx, WebhookSubscription{
		SubscriberURL: prefix + "-confirmed-active",
		TriggerEvent:  WebhookTriggerBookingConfirmed,
		SigningKeyRef: "confirmed-active",
		Active:        true,
	}); err != nil {
		t.Fatal(err)
	}

	subscriptions, err := store.ReadWebhookSubscriptionsByTrigger(ctx, WebhookTriggerBookingCancelled)
	if err != nil {
		t.Fatal(err)
	}
	if len(subscriptions) != 1 {
		t.Fatalf("subscriptions = %d, want 1", len(subscriptions))
	}
	if subscriptions[0].SubscriberURL != prefix+"-cancelled-active" {
		t.Fatalf("subscriber url = %q", subscriptions[0].SubscriberURL)
	}
	if subscriptions[0].TriggerEvent != WebhookTriggerBookingCancelled {
		t.Fatalf("trigger event = %q", subscriptions[0].TriggerEvent)
	}
	if subscriptions[0].SigningKeyRef != "cancelled-active" {
		t.Fatalf("signing key ref = %q", subscriptions[0].SigningKeyRef)
	}
}

func TestPostgresRepositoryFallsBackToFixturePayload(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("fixture-only-booking-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	bookingValue := Booking{
		UID:         uid,
		ID:          765,
		Title:       "Fixture Only",
		Status:      "accepted",
		Start:       "2026-05-04T15:00:00.000Z",
		End:         "2026-05-04T15:30:00.000Z",
		EventTypeID: 1001,
		Attendees: []Attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: "America/Chicago",
			},
		},
		Responses: map[string]any{
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "jsonb-fallback",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
		RequestID: "fixture-fallback-request",
	}
	raw, err := json.Marshal(bookingValue)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into booking_fixtures (uid, payload)
		values ($1, $2)
	`, uid, string(raw)); err != nil {
		t.Fatal(err)
	}

	found, ok, err := repo.ReadByUID(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("fixture-only booking was not found")
	}
	if found.RequestID != "fixture-fallback-request" {
		t.Fatalf("request id = %q", found.RequestID)
	}
	if found.Metadata["fixture"] != "jsonb-fallback" {
		t.Fatalf("metadata = %#v", found.Metadata)
	}
}

func repositoryTestBooking(uid string, requestID string) Booking {
	return Booking{
		UID:         uid,
		ID:          654,
		Title:       "Repository Fixture",
		Status:      "accepted",
		Start:       "2026-05-03T15:00:00.000Z",
		End:         "2026-05-03T15:30:00.000Z",
		EventTypeID: 1001,
		Attendees: []Attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: "America/Chicago",
			},
		},
		Responses: map[string]any{
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "postgres-repository",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
		RequestID: requestID,
	}
}

func TestPostgresRepositoryReturnsFalseForMissingRows(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	missingUID := fmt.Sprintf("missing-booking-%d", time.Now().UnixNano())
	if _, ok, err := repo.ReadByUID(ctx, missingUID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing uid was found")
	}
	if _, ok, err := repo.ReadByIdempotencyKey(ctx, missingUID+"-idempotency"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("missing idempotency key was found")
	}
}

func testPostgresRepositoryPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("CALDIY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("CALDIY_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set CALDIY_TEST_DATABASE_URL or CALDIY_DATABASE_URL to run Postgres repository tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func assertExplicitBookingRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, uid string, expectedStatus string, expectedAttendees int) {
	t.Helper()
	var status string
	if err := pool.QueryRow(ctx, `select status from bookings where uid = $1`, uid).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != expectedStatus {
		t.Fatalf("explicit booking status = %q, want %q", status, expectedStatus)
	}
	var attendees int
	if err := pool.QueryRow(ctx, `select count(*) from booking_attendees where booking_uid = $1`, uid).Scan(&attendees); err != nil {
		t.Fatal(err)
	}
	if attendees != expectedAttendees {
		t.Fatalf("attendee row count = %d, want %d", attendees, expectedAttendees)
	}
}

func assertBookingRowCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, uid string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from bookings where uid = $1`, uid).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("booking row count = %d, want %d", count, expected)
	}
}

func assertPlannedSideEffectRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, uid string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from booking_planned_side_effects where booking_uid = $1`, uid).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("planned side-effect row count = %d, want %d", count, expected)
	}
}

func assertPayloadField(t *testing.T, payload map[string]any, key string, expected string) {
	t.Helper()
	value, _ := payload[key].(string)
	if value != expected {
		t.Fatalf("payload[%q] = %q, want %q", key, value, expected)
	}
}
