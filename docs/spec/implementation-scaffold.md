# Implementation Scaffold

This document describes the initial project shape for the Go backend and Next.js frontend work. It is source-neutral and intentionally does not mirror the reference implementation structure.

## Goals

- Build a Go backend that can serve public API compatibility routes.
- Keep Next.js as the frontend and initial tRPC compatibility bridge.
- Centralize domain behavior in Go services.
- Keep adapters thin and explicit.
- Make tests fixture-driven from the start.

## Suggested Repository Shape

```text
backend/
  cmd/
    api/
    worker/
    fixture-runner/
  internal/
    auth/
    authz/
    booking/
    calendar/
    config/
    database/
    datacontract/
    eventtype/
    fixture/
    httpapi/
    integrations/
    jobs/
    logging/
    oauth/
    schedule/
    slots/
    webhooks/
  migrations/
  openapi/
frontend/
  app/
  lib/
  server/
contracts/
  openapi/
  fixtures/
  manifests/
  registries/
  schemas/
  security/
  state/
docs/
  internal/
  spec/
```

This is a starting point, not a hard requirement. The important boundary is that public compatibility adapters remain separate from domain services.

## Go Service Boundaries

| Package | Responsibility |
| --- | --- |
| `auth` | Verify credentials and produce principals. |
| `authz` | Enforce policies for users, teams, organizations, platform clients, and bookings. |
| `httpapi` | Route compatibility, validation, response envelopes, error mapping. |
| `database` | Typed queries, transactions, migrations, state snapshot helpers. |
| `datacontract` | Schema registry, enum registry, identifier registry, and data compatibility checks. |
| `booking` | Booking aggregate, lifecycle transitions, side-effect orchestration. |
| `slots` | Slot computation and reservation behavior. |
| `schedule` | Availability and schedule behavior. |
| `eventtype` | Event type aggregate and public read behavior. |
| `integrations` | Provider ports and credential handling. |
| `calendar` | Calendar provider abstraction and freebusy behavior. |
| `oauth` | OAuth clients, authorization codes, access tokens, refresh tokens. |
| `webhooks` | Subscriber selection, payload generation, signing, delivery. |
| `jobs` | Worker queues, cron triggers, idempotency, retries. |
| `fixture` | Fixture replay, normalization, diff reporting. |
| `logging` | Structured logging, redaction, request context. |

## Next.js Boundary

The frontend should initially own:

- rendering;
- session cookie integration where needed;
- tRPC protocol compatibility;
- route handlers that bridge legacy UI calls to Go;
- feature flags for route cutover.

The frontend should not own long-term:

- booking write logic;
- provider credential logic;
- recurring jobs;
- webhook delivery;
- platform OAuth token writes.

## API Adapter Rules

Adapters should:

- parse public inputs;
- authenticate and authorize;
- call domain services;
- map domain results to accepted response envelopes;
- map domain errors to accepted HTTP or tRPC errors;
- map domain types to allowlisted public data structures;
- attach request ids and cache headers where required.

Adapters should not:

- contain business rules;
- leak database rows directly;
- reuse persistence structs as response structs;
- return sensitive fields;
- call providers without going through provider ports;
- skip fixture coverage.

## Initial Build Order

