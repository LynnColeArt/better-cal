package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/calendar"
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
	signingKeyRef := "dispatch-key-ref"
	signingSecret := "dispatch-signing-secret"
	var requestCount atomic.Int32
	var capturedBody atomic.Value
	var capturedSignature atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		capturedBody.Store(string(bodyRaw))
		capturedSignature.Store(r.Header.Get(webhookSignatureHeaderName))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	subscriberURL := server.URL + "/caldiy/webhook"
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
		NewHTTPWebhookTransport(server.Client()),
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
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("webhook requests = %d, want 1", got)
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
	var attemptResponseStatus int
	var attemptCount int
	var delivered bool
	var attemptLastError string
	if err := pool.QueryRow(ctx, `
		select count(*), min(subscriber_url), min(trigger_event), min(content_type), min(signature_header_name), min(signature_header_value), min(body), min(coalesce(response_status, 0)), min(attempt_count), bool_and(delivered_at is not null), min(coalesce(last_error, ''))
		from booking_webhook_delivery_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&attemptRows, &attemptSubscriberURL, &attemptTriggerEvent, &attemptContentType, &signatureHeaderName, &signatureHeaderValue, &attemptBody, &attemptResponseStatus, &attemptCount, &delivered, &attemptLastError); err != nil {
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
	if attemptResponseStatus != http.StatusAccepted {
		t.Fatalf("attempt response status = %d", attemptResponseStatus)
	}
	if attemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", attemptCount)
	}
	if !delivered {
		t.Fatal("expected delivered webhook attempt")
	}
	if attemptLastError != "" {
		t.Fatalf("attempt last_error = %q", attemptLastError)
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
	if got, _ := capturedBody.Load().(string); got != attemptBody {
		t.Fatalf("captured webhook body = %q", got)
	}
	if got, _ := capturedSignature.Load().(string); got != signatureHeaderValue {
		t.Fatalf("captured webhook signature = %q", got)
	}
}

func TestPostgresSideEffectDispatcherRecordsCalendarDispatchOnce(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-calendar-dispatch-%d", time.Now().UnixNano())
	rescheduleUID := "old-booking-" + uid
	var requestCount atomic.Int32
	var capturedBody atomic.Value
	var capturedAction atomic.Value
	var capturedProvider atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		capturedBody.Store(string(bodyRaw))
		capturedAction.Store(r.Header.Get("X-Cal-Calendar-Action"))
		capturedProvider.Store(r.Header.Get("X-Cal-Calendar-Provider"))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	targetURL := server.URL + "/caldiy/calendar-dispatch"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	bookingValue := repositoryTestBooking(uid, "calendar-dispatch-request")
	bookingValue.UpdatedAt = "2026-01-01T00:06:00.000Z"
	if err := repo.Save(ctx, []PlannedSideEffect{
		{
			Name:       SideEffectCalendarRescheduled,
			BookingUID: uid,
			RequestID:  "calendar-dispatch-request",
			Payload: map[string]any{
				"rescheduleUid": rescheduleUID,
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
		nil,
		nil,
		nil,
		WithCalendarProvider(calendar.NewGoogleFixtureProvider()),
		WithCalendarTransport(NewHTTPCalendarTransport(server.Client())),
		WithCalendarDispatchURL(targetURL),
	)
	record := PlannedSideEffectRecord{
		ID:         sideEffectID,
		Name:       SideEffectCalendarRescheduled,
		BookingUID: uid,
		RequestID:  "calendar-dispatch-request",
		Payload: map[string]any{
			"rescheduleUid": rescheduleUID,
		},
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("calendar requests = %d, want 1", got)
	}

	var dispatchRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from booking_side_effect_dispatch_log
		where side_effect_id = $1
			and booking_uid = $2
			and name = $3
			and request_id = $4
	`, sideEffectID, uid, string(SideEffectCalendarRescheduled), "calendar-dispatch-request").Scan(&dispatchRows); err != nil {
		t.Fatal(err)
	}
	if dispatchRows != 1 {
		t.Fatalf("dispatch rows = %d, want 1", dispatchRows)
	}

	var action string
	var contentType string
	var createdAtWire string
	var bodyRaw []byte
	if err := pool.QueryRow(ctx, `
		select action, content_type, created_at_wire, body
		from booking_calendar_dispatches
		where side_effect_id = $1
	`, sideEffectID).Scan(&action, &contentType, &createdAtWire, &bodyRaw); err != nil {
		t.Fatal(err)
	}
	if action != string(CalendarDispatchBookingRescheduled) {
		t.Fatalf("action = %q", action)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	if createdAtWire != bookingValue.UpdatedAt {
		t.Fatalf("created_at_wire = %q, want %q", createdAtWire, bookingValue.UpdatedAt)
	}

	var envelope CalendarDispatchEnvelope
	if err := json.Unmarshal(bodyRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Action != CalendarDispatchBookingRescheduled {
		t.Fatalf("envelope action = %q", envelope.Action)
	}
	if envelope.Payload.UID != uid {
		t.Fatalf("envelope payload uid = %q", envelope.Payload.UID)
	}
	if envelope.Payload.RescheduleUID != rescheduleUID {
		t.Fatalf("envelope payload reschedule uid = %q", envelope.Payload.RescheduleUID)
	}
	if envelope.Payload.RequestID != "calendar-dispatch-request" {
		t.Fatalf("envelope payload request id = %q", envelope.Payload.RequestID)
	}

	var rawBody map[string]any
	if err := json.Unmarshal(bodyRaw, &rawBody); err != nil {
		t.Fatal(err)
	}
	payloadValue, _ := rawBody["payload"].(map[string]any)
	if _, ok := payloadValue["attendees"]; ok {
		t.Fatal("calendar dispatch exposed attendees")
	}
	if _, ok := payloadValue["responses"]; ok {
		t.Fatal("calendar dispatch exposed responses")
	}
	if _, ok := payloadValue["metadata"]; ok {
		t.Fatal("calendar dispatch exposed metadata")
	}

	var attemptRows int
	var attemptTargetURL string
	var attemptAction string
	var attemptContentType string
	var attemptBody string
	var attemptResponseStatus int
	var attemptCount int
	var delivered bool
	var attemptLastError string
	if err := pool.QueryRow(ctx, `
		select count(*), min(target_url), min(action), min(content_type), min(body), min(coalesce(response_status, 0)), min(attempt_count), bool_and(delivered_at is not null), min(coalesce(last_error, ''))
		from booking_calendar_dispatch_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&attemptRows, &attemptTargetURL, &attemptAction, &attemptContentType, &attemptBody, &attemptResponseStatus, &attemptCount, &delivered, &attemptLastError); err != nil {
		t.Fatal(err)
	}
	if attemptRows != 1 {
		t.Fatalf("attempt rows = %d, want 1", attemptRows)
	}
	if attemptTargetURL != targetURL {
		t.Fatalf("attempt target url = %q", attemptTargetURL)
	}
	if attemptAction != string(CalendarDispatchBookingRescheduled) {
		t.Fatalf("attempt action = %q", attemptAction)
	}
	if attemptContentType != "application/json" {
		t.Fatalf("attempt content type = %q", attemptContentType)
	}
	if attemptResponseStatus != http.StatusAccepted {
		t.Fatalf("attempt response status = %d", attemptResponseStatus)
	}
	if attemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", attemptCount)
	}
	if !delivered {
		t.Fatal("expected delivered calendar dispatch attempt")
	}
	if attemptLastError != "" {
		t.Fatalf("attempt last_error = %q", attemptLastError)
	}

	var attemptEnvelope CalendarDispatchEnvelope
	if err := json.Unmarshal([]byte(attemptBody), &attemptEnvelope); err != nil {
		t.Fatal(err)
	}
	if attemptEnvelope.Action != envelope.Action {
		t.Fatalf("attempt action = %q", attemptEnvelope.Action)
	}
	if attemptEnvelope.Payload.UID != envelope.Payload.UID {
		t.Fatalf("attempt payload uid = %q", attemptEnvelope.Payload.UID)
	}
	if attemptEnvelope.Payload.RescheduleUID != envelope.Payload.RescheduleUID {
		t.Fatalf("attempt payload reschedule uid = %q", attemptEnvelope.Payload.RescheduleUID)
	}

	var providerRequest calendar.GoogleFixtureDispatchRequest
	if err := json.Unmarshal([]byte(capturedBody.Load().(string)), &providerRequest); err != nil {
		t.Fatal(err)
	}
	if providerRequest.Provider != "google-calendar-fixture" {
		t.Fatalf("provider request provider = %q", providerRequest.Provider)
	}
	if providerRequest.Operation != "move_event" {
		t.Fatalf("provider request operation = %q", providerRequest.Operation)
	}
	if providerRequest.Event.ID != uid {
		t.Fatalf("provider request event id = %q", providerRequest.Event.ID)
	}
	if providerRequest.Event.Start != bookingValue.Start {
		t.Fatalf("provider request event start = %q", providerRequest.Event.Start)
	}
	if providerRequest.Event.End != bookingValue.End {
		t.Fatalf("provider request event end = %q", providerRequest.Event.End)
	}
	if providerRequest.Event.PreviousID != rescheduleUID {
		t.Fatalf("provider request previous id = %q", providerRequest.Event.PreviousID)
	}
	if got, _ := capturedAction.Load().(string); got != string(CalendarDispatchBookingRescheduled) {
		t.Fatalf("captured calendar action = %q", got)
	}
	if got, _ := capturedProvider.Load().(string); got != "google-calendar-fixture" {
		t.Fatalf("captured calendar provider = %q", got)
	}
}

func TestPostgresSideEffectDispatcherRecordsEmailDispatchOnce(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	uid := fmt.Sprintf("repo-email-dispatch-%d", time.Now().UnixNano())
	var requestCount atomic.Int32
	var capturedBody atomic.Value
	var capturedAction atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		capturedBody.Store(string(bodyRaw))
		capturedAction.Store(r.Header.Get("X-Cal-Email-Action"))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	targetURL := server.URL + "/caldiy/email-dispatch"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from bookings where uid = $1`, uid)
		_, _ = pool.Exec(cleanupCtx, `delete from booking_fixtures where uid = $1`, uid)
	})

	bookingValue := repositoryTestBooking(uid, "email-dispatch-request")
	bookingValue.Status = "cancelled"
	bookingValue.UpdatedAt = "2026-01-01T00:07:00.000Z"
	if err := repo.Save(ctx, []PlannedSideEffect{
		{
			Name:       SideEffectEmailCancelled,
			BookingUID: uid,
			RequestID:  "email-dispatch-request",
			Payload: map[string]any{
				"cancellationReason": "email fixture cancellation",
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
		nil,
		nil,
		nil,
		WithEmailTransport(NewHTTPEmailTransport(server.Client())),
		WithEmailDispatchURL(targetURL),
	)
	record := PlannedSideEffectRecord{
		ID:         sideEffectID,
		Name:       SideEffectEmailCancelled,
		BookingUID: uid,
		RequestID:  "email-dispatch-request",
		Payload: map[string]any{
			"cancellationReason": "email fixture cancellation",
		},
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("email requests = %d, want 1", got)
	}

	var dispatchRows int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from booking_side_effect_dispatch_log
		where side_effect_id = $1
			and booking_uid = $2
			and name = $3
			and request_id = $4
	`, sideEffectID, uid, string(SideEffectEmailCancelled), "email-dispatch-request").Scan(&dispatchRows); err != nil {
		t.Fatal(err)
	}
	if dispatchRows != 1 {
		t.Fatalf("dispatch rows = %d, want 1", dispatchRows)
	}

	var action string
	var contentType string
	var createdAtWire string
	var bodyRaw []byte
	if err := pool.QueryRow(ctx, `
		select action, content_type, created_at_wire, body
		from booking_email_deliveries
		where side_effect_id = $1
	`, sideEffectID).Scan(&action, &contentType, &createdAtWire, &bodyRaw); err != nil {
		t.Fatal(err)
	}
	if action != string(EmailDeliveryBookingCancelled) {
		t.Fatalf("action = %q", action)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	if createdAtWire != bookingValue.UpdatedAt {
		t.Fatalf("created_at_wire = %q, want %q", createdAtWire, bookingValue.UpdatedAt)
	}

	var envelope EmailDeliveryEnvelope
	if err := json.Unmarshal(bodyRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Action != EmailDeliveryBookingCancelled {
		t.Fatalf("envelope action = %q", envelope.Action)
	}
	if envelope.Payload.UID != uid {
		t.Fatalf("envelope payload uid = %q", envelope.Payload.UID)
	}
	if envelope.Payload.CancellationReason != "email fixture cancellation" {
		t.Fatalf("envelope payload cancellation reason = %q", envelope.Payload.CancellationReason)
	}
	if len(envelope.Payload.Recipients) != 1 {
		t.Fatalf("recipient count = %d, want 1", len(envelope.Payload.Recipients))
	}
	if envelope.Payload.Recipients[0].Email != "fixture-attendee@example.test" {
		t.Fatalf("recipient email = %q", envelope.Payload.Recipients[0].Email)
	}

	var rawBody map[string]any
	if err := json.Unmarshal(bodyRaw, &rawBody); err != nil {
		t.Fatal(err)
	}
	payloadValue, _ := rawBody["payload"].(map[string]any)
	if _, ok := payloadValue["responses"]; ok {
		t.Fatal("email delivery exposed responses")
	}
	if _, ok := payloadValue["metadata"]; ok {
		t.Fatal("email delivery exposed metadata")
	}
	recipientsValue, _ := payloadValue["recipients"].([]any)
	if len(recipientsValue) == 0 {
		t.Fatal("expected email recipients")
	}
	firstRecipient, _ := recipientsValue[0].(map[string]any)
	if _, ok := firstRecipient["id"]; ok {
		t.Fatal("email delivery exposed recipient id")
	}

	var attemptRows int
	var attemptTargetURL string
	var attemptAction string
	var attemptContentType string
	var attemptBody string
	var attemptResponseStatus int
	var attemptCount int
	var delivered bool
	var attemptLastError string
	if err := pool.QueryRow(ctx, `
		select count(*), min(target_url), min(action), min(content_type), min(body), min(coalesce(response_status, 0)), min(attempt_count), bool_and(delivered_at is not null), min(coalesce(last_error, ''))
		from booking_email_delivery_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&attemptRows, &attemptTargetURL, &attemptAction, &attemptContentType, &attemptBody, &attemptResponseStatus, &attemptCount, &delivered, &attemptLastError); err != nil {
		t.Fatal(err)
	}
	if attemptRows != 1 {
		t.Fatalf("attempt rows = %d, want 1", attemptRows)
	}
	if attemptTargetURL != targetURL {
		t.Fatalf("attempt target url = %q", attemptTargetURL)
	}
	if attemptAction != string(EmailDeliveryBookingCancelled) {
		t.Fatalf("attempt action = %q", attemptAction)
	}
	if attemptContentType != "application/json" {
		t.Fatalf("attempt content type = %q", attemptContentType)
	}
	if attemptResponseStatus != http.StatusAccepted {
		t.Fatalf("attempt response status = %d", attemptResponseStatus)
	}
	if attemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", attemptCount)
	}
	if !delivered {
		t.Fatal("expected delivered email delivery attempt")
	}
	if attemptLastError != "" {
		t.Fatalf("attempt last_error = %q", attemptLastError)
	}

	var attemptEnvelope EmailDeliveryEnvelope
	if err := json.Unmarshal([]byte(attemptBody), &attemptEnvelope); err != nil {
		t.Fatal(err)
	}
	if attemptEnvelope.Action != envelope.Action {
		t.Fatalf("attempt action = %q", attemptEnvelope.Action)
	}
	if attemptEnvelope.Payload.UID != envelope.Payload.UID {
		t.Fatalf("attempt payload uid = %q", attemptEnvelope.Payload.UID)
	}
	if attemptEnvelope.Payload.CancellationReason != envelope.Payload.CancellationReason {
		t.Fatalf("attempt payload cancellation reason = %q", attemptEnvelope.Payload.CancellationReason)
	}
	if got, _ := capturedBody.Load().(string); got != attemptBody {
		t.Fatalf("captured email body = %q", got)
	}
	if got, _ := capturedAction.Load().(string); got != string(EmailDeliveryBookingCancelled) {
		t.Fatalf("captured email action = %q", got)
	}
}

