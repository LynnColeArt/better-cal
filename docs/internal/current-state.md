# Current State

Last updated: 2026-04-24, during the provider status refresh slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `fb62e581e2 feat: add integration credential metadata canary`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next Phase 6 slice:

- intended commit message: `feat: refresh integration connection status`
- this session has full local Git and network permissions again, so normal commit and push should work.

## Slice Purpose

The previous credential metadata canary added `GET /v2/credentials` and non-secret rows in `integration_credential_metadata`.

This slice adds the next non-secret provider boundary: sanitized integration status refresh. Providers can now return generic status facts for existing credential metadata and calendar connections. The stores update only:

- operational status;
- sanitized status code;
- checked timestamp.

The refresh path does not create credentials or calendar connections, does not store provider credential payloads, and does not store raw provider errors or raw provider responses.

## Implemented Changes

Generic provider status contract:

- [status.go](/home/lynn/projects/cal.diy-security-check/backend/internal/integrations/status.go)
- Adds `StatusProviderAdapter`.
- Adds `StatusInput`, `StatusSnapshot`, `CredentialStatus`, and `CalendarConnectionStatus`.
- This package stays source-neutral and does not contain provider credentials.

Google fixture provider:

- [google_fixture_provider.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendar/google_fixture_provider.go)
- `NewGoogleFixtureProvider()` now implements catalog sync, booking calendar dispatch, and generic integration status refresh.
- For fixture user id `123`, status refresh returns:
  - credential ref `google-calendar-credential-fixture`
  - connection ref `google-calendar-connection-fixture`
  - status `active`
  - status code `ok`

Credential metadata:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/credentials/store.go)
- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/credentials/postgres_repository.go)
- Adds `statusCode` and `statusCheckedAt` to credential metadata responses.
- Adds `RefreshProviderStatus`.
- Validates provider status updates against existing credential refs, provider names, and account refs.
- Rejects unknown or duplicate provider status rows instead of creating new credential metadata.

Calendar connections:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/store.go)
- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/postgres_repository.go)
- Adds `statusCode` and `statusCheckedAt` to calendar connection responses.
- Adds `RefreshProviderConnectionStatus`.
- Validates provider status updates against existing connection refs, provider names, and account refs.
- Records a generic `provider_status_refresh` status transition when the operational status changes.

Migration:

- [0021_integration_status_refresh.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0021_integration_status_refresh.sql)
- Adds nullable `status_code` and `status_checked_at` columns to:
  - `calendar_connections`
  - `integration_credential_metadata`

API startup:

- [main.go](/home/lynn/projects/cal.diy-security-check/backend/cmd/api/main.go)
- Postgres startup now uses one fixture provider for catalog sync and status refresh.
- Startup seeds credential metadata, refreshes credential status, syncs calendar catalog, then refreshes calendar connection status.

Tests:

- [google_fixture_provider_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendar/google_fixture_provider_test.go)
- [store_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/credentials/store_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/credentials/postgres_repository_test.go)
- [store_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/store_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/postgres_repository_test.go)
- Adds fixture provider status coverage.
- Adds in-memory credential and calendar connection status refresh coverage.
- Adds invalid provider status snapshot coverage.
- Adds Postgres status refresh coverage for credential metadata and calendar connection status transitions.

## Verification

Checks passed:

```bash
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/integrations ./internal/calendar ./internal/calendars ./internal/credentials ./cmd/api
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/httpapi
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./...
node tools/contracts/validate-contracts.mjs
git diff --check
cd backend && CALDIY_TEST_DATABASE_URL='postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable' go test ./internal/db ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots -v
docker compose up --build -d
node tools/backend-smoke/smoke-test.mjs
node tools/fixture-capture/smoke-test.mjs
docker compose --profile tools run --rm contracts
docker compose logs --no-color api postgres webhook-sink > /tmp/better-cal-compose.log
node tools/contracts/scan-secrets.mjs --path /tmp/better-cal-compose.log
```

The live API and database probe confirmed `statusCode = ok` and non-null `statusCheckedAt` for the fixture credential metadata and calendar connection rows.

## Next Slice Recommendation

After this status refresh slice is committed, the next useful Phase 6 slice is an app metadata/catalog reader:

1. Add a source-neutral app catalog type with public app metadata only.
2. Seed fixture app metadata for Google Calendar without credential payloads.
3. Connect credential metadata rows to public app metadata by app slug.
4. Keep real OAuth credential encryption/decryption deferred until the non-secret app, credential, calendar, and status boundaries are stable.

Security rule: provider integration state may store opaque refs, public provider metadata, generic scopes, operational status, sanitized status codes, and checked timestamps. It must not store or return raw credential keys, provider tokens, refresh tokens, raw provider responses, raw provider error bodies, or signing material.
