# Better Cal Backend

This is the starter Go backend for the whiteroom replacement project.

The first implementation slice is deliberately small and fixture-driven. It supports:

- `GET /health`
- `GET /v2/me`
- `GET /v2/slots`
- `GET /v2/auth/oauth2/clients/{clientId}`
- `GET /v2/oauth-clients/{clientId}`
- `POST /v2/bookings`
- `GET /v2/bookings/{bookingUid}`
- `POST /v2/bookings/{bookingUid}/cancel`
- `POST /v2/bookings/{bookingUid}/reschedule`
- `POST /v2/bookings/{bookingUid}/confirm`
- `POST /v2/bookings/{bookingUid}/decline`

The API shell includes request ID propagation, panic recovery, structured request logging with secret-bearing headers redacted, a small auth service that resolves fixture API-key principals, OAuth clients, and platform clients, an explicit deny-by-default authz package for named route policies and booking resource checks, a slot service for the current accepted availability fixtures, and a booking service for the current accepted lifecycle fixtures. Booking creation and reschedule calls use the slot service through an availability adapter before persistence so unavailable fixture slots and internally busy slots are rejected by the same service boundary that backs `GET /v2/slots`. Booking read/write/host-action routes require the caller to have both the named permission and the fixture owner or host principal id; wrong-owner fixtures authenticate but fail with `403`. Confirm and decline use the host-action policy and the same typed side-effect planning boundary as cancel and reschedule. When `CALDIY_DATABASE_URL` is set, API-key principal lookup, OAuth client metadata lookup, platform client verification, explicit booking rows, booking fixture fallback state, idempotency keys, planned booking side effects, event type metadata, and fixture availability slots are stored in Postgres through repository adapters; otherwise the service falls back to config and in-memory fixture state.

Run locally:

```bash
cd backend
go run ./cmd/api
```

Run with Docker Compose from the repository root:

```bash
docker compose up --build
```

Run one worker pass through Compose:

```bash
docker compose --profile worker run --rm worker
```

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
CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" go test ./internal/db ./internal/auth ./internal/booking ./internal/slots
```

Run contract replay smoke from the repository root:

```bash
node tools/backend-smoke/smoke-test.mjs
```

When `CALDIY_DATABASE_URL` is set, the API opens Postgres, runs embedded migrations, seeds the fixture API-key principal in `api_key_principals` using a SHA-256 token hash, seeds non-secret OAuth client metadata in `oauth_clients`, stores only a SHA-256 hash for platform client secret verification in `platform_clients`, seeds fixture event type metadata and availability in `event_types` and `availability_slots`, writes booking fields to `bookings` and `booking_attendees`, filters availability against accepted booking rows, and keeps `booking_fixtures` as a compatibility fallback with duplicate create detection in `booking_idempotency_keys`. Booking state and planned side effects are committed in one transaction, and idempotency keys are locked so conflicting retries replay the first booking instead of creating stray state. The worker command claims planned or retryable side effects with Postgres row locks, dispatches through the current durable canary dispatcher, records one `booking_side_effect_dispatch_log` row per side effect, marks delivered rows, and records only a generic failure marker for retryable dispatch errors. Provider integrations will be added behind the same API adapter surface as the accepted contracts expand.
