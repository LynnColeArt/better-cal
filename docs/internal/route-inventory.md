# Route Inventory

This is the first compatibility map for a drop-in backend replacement. It identifies the routes and protocol families that need golden tests before behavior is ported. It does not replace field-level DTO specs; those should be generated or written per route group as the replacement service is built.

Source landmarks:

- tRPC endpoint list: `../../../reference/packages/trpc/react/shared.ts`
- tRPC root router: `../../../reference/packages/trpc/server/routers/_app.ts`
- API v2 module list: `../../../reference/apps/api/v2/src/modules/endpoints.module.ts`
- API v2 controllers: `../../../reference/apps/api/v2/src/modules` and `../../../reference/apps/api/v2/src/platform`
- Next.js app API routes: `../../../reference/apps/web/app/api`

## Compatibility Priority

The backend replacement should treat these route families as externally observable contracts:

1. API v2 and OAuth routes used by platform clients.
2. tRPC procedures used by the web UI.
3. Next.js app API routes used by public pages, cron, callbacks, and existing frontend code.
4. Health and deployment webhook routes used by infrastructure.

For each route group, preserve method, path, auth inputs, status codes, response envelope, validation errors, headers that clients observe, and side effects. Security defects should be handled as explicit contract breaks in [Backend Contracts](backend-contracts.md).

## Web UI tRPC Routes

Base protocol:

- path: `/api/trpc/{endpoint}/{procedure}`
- batching: `?batch=1`
- serialization: `superjson`
- router source: `../../../reference/packages/trpc/server/routers/_app.ts`
- endpoint source: `../../../reference/packages/trpc/react/shared.ts`

Current endpoint names:

| Endpoint | Compatibility Notes |
| --- | --- |
| `loggedInViewerRouter` | Session-bound viewer data and account bootstrap behavior. |
| `admin` | Administrative reads and mutations; policy checks must move into explicit authorization code. |
| `apiKeys` | API key creation, listing, refresh, and revocation contracts. |
| `apps` | App store metadata, setup flows, credential-linked app state. |
| `auth` | Account, session, token, and login-adjacent behaviors used by the web UI. |
| `availability` | Availability reads and mutations feeding schedule and booking flows. |
| `appBasecamp3` | App-specific integration surface. |
| `bookings` | Booking list, read, mutation, and action workflows. |
| `calendars` | Calendar connection, selected calendar, busy-time, and destination calendar flows. |
| `calVideo` | Cal video behavior and meeting metadata. |
| `credentials` | Credential management; replacement must never leak encrypted secret payloads. |
| `deploymentSetup` | Self-hosted or deployment bootstrap state. |
| `eventTypes` | Main event type CRUD and configuration. |
| `eventTypesHeavy` | Expensive event type reads that need performance parity tests. |
| `features` | Feature flag and entitlement reads. |
| `feedback` | Product feedback submission. |
| `holidays` | Holiday calendar reads used by availability. |
| `featureOptIn` | Feature opt-in state. |
| `i18n` | Translation and locale support. |
| `me` | Current user/profile/org state used across the app. |
| `ooo` | Out-of-office behavior that affects availability and booking routing. |
| `payments` | Payment settings, app configuration, and payment state reads. |
| `public` | Public page and unauthenticated booking surface. |
| `timezones` | Timezone lookup and normalization. |
| `slots` | Slot computation and reservation surface. |
| `travelSchedules` | Travel schedule behavior that affects availability. |
| `users` | User reads and mutations. |
| `viewer` | Viewer data and account-level workflows. |
| `webhook` | Webhook configuration in the web UI. |
| `googleWorkspace` | Google Workspace integration workflows. |
| `oAuth` | OAuth app and token management from the web UI. |
| `delegationCredential` | Delegated credential behavior. |
| `credits` | Credits or billing-adjacent state. |
| `filterSegments` | Filtering metadata used by list screens. |
| `phoneNumber` | Phone number verification and account fields. |

Golden tests should start with procedure groups that support core screens: `me`, `eventTypes`, `eventTypesHeavy`, `availability`, `slots`, `bookings`, `calendars`, `credentials`, `webhook`, `oAuth`, and `apiKeys`.

## API v2 Route Groups

API v2 is versioned and DTO-driven. The Go backend can expose these routes directly, while the Next.js web app can keep using a tRPC bridge for UI compatibility.

