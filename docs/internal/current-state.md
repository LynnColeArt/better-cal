# Current State

Last updated: 2026-05-10 23:30 CDT, after replacing the in-memory provider token sink with a sealed Postgres token store.

## Repository

- Working directory: `/home/lynn/projects/better-cal`
- Branch: `main`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`
- Intended commit message for this follow-up: `feat: persist sealed provider token secrets`

The remote `main` already contains `1ef94db test: harden provider oauth postgres smoke`. The current working tree contains the durable sealed-token-store follow-up. It still has not introduced a public provider-code route.

## Slice Purpose

This slice turns the previous simulated app-install ladder into a real-provider-ready internal callback path. Public app-install DTOs still carry only refs, status, mode, app/provider/catalog identifiers, capabilities, and timestamps. Provider codes, OAuth tokens, raw provider responses, credential refs, account refs, and account labels are accepted only by internal ports and are not stored in app-install tables or returned by app-install read models.

The new internal contract has two ports plus one internal entry point:

1. `ProviderOAuthExchangePort`, which receives the provider authorization code and returns provider account metadata plus token payloads.
2. `ProviderCredentialStore`, which is the dedicated credential storage/encryption seam for those token payloads and returns a non-public credential receipt.
3. `HandleProviderOAuthCallback`, a worker-style entry point that resolves the current app install from the opaque handoff state before calling the exchange and credential seams.

## Implemented Changes

Provider OAuth handoff contract:

- [store.go](/home/lynn/projects/better-cal/backend/internal/apps/store.go)
- [store_test.go](/home/lynn/projects/better-cal/backend/internal/apps/store_test.go)
- Adds `ExchangeExternalAuthProviderOAuth` as an internal app-install service method.
- Adds typed provider-OAuth exchange input/result structs and credential-secret/receipt structs.
- Adds explicit failure modes for missing ports, invalid provider exchange results, denied callbacks, invalid credential secrets, and invalid credential receipts.
- Records successful real-provider-ready exchanges as `credential_ready` with `provider_oauth` mode.
- Preserves no-leak behavior for app-install progress, completion, and installation models even when the internal port receives provider codes and token payloads.
- Adds `HandleProviderOAuthCallback`, which accepts only state, authorization code, and optional redirect URI, resolves the callback preflight internally, and rejects replay before calling the provider exchange port.

Fixture provider and credential adapters:

- [fixture_provider_oauth.go](/home/lynn/projects/better-cal/backend/internal/apps/fixture_provider_oauth.go)
- [provider_credential_store.go](/home/lynn/projects/better-cal/backend/internal/credentials/provider_credential_store.go)
- [provider_token_secret_store.go](/home/lynn/projects/better-cal/backend/internal/credentials/provider_token_secret_store.go)
- [main.go](/home/lynn/projects/better-cal/backend/cmd/api/main.go)
- Adds a fixture provider-OAuth exchange port for non-public callback/worker flows.
- Adds a credential-store adapter that writes only non-secret credential metadata through the existing credential repository.
- Adds a Postgres provider token secret store that seals provider token payloads with an AES-GCM envelope and records only key refs, ciphertext, and ciphertext fingerprints.
- Wires the fixture provider exchange and sealed Postgres token secret store into the API process when Postgres is enabled.

Postgres app-install mode support:

- [postgres_repository.go](/home/lynn/projects/better-cal/backend/internal/apps/postgres_repository.go)
- [0031_provider_oauth_app_install_modes.sql](/home/lynn/projects/better-cal/backend/internal/db/migrations/0031_provider_oauth_app_install_modes.sql)
- [0032_provider_oauth_constraint_names.sql](/home/lynn/projects/better-cal/backend/internal/db/migrations/0032_provider_oauth_constraint_names.sql)
- [0033_integration_provider_token_secrets.sql](/home/lynn/projects/better-cal/backend/internal/db/migrations/0033_integration_provider_token_secrets.sql)
- Allows provider exchange status `credential_ready`.
- Allows provider exchange, completion, and activation mode `provider_oauth` alongside `simulated`.
- Adds a repository read for callback preflight ownership/state validation before internal OAuth exchange work.
- Adds repository reads by opaque handoff state and callback preflight so replay can be rejected before provider/credential side effects.
- Adds a compatibility migration for the historical/truncated provider-exchange check-constraint name that live Compose Postgres still had after the first provider-OAuth migration.
- Narrows the older install-intent Postgres test so it asserts the fixture installation is present without assuming a long-lived Compose database has no other user-123 installed apps.
- Adds `integration_provider_token_secrets` as a private sealed-payload table keyed by user and credential ref, with no raw access-token, refresh-token, or provider-response columns.

## Verification

Checks passed:

```bash
cd backend && GOCACHE=/tmp/better-cal-go-build go test ./internal/apps
cd backend && GOCACHE=/tmp/better-cal-go-build go test ./internal/httpapi
cd backend && GOCACHE=/tmp/better-cal-go-build go test ./internal/db ./internal/credentials
cd backend && GOCACHE=/tmp/better-cal-go-build go test ./...
node tools/contracts/validate-contracts.mjs
node tools/contracts/check-policy-coverage.mjs --report
git diff --check
docker compose up --build -d postgres
docker compose exec -T postgres pg_isready -U better_cal -d better_cal
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" GOCACHE=/tmp/better-cal-go-build go test ./internal/apps -run TestPostgresRepositoryRoundTripProviderOAuthCallbackModes -count=1 -v
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" GOCACHE=/tmp/better-cal-go-build go test ./internal/apps -count=1 -v
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" GOCACHE=/tmp/better-cal-go-build go test ./internal/credentials -run 'TestPostgresProviderTokenSecret|TestPostgresProviderOAuthCallbackStoresSealedTokenSecret' -count=1 -v
cd backend && CALDIY_TEST_DATABASE_URL="postgres://better_cal:better_cal_dev@127.0.0.1:54320/better_cal?sslmode=disable" GOCACHE=/tmp/better-cal-go-build go test ./internal/db ./internal/apps ./internal/auth ./internal/authz ./internal/booking ./internal/calendar ./internal/calendars ./internal/credentials ./internal/email ./internal/httpapi ./internal/slots
```

## Next Slice Recommendation

The next useful slice is to make the callback path durable against real provider behavior:

1. add provider callback signature/state fixtures before exposing any compatibility HTTP callback route;
2. move the fixture token sealer behind explicit runtime key configuration or a real key-management boundary;
3. add a dedicated smoke command/script for the provider-OAuth callback path if this starts being run outside `go test`.
