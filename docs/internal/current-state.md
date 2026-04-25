# Current State

Last updated: 2026-04-25, during the OAuth booking-write access-token slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `1eece38d56 feat: rotate oauth refresh tokens`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next Phase 7 OAuth authorization slice:

- intended commit message: `feat: authorize booking writes with oauth access tokens`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

The previous OAuth slice made refresh-token rotation replay-safe and revoked old access-token rows after rotation.

This slice extends OAuth access-token authentication from booking read to the first booking write routes:

- `POST /v2/bookings`
- `POST /v2/bookings/{bookingUid}/cancel`
- `POST /v2/bookings/{bookingUid}/reschedule`

Scoped OAuth access tokens now pass through the same booking write policy as API keys. They must carry `booking:write`, and the existing booking owner resource check still decides whether the principal can mutate the target booking or event type. Host confirm/decline routes remain on the host-action path and are not changed by this slice.

No provider credential storage, provider refresh tokens, or app-store behavior is introduced in this slice.

## Implemented Changes

HTTP API:

- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- `createBooking`, `cancelBooking`, and `rescheduleBooking` now use `authenticateAPIKeyOrOAuthAccessToken`.
- OAuth access-token authentication still falls back only after API-key authentication is absent, preserving the existing API-key path.

Policies:

- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- `policy.booking.write` now includes `oauth-access-token` in its allowed auth modes.
- Required permission and resource resolver remain `booking:write` and `booking-target-event-type`.

Tests:

- [server_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server_test.go)
- Adds HTTP proof that an issued OAuth access token can create, cancel, and reschedule bookings.
- Adds read-only OAuth token denial for a booking write.
- Adds wrong-owner OAuth token denial for a booking write.

Docs:

- [README.md](/home/lynn/projects/cal.diy-security-check/backend/README.md)
- [project-plan.md](/home/lynn/projects/cal.diy-security-check/docs/internal/project-plan.md)
- [implementation-scaffold.md](/home/lynn/projects/cal.diy-security-check/docs/spec/implementation-scaffold.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- Updates the Phase 7 and security notes so booking read plus create/cancel/reschedule write routes are described as scoped OAuth access-token canaries.

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

- `POST /v2/auth/oauth2/token` with the fixture authorization code returned `200`.
- `POST /v2/bookings` with the issued access token and a unique idempotency key returned `201`.
- `POST /v2/bookings/{newBookingUid}/cancel` with the same access token returned `200`.
- Replaying the original authorization code returned `400 invalid_grant`.

## Next Slice Recommendation

After this booking-write OAuth canary is committed, the safest technical continuation is to close the remaining route gap around host-action authorization:

1. decide whether host confirm/decline should accept OAuth access tokens or stay host-action/API-key only;
2. if OAuth is allowed, add an explicit `booking:host-action` scope test plus non-host denial;
3. update the route policy registry and security matrix to keep the auth-mode contract honest.

The more product-visible alternative is to switch back to the app catalog/app-store metadata slice, but that can wait until the route authorization matrix is less lopsided.
