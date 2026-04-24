# Current State

Last updated: 2026-04-24, during the OAuth authorization-code token exchange slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `bfe105ae6f feat: refresh integration connection status`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next Phase 7 OAuth slice:

- intended commit message: `feat: add oauth token exchange canary`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

The previous provider status refresh slice added sanitized status updates for credential metadata and calendar connections.

This slice starts the real OAuth token boundary without introducing provider-token storage. It adds an authorization-code exchange canary for the fixture OAuth client:

- `POST /v2/auth/oauth2/token`
- authorization-code grant only;
- JSON and form-encoded requests;
- one-time code consumption;
- replay denial as `invalid_grant`;
- authorization codes, access tokens, and refresh tokens stored only as SHA-256 hashes.

The raw authorization code is seeded as fixture input only, and raw access/refresh token values are returned only in the successful token exchange response.

## Implemented Changes

Auth service:

- [service.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service.go)
- Adds `OAuthAuthorizationCode`, `OAuthTokenExchangeRequest`, and `OAuthTokenResponse`.
- Adds `FixtureOAuthAuthorizationCode = "mock-oauth-authorization-code"`.
- Adds `ExchangeOAuthToken` for `authorization_code` requests.
- Adds fixture in-memory exchange with replay denial.
- Adds an `OAuthClient.Principal()` view for `policy.oauth2.token.exchange`.

Postgres auth repository:

- [postgres_repository.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository.go)
- Adds `SaveOAuthAuthorizationCode`.
- Adds atomic `ExchangeOAuthAuthorizationCode` with `for update`.
- Marks the authorization code consumed and inserts access/refresh token hashes in the same transaction.
- Does not store raw authorization codes, access tokens, or refresh tokens.

Migration:

- [0022_oauth_authorization_code_exchange.sql](/home/lynn/projects/cal.diy-security-check/backend/internal/db/migrations/0022_oauth_authorization_code_exchange.sql)
- Adds `oauth_authorization_codes`.
- Adds `oauth_tokens`.

HTTP API:

- [server.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server.go)
- [handlers.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/handlers.go)
- Adds `POST /v2/auth/oauth2/token`.
- Enforces `authz.PolicyOAuth2TokenExchange` against the OAuth client principal before exchange.
- Returns OAuth-style token and error responses, not the normal API envelope.

API startup:

- [main.go](/home/lynn/projects/cal.diy-security-check/backend/cmd/api/main.go)
- Postgres startup seeds the fixture OAuth authorization code in hashed form.
- Auth service uses the Postgres OAuth token exchange repository when `CALDIY_DATABASE_URL` is set.

Policy and docs:

- [policy.go](/home/lynn/projects/cal.diy-security-check/backend/internal/authz/policy.go)
- Adds `PolicyOAuth2TokenExchange`.
- Updates backend README, scaffold docs, project plan, and security regression notes to describe the token exchange canary and hashed storage rule.

Tests:

- [service_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/service_test.go)
- [postgres_repository_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/auth/postgres_repository_test.go)
- [server_test.go](/home/lynn/projects/cal.diy-security-check/backend/internal/httpapi/server_test.go)
- Adds in-memory exchange/replay tests.
- Adds Postgres one-time exchange tests, including raw code/token non-storage checks.
- Adds HTTP token exchange shape and replay denial tests.

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
- Immediate replay of the same body returned `400 invalid_grant`.
- Postgres probe showed one consumed fixture authorization code and one token row for `mock-oauth-client`.

## Next Slice Recommendation

After this token exchange slice is committed, the next high-leverage Phase 7 slice is OAuth access-token authentication for one narrow protected path:

1. Add hashed access-token lookup from `oauth_tokens`.
2. Convert a stored token row back into a user principal with scoped permissions.
3. Accept `Authorization: Bearer <access_token>` on one existing low-risk route.
4. Add expiry and revoked-token denial tests.

Refresh-token rotation should come after that. The app catalog/app-store surface remains useful, but it can stay behind the OAuth/runtime-auth spine.
