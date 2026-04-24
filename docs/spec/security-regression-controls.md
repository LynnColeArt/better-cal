# Security Regression Controls

These controls exist to make sure the replacement does not repeat known unsafe patterns. They are source-neutral and are implementation requirements, not suggestions.

## Core Invariants

| ID | Invariant | Required Proof |
| --- | --- | --- |
| SR-001 | Principal identity is server-derived from immutable subject ids. | Session mutation tests, token validation tests, and code review of principal resolver. |
| SR-002 | Authorization is deny-by-default and enforced by policy code, not metadata alone. | Route policy manifest, unauthorized fixtures, and policy coverage report. |
| SR-003 | Secrets are never returned after creation or rotation. | Response schema checks, fixture scanning, and explicit show-once tests. |
| SR-004 | Secrets are stored hashed or encrypted according to use. | Data model review, migration tests, and constant-time verification tests. |
| SR-005 | OAuth codes and refresh tokens are replay-safe. | Concurrent exchange tests, replay fixtures, expiry fixtures, and revocation fixtures. |
| SR-006 | Cron and job trigger credentials are not accepted in query strings. | Route tests for query credential rejection and signed-header success. |
| SR-007 | Logs, traces, metrics, errors, and fixtures redact secret-bearing fields. | Redaction tests with realistic payloads and fixture secret scanner. |
| SR-008 | Webhook verification and signing use the exact bytes and secrets intended by the public contract, with signing secrets resolved outside subscriber persistence, retry state scoped to pending attempts only, and exhausted attempts made visible without storing raw provider responses. | Raw-body inbound tests, outbound signature fixtures, pending-attempt retry tests, dead-letter tests, disabled-subscriber tests, and persistence checks for key-ref-only storage. |
| SR-009 | Booking writes are transactional, idempotent, and authorization-checked before side effects. | State transition tests, retry tests, and provider mock assertions. |
| SR-010 | API responses are allowlisted DTOs, not raw database or provider objects. | Response schema review and sensitive-field tests. |

## Route Policy Manifest

Every public route or tRPC procedure must have a manifest entry before implementation:

```json
{
  "operation": "POST /v2/bookings",
  "authModes": ["api-key", "platform-access-token", "session"],
  "principalTypes": ["user", "platform-client", "managed-user"],
  "requiredPermissions": ["booking:write"],
  "resourceResolver": "booking-target-event-type",
  "securityBreaks": [],
  "negativeFixtures": [
    "missing-credential",
    "wrong-organization",
    "insufficient-permission"
  ]
}
```

CI should fail when a route is implemented without a manifest entry and at least one negative fixture for each auth mode.

## Secret Classification

Every field with names such as `secret`, `token`, `password`, `key`, `credential`, `authorization`, `cookie`, `signature`, or `session` must be classified.

| Class | Meaning | Response Rule |
| --- | --- | --- |
| `public` | Safe public value, such as a public key id or non-secret enum. | May be returned. |
| `input-only` | Accepted from caller, never returned. | Must not be returned. |
| `show-once` | Returned only at creation or rotation. | Must not appear in list/get/update/delete. |
| `internal-only` | Used only by backend services. | Must not cross public API boundaries. |
| `derived` | Hash, fingerprint, or masked suffix. | May be returned only if explicitly allowed. |

Unclassified secret-like fields are treated as `internal-only`.

## Required Negative Test Matrix

Auth and identity:

- session update attempts to change user id, email, role, organization, profile, or team;
- bearer token for user A attempts to access user B resources;
- platform client from organization A attempts organization B resources;
- managed user token attempts owner-only operation;
- expired, malformed, revoked, and wrong-audience tokens;
- ambiguous mixed credentials.

Authorization:

- missing role;
- wrong team;
- wrong organization;
- system-admin-only route as regular user;
- read permission used for write operation;
- metadata-only route without enforced policy must fail review.

Secrets:

- OAuth client list/get/update/delete does not return secret;
- API key list/get does not return plaintext key;
- credential-bearing app response does not return provider tokens;
- webhook secret is not returned except where a show-once contract is approved;
- logs and error responses do not contain submitted secrets.

