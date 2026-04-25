# Better Cal Backend

This is the starter Go backend for the whiteroom replacement project.

The first implementation slice is deliberately small and fixture-driven. It supports:

- `GET /health`
- `GET /v2/me`
- `GET /v2/calendar-connections`
- `GET /v2/calendars`
- `GET /v2/selected-calendars`
- `POST /v2/selected-calendars`
- `DELETE /v2/selected-calendars/{calendarRef}`
- `GET /v2/destination-calendars`
- `POST /v2/destination-calendars`
- `GET /v2/slots`
- `GET /v2/auth/oauth2/clients/{clientId}`
- `POST /v2/auth/oauth2/token`
- `GET /v2/oauth-clients/{clientId}`
- `POST /v2/bookings`
- `GET /v2/bookings/{bookingUid}`
- `POST /v2/bookings/{bookingUid}/cancel`
- `POST /v2/bookings/{bookingUid}/reschedule`
- `POST /v2/bookings/{bookingUid}/confirm`
- `POST /v2/bookings/{bookingUid}/decline`

The API shell includes request ID propagation, panic recovery, structured request logging with secret-bearing headers redacted, a small auth service that resolves fixture API-key principals, OAuth clients, authorization-code token exchange, refresh-token rotation, scoped OAuth access-token lookup, and platform clients, an explicit deny-by-default authz package for named route policies and booking resource checks, a slot service for the current accepted availability fixtures, and a booking service for the current accepted lifecycle fixtures. Booking creation and reschedule calls use the slot service through an availability adapter before persistence so unavailable fixture slots and internally busy slots are rejected by the same service boundary that backs `GET /v2/slots`. Booking read/write/host-action routes require the caller to have both the named permission and the fixture owner or host principal id; wrong-owner fixtures authenticate but fail with `403`. `GET /v2/bookings/{bookingUid}` also accepts scoped OAuth access tokens issued by the fixture token exchange, while broader booking writes remain on the existing API-key path. Confirm and decline use the host-action policy and the same typed side-effect planning boundary as cancel and reschedule. When `CALDIY_DATABASE_URL` is set, API-key principal lookup, OAuth client metadata lookup, OAuth authorization-code exchange state, OAuth refresh-token rotation state, OAuth access-token principal lookup from hashed token rows, platform client verification, explicit booking rows, booking fixture fallback state, idempotency keys, planned booking side effects, event type metadata, and fixture availability slots are stored in Postgres through repository adapters; otherwise the service falls back to config and in-memory fixture state.

Run locally:

```bash
cd backend
go run ./cmd/api
```

Run with Docker Compose from the repository root:

```bash
docker compose up --build
```

Inspect captured Compose webhook, email-provider, and calendar-provider deliveries:

```bash
curl http://127.0.0.1:8090/requests
```

Run one worker pass through Compose:

```bash
docker compose --profile worker run --rm worker
```

Webhook retries use `CALDIY_WEBHOOK_MAX_ATTEMPTS`, which defaults to `3`. When an attempt reaches the threshold, the attempt is dead-lettered and the subscriber row is disabled so future deliveries skip it until an operator re-enables it. Email provider delivery uses `CALDIY_EMAIL_DISPATCH_URL`, which defaults in Compose to the local sink at `http://webhook-sink:8090/caldiy/email-dispatch`. Calendar provider delivery uses `CALDIY_CALENDAR_DISPATCH_URL`, which defaults in Compose to the local sink at `http://webhook-sink:8090/caldiy/calendar-dispatch`.

Run Go tests:

```bash
cd backend
go test ./...
```

Run one planned side-effect dispatch pass:

```bash
cd backend
CALDIY_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" go run ./cmd/worker
```

Run the Postgres-backed database integration tests against the Compose database:

```bash
CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" go test ./internal/db ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots -v
```

Run contract replay smoke from the repository root:

```bash
node tools/backend-smoke/smoke-test.mjs
```

