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

1. Backend skeleton with health route, config, request ids, logging, redaction, panic recovery.
2. Fixture runner with strict, shape, state, side-effect, and security-break comparison modes.
3. Data contract registries for public schemas, enums, identifiers, stored JSON, and secret classification.
4. Database connection and transaction helpers.
5. Principal resolver and policy package.
6. API v2 auth routes and middleware.
7. One low-risk read route.
8. Next.js tRPC bridge proof of concept for one query and one mutation.
9. Slot read fixture replay.
10. Booking validation-only service.
11. Booking write canary path.

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
