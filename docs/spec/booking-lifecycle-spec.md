# Booking Lifecycle Spec

This spec describes the source-neutral booking behavior needed for drop-in compatibility.

## Scope

In scope:

- API v2 booking reads and writes.
- Booking attendees, guests, location changes, references, calendar links, conferencing sessions, recordings, and transcripts.
- Booking side effects: database state, calendar events, conferencing rooms, emails, webhooks, scheduled jobs, audit, and payment state.
- Web UI booking behavior when routed through a Next.js compatibility bridge.

Out of scope:

- Full slot generation internals.
- Provider-specific calendar and conferencing implementations.
- Payment provider settlement internals.

## Public Routes

Core routes:

- `GET /v2/bookings`
- `GET /v2/bookings/{bookingUid}`
- `GET /v2/bookings/{bookingUid}/reschedule`
- `POST /v2/bookings`
- `POST /v2/bookings/recurring`
- `POST /v2/bookings/{bookingUid}/reschedule`
- `POST /v2/bookings/{bookingUid}/cancel`
- `POST /v2/bookings/{bookingUid}/mark-no-show`
- `POST /v2/bookings/{bookingUid}/mark-absent`
- `POST /v2/bookings/{bookingUid}/reassign`
- `POST /v2/bookings/{bookingUid}/reassign/{userId}`
- `POST /v2/bookings/{bookingUid}/confirm`
- `POST /v2/bookings/{bookingUid}/decline`

Related routes:

- `GET /v2/bookings/by-seat/{seatUid}`
- `GET /v2/bookings/{bookingUid}/calendar-links`
- `GET /v2/bookings/{bookingUid}/references`
- `GET /v2/bookings/{bookingUid}/conferencing-sessions`
- `GET /v2/bookings/{bookingUid}/recordings`
- `GET /v2/bookings/{bookingUid}/transcripts`
- `GET /v2/bookings/{bookingUid}/attendees`
- `GET /v2/bookings/{bookingUid}/attendees/{attendeeId}`
- `POST /v2/bookings/{bookingUid}/attendees`
- `DELETE /v2/bookings/{bookingUid}/attendees/{attendeeId}`
- `POST /v2/bookings/{bookingUid}/guests`
- `PATCH /v2/bookings/{bookingUid}/location`

Version-specific availability and field behavior must be locked by fixtures.

## Booking Identity

- `bookingUid` is the public booking identifier.
- Public routes should not require integer booking ids.
- Booking uid values must be unique and high-entropy enough for public flows.
- Related attendee, seat, reference, recording, and transcript ids must preserve accepted public shape.

## Booking States

The replacement must preserve observable booking status values:

- `pending`
- `accepted`
- `cancelled`
- `rejected`
- `awaiting_host`

State transitions:

| Action | Expected Transition |
| --- | --- |
| create confirmed booking | none to accepted |
| create requires host approval | none to pending or awaiting host |
| cancel accepted booking | accepted to cancelled |
| cancel pending booking | pending to cancelled |
| reschedule | old booking cancelled or superseded; new booking created or existing booking updated according to accepted fixtures |
| confirm | pending or awaiting host to accepted |
| decline | pending or awaiting host to rejected |
| mark no-show or absent | preserve booking status unless accepted fixtures show otherwise; record no-show state |
| reassign | preserve booking identity or emit accepted replacement behavior according to fixtures |

## Create Booking Behavior

Required checks:

- event type exists and is bookable;
- requested time is available;
- requester is allowed to book;
- host, team, round-robin, and assignment rules are satisfied;
- selected calendars and existing bookings do not conflict;
- buffers, booking limits, duration limits, minimum notice, timezone, travel schedule, out-of-office, and holidays are applied;
- required booking fields are valid;
- payment requirements are satisfied or initialized;
- idempotency key prevents duplicate effective booking where supplied.

Required writes and side effects:

- booking record;
- attendee records;
- seat records where applicable;
- booking references for external calendar or conferencing providers;
- payment state where applicable;
- audit events;
- confirmation or request emails;
- webhook events;
- scheduled reminder and webhook trigger jobs;
- provider calls for calendar and conferencing creation where configured.

