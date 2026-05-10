# Data Model Contracts

The existing backend is heavily shaped by `../../../reference/packages/prisma/schema.prisma`. A Go rewrite can use a different data-access layer, but it must preserve the persistence behavior that callers and jobs depend on.

## Main Aggregates

### Identity, Organization, and Authorization

Core models:

- `User`
- `UserPassword`
- `Profile`
- `Team`
- `Membership`
- `Role`
- `RolePermission`
- `VerifiedEmail`
- `VerifiedNumber`
- `SecondaryEmail`
- `ApiKey`

Contract notes:

- `User.email` is unique and should not be the service identity. Use `User.id` or `User.uuid`.
- `Profile` represents a user's identity inside an organization. `Profile.uid` and `upId`-style identifiers are part of session behavior.
- `Team` doubles as team and organization. `Team.isOrganization` and `Team.isPlatform` alter authorization and platform API behavior.
- `Membership.role` is `MEMBER`, `ADMIN`, or `OWNER`. This is the authorization backbone for org/team actions.
- API keys store `hashedKey`; plaintext API key is only shown at creation time.

### Scheduling and Event Types

Core models:

- `EventType`
- `Host`
- `HostGroup`
- `HostLocation`
- `Schedule`
- `Availability`
- `SelectedCalendar`
- `DestinationCalendar`
- `TravelSchedule`
- `OutOfOfficeEntry`
- `OutOfOfficeReason`
- `HashedLink`
- `EventTypeTranslation`

Contract notes:

- Event type uniqueness is scoped by user/team and slug.
- Event type JSON fields are contract-heavy: `locations`, `bookingFields`, `recurringEvent`, `metadata`, `bookingLimits`, `durationLimits`, `rrSegmentQueryValue`, and `eventTypeColor`.
- Slot calculation depends on event type settings, host selection, schedules, availability rows, selected calendars, destination calendars, buffers, limits, OOO, holidays, seats, and existing bookings.
- `SelectedCalendar` contains sync subscription fields used by cron/calendar subscription jobs.

### Bookings

Core models:

- `Booking`
- `Attendee`
- `BookingReference`
- `BookingSeat`
- `Payment`
- `Tracking`
- `AssignmentReason`
- `BookingReport`
- `WrongAssignmentReport`
- `BookingInternalNote`
- `WebhookScheduledTriggers`

Contract notes:

- `Booking.uid` is public and unique. Avoid exposing integer ids where legacy routes now require high-entropy ids.
- `Booking.idempotencyKey` prevents duplicate bookings for the same effective slot/request.
- `Booking.status` maps to lower-case database values: `cancelled`, `accepted`, `rejected`, `pending`, and `awaiting_host`.
- `Booking.responses`, `Booking.customInputs`, and `Booking.metadata` are JSON contracts used by emails, webhooks, API output, and UI rendering.
- `BookingReference` stores external calendar/video references. Deleting or changing bookings must update external references carefully.
- Seated events depend on `BookingSeat` and attendee linkage.

### Credentials, Apps, and Integrations

Core models:

- `Credential`
- `DelegationCredential`
- `App`
- `DestinationCalendar`
- `SelectedCalendar`
- `CalendarCache`
- `CalendarCacheEvent`
- `IntegrationAttributeSync`

Contract notes:

- `Credential.key` and `Credential.encryptedKey` are sensitive internal fields. They must not be returned to clients.
- Many provider integrations depend on app-store metadata and app-specific zod schemas.
- Delegation credentials and calendar cache data are backend-only contracts but affect public slot availability.

### OAuth and Platform API

Core models:

- `OAuthClient`
- `AccessCode`
- `PlatformOAuthClient`
- `PlatformAuthorizationToken`
- `AccessToken`
- `RefreshToken`

Contract notes:

- `OAuthClient` is the newer user-created OAuth app flow with `clientId`, `clientSecret`, `clientType`, status, PKCE data, and authorization codes.
- `PlatformOAuthClient` is the API v2/platform client model with permissions bitset, organization, redirect URIs, managed users, and platform tokens.
- Existing `PlatformOAuthClient.secret` is plaintext in the reference implementation. The replacement should store a hash and show secrets only once, while keeping compatibility for existing secrets during migration.
- Authorization codes must be consumed atomically.

### Webhooks, Jobs, and Audit

Core models:

- `Webhook`
- `WebhookScheduledTriggers`
- `Task`
- `BookingAudit`
- `AuditActor`
- `Watchlist`
- `WatchlistAudit`
- `WatchlistEventAudit`

Contract notes:

- `Webhook.eventTriggers` values are external event contract names.
- Webhook uniqueness differs for user subscribers and platform OAuth client subscribers.
- Jobs must be idempotent because cron/task endpoints can be retried.
- Audit events are internal evidence. Preserve actor, source, action, and booking linkage.

## JSON Fields to Promote or Strictly Type

The existing schema relies heavily on JSON fields validated in TypeScript. In Go, define typed structs for:

- event type locations;
- booking fields and booking responses;
- event type metadata;
- booking metadata;
- recurring event config;
- booking/duration limits;
- team and user metadata;
- credential provider payloads;
- webhook payload templates and derived payload data.

Do not treat these as arbitrary `map[string]any` in service code. Use strict structs at API boundaries and storage adapters.

## Migration Rules

- Keep old ids and public identifiers stable.
- Keep enum string values stable.
- Keep timestamps in ISO/RFC3339-compatible UTC outputs unless legacy output explicitly differs.
- Keep null versus omitted-field behavior where clients observe it.
- Keep database writes inside aggregate transactions for booking, OAuth, credential, and webhook operations.
- Build dual-read comparison tooling before switching reads to Go.
- Build shadow-write or audit-only comparison before switching writes to Go.

