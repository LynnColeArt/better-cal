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

The API shell includes request ID propagation, panic recovery, structured request logging with secret-bearing headers redacted, and a small auth service that resolves fixture API-key principals, OAuth clients, and platform clients.

Run locally:

```bash
cd backend
go run ./cmd/api
```

Run Go tests:

```bash
cd backend
go test ./...
```

Run contract replay smoke from the repository root:

```bash
node tools/backend-smoke/smoke-test.mjs
```

The current state uses in-memory fixture data only. Persistence, provider integrations, and durable side effects will be added behind the same API adapter surface as the accepted contracts expand.
