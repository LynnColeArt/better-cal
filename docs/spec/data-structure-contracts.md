# Data Structure Contracts

The replacement may use better internal structures, but the externally visible data contract and the database-visible behavior must remain compatible during the drop-in phase.

## Compatibility Summary

| Layer | Compatibility Requirement | Replacement Freedom |
| --- | --- | --- |
| Public HTTP and tRPC payloads | Exact field names, casing, types, null/omit behavior, status envelopes, errors, and versioned differences except approved security breaks. | Internal DTO types can differ if adapters emit accepted wire shapes. |
| Webhook payloads | Exact trigger names, content type, payload shape, signature input, and template behavior. | Internal event structs can differ. |
| Database-visible state | Preserve ids, constraints, enum values, timestamps, idempotency, uniqueness, and state transitions while sharing production data. | Query layer and domain model can be new. |
| Stored JSON fields | Preserve accepted JSON shapes and passthrough behavior where callers depend on it. | Internally promote JSON to typed structs. |
| Provider-facing payloads | Preserve provider contract and idempotency semantics. | Provider adapters can be rewritten. |
| Internal Go domain model | No compatibility requirement outside adapters and tests. | Prefer clear aggregates over copying old object shapes. |

## Required Contract Artifacts

Before implementation starts for a domain, create or accept these artifacts:

| Artifact | Contents | Used By |
| --- | --- | --- |
| Public schema registry | Request and response schemas for routes, tRPC procedures, and webhooks. | API adapters, tests, docs. |
| Persistence contract | Tables, columns, types, nullability, defaults, indexes, unique constraints, foreign keys, cascade behavior, public ids. | Data access layer and migration plan. |
| Enum registry | Public and persisted enum string values, deprecation status, aliases. | API adapters, DB migrations, fixtures. |
| Identifier registry | Public id names, id type, entropy requirement, uniqueness scope, whether integer ids may be exposed. | API adapters and security review. |
| JSON shape registry | Stored JSON fields, allowed object shapes, unknown-field behavior, versioning rules. | Domain services and migration tooling. |
| Secret classification | Secret-like fields and whether they are public, input-only, show-once, internal-only, or derived. | Response mappers and scanners. |
| State transition catalog | Accepted lifecycle transitions and side effects for mutable aggregates. | Service tests and fixture replay. |
| Migration ledger | Additive changes, compatibility views, backfills, irreversible operations, rollback notes. | Cutover and operations. |

The initial machine-readable registries live in [Contract Registries](../../contracts/registries/README.md).

## Public Wire Shape Rules

Public request and response structures must preserve:

- field names and casing;
- numeric versus string representation;
- boolean defaults;
- `null` versus omitted fields;
- array ordering where callers observe order;
- timestamp format and timezone normalization;
- enum string values;
- route-version-specific field differences;
- response envelopes and pagination shapes;
- validation error shapes;
- cache and content-type headers where clients observe them.

Unknown-field behavior must be captured by fixtures. If legacy behavior accepts unknown fields, the replacement may reject them only as an approved compatibility break.

## Persistence Compatibility Rules

During drop-in replacement, assume the existing production data contract is reused.

Preserve:

- primary keys and foreign keys;
- public ids and slugs;
- unique constraints;
- soft-delete and cascade behavior;
- idempotency keys;
- enum string values;
- timestamp meaning;
- nullability;
- default values;
- row creation and update side effects;
- transactional boundaries for high-risk aggregates.

Do not expose database rows directly. Database structs, domain structs, and API response structs should be separate.

## Aggregate Contracts

| Aggregate | Compatibility Requirements |
| --- | --- |
| Identity and profile | Immutable user identity, profile and organization scope, membership roles, verified contact methods, API keys, account setup state. |
| Organization and team | Membership, owner/admin/member roles, platform organization flags, team ownership, onboarding and billing-adjacent state. |
| Event type | Slugs, host configuration, booking fields, locations, durations, limits, seats, private links, translations, managed children, public read shapes. |
| Schedule and availability | Weekly availability, date overrides, timezone, default schedule, travel and out-of-office effects, selected calendars. |
| Slot | Slot response shape, reservation state, timezone behavior, buffers, conflicts, booking limits, and capacity. |
| Booking | Public uid, lifecycle status, attendees, guests, seats, responses, metadata, references, payments, audit, scheduled triggers. |
| Credential and app | Public app metadata, private provider credential state, destination and selected calendars, default conferencing app. |
| OAuth and tokens | Client ids, redirect URIs, authorization codes, access tokens, refresh tokens, permissions, managed users, secret migration. |
| Webhook and jobs | Subscriber configuration, event triggers, secrets, payload templates, scheduled triggers, job idempotency, delivery attempts. |
| Audit and safety | Actor, source, action, target linkage, watchlist and abuse signals, immutable evidence fields. |

