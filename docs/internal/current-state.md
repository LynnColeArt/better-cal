# Current State

Last updated: 2026-04-24, during the OAuth access-token authentication slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `d07f9c20c2 feat: add oauth token exchange canary`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next Phase 7 OAuth slice:

- intended commit message: `feat: authenticate booking reads with oauth access token`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

The previous OAuth slice added a fixture authorization-code exchange and persisted only hashed authorization codes plus hashed access/refresh tokens.

This slice makes the issued access token useful for one narrow protected route:

- `GET /v2/bookings/{bookingUid}` accepts either the existing API key or a valid OAuth access token;
- access tokens are looked up by SHA-256 hash only;
- expired or revoked access tokens do not authenticate;
- effective principal permissions are narrowed to the token scopes before policy checks;
- insufficient OAuth scope reaches the route as a valid principal and fails authorization with `403`.

Broader booking writes, refresh-token rotation, and multi-route OAuth rollout remain deferred.

## Implemented Changes

Auth service:

- [service.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service.go)
- Adds `AuthenticateOAuthAccessTokenContext`.
- Stores issued fixture access tokens in memory only for the non-Postgres test path.
- Narrows effective token permissions to the intersection of stored principal permissions and OAuth scopes.

Postgres auth repository:

- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository.go)
- Adds `ReadOAuthAccessTokenPrincipal`.
- Reads `oauth_tokens` by `access_token_sha256`.
- Requires `access_expires_at > now` and `revoked_at is null`.
- Returns only scope-limited permissions.

HTTP API:

- [server.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server.go)
- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- Adds `authenticateAPIKeyOrOAuthAccessToken`.
- Wires it only into `GET /v2/bookings/{bookingUid}` for this canary.

Contracts and docs:

- [enums.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/enums.json)
- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- Adds `oauth-access-token` as a draft auth mode.
- Adds `oauth-access-token` to `policy.booking.read`.
- Updates README, project plan, scaffold docs, and security regression notes.

Tests:

- [service_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository_test.go)
- [server_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server_test.go)
- Adds fixture access-token authentication coverage.
- Adds Postgres lookup coverage for valid, missing, expired, and revoked access tokens.
- Adds HTTP proof that an exchanged token can read a booking.
- Adds HTTP proof that a valid token without `booking:read` fails with `403`.

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

- First `POST /v2/auth/oauth2/token` with the fixture code returned `200`.
- `GET /v2/bookings/mock-booking-personal-basic` with the returned access token returned `200`.
- Immediate replay of the same authorization code returned `400 invalid_grant`.

## Next Slice Recommendation

After this access-token canary is committed, the next high-leverage Phase 7 slice is refresh-token rotation:

1. Add `grant_type=refresh_token`.
2. Look up refresh tokens by hash.
3. Rotate refresh tokens atomically.
4. Revoke or mark the old refresh token as consumed.
5. Deny refresh-token replay.

The app catalog/app-store surface is still a good Phase 6/7 adjacent slice, but refresh rotation completes the OAuth runtime loop we just opened.