| Route Group | Versions or Base Path | Major Operations | Source |
| --- | --- | --- | --- |
| Health | `/health` | service health check | `../../../reference/apps/api/v2/src/app.controller.ts` |
| Deployment webhook | `/v2/webhooks/vercel/deployment-promoted` | Vercel deployment callback | `../../../reference/apps/api/v2/src/vercel-webhook.controller.ts` |
| Billing webhook gap | `/v2/billing/webhook`, `/api/v2/billing/webhook` | raw-body middleware is configured, but no controller was found in the API v2 reference; verify whether this is deprecated, missing, or handled outside this app before re-exposing | `../../../reference/apps/api/v2/src/app.module.ts` |
| Bookings | `/v2/bookings`, versions `2024-04-15`, `2024-06-11`, `2024-06-14`, `2024-08-13` | list, get, create, recurring create, reschedule, cancel, mark no-show or absent, reassign, confirm, decline, by-seat lookup, calendar links, references, conferencing sessions, recordings, transcripts | `../../../reference/apps/api/v2/src/platform/bookings` |
| Booking attendees | `/v2/bookings/:bookingUid/attendees` | list, get, add, delete | `../../../reference/apps/api/v2/src/platform/bookings/2024-08-13/controllers/booking-attendees.controller.ts` |
| Booking guests | `/v2/bookings/:bookingUid/guests` | add guest | `../../../reference/apps/api/v2/src/platform/bookings/2024-08-13/controllers/booking-guests.controller.ts` |
| Booking location | `/v2/bookings/:bookingUid/location` | update booking location | `../../../reference/apps/api/v2/src/platform/bookings/2024-08-13/controllers/booking-location.controller.ts` |
| Slots | `/v2/slots`, versions `2024-04-15`, `2024-06-11`, `2024-06-14`, `2024-08-13`, `2024-09-04` | available slots, reserve, delete selected slot, slot reads, reservation create/read/update/delete | `../../../reference/apps/api/v2/src/modules/slots` |
| Schedules | `/v2/schedules`, versions `2024-04-15`, `2024-06-11`, `2024-06-14` | create, default read, read, list, patch, delete | `../../../reference/apps/api/v2/src/platform/schedules` |
| Event types | `/v2/event-types`, versions `2024-04-15`, `2024-06-11`, `2024-06-14` | create, read, list, public profile reads, patch, delete | `../../../reference/apps/api/v2/src/platform/event-types` |
| Event type private links | `/v2/event-types/:eventTypeId/private-links` | create, list, update, delete | `../../../reference/apps/api/v2/src/platform/event-types-private-links` |
| Event type webhooks | `/v2/event-types/:eventTypeId/webhooks` | create, list, get, update, delete one, delete all | `../../../reference/apps/api/v2/src/modules/event-types/controllers/event-types-webhooks.controller.ts` |
| Calendars | `/v2/calendars` | ICS feed save/check, busy times, list, connect, save, create credentials, check, disconnect | `../../../reference/apps/api/v2/src/platform/calendars/controllers/calendars.controller.ts` |
| Unified calendars | `/v2/calendars` | connections, events CRUD, freebusy, legacy event aliases | `../../../reference/apps/api/v2/src/modules/cal-unified-calendars/controllers/cal-unified-calendars.controller.ts` |
| Selected calendars | `/v2/selected-calendars` | select and deselect calendars | `../../../reference/apps/api/v2/src/modules/selected-calendars/controllers/selected-calendars.controller.ts` |
| Destination calendars | `/v2/destination-calendars` | set destination calendar | `../../../reference/apps/api/v2/src/modules/destination-calendars/controllers/destination-calendars.controller.ts` |
| Conferencing | `/v2/conferencing` | connect, OAuth auth URL, OAuth callback, list apps, set default, get default, disconnect | `../../../reference/apps/api/v2/src/modules/conferencing/controllers/conferencing.controller.ts` |
| Google Calendar helper | `/v2/gcal` | OAuth auth URL, OAuth save, check connection | `../../../reference/apps/api/v2/src/platform/gcal/gcal.controller.ts` |
| Webhooks | `/v2/webhooks` | create, update, get, list, delete | `../../../reference/apps/api/v2/src/modules/webhooks/controllers/webhooks.controller.ts` |
| Platform OAuth clients | `/v2/oauth-clients` | create, list, get, managed users read, patch, delete | `../../../reference/apps/api/v2/src/modules/oauth-clients/controllers/oauth-clients/oauth-clients.controller.ts` |
| Platform OAuth client users | `/v2/oauth-clients/:clientId/users` | list, create, get, patch, delete, force refresh | `../../../reference/apps/api/v2/src/modules/oauth-clients/controllers/oauth-client-users/oauth-client-users.controller.ts` |
| Platform OAuth client webhooks | `/v2/oauth-clients/:clientId/webhooks` | create, update, get, list, delete one, delete all | `../../../reference/apps/api/v2/src/modules/oauth-clients/controllers/oauth-client-webhooks/oauth-client-webhooks.controller.ts` |
| Platform OAuth flow | `/v2/oauth/:clientId` | authorize, exchange, refresh | `../../../reference/apps/api/v2/src/modules/oauth-clients/controllers/oauth-flow/oauth-flow.controller.ts` |
| OAuth2 token API | `/v2/auth/oauth2` | client metadata and token endpoint | `../../../reference/apps/api/v2/src/modules/auth/oauth2/controllers/oauth2.controller.ts` |
| Atoms OAuth2 | `/v2/atoms/auth/oauth2/clients/:clientId` | Atoms OAuth client metadata | `../../../reference/apps/api/v2/src/modules/auth/oauth2/controllers/atoms-oauth2.controller.ts` |
| Atoms verification | `/v2/atoms` | email/phone verification code request and verification, verified email reads/writes | `../../../reference/apps/api/v2/src/modules/atoms/controllers/atoms.verification.controller.ts` |
| Atoms schedules | `/v2/atoms` | schedule reads, event-type schedule reads, create, duplicate, patch | `../../../reference/apps/api/v2/src/modules/atoms/controllers/atoms.schedules.controller.ts` |
| Atoms event types | `/v2/atoms` | event type reads, app event types, payment lookup, bulk default location update, patch | `../../../reference/apps/api/v2/src/modules/atoms/controllers/atoms.event-types.controller.ts` |
| Atoms conferencing | `/v2/atoms` | conferencing app reads | `../../../reference/apps/api/v2/src/modules/atoms/controllers/atoms.conferencing-apps.controller.ts` |
| Me | `/v2/me` | read and patch current user | `../../../reference/apps/api/v2/src/platform/me/me.controller.ts` |
| Verified resources | `/v2/verified-resources` | email and phone verification request, verify, list, get | `../../../reference/apps/api/v2/src/modules/verified-resources/controllers/users-verified-resources.controller.ts` |
| Provider | `/v2/provider` | provider metadata and provider access token | `../../../reference/apps/api/v2/src/platform/provider/provider.controller.ts` |
| Stripe | `/v2/stripe` | connect, save, check | `../../../reference/apps/api/v2/src/modules/stripe/controllers/stripe.controller.ts` |
| API keys | `/v2/api-keys` | refresh API key | `../../../reference/apps/api/v2/src/modules/api-keys/controllers/api-keys.controller.ts` |
| Timezones | `/v2/timezones` | list timezones | `../../../reference/apps/api/v2/src/modules/timezones/controllers/timezones.controller.ts` |

