# Compatibility Matrix

This matrix classifies each backend surface for the whiteroom rebuild. "Exact" means preserve caller-visible behavior except for approved security breaks. "Bridge" means keep a compatibility layer while moving behavior behind it. "Verify" means the route or behavior needs confirmation before implementation.

| Surface | Initial Treatment | Compatibility Level | Security Breaks | Fixture Priority | Notes |
| --- | --- | --- | --- | --- | --- |
| API v2 health | Go direct | Exact | None expected | Low | Preserve simple health response and status. |
| API v2 auth and token routes | Go direct | Exact with hardening | Secret handling, token validation, identity resolution | Critical | See [API v2 Auth Spec](api-v2-auth-spec.md). |
| API v2 API-key auth | Go direct | Exact with hardening | Constant-time comparisons, no plaintext key exposure | Critical | Preserve header behavior and invalid credential outcomes. |
| API v2 platform OAuth client credentials | Go direct | Exact with hardening | Do not return client secrets; migrate stored plaintext secrets to hashes | Critical | Preserve client id and secret input names. |
| API v2 platform access tokens | Go direct | Exact with hardening | Stronger origin and permission enforcement where unsafe | Critical | Preserve bearer token use and permission denials. |
| API v2 bookings | Go direct after fixtures | Exact with hardening | Authz fixes, no identity-by-email trust | Critical | See [Booking Lifecycle Spec](booking-lifecycle-spec.md). |
| API v2 booking attendees, guests, location | Go direct after booking core | Exact | Authz fixes where needed | High | Depends on booking aggregate behavior. |
| API v2 slots | Go direct after slot engine | Exact | None expected | Critical | Slot behavior controls booking correctness. |
| API v2 schedules | Go direct | Exact | Authz fixes where needed | High | Preserve availability and default schedule semantics. |
| API v2 event types | Go direct | Exact | Authz fixes where needed | High | Include private links and event-type webhooks. |
| API v2 calendars and unified calendars | Go direct after provider ports | Exact with hardening | No credential leakage | High | Preserve connection, freebusy, event, selected calendar, and destination calendar behavior. |
| API v2 conferencing | Go direct after provider ports | Exact with hardening | No credential leakage | High | Preserve connect, callback, default app, and disconnect behavior. |
| API v2 user webhooks | Go direct | Exact | Validate subscriber URLs and secrets safely | High | Preserve payload templates and delivery settings. |
| API v2 OAuth-client webhooks | Go direct | Exact with hardening | Role enforcement and secret behavior | High | Platform owners/admins only where required. |
| API v2 verified resources | Go direct | Exact with hardening | Rate limits and verification-code secrecy | Medium | Preserve email and phone verification outcomes. |
| API v2 Atoms routes | Go direct or bridge | Exact | Authz fixes where needed | Medium | External embed clients may rely on shapes. |
| API v2 provider route | Go direct | Exact with hardening | Access token redaction where applicable | Medium | Confirm token response is externally required. |
| API v2 Stripe route | Go direct after payment ports | Exact with hardening | Provider credential secrecy | Medium | Payment side effects must be idempotent. |
| API v2 billing webhook | Verify before exposing | Unknown | Raw-body signature handling | High | Middleware route exists; controller ownership must be confirmed. |
| API v2 deployment webhook | Go direct or ops bridge | Exact with hardening | Signature validation and secret logging | Low | Preserve raw-body signature behavior. |
| Web UI tRPC routes | Next.js bridge first | Exact at protocol boundary | Session identity and authz fixes | Critical | Keep tRPC protocol in Next.js until fixtures prove a replacement gateway. |
| Web UI `me`, profile, org, team reads | Next.js bridge to Go | Exact with hardening | Immutable identity | Critical | First UI domain to migrate. |
| Web UI event type management | Next.js bridge to Go | Exact | Authz fixes where needed | High | Heavy response shapes need golden fixtures. |
| Web UI availability and slots | Next.js bridge to Go | Exact | None expected | Critical | Must match timezone, OOO, travel, buffers, and calendar effects. |
| Web UI bookings | Next.js bridge to Go | Exact with hardening | Authz fixes | Critical | Must preserve booking side effects. |
| Web UI app and credential settings | Next.js bridge to Go | Exact with hardening | No credential payload exposure | High | Split public app metadata from credential secrets. |
| Next.js account API routes | Bridge or keep in Next.js | Exact with hardening | Session identity, token secrecy | High | Decide whether NextAuth remains frontend-owned. |
| Next.js public booking routes | Bridge to Go | Exact | Token validation hardening | High | Public booking flows must not change. |
| Next.js cron routes | Thin trigger to Go workers | Exact with hardening | No query-string cron secrets | High | Preserve trigger response shape where monitored. |
| Next.js webhook receivers | Move to Go when raw-body fixtures exist | Exact with hardening | Signature validation, replay protection | High | Provider retry behavior matters. |
| Webhook delivery | Go worker | Exact | Stronger redaction and retry safety | Critical | Payload and signature fixtures are required. |
| Jobs and scheduled triggers | Go worker | Exact with hardening | Idempotency and job auth | High | Duplicate execution must be safe. |
| Existing database schema | Reuse initially | Exact | Sensitive columns protected by access layer | Critical | Prefer behavioral parity before schema redesign. See [Data Structure Contracts](data-structure-contracts.md). |
| New internal Go APIs | New design | Not externally compatible | Security baseline applies | Medium | Internal shape is free as long as adapters preserve contracts. |

## Classification Rules

- Use **Go direct** for API v2 surfaces that can be expressed as stable HTTP contracts.
- Use **Next.js bridge** for tRPC until a Go gateway can reproduce batching, serialization, and error envelopes.
- Use **verify** for routes with evidence of deployment behavior but unclear ownership.
- Keep compatibility exact unless a security break is approved.

## Exit Criteria For Matrix Rows

Each row can move to implementation when it has:

- accepted fixture coverage;
- owner and phase;
- security-break decision;
- rollback or bridge strategy;
- observability expectations.
