# Contract Artifacts

This directory contains machine-readable artifacts for the whiteroom reimplementation.

The docs in `../docs/spec` define the rules. The files here are the artifacts that agents, tests, CI, and reviewers can consume.

## Layout

- `registries/`: route, policy, identifier, enum, secret, and JSON-shape registries.
- `manifests/`: fixture manifest sets for captured behavior.
- `fixtures/`: redacted request, response, state, side-effect, and provider fixture payloads.
- `schemas/`: JSON Schemas for contract files and public payloads.
- `security/`: security-specific generated reports and review inputs.
- `state/`: state snapshot definitions and redaction profiles.
- `openapi/`: OpenAPI contracts for public REST surfaces.

## Status Values

- `draft`: useful starting point, not accepted for implementation.
- `accepted`: approved implementation input.
- `needs-capture`: known contract area, fixture or schema capture still required.
- `security-break`: intentionally differs from legacy behavior for safety.
- `deprecated`: kept only for historical or migration tracking.

Implementation should depend only on `accepted` artifacts. Draft artifacts are scaffolding for contract capture.
