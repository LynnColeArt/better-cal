# Current State

Last updated: 2026-04-24, during the OAuth refresh-token rotation slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `39b43d8399 feat: authenticate booking reads with oauth access token`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next Phase 7 OAuth slice:

- intended commit message: `feat: rotate oauth refresh tokens`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

The previous OAuth slice made issued access tokens usable for `GET /v2/bookings/{bookingUid}` with permissions narrowed to token scopes.

This slice adds refresh-token rotation to the existing token endpoint:

- `POST /v2/auth/oauth2/token` accepts `grant_type=refresh_token`;
- refresh tokens are looked up only by SHA-256 hash;
- the old token row is revoked when rotation succeeds;
- a new access/refresh token pair is inserted in the same transaction;
- old refresh-token replay returns `invalid_grant`;
- old access tokens tied to the revoked row no longer authenticate;
- expired refresh tokens return `invalid_grant`.

No provider credential storage, provider refresh tokens, or app-store behavior is introduced in this slice.

## Implemented Changes

Auth service:

- [service.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service.go)
- Extends `OAuthTokenExchangeRequest` with `RefreshToken`.
- Supports both `authorization_code` and `refresh_token` grants.
- Adds fixture refresh-token rotation for the non-Postgres test path.
- Deletes the old fixture access token and old refresh token during rotation.

Postgres auth repository:

- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository.go)
- Adds `ExchangeOAuthRefreshToken`.
- Locks the old refresh-token row with `for update`.
- Rejects wrong-client, revoked, missing, and expired refresh tokens.
- Sets `revoked_at` on the old row and inserts the new hashed access/refresh pair in one transaction.

HTTP API:

- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- Decodes `refresh_token` in JSON and form-encoded token requests.
- Passes refresh-token grants through the existing OAuth token endpoint and error mapping.

Contracts and docs:

- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- Adds refresh-token replay and expired-refresh-token negative fixture names to `policy.oauth2.token.exchange`.
- Updates README, project plan, scaffold docs, and security regression notes.

Tests:

- [service_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository_test.go)
- [server_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server_test.go)
- Adds in-memory refresh-token rotation coverage.
- Adds Postgres rotation coverage for new token issuance, old-row revocation, old access-token denial, replay denial, expired refresh-token denial, and raw token non-storage.
- Adds HTTP proof that refresh rotation returns a new access token, old access stops authenticating, new access reads the booking, and refresh replay is denied.

## Verification

Checks passed:

```bash
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./...
node tools/contracts/validate-contracts.mjs
git diff --check
cd backend && CALDIY_TEST_DATABASE_URL='postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable' GOCACHE=/tmp/caldiy-go-build go test ./internal/db ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots -v
docker compose up --build -d
node tools/backend-smoke/smoke-test.mjs
node tools/fixture-capture/smoke-test.mjs
docker compose --profile tools run --rm contracts
docker compose logs --no-color api postgres webhook-sink > /tmp/better-cal-compose.log
node tools/contracts/scan-secrets.mjs --path /tmp/better-cal-compose.log
```

Live API probe:

- First `POST /v2/auth/oauth2/token` with the fixture authorization code returned `200`.
- `grant_type=refresh_token` with the returned refresh token returned `200`.
- `GET /v2/bookings/mock-booking-personal-basic` with the old access token returned `401`.
- The same booking read with the rotated access token returned `200`.
- Replaying the old refresh token returned `400 invalid_grant`.
- Replaying the original authorization code still returned `400 invalid_grant`.

## Next Slice Recommendation

After this refresh-token rotation canary is committed, the OAuth runtime loop is coherent enough to either:

1. extend OAuth access-token auth from booking read to a small booking write path with scope and resource tests; or
2. switch back to the app catalog/app-store metadata slice, now that the auth spine is less shaky.

The safer technical continuation is the first option; the more product-visible continuation is the second.
