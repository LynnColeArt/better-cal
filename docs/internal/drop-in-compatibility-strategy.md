# Drop-In Compatibility Strategy

The replacement backend should be built as a compatibility service first. Better internal architecture is valuable only if existing clients and the Next.js UI can continue to work.

## Target Shape

```text
Browser / external clients
  |
  | existing routes, cookies, headers, payloads
  v
Next.js frontend and compatibility routes
  |
  | internal typed calls, ideally generated clients
  v
Go backend service
  |
  | PostgreSQL, Redis, queues, provider APIs
  v
Data and side effects
```

The compatibility layer can shrink over time, but it lets the Go service be introduced without rewriting the UI and every external integration in the same move.

## Why Not Start With a Pure Go tRPC Server

The current web UI contract is not plain JSON REST. It is tRPC over HTTP with:

- endpoint path rewriting based on `ENDPOINTS`;
- operation paths like `me.get` and `eventTypesHeavy.update`;
- batching via `?batch=1`;
- `superjson` serialization;
- tRPC error envelopes;
- cache headers on selected public queries.

A pure Go drop-in can implement this, but it is a compatibility project by itself. The safer first implementation is:

1. keep Next.js tRPC handlers as protocol adapters;
2. replace handler bodies with calls to Go;
3. once behavior is proven, decide whether to keep the adapter or replace it with a Go tRPC-compatible gateway.

## Compatibility Layers

### Layer A: Public/API v2 Compatibility

Expose `/v2/*` and `/api/v2/*` from Go, matching the Nest API contracts.

Use:

- OpenAPI generation for request/response structs;
- middleware for auth, rate limiting, request id, panic recovery, validation, and error shape;
- route versioning that mirrors existing `2024-*` controller versions.

### Layer B: Web tRPC Compatibility

Initial path:

- Next.js keeps `/api/trpc/*`;
- tRPC procedures call Go internal endpoints;
- tRPC still owns `superjson`, batching, and error envelopes.

Later path:

- Go exposes a tRPC-compatible gateway, or the UI migrates from tRPC to generated clients endpoint by endpoint.

### Layer C: App API Compatibility

Move product-specific routes from `apps/web/app/api` gradually:

- auth/session routes may remain in Next.js if NextAuth remains frontend-owned;
- business routes call Go;
- cron routes become thin triggers for Go jobs;
- webhook receiver routes can move directly to Go when raw-body signature handling is covered by tests.

### Layer D: Internal Domain APIs

Inside Go, use explicit domain services instead of mirroring tRPC/Nest controllers:

- identity service;
- organization service;
- event type service;
- availability/slot service;
- booking service;
- integration credential service;
- calendar/conferencing provider ports;
- webhook service;
- job service;
- platform OAuth service.

Internal APIs can be Connect/gRPC, REST, or direct package boundaries. The external compatibility adapters should be thin.

## Acceptance Criteria for Drop-In

An endpoint/domain is compatible when:

- golden request fixtures produce matching status, headers that matter, and response body;
- auth accepts the same legitimate credentials and rejects the same invalid credentials;
- security-fixed behavior is documented and tested;
- database state after the call matches expected legacy state;
- emitted emails, webhooks, jobs, and provider calls match expected legacy side effects;
- the existing Next.js screen or API client works without changes.

## Security Contract Breaks to Make Intentionally

These should not be preserved:

- trusting client-supplied session update fields for identity;
- returning platform OAuth client secrets on list/get/update/delete;
- treating metadata-only decorators as authorization;
- accepting cron secrets in query strings;
- logging secrets after stringification;
- non-atomic authorization code consumption.

Each break needs a compatibility note and migration path, especially if external API clients relied on the old behavior.

## Suggested Go Stack

Prefer boring, explicit parts:

- `net/http` plus `chi` or standard mux for routing;
- `pgx` plus `sqlc` for typed PostgreSQL queries;
- `goose` or `tern` for migrations;
- Redis client for rate limiting, queues, and cache;
- OpenAPI generation for public REST contracts;
- background worker process in the same repo with shared domain packages;
- structured logging with field redaction by default;
- OpenTelemetry traces and metrics.

Avoid re-creating implicit framework magic. Authorization should be explicit middleware plus service-level policy checks.