When `CALDIY_DATABASE_URL` is set, the API opens Postgres, runs embedded migrations, seeds the fixture API-key principal in `api_key_principals` using a SHA-256 token hash, seeds non-secret OAuth client metadata in `oauth_clients`, seeds a fixture OAuth authorization code in `oauth_authorization_codes` using only a SHA-256 code hash, exchanges that code atomically through `POST /v2/auth/oauth2/token`, stores issued access and refresh tokens only as hashes in `oauth_tokens`, rotates refresh tokens by revoking the old hashed token row and inserting a new hashed access/refresh pair in one transaction, resolves valid unexpired and unrevoked access-token hashes into scope-limited booking-read principals, stores only a SHA-256 hash for platform client secret verification in `platform_clients`, seeds fixture event type metadata and availability in `event_types` and `availability_slots`, transactionally syncs current-user calendar connections and provider-backed catalog rows through the calendar provider adapter into `calendar_connections` and `calendar_catalog`, prunes stale synced catalog state, records provider connection status changes in `calendar_connection_status_history`, persists current-user selected calendars in `selected_calendars` plus current-user destination pointers in `destination_calendars`, seeds non-secret integration credential metadata in `integration_credential_metadata`, refreshes credential and calendar connection operational status through a generic provider-status adapter that stores only sanitized status codes and timestamps, exposes current-user calendar connection, calendar catalog, credential metadata, selected-calendar, and destination-calendar management routes without returning provider secrets or credential payloads, and validates selected-calendar writes against the synced catalog before snapshotting non-secret calendar metadata, writes booking fields plus internal selected-calendar refs, destination-calendar refs, and opaque external calendar event ids to `bookings` and `booking_attendees`, filters availability against accepted booking rows, and keeps `booking_fixtures` as a compatibility fallback with duplicate create detection in `booking_idempotency_keys`. Booking state and planned side effects are committed in one transaction, and idempotency keys are locked so conflicting retries replay the first booking instead of creating stray state. Planned side-effect rows now persist typed payload hints for retry-safe webhook and email fields such as cancellation, reschedule, and rejection reasons, while calendar side effects now also persist selected/destination calendar refs and external event ids needed for retry-safe provider dispatch. The API and worker seed fixture webhook subscriptions into `booking_webhook_subscriptions` using subscriber URL, trigger event, and signing key ref only; seeding will not reactivate a subscription disabled by dead-letter handling. The worker command claims planned or retryable side effects with Postgres row locks, records one `booking_side_effect_dispatch_log` row per side effect, writes typed booking webhook envelopes to `booking_webhook_deliveries`, snapshots signed per-subscriber attempts in `booking_webhook_delivery_attempts`, and sends real outbound HTTP POST requests through the webhook transport. The same dispatcher now writes slim email envelopes to `booking_email_deliveries`, snapshots outbound attempts in `booking_email_delivery_attempts`, feeds those generic queue records through the first typed email provider adapter in `backend/internal/email/`, and sends provider-shaped HTTP POST requests to `CALDIY_EMAIL_DISPATCH_URL` with `X-Cal-Email-Action` plus `X-Cal-Email-Provider` headers. It also writes slim calendar canary envelopes to `booking_calendar_dispatches`, snapshots outbound attempts in `booking_calendar_dispatch_attempts`, feeds those generic queue records through the first typed calendar provider adapter in `backend/internal/calendar/`, and sends provider-shaped HTTP POST requests to `CALDIY_CALENDAR_DISPATCH_URL` with `X-Cal-Calendar-Action` plus `X-Cal-Calendar-Provider` headers using the persisted selected/destination calendar refs and external event ids without widening the public booking DTO surface. Attempt rows now track `attempt_count`, `last_attempted_at`, `response_status`, `last_error`, `delivered_at`, and `dead_lettered_at`, so queue retries only resend webhook subscribers that are still pending, while email and calendar provider attempts keep generic error text and response codes without storing provider-specific response bodies. Exhausted webhook attempts disable their subscription with a generic reason, and worker logs include counts for pending, failed-pending, dead-lettered attempts, and disabled subscribers. The fixture signer resolves the shared secret from runtime config by key ref, so the database stores key refs and signed attempt snapshots but not raw webhook signing secrets, and Compose now includes a local sink for end-to-end verification of webhook, email-provider, and calendar-provider deliveries. Delivery remains at-least-once across network and persistence boundaries, so receivers should continue to de-duplicate by contract identifiers.
