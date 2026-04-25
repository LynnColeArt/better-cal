# Current State

Last updated: 2026-04-25 04:52 CDT, during the app catalog metadata canary slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `084957249d chore: add route auth mode coverage report`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree contains the app catalog metadata canary slice:

- intended commit message: `feat: add app catalog metadata canary`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

This slice starts the app store/catalog path without implementing app install flows, real provider onboarding, or credential payload storage.

The implemented boundary is deliberately narrow:

1. persist a small non-secret app catalog table;
2. seed fixture app metadata for Google Calendar and Resend;
3. expose `GET /v2/apps` behind an enforced `apps:read` policy;
4. keep credential refs, account refs, provider tokens, raw provider responses, provider error bodies, and signing material out of both storage and response tests.

## Implemented Changes

App catalog:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/apps/store.go)
- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/apps/postgres_repository.go)
- [0023_integration_app_catalog.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0023_integration_app_catalog.sql)
- Adds `integration_app_catalog` with app slug, category, provider, name, description, auth type, capabilities, and timestamps only.
- Adds fixture catalog rows for `google-calendar` and `resend-email`.

API and policy:

- [server.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server.go)
- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- [main.go](/home/lynn/projects/cal.diy-security-check/backend/cmd/api/main.go)
- [policy.go](/home/lynn/projects/cal.diy-security-check/backend/internal/authz/policy.go)
- Adds `GET /v2/apps`, `policy.apps.read`, and fixture principal permission `apps:read`.
- Seeds app catalog metadata when `CALDIY_DATABASE_URL` is configured.

Contracts and docs:

- [routes.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/routes.json)
- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- [route-auth-mode-coverage.md](/home/lynn/projects/cal.diy-security-check/contracts/security/route-auth-mode-coverage.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- Records the route/policy mapping and the app catalog no-secret invariant.

Test stability:

- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/booking/postgres_repository_test.go)
- Tightens planned side-effect claim tests so they temporarily park unrelated claimable rows in a shared Compose test database and restore them during cleanup.

## Verification

Checks passed:

```bash
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./...
node tools/contracts/check-policy-coverage.mjs
node tools/contracts/validate-contracts.mjs
git diff --check
docker compose up --build -d
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" GOCACHE=/tmp/caldiy-go-build go test ./internal/db ./internal/apps ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots -v
node tools/backend-smoke/smoke-test.mjs
docker compose --profile tools run --rm contracts
```

Live route probe:

```bash
curl -fsS -H 'Authorization: Bearer cal_test_valid_mock' http://127.0.0.1:8080/v2/apps
```

Result:

- returned `google-calendar` and `resend-email`;
- no forbidden app catalog response terms were present: `secret`, `token`, `encrypted`, `refresh`, `access_token`, `refresh_token`, `credentialRef`, `providerPayload`, `rawProvider`, `accountRef`, or `accountLabel`.

Report consistency check:

- The table in `contracts/security/route-auth-mode-coverage.md` matched `node tools/contracts/check-policy-coverage.mjs --report`.

## Next Slice Recommendation

The next useful product-visible slice is app install intent without secrets:

1. add an install-intent DTO for a selected app;
2. route the intent through policy and validation;
3. persist only opaque refs and status, leaving provider credentials and token exchange for a later OAuth/install slice.