OAuth lifecycle:

- authorization code replay;
- authorization code used by wrong client;
- authorization code used with wrong redirect URI;
- expired authorization code;
- refresh token replay after rotation where rotation is required;
- revoked refresh token;
- confidential client without secret;
- public client without required verifier.

Cron and jobs:

- query-string credential rejected;
- missing signed header rejected;
- invalid signed header rejected;
- replayed signed request rejected when timestamp window is enforced;
- duplicate job execution is idempotent.

Booking:

- booking create with unavailable slot;
- booking create with insufficient permission;
- booking read/write by a permissioned wrong owner;
- booking confirm or decline by a permissioned non-host;
- booking create duplicate idempotency key;
- booking create duplicate idempotency conflict does not overwrite the first booking;
- booking write rolls back if planned side-effect persistence fails;
- booking side-effect dispatch failures remain retryable without storing raw provider error details;
- booking dispatch logs contain only side-effect ids, names, booking ids, request ids, and timestamps;
- booking queued webhook payload hints contain only contract fields needed for retry-safe delivery reconstruction;
- booking queued calendar payload hints and calendar dispatch envelopes contain only contract fields needed for retry-safe delivery reconstruction and do not store attendee ids, responses, metadata, provider credentials, or raw provider responses;
- booking webhook subscriptions select only active subscribers for the matching trigger event;
- booking webhook signing secrets never appear in subscription, delivery, or attempt tables and are resolved only through key refs at dispatch time;
- booking webhook retry skips subscriber attempts already marked delivered and stores only response codes plus generic failure text;
- booking calendar canary retry skips attempts already marked delivered and stores only response codes plus generic failure text;
- booking webhook attempts are dead-lettered after the configured threshold and disabled subscriptions are not reactivated by fixture seeding;
- booking cancel by unauthorized user;
- booking reschedule by unauthorized user;
- booking side-effect retry behavior is documented as at-least-once whenever network or persistence acknowledgements are ambiguous.

## Static And Fixture Scanners

The project should include scanners that fail CI when:

- accepted fixtures contain unredacted secrets;
- route manifests are missing for public routes;
- response schemas expose unclassified secret-like fields;
- logs are written with raw request bodies or raw auth headers;
- query parameters named `apiKey`, `secret`, `token`, `password`, or `authorization` are accepted as credentials;
- new migrations introduce plaintext secret columns without an approved migration note.

Scanner results should be treated as release blockers unless the security owner approves a documented exception.

## Review Gates

Before a domain can be implemented:

- route policy manifests exist;
- secret classification exists;
- negative fixtures are accepted;
- security breaks are listed;
- owner has signed off on unresolved ambiguity.

Before a domain can ship:

- all accepted security fixtures pass;
- redaction scanner passes;
- authorization coverage report shows every route has an enforced policy;
- secret scanner passes fixtures and logs from test runs;
- replay and concurrency tests pass for token and booking writes;
- rollback plan is tested.

## Abuse Cases To Keep In The Test Suite

- A user changes session-visible profile data and attempts to access another profile.
- A bearer token is paired with platform client headers for a different tenant.
- A platform client secret is submitted correctly and then searched for in every response, log, and fixture.
- A webhook receiver validates JSON-parsed body instead of raw body bytes.
- A cron URL with `?apiKey=` is called and rejected.
- Two concurrent OAuth code exchanges race against each other.
- Two concurrent booking creates target the same slot.
- Provider callback state is tampered with.
- Error paths include submitted tokens in exception messages.
- A response mapper receives a full database row and must prove only allowed fields leave the adapter.

## Security Owner Signoff

Security owner signoff is required for:

- adding or changing auth modes;
- exposing a secret-like field;
- accepting query-string credentials for any reason;
- weakening token validation;
- changing webhook signature behavior;
- changing booking idempotency behavior;
- skipping a negative fixture;
- shipping a known fixture mismatch as an approved security break.
