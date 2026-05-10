# Fixture Manifests

Fixture manifests describe captured behavior and point to redacted fixture payloads.

Manifest sets use `../schemas/fixture-set.schema.json`. Individual fixture objects use `../schemas/fixture-manifest.schema.json`.

Fixtures start as `draft` or `needs-capture`. They become implementation inputs only after review changes their status to `accepted` or `security-break`.

Use `tools/contracts/review-fixtures.mjs` after capture to infer schema snapshots, check redaction coverage, and promote captured fixtures. Approval records the canonical files for a fixture:

- `request.redacted.json`
- `response.json`
- `capture-metadata.json`
- `request.schema.json`
- `response.schema.json`
