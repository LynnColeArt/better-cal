# Fixture Capture Tool

This tool captures source-neutral request/response fixtures from a running reference or replacement server.

It does not inspect source code. It reads:

- a fixture manifest set from `contracts/manifests`;
- a request template from `contracts/fixtures/<set-id>/<fixture-id>/request.template.json`;
- redaction rules from `contracts/registries/secrets.json`.

## Dry Run

```bash
node tools/fixture-capture/capture-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --base-url http://localhost:5555 \
  --dry-run
```

Dry run reports which fixture templates exist and which environment variables are required.

## Capture One Fixture

```bash
CALDIY_API_KEY=cal_test_example \
node tools/fixture-capture/capture-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --fixture api-v2-auth.api-key.success \
  --base-url http://localhost:5555
```

Captured files are written next to the template:

- `request.redacted.json`
- `response.json`
- `capture-metadata.json`

## Review And Approval

After capture, review the redacted payloads and write schema snapshots:

```bash
node tools/contracts/review-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --write-schemas
```

The reviewer checks that secret fields and fixture redaction paths are redacted, validates capture metadata, and infers:

- `request.schema.json`
- `response.schema.json`

Promote one reviewed fixture into the manifest:

```bash
node tools/contracts/review-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --approve api-v2-auth.api-key.success
```

Promotion fills the fixture `inputs`, `outputs`, and `schemas` arrays and moves normal fixtures to `accepted`. Security-break fixtures stay `security-break` unless `--status` is provided.

## Template Placeholders

Use `${ENV_NAME}` in request templates. The capture tool substitutes environment variables at runtime and refuses to capture if required variables are missing.

Manual fixtures use `"captureMode": "manual"` and are skipped by the HTTP capture runner.

## Smoke Test

Run the mock API v2 server and capture fixtures into a temporary directory:

```bash
node tools/fixture-capture/smoke-test.mjs
```

The smoke test proves redaction, expected-status enforcement, manual fixture skipping, and temp-output capture without using real credentials.

It currently captures the starter API v2 auth fixtures and booking lifecycle fixtures against the synthetic mock server, writes schema snapshots, dry-runs fixture approval, and replays the captured fixtures against the mock server.
