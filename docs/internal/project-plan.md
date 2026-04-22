# Project Plan: Go Backend Drop-In Replacement

This plan assumes the feature inventory in [Feature Inventory](feature-inventory.md), the route map in [Route Inventory](route-inventory.md), the compatibility rules in [Backend Contracts](backend-contracts.md), the persistence notes in [Data Model Contracts](data-model-contracts.md), the source-neutral data rules in [Data Structure Contracts](../spec/data-structure-contracts.md), the migration approach in [Drop-In Compatibility Strategy](drop-in-compatibility-strategy.md), the source-informed issues in [Security Audit Ledger](security-audit-ledger.md), and the source-neutral implementation process in [Whiteroom Protocol](../spec/whiteroom-protocol.md).

## Phase 0: Contract Freeze and Golden Tests

Goal: define what "drop-in" means before writing replacement behavior.

Deliverables:

- frozen route specs for `/api/trpc/*`, `/v2/*`, `/api/v2/*`, and `apps/web/app/api/*`;
- accepted source-neutral specs for implementation engineers;
- public schema, enum, identifier, stored JSON, and persistence contract artifacts;
- golden fixtures for core request/response shapes;
- DB side-effect assertions for booking, event type, OAuth, credential, webhook, and schedule writes;
- webhook payload/signature fixtures for every public trigger;
- auth matrix covering session, API key, platform OAuth client headers, access tokens, and invalid credentials;
- security regression controls for identity, authorization, secrets, token replay, cron auth, logging, webhooks, and booking writes;
- list of intentional security contract breaks.

References:

- [Backend Contracts](backend-contracts.md)
- [Route Inventory](route-inventory.md)
- [Feature Inventory](feature-inventory.md)
- [Whiteroom Protocol](../spec/whiteroom-protocol.md)
- [Data Structure Contracts](../spec/data-structure-contracts.md)
- [Fixture Harness](../spec/fixture-harness.md)
- [Security Audit Ledger](security-audit-ledger.md)
- [Security Regression Controls](../spec/security-regression-controls.md)

Exit criteria:

- every phase-1 endpoint has at least one golden read test;
- every write endpoint has an expected state transition;
- implementation engineers have accepted source-neutral specs for the endpoint;
- data structure contracts exist for public payloads and database-visible writes;
- every high-risk route has negative security fixtures;
- security breaks are approved and documented.

## Phase 1: Compatibility Gateway Skeleton

Goal: stand up the Go service and route traffic to it without changing product behavior.

Deliverables:

- Go service skeleton with request id, logging, panic recovery, validation, auth middleware, and health endpoints;
- PostgreSQL and Redis clients;
- generated data-access layer for key tables;
- data contract registry for public schemas, enums, identifiers, stored JSON fields, and secret classifications;
- OpenAPI skeleton for API v2 compatibility routes;
- Next.js tRPC bridge proof of concept for one query and one mutation;
- feature flag or routing switch to choose legacy versus Go per endpoint.

References:

- [Drop-In Compatibility Strategy](drop-in-compatibility-strategy.md)
- [Backend Contracts](backend-contracts.md)

Exit criteria:

- one read endpoint and one write endpoint can run through Go in shadow mode;
- legacy and Go responses are diffed automatically.

## Phase 2: Identity, Session, and Authorization Core

Goal: make identity safe and shared before porting business behavior.

Deliverables:

- session verifier for NextAuth JWT or a Next.js identity bridge;
- user/profile/org/team membership resolver;
- explicit policy package for system admin, org owner/admin/member, team roles, platform OAuth permissions, PBAC-style checks; the first booking read/write/host-action canary now requires both named permissions and owner or host resource scope;
- route policy manifests and secret classification for every auth-protected route;
- API key verifier;
- platform OAuth client/access-token verifier;
- fixes for known unsafe contracts: immutable identity, real membership role enforcement, no secret-bearing DTOs.

References:

