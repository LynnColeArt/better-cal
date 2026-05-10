# Backend Contracts

This document describes the compatibility surface that a Go backend must preserve to behave as a drop-in replacement.

## Compatibility Rule

For any existing caller, preserve:

- URL path and HTTP method;
- query/body/header names;
- content type and serialization, including tRPC batching and `superjson`;
- authentication inputs and identity semantics;
- response envelope, status code, error shape, and cache headers;
- database-visible side effects;
- emitted webhooks, scheduled jobs, emails, calendar writes, and payment side effects.

Unsafe behavior is the exception: for example, mutable identity from session updates, exposed platform OAuth secrets, query-string cron secrets, and no-op role decorators should be fixed intentionally and documented as security contract breaks.

## Contract Planes

### 1. Web UI tRPC Plane

The Next.js client calls `/api/trpc/{endpoint}/{procedure}` with optional `?batch=1`.

Source landmarks:

- root router: `../../../reference/packages/trpc/server/routers/_app.ts`
- endpoint list: `../../../reference/packages/trpc/react/shared.ts`
- client link and batching: `../../../reference/apps/web/app/_trpc/trpc-client.ts`
- tRPC server adapter: `../../../reference/packages/trpc/server/createNextApiHandler.ts`
- session middleware: `../../../reference/packages/trpc/server/middlewares/sessionMiddleware.ts`
- initial route map: [Route Inventory: Web UI tRPC Routes](route-inventory.md#web-ui-trpc-routes)

Current tRPC endpoints are:

`loggedInViewerRouter`, `admin`, `apiKeys`, `apps`, `auth`, `availability`, `appBasecamp3`, `bookings`, `calendars`, `calVideo`, `credentials`, `deploymentSetup`, `eventTypes`, `eventTypesHeavy`, `features`, `feedback`, `holidays`, `featureOptIn`, `i18n`, `me`, `ooo`, `payments`, `public`, `timezones`, `slots`, `travelSchedules`, `users`, `viewer`, `webhook`, `googleWorkspace`, `oAuth`, `delegationCredential`, `credits`, `filterSegments`, and `phoneNumber`.

Drop-in options:

- **Preferred migration bridge:** keep a minimal Next.js tRPC adapter that speaks the current tRPC protocol to the UI and calls Go internal APIs. This avoids implementing tRPC protocol details in Go immediately.
- **Full drop-in Go handler:** implement tRPC v10 HTTP protocol, batching, operation path routing, `superjson` serialization, tRPC error envelopes, and cache header behavior in Go.

The bridge is safer for early migration. The full Go handler is cleaner only after golden tests prove exact protocol behavior.

### 2. API v2 Plane

API v2 is a Nest service with versioned modules and class DTOs. Its public contract is REST-ish JSON under `/v2/*` and `/api/v2/*`.

Source landmarks:

- module list: `../../../reference/apps/api/v2/src/modules/endpoints.module.ts`
- bootstrap/middleware: `../../../reference/apps/api/v2/src/app.module.ts`
- platform DTOs: `../../../reference/packages/platform/types`
- controllers: `../../../reference/apps/api/v2/src/modules` and `../../../reference/apps/api/v2/src/platform`
- initial route map: [Route Inventory: API v2 Route Groups](route-inventory.md#api-v2-route-groups)

Major API v2 groups:

- bookings: `platform/bookings/2024-04-15`, `platform/bookings/2024-08-13`
- slots: `modules/slots/slots-2024-04-15`, `modules/slots/slots-2024-09-04`
- schedules: `platform/schedules/schedules_2024_04_15`, `platform/schedules/schedules_2024_06_11`
- event types: `platform/event-types/event-types_2024_04_15`, `platform/event-types/event-types_2024_06_14`
- calendars and unified calendars
- me, verified resources, selected calendars, destination calendars
- conferencing, Stripe, webhooks, event-type webhooks
- OAuth2, platform OAuth clients, managed users, API keys

API v2 compatibility requirements:

- preserve versioned DTO names and fields;
- preserve `{ status, data }` and pagination envelopes where used;
- preserve route versions and aliases;
- preserve OAuth and API-key headers: `Authorization`, `x-cal-client-id`, `x-cal-secret-key`;
- preserve raw-body handling for billing and deployment webhooks;
- preserve urlencoded handling for OAuth token routes;
- preserve request id, rate limit, and error response semantics where clients depend on them.

### 3. Next.js App API Plane

The web app also exposes product-specific API routes in `../../../reference/apps/web/app/api`.

Initial route map: [Route Inventory: Next.js App API Routes](route-inventory.md#nextjs-app-api-routes).

Important groups:

- auth and account setup: signup, password reset, two-factor setup/enable/disable, OAuth token/refresh/me;
- booking actions: cancel, link confirmation, verify booking token;
- public utilities: avatar, logo, csrf, geolocation, ip, username, version;
- media/video: Daily recording, guest session, video recording;
- cron: booking reminders, timezone change, calendar subscriptions, selected calendars, webhook triggers, app metadata sync;
- webhook receivers: calendar subscription provider callbacks, app credential webhook, Helpscout sync;
- tasker endpoints: cleanup and cron.

Drop-in strategy:

- keep these paths stable;
- migrate route handlers one group at a time to call Go;
- move cron/task execution to Go workers, but leave compatibility endpoints that enqueue/trigger the same jobs.

### 4. Database Contract Plane

PostgreSQL and Prisma models are an implicit contract because many behaviors are defined by DB state and relationships.

Source: `../../../reference/packages/prisma/schema.prisma`.

The Go backend should not copy Prisma as an implementation pattern, but it must respect:

- primary keys and public identifiers such as `uid`, `uuid`, `slug`, `clientId`, and token ids;
- unique constraints used for idempotency, especially booking `uid` and `idempotencyKey`;
- JSON sub-shapes validated in `zod-utils`, such as event type metadata, booking metadata, responses, locations, and booking fields;
- cascading delete semantics;
- enum string mappings, especially `BookingStatus`, `WebhookTriggerEvents`, `OAuthClientType`, and `OAuthClientStatus`.

### 5. Webhook and Event Contract Plane

Webhook subscribers are selected by platform, user, event type, managed parent event type, team/org, and platform OAuth client.

Source landmarks:

- trigger enum and webhook model: `../../../reference/packages/prisma/schema.prisma`
- subscriber selection: `../../../reference/packages/features/webhooks/lib/getWebhooks.ts`
- payload composition: `../../../reference/packages/features/webhooks/lib/sendPayload.ts`
- booking payload bridge: `../../../reference/packages/features/bookings/lib/getWebhookPayloadForBooking.ts`

Preserve:

- trigger names from `WebhookTriggerEvents`;
- default JSON body shape: `{ triggerEvent, createdAt, payload }`;
- payload template behavior and content type selection;
- Zapier special payload shape;
- HMAC signature behavior for `secret`;
- meeting start/end/no-show scheduled trigger behavior.

### 6. Job and Cron Plane

The current system uses Next app cron routes, tasker, Redis/Bull in API v2, and DB-backed scheduled trigger records.

Source landmarks:

- cron routes: `../../../reference/apps/web/app/api/cron`
- tasker: `../../../reference/packages/features/tasker`
- API v2 Bull setup: `../../../reference/apps/api/v2/src/app.module.ts`

The replacement backend should centralize jobs in Go workers. Compatibility endpoints should trigger or enqueue the same work and then return the legacy response shape.

## Auth Contracts

### Web Session

Current web session uses NextAuth with JWT strategy.

Source landmarks:

- NextAuth options: `../../../reference/packages/features/auth/lib/next-auth-options.ts`
- server-side session shaping: `../../../reference/packages/features/auth/lib/getServerSession.ts`
- tRPC user enrichment: `../../../reference/packages/features/auth/lib/userFromSessionUtils.ts`

Fields that callers expect include:

- `session.user.id`, `uuid`, `name`, `username`, `email`, `emailVerified`, `role`, `locale`;
- `session.user.org`, `orgAwareUsername`, `profile`;
- `session.profileId`, `session.upId`;
- `session.hasValidLicense`;
- impersonation metadata when present.

Security correction required:

- resolve identity by immutable user id from token `sub`;
- do not trust client-supplied session update fields for email, user id, role, org, `profileId`, or `upId` without server-side validation.

### API v2 Auth

Current API v2 supports:

- API keys in `Authorization: Bearer ...`;
- platform OAuth client credentials via `x-cal-client-id` and `x-cal-secret-key`;
- platform access tokens in `Authorization`;
- third-party access tokens;
- NextAuth session fallback.

Source: `../../../reference/apps/api/v2/src/modules/auth/strategies/api-auth/api-auth.strategy.ts`.

Compatibility requirements:

- preserve header names and auth method precedence for existing clients;
- preserve CORS/origin checks for platform access tokens;
- preserve permission checks for OAuth permissions bitsets;
- repair role enforcement where decorators currently do not enforce anything;
- move secret comparisons to constant-time checks where feasible.

## Error and Response Contracts

### tRPC

Current tRPC uses:

- `superjson` transformer;
- tRPC batching;
- `TRPCError` codes;
- custom error formatter;
- selected cache headers on public queries.

The Go backend must either reproduce this or let a Next.js bridge keep reproducing it.

### API v2

API v2 uses DTO response classes, Nest exceptions, filters, and response interceptors. Preserve externally visible:

- HTTP status;
- JSON response envelope;
- validation error shape;
- request id header behavior where clients observe it.

## Side-Effect Contracts

The high-risk side effects are:

- booking writes: DB rows, attendees, seats, payments, booking references, calendar events, video rooms, emails, webhooks, scheduled triggers;
- event type writes: availability, children/managed event types, hosts, selected calendars, translations, private links;
- credential writes: encryption, provider token refresh, selected/destination calendar linkage;
- OAuth writes: authorization code issuance, token exchange/refresh, managed user token creation;
- webhook writes: subscription uniqueness and scheduled backfill behavior;
- cron/job writes: repeated/idempotent processing.

Every replacement endpoint in these areas needs compatibility tests that assert state and emitted events, not just returned JSON.