## Cancel Booking Behavior

Required checks:

- booking exists;
- caller has permission;
- cancellation token or public link token is valid where used;
- cancellation reason and attendee context are accepted where required.

Required writes and side effects:

- booking status or cancellation fields updated;
- external calendar event cancelled or updated;
- conferencing cleanup where applicable;
- payment or no-show fee behavior preserved;
- attendee and host emails emitted where configured;
- cancellation webhook emitted;
- scheduled jobs cancelled or updated;
- audit event recorded.

## Reschedule Behavior

Required checks:

- original booking exists and can be rescheduled;
- new time is available;
- caller has permission or valid reschedule token;
- reschedule limits and event type rules are satisfied.

Required writes and side effects:

- original and replacement booking relationship preserved according to fixtures;
- external calendar event updated or recreated;
- conferencing references updated or recreated;
- reschedule emails emitted;
- reschedule webhook emitted;
- scheduled jobs moved;
- audit event recorded.

## Confirm And Decline Behavior

Required checks:

- booking exists;
- booking is in a confirmable or declinable state;
- caller is an authorized host or platform principal.

Required side effects:

- accepted status for confirm;
- rejected status for decline;
- emails and webhooks emitted;
- calendar and conferencing side effects performed only when accepted fixtures show they occur.

## Attendees, Guests, And Location

Attendee operations must:

- preserve attendee identity and contact fields;
- enforce capacity and seat constraints;
- emit side effects only where accepted fixtures require them.

Guest operations must:

- validate guest input;
- avoid duplicate guest records according to fixtures;
- preserve guest visibility in booking responses and emails.

Location operations must:

- validate allowed location types;
- update booking-visible location fields;
- update provider references where required.

## Read Behavior

Booking reads must preserve:

- response envelope;
- field names and null versus omitted behavior;
- timezone normalization;
- attendee, guest, host, location, payment, reference, and metadata shapes;
- pagination and filtering behavior for lists;
- authorization denials and not-found behavior.

Sensitive internals must not be exposed unless they are accepted public contract fields.

## Idempotency And Concurrency

- Create booking must prevent duplicate bookings for the same idempotency key.
- Slot reservation and booking creation must be race safe.
- Authorization code, payment, provider, webhook, and email side effects must tolerate retries.
- Cancellation and reschedule must be safe to retry where accepted fixtures show idempotent behavior.

## Webhook Events

Fixtures must define emitted trigger names and payloads for:

- booking created;
- booking requested;
- booking cancelled;
- booking rescheduled;
- booking confirmed;
- booking rejected;
- attendee added or removed where public;
- meeting started, ended, or no-show where scheduled triggers apply.

Webhook signatures, payload templates, content types, and retry behavior are part of the contract.

## Authorization

Booking access can be granted by:

- owning user;
- host;
- team or organization role;
- platform OAuth permission;
- valid public booking token;
- system job principal for scheduled side effects.

The replacement must not grant access from mutable email identity alone.

## Required Fixtures

Critical:

- create personal booking;
- create team booking;
- create round-robin booking;
- create seated booking;
- create recurring booking;
- create paid booking;
- create platform booking;
- create duplicate with same idempotency key;
- create conflict at same slot;
- cancel by owner;
- cancel by public token;
- reschedule by owner;
- reschedule by public token;
- confirm pending booking;
- decline pending booking;
- mark no-show or absent;
- reassign booking;
- list bookings with pagination and filters;
- read by booking uid;
- read recordings, transcripts, calendar links, and references.

Side-effect fixtures:

- calendar create/update/delete provider calls;
- conferencing create/update/delete provider calls;
- emails;
- webhook payloads and signatures;
- scheduled jobs;
- payment state transitions;
- audit events.

## Open Decisions

- Exact reschedule persistence model for each API version.
- Whether repeated cancel/reschedule calls should return prior success or a conflict.
- Exact no-show and absent response shape by API version.
- Which recording and transcript fields are public versus privileged.