- [Backend Contracts: Auth Contracts](backend-contracts.md#auth-contracts)
- [Data Model Contracts: Identity, Organization, and Authorization](data-model-contracts.md#identity-organization-and-authorization)

Exit criteria:

- auth matrix passes in Go and matches legacy for valid behavior;
- intentional security breaks have regression tests;
- route policy coverage and secret scanners pass for implemented routes;
- no service endpoint relies on email as primary identity; the implemented booking routes now gate fixture resources by immutable principal id, not email.

## Phase 3: Read-Heavy Domains

Goal: move low-risk reads first while preserving UI payloads.

Candidate domains:

- `me.get`, `platformMe`, user profile reads;
- public event type reads;
- event type list/read;
- schedules read/default schedule;
- connected calendars read without credential exposure;
- timezones/i18n/feature map reads;
- API v2 provider/me/timezone reads.

References:

- [Feature Inventory](feature-inventory.md)
- [Data Model Contracts](data-model-contracts.md)

Exit criteria:

- reads pass golden response tests;
- Next.js screens load unchanged through the bridge;
- no sensitive fields appear in responses.

## Phase 4: Availability and Slot Engine

Goal: port the scheduling core before booking writes.

Deliverables:

- schedule and availability services;
- slot lookup by event type, time range, and timezone; the first `GET /v2/slots` canary is now captured and served by `backend/internal/slots/`, booking creation calls the same service through a validation-only availability adapter, and the Compose path now reads the fixture slot from `event_types` and `availability_slots`;
- busy-time provider ports for internal bookings and external calendars; the first internal-booking provider filters accepted `bookings` rows from slot reads and booking availability checks;
- selected-calendar and destination-calendar handling;
- timezone and DST test suite;
- OOO, travel schedule, holiday, buffers, booking limits, duration limits, and seated-event support;
- slot reservation compatibility.

References:

- [Feature Inventory: Availability, schedules, slots](feature-inventory.md#feature-domains)
- [Data Model Contracts: Scheduling and Event Types](data-model-contracts.md#scheduling-and-event-types)

Exit criteria:

- slot golden tests match legacy for representative scenarios;
- performance is measured against existing heavy slot paths;
- booking creation and reschedule can call the Go slot engine in validation-only mode. The first slot-service adapter rejects unavailable create and reschedule requests before persistence, can read fixture slots from Postgres, and filters accepted internal bookings.

## Phase 5: Booking Write Path

Goal: port create/cancel/reschedule/confirm/decline with all side effects.

Deliverables:

- booking aggregate service with transactional writes; booking rows, idempotency keys, and planned side-effect records now commit together in the Postgres canary, and idempotency conflicts replay the first accepted booking;
- attendee, guest, seat, recurring, no-show, reassignment, report, and internal-note behavior;
- calendar event create/update/delete provider ports; cancel, reschedule, confirm, and decline now start with typed fixture side-effect planners;
- conferencing creation and cleanup ports;
- email enqueueing; cancel, reschedule, confirm, and decline now expose planned fixture email effects through the same port boundary;
- webhook emission; cancel, reschedule, confirm, and decline now expose planned fixture webhook effects through the same port boundary, and the current Postgres canary persists typed webhook payload hints with the queued side effect;
- side-effect worker boundary; the current canary is packaged as an optional Compose worker, claims planned or retryable rows with row locks, writes a durable dispatch-log row through the dispatcher port, records typed booking webhook envelopes in `booking_webhook_deliveries`, marks delivered rows, and stores only a generic retry failure marker;
- payment state integration;
- idempotency and retry semantics; the current Postgres repository rejects conflicting idempotency writes without overwriting the original booking.

References:

- [Backend Contracts: Side-Effect Contracts](backend-contracts.md#side-effect-contracts)
- [Data Model Contracts: Bookings](data-model-contracts.md#bookings)

Exit criteria:

- golden booking state tests pass;
- provider calls are mocked and asserted;
- duplicate booking and retry tests pass; the current Postgres canary includes idempotency conflict replay, rollback coverage for failed planned side-effect writes, queued webhook payload coverage, and worker canary coverage for claiming, webhook-envelope recording, dispatch-log recording, delivering, and retry-marking planned side effects;
- existing booking UI flows pass through the Next.js bridge.

## Phase 6: Integrations, Credentials, and App Store

Goal: port integration management while tightening secret boundaries.

Deliverables:

- credential encryption/decryption service;
- provider-specific credential structs;
- app metadata reader or generated app catalog;
- calendar, conferencing, CRM, analytics, and payment provider ports;
- selected/destination calendar mutation flows;
- default conferencing app behavior;
- explicit no-leak tests for credential fields.

References:

- [Feature Inventory: Apps and credentials](feature-inventory.md#feature-domains)
- [Data Model Contracts: Credentials, Apps, and Integrations](data-model-contracts.md#credentials-apps-and-integrations)

Exit criteria:

- integration settings screens work unchanged;
- credential secrets are never returned;
- provider callback flows have signature/state tests.

## Phase 7: Platform API and OAuth

Goal: expose the external API as a compatible Go service.

Deliverables:

- API v2 routes with matching versioned DTOs;
- OAuth2 authorization, token exchange, refresh, and provider endpoints;
- platform OAuth clients, managed users, tokens, permissions, and webhooks;
- API key refresh and validation;
- atomic authorization code consumption;
- secret rotation and hashed platform OAuth client secrets.

References:

- [Backend Contracts: API v2 Plane](backend-contracts.md#2-api-v2-plane)
- [Data Model Contracts: OAuth and Platform API](data-model-contracts.md#oauth-and-platform-api)

Exit criteria:

- API v2 golden tests pass;
- external clients can use old routes without code changes except intentional secret behavior changes;
- security audit findings in this area are closed.

## Phase 8: Jobs, Webhooks, and Operations

Goal: move background work out of Next.js and into Go workers.

Deliverables:

- Go worker process;
- cron compatibility endpoints;
- webhook delivery queue with retries and observability;
- booking reminder, timezone-change, selected-calendar, subscription cleanup, no-show, audit, analytics, and translation jobs;
- idempotent job locks and retry policies.

References:

- [Backend Contracts: Webhook and Event Contract Plane](backend-contracts.md#5-webhook-and-event-contract-plane)
- [Backend Contracts: Job and Cron Plane](backend-contracts.md#6-job-and-cron-plane)

Exit criteria:

- cron routes trigger Go jobs;
- duplicate job execution is safe;
- webhook payload golden fixtures pass;
- job metrics and dead-letter visibility exist.

## Phase 9: Cutover, Shadowing, and Decommission

Goal: switch traffic safely and remove compatibility scaffolding only when it is no longer needed.

Deliverables:

- endpoint-by-endpoint shadow-read comparisons;
- write canaries for low-risk domains;
- rollback switches;
- operational dashboards for latency, errors, queue depth, provider failures, webhook failures, and auth denials;
- deprecation map for old Next.js backend code;
- post-cutover security review.

Exit criteria:

- all target endpoints run through Go in production;
- Next.js backend handlers are either deleted or reduced to compatibility proxies;
- no unowned cron/task routes remain;
- contract docs are updated to describe the new source of truth.
