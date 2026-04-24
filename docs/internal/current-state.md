# Current State

Last updated: 2026-04-24, during the non-secret integration credential metadata slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Current local `HEAD`: `458e038159 feat: sync calendar catalog from provider`
- Last pushed commit before this handoff: `458e038159 feat: sync calendar catalog from provider`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The current working tree contains an implemented but not locally committed slice:

- intended commit message: `feat: add integration credential metadata canary`
- the previous calendar catalog sync slice has been committed and pushed as `458e038159`.
- `.git` is still not writable from the Codex sandbox, so commit from a normal terminal or fresh writable session.

## Why This Slice Exists

The previous slice added current-user calendar management routes and catalog tables:

- `GET /v2/calendar-connections`
- `GET /v2/calendars`
- selected/destination calendar writes backed by `calendar_connections` and `calendar_catalog`

This slice turns those rows into provider-driven state instead of purely seeded fixture data. The public API stays stable. The new behavior is an internal sync boundary that refreshes non-secret calendar connection/catalog state from the typed calendar provider adapter.

## Implemented Changes

Calendar provider contract:

- [provider.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendar/provider.go)
- Adds `CatalogProviderAdapter`.
- Adds `CatalogInput`, `CatalogSnapshot`, `CatalogConnection`, and `CatalogCalendar`.
- This is deliberately separate from the existing booking dispatch `ProviderAdapter`, though the Google fixture provider implements both.

Google fixture provider:

- [google_fixture_provider.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendar/google_fixture_provider.go)
- `NewGoogleFixtureProvider()` now supports `ReadCatalog`.
- For fixture user id `123`, it returns:
  - one active connection: `google-calendar-connection-fixture`
  - three catalog calendars: `destination-calendar-fixture`, `selected-calendar-fixture`, `team-calendar-fixture`
- For other users, it returns an empty catalog snapshot.
- No provider secrets, tokens, refresh tokens, raw provider responses, or credential payloads are represented.

Calendar store:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/store.go)
- Adds `WithCatalogProvider`.
- Adds `SyncProviderCatalog(ctx, userID)`.
- Converts provider snapshots into internal `CalendarConnection` and `CatalogCalendar` values.
- Validates that every catalog calendar references a synced connection.
- Persists connection/catalog rows through the repository.
- Records connection status transitions when a provider sync changes a connection status.
- Refreshes selected-calendar snapshots from synced catalog rows and clears selected/destination state for calendars that disappear from the provider snapshot.
- Keeps selected/destination calendar writes catalog-backed, so clients still cannot inject arbitrary provider metadata.
- In-memory mode can sync provider state too, which keeps unit tests and no-DB fixtures useful.

Postgres repository:

- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/postgres_repository.go)
- Adds `RecordCalendarConnectionStatusTransition`.
- Adds transactional `SyncCalendarCatalog`.
- Stores only generic transition data:
  - user id
  - connection ref
  - provider
  - previous status
  - next status
  - reason
- The sync reason currently used by the store is `provider_catalog_sync`.
- Provider sync now reads existing rows, prunes stale catalog/connection rows, refreshes still-selected calendar snapshots, and records status transitions in one transaction so a failed history write cannot leave a status update without its transition row.

Migration:

- [0019_calendar_connection_status_history.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0019_calendar_connection_status_history.sql)
- Adds `calendar_connection_status_history`.
- Foreign key points to `(user_id, connection_ref)` in `calendar_connections`.
- Adds an index on `(user_id, connection_ref, created_at desc)`.

API startup:

- [main.go](/home/lynn/projects/cal.diy-security-check/backend/cmd/api/main.go)
- When Postgres is enabled, API startup now creates the calendar store with:
  - `calendars.NewPostgresRepository(pool)`
  - `calendars.WithCatalogProvider(calendarprovider.NewGoogleFixtureProvider())`
- Startup then calls `calendarStore.SyncProviderCatalog(ctx, auth.FixtureAPIKeyPrincipal().ID)` before installing the store into the HTTP server.

Tests:

- [google_fixture_provider_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendar/google_fixture_provider_test.go)
- [store_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/store_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/calendars/postgres_repository_test.go)
- [db_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/db/db_test.go)
- Adds provider catalog snapshot coverage.
- Adds in-memory store sync coverage.
- Adds in-memory selected-calendar refresh/pruning coverage and invalid provider snapshot coverage.
- Adds Postgres store sync coverage that updates connection status from `active` to `disconnected`, asserts a `calendar_connection_status_history` row, prunes stale catalog/connection rows, refreshes selected-calendar snapshots, and clears stale destination calendars.
- Adds migration table coverage for `calendar_connection_status_history`.

Docs:

- [backend README](/home/lynn/projects/cal.diy-security-check/backend/README.md)
- [project-plan.md](/home/lynn/projects/cal.diy-security-check/docs/internal/project-plan.md)
- [implementation-scaffold.md](/home/lynn/projects/cal.diy-security-check/docs/spec/implementation-scaffold.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- These now describe provider catalog sync, connection status history, and the security rule that sync stores only non-secret snapshots and generic transitions.

Credential metadata:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/credentials/store.go)
- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/credentials/postgres_repository.go)
- [0020_integration_credential_metadata.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0020_integration_credential_metadata.sql)
- Adds non-secret `CredentialMetadata` values with:
  - credential ref
  - app slug/category
  - provider
  - account ref/label
  - status
  - scopes
  - timestamps
- Adds `GET /v2/credentials` with `policy.credentials.read`.
- Seeds fixture metadata for the fixture user when Postgres is enabled.
- The table and response intentionally have no provider keys, access tokens, refresh tokens, encrypted payloads, raw provider responses, raw provider errors, or signing material.

## Current Working Tree

At this handoff, `git status --short` showed:

```text
 M backend/README.md
 M backend/cmd/api/main.go
 M backend/internal/auth/service.go
 M backend/internal/authz/policy.go
 M backend/internal/authz/policy_test.go
 M backend/internal/db/db_test.go
 M backend/internal/httpapi/handlers.go
 M backend/internal/httpapi/server.go
 M backend/internal/httpapi/server_test.go
 M contracts/registries/policies.json
 M contracts/registries/routes.json
 M docs/internal/project-plan.md
 M docs/internal/route-inventory.md
 M docs/spec/implementation-scaffold.md
 M docs/spec/security-regression-controls.md
?? backend/internal/credentials/
?? backend/internal/db/migrations/0020_integration_credential_metadata.sql
?? docs/internal/current-state.md
```

The next session should include this file in the commit unless the user explicitly wants handoff notes kept out of history.

## Verification Completed

These checks passed in the restricted session:

```bash
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/calendar ./internal/calendars ./internal/db ./internal/authz ./internal/email ./internal/booking ./internal/slots
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/calendars -count=1
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/credentials ./internal/auth ./internal/authz ./internal/db ./cmd/api
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/calendar ./internal/calendars ./internal/credentials ./internal/db ./internal/auth ./internal/authz ./internal/email ./internal/booking ./internal/slots
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./... -run '^$'
node tools/contracts/validate-contracts.mjs
git diff --check
```

The package-level test run includes the new in-memory provider sync, selected-calendar refresh/pruning, invalid-snapshot tests, and credential metadata no-leak tests. The compile-only `go test ./... -run '^$'` confirms the whole backend still builds in the sandbox. Postgres integration tests were not run in this sandbox because neither `CALDIY_TEST_DATABASE_URL` nor `CALDIY_DATABASE_URL` is set.

## Verification Blocked In This Session

The active environment still blocked these, even after the permission update:

- `.git` was mounted read-only:
  - local `git commit` failed because Git could not create `.git/index.lock`.
- Docker daemon access was denied:
  - `docker compose ps` failed with `connect: operation not permitted`.
- Localhost listen/connect was denied:
  - HTTP smoke tests using local servers failed with `listen EPERM`.
  - Postgres integration tests against `127.0.0.1:54320` failed with socket permission errors.
- Shell Git push could not resolve GitHub:
  - `git push origin <sha>:refs/heads/main` failed with `Could not resolve host: github.com`.

Because of that, the following checks still need to be rerun in a fresh session with full permissions:

```bash
cd backend && go test ./...
cd backend && CALDIY_TEST_DATABASE_URL='postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable' go test ./internal/db ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots -v
docker compose up --build -d
node tools/contracts/validate-contracts.mjs
node tools/fixture-capture/smoke-test.mjs
node tools/backend-smoke/smoke-test.mjs
docker compose --profile tools run --rm contracts
docker compose logs --no-color api postgres webhook-sink > /tmp/better-cal-compose.log
node tools/contracts/scan-secrets.mjs --path /tmp/better-cal-compose.log
git diff --check
```

