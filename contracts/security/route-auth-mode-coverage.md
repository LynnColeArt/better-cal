# Route Auth-Mode Coverage

Last refreshed: 2026-04-25.

This report is generated from implemented Go routes, `contracts/registries/routes.json`, and `contracts/registries/policies.json` by:

```bash
node tools/contracts/check-policy-coverage.mjs --report
```

The validator allows future registry modes such as `session`, `platform-access-token`, `public-booking-token`, and `public-pkce-client` to remain listed before their runtime support lands. It fails when a currently implemented mode, such as `api-key`, `oauth-access-token`, `platform-client-secret`, or `oauth-client`, is missing from either the handler or the route policy contract.

| Route | Handler | Policy | Registry auth modes | Implemented auth modes |
| --- | --- | --- | --- | --- |
| `GET /health` | `health` | `policy.public.health` | none | (none) |
| `GET /v2/me` | `me` | `policy.me.read` | api-key, session, platform-access-token | api-key |
| `GET /v2/apps` | `readAppCatalog` | `policy.apps.read` | api-key, session, platform-access-token | api-key |
| `POST /v2/app-install-intents` | `createAppInstallIntent` | `policy.apps.install` | api-key, session, platform-access-token | api-key |
| `GET /v2/calendar-connections` | `readCalendarConnections` | `policy.calendar-connections.read` | api-key, session, platform-access-token | api-key |
| `GET /v2/calendars` | `readCalendarCatalog` | `policy.calendars.read` | api-key, session, platform-access-token | api-key |
| `GET /v2/credentials` | `readCredentialMetadata` | `policy.credentials.read` | api-key, session, platform-access-token | api-key |
| `GET /v2/selected-calendars` | `readSelectedCalendars` | `policy.selected-calendars.read` | api-key, session, platform-access-token | api-key |
| `POST /v2/selected-calendars` | `saveSelectedCalendar` | `policy.selected-calendars.write` | api-key, session, platform-access-token | api-key |
| `DELETE /v2/selected-calendars/{calendarRef}` | `deleteSelectedCalendar` | `policy.selected-calendars.write` | api-key, session, platform-access-token | api-key |
| `GET /v2/destination-calendars` | `readDestinationCalendar` | `policy.destination-calendars.read` | api-key, session, platform-access-token | api-key |
| `POST /v2/destination-calendars` | `saveDestinationCalendar` | `policy.destination-calendars.write` | api-key, session, platform-access-token | api-key |
| `GET /v2/slots` | `readSlots` | `policy.slots.read` | none | (none) |
| `POST /v2/bookings` | `createBooking` | `policy.booking.write` | api-key, oauth-access-token, session, platform-access-token, public-booking-token | api-key, oauth-access-token |
| `GET /v2/bookings/{bookingUid}` | `readBooking` | `policy.booking.read` | api-key, oauth-access-token, session, platform-access-token, public-booking-token | api-key, oauth-access-token |
| `POST /v2/bookings/{bookingUid}/cancel` | `cancelBooking` | `policy.booking.write` | api-key, oauth-access-token, session, platform-access-token, public-booking-token | api-key, oauth-access-token |
| `POST /v2/bookings/{bookingUid}/reschedule` | `rescheduleBooking` | `policy.booking.write` | api-key, oauth-access-token, session, platform-access-token, public-booking-token | api-key, oauth-access-token |
| `POST /v2/bookings/{bookingUid}/confirm` | `confirmBooking` | `policy.booking.host-action` | api-key, oauth-access-token, session, platform-access-token | api-key, oauth-access-token |
| `POST /v2/bookings/{bookingUid}/decline` | `declineBooking` | `policy.booking.host-action` | api-key, oauth-access-token, session, platform-access-token | api-key, oauth-access-token |
| `GET /v2/auth/oauth2/clients/{clientId}` | `oauthClientMetadata` | `policy.oauth2.client.read` | api-key | api-key |
| `POST /v2/auth/oauth2/token` | `oauthToken` | `policy.oauth2.token.exchange` | oauth-client, public-pkce-client | oauth-client |
| `GET /v2/oauth-clients/{clientId}` | `platformClient` | `policy.platform-client.read` | platform-client-secret | platform-client-secret |
