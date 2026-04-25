# Current State

Last updated: 2026-04-25 07:07 CDT, during the app install intent slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `95606b60df feat: add app catalog metadata canary`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree contains the app install intent slice:

- intended commit message: `feat: add app install intent canary`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

This slice adds the first product-visible app install action without implementing real OAuth, credential exchange, app enablement, or provider onboarding.

The boundary is deliberately small:

1. accept a selected app slug through `POST /v2/app-install-intents`;
2. require `apps:install` through `policy.apps.install`;
3. validate that the selected app exists in the non-secret app catalog;
4. persist only user id, selected app slug, an opaque install intent ref, pending status, and timestamps;
5. return only the intent ref, selected app slug, pending status, timestamps, and request id.

## Implemented Changes

App install intent storage:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/apps/store.go)
- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/apps/postgres_repository.go)
- [0024_integration_app_install_intents.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0024_integration_app_install_intents.sql)
- Adds `integration_app_install_intents` with no credential, token, provider payload, raw response, or signing fields.
- Keeps the returned `AppInstallIntent` DTO free of internal user ids.

API and policy:

- [server.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server.go)
- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- [policy.go](/home/lynn/projects/cal.diy-security-check/backend/internal/authz/policy.go)
- Adds `POST /v2/app-install-intents`, `policy.apps.install`, and fixture principal permission `apps:install`.
- Unknown app slugs return `404`; missing `apps:install` returns `403`.

Contracts and docs:

- [routes.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/routes.json)
- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- [route-auth-mode-coverage.md](/home/lynn/projects/cal.diy-security-check/contracts/security/route-auth-mode-coverage.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- Records the route/policy mapping and the no-secret invariant for install intents.

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

Live route probes:

```bash
curl -fsS -X POST \
  -H 'Authorization: Bearer cal_test_valid_mock' \
  -H 'Content-Type: application/json' \
  --data '{"appSlug":"google-calendar"}' \
  http://127.0.0.1:8080/v2/app-install-intents
```

Result:

- returned a pending `installIntent` for `google-calendar`;
- no forbidden response terms were present: `secret`, `token`, `encrypted`, `refresh`, `access_token`, `refresh_token`, `credentialRef`, `providerPayload`, `rawProvider`, `accountRef`, `accountLabel`, `userId`, or `user_id`.

Unknown app probe:

```bash
curl -sS -o /tmp/app-install-unknown.out -w '%{http_code}' \
  -X POST \
  -H 'Authorization: Bearer cal_test_valid_mock' \
  -H 'Content-Type: application/json' \
  --data '{"appSlug":"unknown-app"}' \
  http://127.0.0.1:8080/v2/app-install-intents
```

Result: `404` with `{"code":"NOT_FOUND","message":"App not found"}`.

Report consistency check:

- The table in `contracts/security/route-auth-mode-coverage.md` matched `node tools/contracts/check-policy-coverage.mjs --report`.

## Next Slice Recommendation

The next useful slice is to turn pending install intents into a safe handoff state:

1. add a read route for current-user install intents;
2. add an explicit `requires_external_auth` or `ready_for_credential_exchange` status transition;
3. keep token exchange, provider credentials, and OAuth callback handling out until the state machine is covered by no-secret tests.
