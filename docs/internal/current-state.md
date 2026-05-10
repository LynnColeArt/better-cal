# Current State

Last updated: 2026-05-02 04:36 CDT, after validating the simulated app installation activation canary.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`
- Intended commit message for this slice: `feat: activate simulated app installations`

The working tree contains the validated activation slice. Normal local commit and push are expected to work in this session.

## Slice Purpose

This slice projects a completed simulated app install into current-user installed-app metadata without crossing into real provider credential work. Activation happens only after terminal simulated completion and records an `active` app installation in `simulated` mode.

It still does not call Google, exchange provider authorization codes, create provider accounts, create credential refs, persist credential payloads, store raw callback query data, generate redirect URLs, or return provider responses.

The activation request accepts only:

1. the sanitized `installCompletionRef`.

The activation response, installed-app list, and progress read model expose only refs, status, mode, app/provider/catalog identifiers, capabilities, and timestamps.

## Implemented Changes

App installation activation:

- [store.go](/home/lynn/projects/cal.diy-security-check/backend/internal/apps/store.go)
- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/apps/postgres_repository.go)
- [0030_integration_app_installations.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0030_integration_app_installations.sql)
- Adds `integration_app_installations` with one current-user installed-app projection per install completion and install intent.
- Records activation status `active` and activation mode `simulated`.
- Rejects wrong-owner activation, missing completion refs, blocked completions, not-ready completions, invalid payloads, and activation replay.
- Adds `GET /v2/app-installations` for current-user installed-app metadata.
- Extends `GET /v2/app-install-intents/{installIntentRef}/progress` so terminal completed installs become `active` once an installation projection exists.
- Keeps the installation table free of provider code, token, credential, account, raw query, redirect URL, callback URL, provider response, provider error, or secret columns.

API and policy:

- [server.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server.go)
- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- [policy.go](/home/lynn/projects/cal.diy-security-check/backend/internal/authz/policy.go)
- Adds `POST /v2/app-install-intents/{installIntentRef}/activate`.
- Adds `GET /v2/app-installations`.
- Enforces `policy.app-install-intents.activate` with `apps:install`.
- Enforces `policy.app-installations.read` with `apps:read`.
- Strict JSON rejects provider-code/raw callback/query/provider response fields.
- Wrong-owner or missing completion returns `404`; blocked completion and replay return `409`.

Contracts and docs:

- [routes.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/routes.json)
- [policies.json](/home/lynn/projects/cal.diy-security-check/contracts/registries/policies.json)
- [route-auth-mode-coverage.md](/home/lynn/projects/cal.diy-security-check/contracts/security/route-auth-mode-coverage.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- Registers the new routes and policies.
- Adds negative fixtures for unknown install intent, unknown install completion, wrong owner, completion not ready, activation replay, provider-code rejection, insufficient permission, secret leak, and installation response leak.
- Updates README/project-plan/scaffold docs to describe the no-secret installed-app activation boundary.

## Verification

Checks passed:

```bash
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./internal/apps ./internal/authz ./internal/httpapi
node tools/contracts/validate-contracts.mjs
node tools/contracts/check-policy-coverage.mjs --report
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./...
docker compose up --build -d
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" GOCACHE=/tmp/caldiy-go-build go test ./internal/db ./internal/apps ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots -v
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal_activation_scratch?sslmode=disable" GOCACHE=/tmp/caldiy-go-build go test -p 1 ./internal/db ./internal/apps
node tools/backend-smoke/smoke-test.mjs
docker compose --profile tools run --rm contracts
git diff --check
```

The generated policy report matched `contracts/security/route-auth-mode-coverage.md`.

Live route probe summary:

```json
{
  "installIntentRef": "app-intent-6561d0947ea2f42ae22d1b25",
  "providerExchangeRef": "provider-exchange-c2297066c71a7bba30801425ff3b1341",
  "installCompletionRef": "install-completion-325bca2aeb4b40e57d21cf35d8fe4d15",
  "appInstallationRef": "app-installation-a0d022ab82bd442b7730206a8af1bf78",
  "activationStatus": "active",
  "activationMode": "simulated",
  "strictActivationStatus": 400,
  "wrongOwnerActivationStatus": 404,
  "replayActivationStatus": 409,
  "progressStatus": "active",
  "appInstallationRecorded": true,
  "appInstallationStatus": "active",
  "nextAction": "none"
}
```

No forbidden response terms were present in the live create, handoff, descriptor, authorization preview, callback preflight, provider exchange, completion, strict activation rejection, wrong-owner activation, valid activation, installation list, activation replay, or progress responses: `secret`, `encrypted`, `refresh`, `access_token`, `refresh_token`, `credentialRef`, `providerPayload`, `rawProvider`, `userId`, `user_id`, `redirectUrl`, `callbackUrl`, `authorizationCode`, `providerCode`, `rawQuery`, `queryString`, or `providerResponse`.

## Next Slice Recommendation

The next useful slice is a provider-port-shaped OAuth handoff contract, still without secrets in public DTOs:

1. define the internal provider account/credential creation ports and failure modes;
2. keep public app-install responses on the current safe refs/status/timestamps surface;
3. add tests proving real provider token payloads can only enter dedicated credential storage/encryption seams, not app-install tables or responses.
