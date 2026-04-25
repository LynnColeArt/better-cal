# Current State

Last updated: 2026-04-25, during the route auth-mode coverage slice.

## Repository

- Working directory: `/home/lynn/projects/cal.diy-security-check`
- Branch: `main`
- Last pushed commit before this slice: `f6fbb28c54 feat: authorize booking host actions with oauth access tokens`
- Target remote: `origin https://github.com/LynnColeArt/better-cal.git`

The working tree now contains the next security tooling slice:

- intended commit message: `chore: add route auth mode coverage report`
- this session has full local Git and network permissions, so normal commit and push should work.

## Slice Purpose

The previous OAuth slices made the starter booking lifecycle OAuth route matrix coherent:

1. booking read uses `booking:read`;
2. create/cancel/reschedule use `booking:write`;
3. confirm/decline use `booking:host-action`.

This slice adds a compact guard so the route policy registry cannot drift away from the implemented handler auth shape.

The validator now checks two directions for currently implemented auth modes:

- handlers cannot implement `api-key`, `oauth-access-token`, `platform-client-secret`, or `oauth-client` unless the route policy lists that mode;
- policies cannot list one of those currently implemented modes for an implemented route unless the handler actually uses it.

Future registry modes such as `session`, `platform-access-token`, `public-booking-token`, and `public-pkce-client` may still remain listed before their runtime support lands.

## Implemented Changes

Contract tooling:

- [check-policy-coverage.mjs](/home/lynn/projects/cal.diy-security-check/tools/contracts/check-policy-coverage.mjs)
- Infers implemented auth modes from handler calls such as `authenticateAPIKey`, `authenticateAPIKeyOrOAuthAccessToken`, `VerifyPlatformClientContext`, and OAuth token client exchange handling.
- Keeps the existing route registry and policy constant coverage checks.
- Adds `--report` output for a compact Markdown route/auth-mode matrix.

Security artifacts:

- [route-auth-mode-coverage.md](/home/lynn/projects/cal.diy-security-check/contracts/security/route-auth-mode-coverage.md)
- Captures the current implemented-route auth-mode matrix.
- Shows future registry modes separately from currently implemented handler modes.

Docs:

- [README.md](/home/lynn/projects/cal.diy-security-check/README.md)
- [contracts/security/README.md](/home/lynn/projects/cal.diy-security-check/contracts/security/README.md)
- [project-plan.md](/home/lynn/projects/cal.diy-security-check/docs/internal/project-plan.md)
- [implementation-scaffold.md](/home/lynn/projects/cal.diy-security-check/docs/spec/implementation-scaffold.md)
- [security-regression-controls.md](/home/lynn/projects/cal.diy-security-check/docs/spec/security-regression-controls.md)
- Updates the quick checks and security gates to include auth-mode coverage, not only policy-name coverage.

## Verification

Checks passed:

```bash
node tools/contracts/check-policy-coverage.mjs
node tools/contracts/check-policy-coverage.mjs --report
node tools/contracts/validate-contracts.mjs
git diff --check
cd backend && GOCACHE=/tmp/caldiy-go-build go test ./...
docker compose --profile tools run --rm contracts
```

Report consistency check:

- The table in `contracts/security/route-auth-mode-coverage.md` matched `node tools/contracts/check-policy-coverage.mjs --report`.

## Next Slice Recommendation

After this route auth-mode coverage slice is committed, the safest technical continuation is complete enough for now. The next product-visible slice should switch back to the app catalog/app-store metadata path:

1. define the first non-secret app catalog DTO and registry entries;
2. add a fixture provider/store canary for installed or available apps;
3. expose the smallest read route without credential payloads or provider tokens.
