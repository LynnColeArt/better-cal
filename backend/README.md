# Better Cal Backend

This is the starter Go backend for the whiteroom replacement project.

The first implementation slice is deliberately small and fixture-driven. It supports:

- `GET /health`
- `GET /v2/me`
- `GET /v2/auth/oauth2/clients/{clientId}`
- `GET /v2/oauth-clients/{clientId}`
- `POST /v2/bookings`
- `GET /v2/bookings/{bookingUid}`
- `POST /v2/bookings/{bookingUid}/cancel`
- `POST /v2/bookings/{bookingUid}/reschedule`

The API shell includes request ID propagation, panic recovery, structured request logging with secret-bearing headers redacted, a small auth service that resolves fixture API-key principals, OAuth clients, and platform clients, an explicit deny-by-default authz package for named route policies, and a booking service for the current accepted lifecycle fixtures. When `CALDIY_DATABASE_URL` is set, API-key principal lookup, OAuth client metadata lookup, booking fixture state, and idempotency keys are stored in Postgres through repository adapters; otherwise the service falls back to config and in-memory fixture state.

Run locally:

```bash
cd backend
go run ./cmd/api
```

Run with Docker Compose from the repository root:

```bash
docker compose up --build
```

Run Go tests:

```bash
cd backend
go test ./...
```

Run the Postgres-backed database integration tests against the Compose database:

```bash
CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" go test ./internal/db ./internal/auth ./internal/booking
```

Run contract replay smoke from the repository root:

```bash
node tools/backend-smoke/smoke-test.mjs
```

When `CALDIY_DATABASE_URL` is set, the API opens Postgres, runs embedded migrations, seeds the fixture API-key principal in `api_key_principals` using a SHA-256 token hash, seeds non-secret OAuth client metadata in `oauth_clients`, and stores the booking fixture canary in `booking_fixtures` with duplicate create detection in `booking_idempotency_keys`. Provider integrations and durable side effects will be added behind the same API adapter surface as the accepted contracts expand.