## Stored JSON Contracts

Stored JSON must not become untyped `map[string]any` in service code. Define typed structs at storage adapters and domain boundaries.

High-priority JSON shapes:

- event type locations;
- booking fields;
- booking responses;
- event type metadata;
- booking metadata;
- recurring event configuration;
- booking limits and duration limits;
- user and team metadata;
- credential provider payloads;
- webhook payload templates;
- webhook derived payload data;
- provider references;
- payment metadata.

For each JSON shape, capture:

- current accepted fields;
- optional versus required fields;
- nullability;
- unknown-field behavior;
- versioning strategy;
- secret fields;
- API exposure rules;
- migration/backfill strategy.

## Identifier Rules

Every public identifier needs a registry entry:

| Identifier Type | Required Properties |
| --- | --- |
| User-facing public ids | Stable, unique, non-guessable where used for public access. |
| Database integer ids | Internal unless an accepted public contract exposes them. |
| Slugs | Uniqueness scope and normalization rules are contract behavior. |
| OAuth client ids | Globally unique enough for public OAuth use and bound to allowed redirect URIs. |
| Authorization codes | Single-use, short-lived, client-bound, redirect-bound. |
| Booking uids | Public, stable, unique, and suitable for booking links. |
| Webhook ids | Scoped authorization checks required before read/write. |
| Provider ids | Stored as external references, not trusted as authorization by themselves. |

## Enum Rules

Enum values are wire and persistence contracts. Preserve exact strings unless a compatibility break is approved.

High-priority enum families:

- booking status;
- webhook trigger events;
- OAuth client type and status;
- membership role;
- payment status;
- credential and app type;
- scheduling and availability types;
- location type;
- workflow or job trigger type where public or persisted.

Unknown enum values in stored data should be handled safely and observably. Do not crash hot paths because old data contains a value the new service does not recognize.

## Time, Money, And Locale Rules

Time:

- Store and compare instants consistently.
- Preserve public timezone fields.
- Test daylight-saving transitions.
- Preserve minimum notice, buffer, travel, out-of-office, and recurrence behavior.

Money:

- Preserve currency codes.
- Preserve integer versus decimal unit behavior.
- Preserve payment status transitions.
- Avoid floating point for monetary values.

Locale:

- Preserve locale fields and fallback behavior.
- Preserve translated event type and notification-visible fields.

## Security Rules For Data Structures

- Response structs use allowlists.
- Secret-like fields are denied by default.
- Internal credential structs never double as response structs.
- Public DTOs do not embed persistence structs.
- Logs and fixtures use redacted versions of structs.
- State snapshots for fixtures redact secrets before storage.
- Secret migration preserves verification for old clients only as long as needed.

## Compatibility Tests

Required tests:

- public schema diff for each route/procedure/webhook;
- state snapshot diff for write paths;
- enum registry diff;
- identifier shape and uniqueness tests;
- null versus omitted fixtures;
- JSON round-trip tests for stored JSON fields;
- migration up/down or rollback tests where rollback exists;
- secret-field scanner against response schemas and fixtures;
- timezone and DST fixtures;
- idempotency and concurrency tests for booking and OAuth writes.

## Internal Model Guidance

Do not copy old structures just because they exist. Use separate types:

- `store` types for database rows and JSON storage;
- `domain` types for business rules;
- `api` types for public request and response payloads;
- `event` types for webhooks and jobs;
- `provider` types for third-party integration payloads.

Mapping code is a security boundary. It must be tested for sensitive-field exclusion and compatibility shape.

## Compatibility Status

The current documentation covers aggregate-level data contracts. It is not yet field-complete. Field-complete compatibility requires accepted schema registries and fixtures for each route, procedure, webhook, and high-risk stored JSON field.