1. Backend skeleton with health route, config, request ids, structured request logging, panic recovery, and API adapter routing. Started in `backend/`.
2. Fixture runner with strict HTTP response replay and redaction-aware comparison. Started in `tools/fixture-replay/`.
3. Data contract registries for public schemas, enums, identifiers, stored JSON, and secret classification. Started in `contracts/registries/`.
4. API v2 auth route compatibility for the starter fixture set. Started in `backend/internal/httpapi/` with credential verification factored into `backend/internal/auth/`; the first OAuth token endpoint canary now exchanges a fixture authorization code once, rotates refresh tokens once, stores only hashed authorization codes and token values in Postgres, denies authorization-code and refresh-token replay, and lets a valid unexpired access token authenticate booking read, create/cancel/reschedule, and confirm/decline routes with permissions narrowed to token scopes.
5. Booking lifecycle canary route compatibility for the starter fixture set. Started in `backend/internal/httpapi/` with fixture state and transitions factored into `backend/internal/booking/`.
6. Database connection and transaction helpers. Started in `backend/internal/db/` with Postgres pool, ping, embedded migrations, and transaction tests against the Compose database.
7. Principal resolver and policy package. Principal fixture resolution has started in `backend/internal/auth/`; API-key principal lookup now has a hashed-token Postgres canary, OAuth client metadata lookup has a non-secret Postgres canary, OAuth access-token lookup now reads hashed token rows with expiry and revocation checks, OAuth refresh-token rotation revokes old hashed token rows before inserting the new hashed token pair, platform client verification has a hashed-secret Postgres canary, and named, deny-by-default policy enforcement has started in `backend/internal/authz/`. Booking read/write/host-action routes now have the first resource-scoped owner and host checks with wrong-owner fixtures.
8. One low-risk persisted read/write route. Started with the booking fixture canary and idempotency key repository in `backend/internal/booking/`; booking fields now also write explicit `bookings` and `booking_attendees` rows before falling back to JSON fixtures.
9. Next.js tRPC bridge proof of concept for one query and one mutation.
10. Slot read fixture replay. Started with `GET /v2/slots` fixture capture, a source-neutral `backend/internal/slots/` service, replay coverage for the personal event type canary, an internal availability check used by booking creation, a Postgres repository canary for `event_types` plus `availability_slots`, and an internal busy-time provider that filters accepted booking rows.
11. Booking validation service with durable state and side-effect ports. Fixture lifecycle behavior, explicit booking row persistence, and the first persistence canary have started in `backend/internal/booking/`; request validation now rejects unsupported fixture event types, malformed times, invalid attendee contact data, secret-bearing echoed maps, unavailable create slots, and unavailable reschedule slots through the shared slots-service adapter. Cancel, reschedule, confirm, and decline now plan fixture side effects through typed ports that receive minimal booking snapshots rather than raw response or metadata maps. The Postgres repository now commits booking rows, idempotency keys, and planned side-effect rows atomically, with tests for duplicate idempotency replay and rollback on failed side-effect persistence. Planned webhook side effects now persist typed payload hints so retries can rebuild webhook bodies without re-reading request-only inputs, email side effects now persist the payload hints needed for retry-safe email reconstruction, and calendar side effects now persist the payload hints needed for retry-safe reschedule canaries, including selected/destination calendar refs and opaque external event ids. A matching current-user calendar-management slice now lives in `backend/internal/calendars/`, with Postgres-backed selected-calendar rows, destination-calendar pointers, and API routes for reading, adding, deleting, and choosing the destination calendar without exposing provider secrets. The provider-backed catalog slice now syncs Postgres-backed calendar connection rows plus calendar catalog rows from the typed calendar provider adapter, exposes read routes for each, records connection status transitions transactionally, prunes stale provider catalog rows, refreshes selected-calendar snapshots, and validates selected-calendar writes against that synced catalog so calendar refs come from an explicit integration boundary instead of arbitrary user-supplied metadata. The first credential metadata canary in `backend/internal/credentials/` now persists and exposes only non-secret integration refs, app/provider metadata, account labels, status, and scopes through `GET /v2/credentials`, while `backend/internal/integrations/` provides a generic status refresh boundary that updates only sanitized credential and calendar connection status codes plus checked timestamps. The first app catalog canary in `backend/internal/apps` now persists and exposes only non-secret app/category/provider/auth/capability metadata through `GET /v2/apps` without credential refs, account refs, provider tokens, raw provider responses, or signing material. The first worker canary is packaged as an optional Compose service, claims planned side-effect rows with row locks, dispatches them through durable Postgres dispatch-log, booking-webhook-delivery, booking-email-delivery, and booking-calendar-dispatch adapters, selects active webhook subscribers by trigger, resolves signing secrets by key ref, snapshots signed webhook attempts, snapshots email attempts, snapshots calendar canary attempts, sends real HTTP webhook POSTs, routes email attempts through the first typed provider adapter in `backend/internal/email/`, sends real HTTP email provider POSTs, routes calendar canary attempts through the first typed provider adapter in `backend/internal/calendar/`, sends real HTTP calendar provider POSTs, retries only the pending subscriber rows, records generic email and calendar failure text plus response codes, dead-letters exhausted webhook attempts, disables failing subscriptions, and logs basic delivery metrics while the real provider integrations are still pending.

## Testing Layers

| Layer | Purpose |
| --- | --- |
| Unit tests | Package-level logic with synthetic inputs. |
| Contract tests | Route and procedure behavior against accepted fixtures. |
| Schema tests | Public wire shapes, stored JSON shapes, enum registries, identifier rules, null-versus-omitted behavior. |
| State tests | Database before/after comparisons. |
| Side-effect tests | Provider calls, emails, webhooks, jobs, and audit events. |
| Security tests | Approved security breaks, redaction, auth denial behavior. |
| Shadow tests | Compare replacement behavior against captured reference behavior. |

## CI Gates

Minimum gates before merge:

- Go tests pass.
- Contract validation passes, including route policy coverage, auth-mode coverage, and generated fixture secret scanning.
- Fixture replay for touched domains passes.
- Static analysis passes.
- Generated contracts are current.
- Data structure compatibility checks pass for touched schemas.
- Redaction tests pass for touched logging paths.
- No fixture contains unredacted secrets.

Minimum gates before production cutover:

- Full accepted fixture suite passes.
- Shadow diff threshold is met.
- Observability dashboards exist.
- Rollback switch is tested.
- Security owner signs off on approved breaks.

## Dependency Guidance

Use boring, maintainable dependencies:

- standard `net/http` or a small router;
- typed SQL generation or explicit query layer;
- PostgreSQL driver with transaction support;
- Redis client for locks, queues, and cache where needed;
- OpenAPI tooling for public REST contracts;
- structured logging with redaction support;
- OpenTelemetry for traces and metrics.

Avoid framework magic that hides authorization, serialization, or side effects.
