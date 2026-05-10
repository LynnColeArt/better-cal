# Fixture Harness

Fixtures are black-box evidence of compatibility. They let the replacement match external behavior without using reference source code as an implementation guide.

## Fixture Types

| Type | Captures | Required For |
| --- | --- | --- |
| HTTP route fixture | method, path, headers, body, status, response headers, response body | API v2 and Next.js app API routes |
| tRPC fixture | endpoint, procedure, input, batching mode, status, tRPC envelope, response body | Web UI compatibility |
| Auth fixture | credential type, success or failure, resolved principal, denial reason | All authenticated routes |
| Schema fixture | field names, types, nullability, enum values, version differences, null-versus-omitted behavior | Public payloads and persisted JSON |
| State fixture | selected database-visible state before and after a call | Mutations and jobs |
| Side-effect fixture | email, webhook, queue job, provider call, audit event | Booking, webhook, credential, payment, and cron domains |
| Provider fixture | outbound request shape and mocked provider response | Calendar, conferencing, payment, and identity providers |
| Webhook fixture | trigger, payload, content type, signature header, retry behavior | Webhook delivery |

## Fixture Manifest

Each fixture should have a manifest with this shape:

```json
{
  "id": "api-v2-bookings-create-basic",
  "surface": "api-v2",
  "operation": "POST /v2/bookings",
  "status": "accepted",
  "auth": "api-key",
  "comparison": "strict",
  "securityBreaks": [],
  "inputs": ["request.json"],
  "outputs": ["response.json", "state-after.json", "side-effects.json"],
  "schemas": ["request.schema.json", "response.schema.json", "state.schema.json"],
  "redactions": ["authorization", "set-cookie", "provider-token"],
  "unstableFields": ["requestId", "createdAt", "updatedAt"],
  "owner": "compatibility-qa"
}
```

Manifest sets live in [Contract Manifests](../../contracts/manifests/README.md), with schemas in [Contract Schemas](../../contracts/schemas/README.md).

The starter capture runner lives in [Fixture Capture Tool](../../tools/fixture-capture/README.md).
The starter replay comparator lives in [Fixture Replay Tool](../../tools/fixture-replay/README.md).

Manifest status values:

- `needs-capture`: fixture intent exists, but approved payloads do not;
- `draft`: captured but not approved;
- `accepted`: approved as implementation input;
- `amended`: superseded by a newer fixture;
- `security-break`: expected to differ for an approved security reason;
- `deprecated`: no longer part of the target contract.

## Redaction Rules

Fixtures must never contain:

- live secrets;
- plaintext passwords;
- access tokens or refresh tokens without replacement test values;
- provider credentials;
- private calendar event contents unless synthetic;
- customer personal data.

Use stable synthetic values wherever possible. When real values are unavoidable in capture, replace them with deterministic placeholders before approval.

## Normalization Rules

Normalize:

- generated ids where clients do not depend on the exact value;
- timestamps to fixed examples unless timestamp behavior is the subject of the fixture;
- request ids;
- provider request ids;
- queue job ids;
- signature timestamps when the signature fixture has a fixed signing input;
- order of unordered arrays.

Do not normalize fields that are part of the public contract, such as booking uid shape, enum values, or webhook trigger names.

## Comparison Modes

| Mode | Meaning |
| --- | --- |
| `strict` | Status, body, and selected headers must match exactly after normalization. |
| `shape` | Field presence and types must match; values may differ where listed. |
| `schema` | Field names, types, enum values, nullability, and version-specific differences must match. |
| `state` | Database-visible state transition must match the accepted expected state. |
| `side-effect` | Expected emails, webhooks, jobs, provider calls, or audit events must be emitted. |
| `security-break` | Replacement must differ in the approved safer way. |

## Required Phase 0 Fixture Set

Critical:

- API v2 auth success and failure for API key, platform OAuth client credentials, platform access token, third-party access token, and session-backed calls.
- Public schema fixtures for API v2 auth, booking, slot, event type, schedule, webhook, and OAuth payloads.
- Stored JSON schema fixtures for event type, booking, credential, webhook, metadata, and provider reference fields.
- API v2 token exchange and token refresh.
- Booking create, cancel, reschedule, confirm, decline.
- Booking create with team host, round robin, seats, recurring event, payment, and platform OAuth client context.
- Slot lookup across timezone, selected calendar, out-of-office, travel schedule, buffer, and booking limit cases.
- Webhook payload and signature for each public trigger.

High:

- Event type create, update, list, public read, private link, and event-type webhook.
- Schedule create, update, default read, delete.
- Calendar connect, freebusy, selected calendar, destination calendar.
- Conferencing connect, default, disconnect, booking meeting creation.
- Cron trigger idempotency.

Medium:

- Public utility routes, avatars, version, geolocation, username availability.
- Atoms embed routes.
- Admin and deployment setup reads.

## Fixture Approval Checklist

- Synthetic or redacted data only.
- Capture review passes with no secret or fixture redaction leaks.
- Auth mode documented.
- Security breaks listed.
- Schema files included for public payloads and high-risk stored JSON.
- Stable comparison mode selected.
- State and side effects included for writes.
- Unstable fields named.
- Owner assigned.

Use the review command after capture to infer schema snapshots and promote fixtures:

```bash
node tools/contracts/review-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --write-schemas

node tools/contracts/review-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --approve api-v2-auth.api-key.success
```

## Replay Requirements

The replacement test harness should:

- replay accepted fixtures against the Go service and Next.js bridge;
- run with a fixed clock and timezone;
- run with deterministic provider mocks;
- compare state snapshots after each write;
- emit a diff report that separates strict mismatches from approved security breaks;
- fail CI on unapproved drift.

The first replay comparator executes request templates against a target base URL and compares normalized HTTP responses against accepted fixture outputs. State snapshots, side-effect assertions, and provider-call assertions should layer on top of this rather than replace it.
