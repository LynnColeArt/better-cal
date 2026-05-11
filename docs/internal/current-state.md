# Current State

Last updated: 2026-05-10 12:45 CDT, after wiring the provider-OAuth callback adapter and fixture credential store.

## Repository

- Working directory: `/home/lynn/projects/better-cal`
- Branch: `main`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`
- Intended commit message for this slice: `feat: wire provider oauth callback handoff`

The working tree contains the provider-OAuth callback handoff slice. It still has not introduced a public provider-code route.

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
- [main.go](/home/lynn/projects/better-cal/backend/cmd/api/main.go)
- Adds a fixture provider-OAuth exchange port for non-public callback/worker flows.
- Adds a credential-store adapter that writes only non-secret credential metadata through the existing credential repository.
- Adds a dedicated token secret sink so provider token payload handling stays behind an internal seam rather than in app-install tables or public DTOs.
- Wires the fixture provider exchange and fixture token secret sink into the API process when Postgres is enabled.

Postgres app-install mode support:

- [postgres_repository.go](/home/lynn/projects/better-cal/backend/internal/apps/postgres_repository.go)
- [0031_provider_oauth_app_install_modes.sql](/home/lynn/projects/better-cal/backend/internal/db/migrations/0031_provider_oauth_app_install_modes.sql)
- Allows provider exchange status `credential_ready`.
- Allows provider exchange, completion, and activation mode `provider_oauth` alongside `simulated`.
- Adds a repository read for callback preflight ownership/state validation before internal OAuth exchange work.
- Adds repository reads by opaque handoff state and callback preflight so replay can be rejected before provider/credential side effects.

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
```

## Next Slice Recommendation

The next useful slice is to make the callback path durable against real provider behavior:

1. run the provider-OAuth callback mode through a live Postgres smoke script once the local Compose DB is up;
2. replace the in-memory fixture token sink with an encrypted token secret repository;
3. add provider callback signature/state fixtures before exposing any compatibility HTTP callback route.