func TestPostgresSideEffectDispatcherRetriesFailedWebhookAttempt(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	subscriptionStore := NewPostgresWebhookSubscriptionStore(pool)
	uid := fmt.Sprintf("repo-side-effect-retry-%d", time.Now().UnixNano())
	signingKeyRef := "retry-key-ref"
	signingSecret := "retry-signing-secret"
	var requestCount atomic.Int32
	var responseStatus atomic.Int32
	responseStatus.Store(http.StatusBadGateway)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(int(responseStatus.Load()))
	}))
	defer server.Close()

	subscriberURL := server.URL + "/caldiy/webhook"
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

	bookingValue := repositoryTestBooking(uid, "retry-request")
	bookingValue.Status = "cancelled"
	if err := repo.Save(ctx, []PlannedSideEffect{
		{
			Name:       SideEffectWebhookBookingCancelled,
			BookingUID: uid,
			RequestID:  "retry-request",
			Payload: map[string]any{
				"cancellationReason": "retry fixture cancellation",
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
		NewHTTPWebhookTransport(server.Client()),
	)
	record := PlannedSideEffectRecord{
		ID:         sideEffectID,
		Name:       SideEffectWebhookBookingCancelled,
		BookingUID: uid,
		RequestID:  "retry-request",
		Payload: map[string]any{
			"cancellationReason": "retry fixture cancellation",
		},
	}

	err := dispatcher.Dispatch(ctx, record)
	if err == nil {
		t.Fatal("expected initial dispatch failure")
	}
	if err.Error() != "webhook delivery returned status 502" {
		t.Fatalf("dispatch error = %q", err)
	}

	var firstAttemptCount int
	var firstResponseStatus int
	var firstLastError string
	var firstDelivered bool
	var firstDeadLettered bool
	if err := pool.QueryRow(ctx, `
		select min(attempt_count), min(coalesce(response_status, 0)), min(coalesce(last_error, '')), bool_and(delivered_at is not null), bool_and(dead_lettered_at is not null)
		from booking_webhook_delivery_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&firstAttemptCount, &firstResponseStatus, &firstLastError, &firstDelivered, &firstDeadLettered); err != nil {
		t.Fatal(err)
	}
	if firstAttemptCount != 1 {
		t.Fatalf("first attempt count = %d", firstAttemptCount)
	}
	if firstResponseStatus != http.StatusBadGateway {
		t.Fatalf("first response status = %d", firstResponseStatus)
	}
	if firstLastError != "webhook delivery returned status 502" {
		t.Fatalf("first last_error = %q", firstLastError)
	}
	if firstDelivered {
		t.Fatal("unexpected delivered webhook attempt after failed transport")
	}
	if firstDeadLettered {
		t.Fatal("unexpected dead-lettered webhook attempt after first failure")
	}

	responseStatus.Store(http.StatusAccepted)
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if got := requestCount.Load(); got != 2 {
		t.Fatalf("webhook requests = %d, want 2", got)
	}

	var finalAttemptCount int
	var finalResponseStatus int
	var finalLastError string
	var finalDelivered bool
	var finalDeadLettered bool
	if err := pool.QueryRow(ctx, `
		select min(attempt_count), min(coalesce(response_status, 0)), min(coalesce(last_error, '')), bool_and(delivered_at is not null), bool_and(dead_lettered_at is not null)
		from booking_webhook_delivery_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&finalAttemptCount, &finalResponseStatus, &finalLastError, &finalDelivered, &finalDeadLettered); err != nil {
		t.Fatal(err)
	}
	if finalAttemptCount != 2 {
		t.Fatalf("final attempt count = %d", finalAttemptCount)
	}
	if finalResponseStatus != http.StatusAccepted {
		t.Fatalf("final response status = %d", finalResponseStatus)
	}
	if finalLastError != "" {
		t.Fatalf("final last_error = %q", finalLastError)
	}
	if !finalDelivered {
		t.Fatal("expected delivered webhook attempt after retry")
	}
	if finalDeadLettered {
		t.Fatal("unexpected dead-lettered webhook attempt after successful retry")
	}
}

func TestPostgresSideEffectDispatcherDeadLettersAndDisablesSubscriber(t *testing.T) {
	pool := testPostgresRepositoryPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewPostgresRepository(pool)
	subscriptionStore := NewPostgresWebhookSubscriptionStore(pool)
	uid := fmt.Sprintf("repo-side-effect-deadletter-%d", time.Now().UnixNano())
	signingKeyRef := "deadletter-key-ref"
	signingSecret := "deadletter-signing-secret"
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	subscriberURL := server.URL + "/caldiy/webhook"
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

	bookingValue := repositoryTestBooking(uid, "deadletter-request")
	bookingValue.Status = "cancelled"
	if err := repo.Save(ctx, []PlannedSideEffect{
		{
			Name:       SideEffectWebhookBookingCancelled,
			BookingUID: uid,
			RequestID:  "deadletter-request",
			Payload: map[string]any{
				"cancellationReason": "deadletter fixture cancellation",
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
		NewHTTPWebhookTransport(server.Client()),
		WithWebhookMaxAttempts(2),
	)
	record := PlannedSideEffectRecord{
		ID:         sideEffectID,
		Name:       SideEffectWebhookBookingCancelled,
		BookingUID: uid,
		RequestID:  "deadletter-request",
		Payload: map[string]any{
			"cancellationReason": "deadletter fixture cancellation",
		},
	}

	if err := dispatcher.Dispatch(ctx, record); err == nil {
		t.Fatal("expected first dispatch failure")
	}
	if err := dispatcher.Dispatch(ctx, record); err != nil {
		t.Fatal(err)
	}
	if got := requestCount.Load(); got != 2 {
		t.Fatalf("webhook requests = %d, want 2", got)
	}

	var attemptCount int
	var responseStatus int
	var delivered bool
	var deadLettered bool
	var lastError string
	if err := pool.QueryRow(ctx, `
		select min(attempt_count), min(coalesce(response_status, 0)), bool_and(delivered_at is not null), bool_and(dead_lettered_at is not null), min(coalesce(last_error, ''))
		from booking_webhook_delivery_attempts
		where side_effect_id = $1
	`, sideEffectID).Scan(&attemptCount, &responseStatus, &delivered, &deadLettered, &lastError); err != nil {
		t.Fatal(err)
	}
	if attemptCount != 2 {
		t.Fatalf("attempt count = %d, want 2", attemptCount)
	}
	if responseStatus != http.StatusServiceUnavailable {
		t.Fatalf("response status = %d", responseStatus)
	}
	if delivered {
		t.Fatal("unexpected delivered webhook attempt")
	}
	if !deadLettered {
		t.Fatal("expected dead-lettered webhook attempt")
	}
	if lastError != "webhook delivery returned status 503" {
		t.Fatalf("last_error = %q", lastError)
	}

	var active bool
	var failureCount int
	var disabledReason string
	if err := pool.QueryRow(ctx, `
		select active, failure_count, coalesce(disabled_reason, '')
		from booking_webhook_subscriptions
		where subscriber_url = $1
			and trigger_event = $2
	`, subscriberURL, string(WebhookTriggerBookingCancelled)).Scan(&active, &failureCount, &disabledReason); err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("dead-lettered subscriber remained active")
	}
	if failureCount != 2 {
		t.Fatalf("failure count = %d, want 2", failureCount)
	}
	if disabledReason != "delivery attempts exhausted" {
		t.Fatalf("disabled reason = %q", disabledReason)
	}

	metrics, err := ReadWebhookDeliveryMetrics(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.DeadLetteredAttempts == 0 {
		t.Fatal("metrics did not include dead-lettered attempt")
	}
	if metrics.DisabledSubscribers == 0 {
		t.Fatal("metrics did not include disabled subscriber")
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
