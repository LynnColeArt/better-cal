# Better Cal

Better Cal is a whiteroom-oriented replacement project for the Cal.diy backend and frontend contracts.

The current repository root contains source-neutral contracts, internal design notes, and fixture tooling. The original reference implementation is intentionally not part of this repository tree.

## Project Map

- `contracts/`: route, data, security, fixture, and schema contracts.
- `backend/`: starter Go API service.
- `docs/internal/`: implementation planning and backend contract notes.
- `docs/spec/`: whiteroom protocol, security baseline, compatibility plans, and fixture harness specs.
- `tools/`: contract validation, fixture capture, fixture review, and fixture replay utilities.

## Quick Checks

```bash
node tools/contracts/validate-contracts.mjs
node tools/fixture-capture/smoke-test.mjs
node tools/backend-smoke/smoke-test.mjs
```

The fixture smoke test captures synthetic API v2 auth and booking fixtures, reviews redaction and schemas, dry-runs approval, and replays the captured fixtures. The backend smoke test replays those same contracts against the starter Go API service.