API v2 auth and protocol details to lock down:

- headers: `Authorization`, `x-cal-client-id`, `x-cal-secret-key`;
- platform access token CORS and origin behavior;
- urlencoded token body handling;
- request id behavior and rate limits;
- DTO envelopes, especially `{ status, data }` and paginated lists;
- legacy aliases under both `/v2/*` and `/api/v2/*` where deployed.

## Next.js App API Routes

These routes currently live inside the web application. Some should remain as thin Next.js compatibility handlers, while the business behavior should move to Go services or workers.

| Route Family | Paths | Compatibility Notes |
| --- | --- | --- |
| Account auth | `/api/auth/forgot-password`, `/api/auth/reset-password`, `/api/auth/setup`, `/api/auth/signup` | Preserve response shapes and email side effects while moving user/account behavior behind Go services. |
| OAuth token helpers | `/api/auth/oauth/me`, `/api/auth/oauth/refreshToken`, `/api/auth/oauth/token` | Token payload, expiry, refresh, and error envelopes are client-visible. |
| Two-factor auth | `/api/auth/two-factor/totp/setup`, `/api/auth/two-factor/totp/enable`, `/api/auth/two-factor/totp/disable` | Preserve setup and verification flow while tightening secret handling. |
| Availability utility | `/api/availability/calendar` | Calendar availability response shape used by frontend flows. |
| Public assets and utility reads | `/api/avatar/[uuid]`, `/api/logo`, `/api/csrf`, `/api/geolocation`, `/api/ip`, `/api/username`, `/api/version`, `/api/me` | Cache headers and unauthenticated behavior matter for public pages. |
| Booking actions | `/api/cancel`, `/api/link`, `/api/verify-booking-token` | Public booking flows; preserve token validation, redirects or JSON, and booking side effects. |
| Email utility | `/api/email` | Confirm whether this is public, internal, or development-only before re-exposing. |
| Video and recordings | `/api/recorded-daily-video`, `/api/video/guest-session`, `/api/video/recording` | Preserve Daily/video provider auth, metadata, and response shape. |
| Cron routes | `/api/cron/bookingReminder`, `/api/cron/calendar-subscriptions-cleanup`, `/api/cron/calendar-subscriptions`, `/api/cron/changeTimeZone`, `/api/cron/selected-calendars`, `/api/cron/syncAppMeta`, `/api/cron/webhookTriggers` | Move execution to Go workers; keep routes as compatibility triggers. Avoid query-string secrets in the replacement. |
| Tasker routes | `/api/tasks/cleanup`, `/api/tasks/cron` | Preserve scheduler trigger semantics and idempotency. |
| Webhook receivers | `/api/webhook/app-credential`, `/api/webhooks/calendar-subscription/[provider]`, `/api/sync/helpscout` | Raw body, signature verification, and provider retry behavior are part of the contract. |
| Referrals | `/api/user/referrals-token` | Preserve account-scoped token generation semantics. |

## Contract Detail Needed Per Route

Each route group should get a focused spec before it is ported:

- request method, path, query, body, and content type;
- auth modes and policy checks;
- response status, headers, envelope, and validation error shape;
- database rows read and written;
- outbound provider calls;
- emitted webhooks, jobs, emails, analytics, and audit logs;
- idempotency and retry behavior;
- intentional security breaks, if any.

The first detailed route specs should cover identity/auth, slots, booking writes, API v2 OAuth, and webhook delivery because those areas have the highest compatibility and security risk.
