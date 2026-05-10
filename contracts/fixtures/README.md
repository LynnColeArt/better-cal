# Fixtures

Fixture payloads will live here after capture and redaction.

Use one directory per fixture set, with stable file names such as:

- `request.json`
- `response.json`
- `state-before.json`
- `state-after.json`
- `side-effects.json`
- `provider-calls.json`
- `logs.json`

Fixtures must not contain live secrets, customer data, provider tokens, plaintext passwords, or unredacted session material.

Use `../../tools/fixture-capture/capture-fixtures.mjs` to capture HTTP fixtures from request templates.