## Suggested Live Probe For The Fresh Session

After `docker compose up --build -d`, verify provider-backed catalog sync with API and database state:

```bash
curl -fsS -H 'Authorization: Bearer cal_test_valid_mock' http://127.0.0.1:8080/v2/calendar-connections
curl -fsS -H 'Authorization: Bearer cal_test_valid_mock' http://127.0.0.1:8080/v2/calendars
curl -fsS -H 'Authorization: Bearer cal_test_valid_mock' http://127.0.0.1:8080/v2/credentials
curl -fsS -H 'Authorization: Bearer cal_test_valid_mock' -H 'content-type: application/json' \
  --data-binary '{"calendarRef":"team-calendar-fixture"}' \
  http://127.0.0.1:8080/v2/selected-calendars
curl -fsS -H 'Authorization: Bearer cal_test_valid_mock' -H 'content-type: application/json' \
  --data-binary '{"calendarRef":"team-calendar-fixture"}' \
  http://127.0.0.1:8080/v2/destination-calendars
```

Then inspect the DB:

```bash
docker compose exec -T postgres psql -U better_cal -d better_cal -At -F '|' -c "
select connection_ref, provider, account_ref, account_email, status
from calendar_connections
where user_id = 123
order by connection_ref;

select calendar_ref, connection_ref, provider, external_id, name, is_primary, writable
from calendar_catalog
where user_id = 123
order by calendar_ref;

select credential_ref, app_slug, app_category, provider, account_ref, account_label, status, scopes
from integration_credential_metadata
where user_id = 123
order by credential_ref;
"
```

Expected highlights:

- connection ref: `google-calendar-connection-fixture`
- connection status: `active`
- catalog refs: `destination-calendar-fixture`, `selected-calendar-fixture`, `team-calendar-fixture`
- credential ref: `google-calendar-credential-fixture`
- no provider tokens or credential payloads in API responses, catalog tables, or credential metadata tables

The status transition table is covered by the Postgres test with a mutable provider. A live status-transition probe would require either a test-only provider or a one-off fixture mutation; avoid adding that to the public API unless it becomes an explicit operator feature.

## Commit And Push Plan

In the fresh session, if normal Git permissions are available:

```bash
git status --short
git add \
  backend/README.md \
  backend/cmd/api/main.go \
  backend/internal/auth/service.go \
  backend/internal/authz/policy.go \
  backend/internal/authz/policy_test.go \
  backend/internal/credentials/postgres_repository.go \
  backend/internal/credentials/postgres_repository_test.go \
  backend/internal/credentials/store.go \
  backend/internal/credentials/store_test.go \
  backend/internal/db/db_test.go \
  backend/internal/db/migrations/0020_integration_credential_metadata.sql \
  backend/internal/httpapi/handlers.go \
  backend/internal/httpapi/server.go \
  backend/internal/httpapi/server_test.go \
  contracts/registries/policies.json \
  contracts/registries/routes.json \
  docs/internal/project-plan.md \
  docs/internal/route-inventory.md \
  docs/internal/current-state.md \
  docs/spec/implementation-scaffold.md \
  docs/spec/security-regression-controls.md
git commit -m "feat: add integration credential metadata canary"
git push origin main
```

If the fresh session sees the old temporary commit object `50553627c9`, ignore it. The calendar catalog slice is already pushed as `458e038159`; this commit should contain only the credential metadata canary plus the previously uncommitted docs.

## Next Slice Recommendation

After the credential metadata canary is committed, the next useful slice is provider connection health/status refresh, still without real provider secrets:

1. Extend credential metadata and calendar connections with generic status/error-code refresh results only.
2. Keep catalog sync consuming only non-secret adapter output.
3. Add no-leak tests around provider error text before any real OAuth tokens or provider secrets are introduced.
4. Defer real credential encryption/decryption until the non-secret read and status boundaries are stable.

The guiding security rule remains: provider integration state may store opaque refs, public provider metadata, generic scopes, and generic operational status, but must not store or return raw credential keys, provider tokens, refresh tokens, raw provider responses, or raw provider error bodies.
