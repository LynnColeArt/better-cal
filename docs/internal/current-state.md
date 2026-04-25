# Current State

Last updated: 2026-04-25, during the OAuth booking host-action access-token slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `2695a2e0d3 feat: authorize booking writes with oauth access tokens`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next Phase 7 OAuth authorization slice:

- intended commit message: `feat: authorize booking host actions with oauth access tokens`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

The previous OAuth slice extended scoped access-token auth from booking reads to create, cancel, and reschedule writes.

This slice closes the remaining starter booking lifecycle route gap by allowing OAuth access tokens on host actions:

- `POST /v2/bookings/{bookingUid}/confirm`
- `POST /v2/bookings/{bookingUid}/decline`

Scoped OAuth access tokens now carry `booking:host-action` from the fixture authorization code. Confirm and decline still use `policy.booking.host-action`; the token must have the host-action scope and the principal must match the booking host resource. A token with only booking read/write scopes fails, and a permissioned non-host still fails.

No provider credential storage, provider refresh tokens, or app-store behavior is introduced in this slice.

## Implemented Changes

Auth service:

- [service.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service.go)
- The fixture OAuth authorization code now issues `booking:host-action` alongside `booking:read` and `booking:write`.
- Refresh-token rotation preserves the same scope set because it rotates from the stored token row scopes.

HTTP API:

- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- `confirmBooking` and `declineBooking` now use `authenticateAPIKeyOrOAuthAccessToken`.
- The existing `policy.booking.host-action` permission and host resource checks remain in place.

Policies:

- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- `policy.booking.host-action` now includes `oauth-access-token` in its allowed auth modes.
- Required permission and resource resolver remain `booking:host-action` and `booking-host-or-platform-permission`.

Tests:

- [service_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository_test.go)
- [server_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server_test.go)
- Updates OAuth token scope expectations to include `booking:host-action`.
- Adds HTTP proof that an issued OAuth access token can confirm and decline pending bookings.
- Adds read/write-only OAuth token denial for a host action.
- Adds non-host OAuth token denial for a host action.

Docs:

- [README.md](/home/lynn/projects/cal.diy-security-check/backend/README.md)
- [project-plan.md](/home/lynn/projects/cal.diy-security-check/docs/internal/project-plan.md)
- [implementation-scaffold.md](/home/lynn/projects/cal.diy-security-check/docs/spec/implementation-scaffold.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- Updates the Phase 7 and security notes so booking read/write plus confirm/decline host-action routes are described as scoped OAuth access-token canaries.

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
- The token scope included `booking:host-action`.
- `POST /v2/bookings/mock-booking-pending-confirm/confirm` with the issued access token returned `200`.
- `POST /v2/bookings/mock-booking-pending-decline/decline` with the same access token returned `200`.
- Replaying the original authorization code returned `400 invalid_grant`.

## Next Slice Recommendation

After this host-action OAuth canary is committed, the starter booking lifecycle OAuth route matrix is coherent:

1. booking read uses `booking:read`;
2. create/cancel/reschedule use `booking:write`;
3. confirm/decline use `booking:host-action`.

The next technical slice should either add a compact route/auth-mode coverage report for the policy registry or switch back to the app catalog/app-store metadata slice for product-visible integration work.
