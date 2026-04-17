# Security Audit Ledger

This source-informed ledger records security mistakes that the replacement must not repeat. It is for reviewers who are allowed to inspect the reference implementation. Implementation engineers should use the source-neutral controls in `../spec/security-regression-controls.md` instead.

## Scope

This is not a complete penetration test. It is a non-regression ledger for the most important issues found during the initial review and documentation pass.

## Findings

| ID | Severity | Risk | Source Landmarks | Replacement Rule |
| --- | --- | --- | --- | --- |
| SEC-001 | Critical | Session/JWT update data can influence identity-sensitive fields. Downstream API code then risks trusting mutable identity such as email, profile, or organization context. | `../../../reference/packages/features/auth/lib/next-auth-options.ts`, `../../../reference/apps/api/v2/src/modules/auth/strategies/api-auth/api-auth.strategy.ts`, `../../../reference/apps/api/v2/src/modules/auth/strategies/next-auth/next-auth.strategy.ts` | Resolve principal identity from immutable server-owned subject ids. Re-resolve profile, organization, and membership context server-side. Treat session update payloads as untrusted. |
| SEC-002 | Critical | Authorization metadata can exist without reliable enforcement. This creates routes that look protected in code review but do not actually enforce role membership. | `../../../reference/apps/api/v2/src/modules/auth/decorators/roles/membership-roles.decorator.ts`, `../../../reference/apps/api/v2/src/modules/auth/guards/roles/roles.guard.ts`, `../../../reference/apps/api/v2/src/modules/auth/guards/permissions/permissions.guard.ts` | Deny by default. Every route group needs an enforced policy binding and tests for unauthorized users. Metadata-only annotations are not authorization. |
| SEC-003 | Critical | Platform OAuth client secrets can be exposed after creation, and platform client secrets are compared as plaintext values. | `../../../reference/apps/api/v2/src/modules/oauth-clients/services/oauth-clients/oauth-clients-output.service.ts`, `../../../reference/apps/api/v2/src/modules/auth/strategies/api-auth/api-auth.strategy.ts`, `../../../reference/apps/api/v2/src/modules/oauth-clients/controllers/oauth-clients/oauth-clients.controller.ts` | Show client secrets only once at creation or rotation. Store hashes, compare in constant time, and never return existing secrets in list/get/update/delete responses. |
| SEC-004 | High | OAuth authorization code and token exchange behavior can be vulnerable to replay or race conditions if code consumption is not atomic. | `../../../reference/apps/api/v2/src/modules/oauth-clients/services/oauth-flow.service.ts`, `../../../reference/apps/api/v2/src/modules/oauth-clients/controllers/oauth-flow/oauth-flow.controller.ts`, `../../../reference/apps/api/v2/src/modules/tokens/tokens.repository.ts` | Authorization codes are single-use, client-bound, redirect-bound, expiry-bound, and consumed transactionally. Replay attempts must fail. |
| SEC-005 | High | Cron endpoints accept secrets from query parameters. Query strings are commonly logged by proxies, servers, and observability tooling. | `../../../reference/apps/web/app/api/cron/bookingReminder/route.ts`, `../../../reference/apps/web/app/api/cron/calendar-subscriptions-cleanup/route.ts`, `../../../reference/apps/web/app/api/cron/calendar-subscriptions/route.ts`, `../../../reference/apps/web/app/api/cron/changeTimeZone/route.ts`, `../../../reference/apps/web/app/api/cron/selected-calendars/route.ts`, `../../../reference/apps/web/app/api/cron/syncAppMeta/route.ts`, `../../../reference/apps/web/app/api/cron/webhookTriggers/route.ts` | Keep route paths if needed, but authenticate jobs with signed headers, scheduler identity, or private ingress. Reject query-string credentials. |
| SEC-006 | High | Debug logging can stringify credentials, tokens, session state, user records, provider account state, or full request context. | `../../../reference/packages/features/auth/lib/next-auth-options.ts`, `../../../reference/packages/features/auth/lib/getServerSession.ts`, `../../../reference/packages/features/auth/lib/userFromSessionUtils.ts`, `../../../reference/apps/api/v2/src/middleware/app.logger.middleware.ts` | Centralize structured logging with redaction. Do not log raw credentials, sessions, tokens, provider account payloads, request bodies, or secret-bearing errors. |
| SEC-007 | High | Webhook secrets and payload signing are high-impact contracts. Secret exposure or mismatched raw-body handling breaks both security and compatibility. | `../../../reference/packages/features/webhooks/lib/sendPayload.ts`, `../../../reference/packages/features/webhooks/lib/WebhookService.ts`, `../../../reference/packages/features/webhooks/lib/infrastructure/mappers/WebhookOutputMapper.ts`, `../../../reference/apps/api/v2/src/vercel-webhook.guard.ts` | Webhook secrets are write-only or show-once unless an accepted public contract says otherwise. Inbound verification uses raw body bytes. Outbound signing is fixture-tested. |
| SEC-008 | High | Booking writes combine identity, authorization, availability, payment, provider calls, email, webhooks, and scheduled jobs. Partial failures or retries can create privilege or consistency bugs. | `../../../reference/apps/api/v2/src/platform/bookings`, `../../../reference/packages/features/bookings`, `../../../reference/packages/features/webhooks` | Booking writes require transactional state changes, idempotency keys, provider side-effect tracking, authorization tests, and replay-safe jobs. |
| SEC-009 | Medium | Error filters and exception logging may disclose implementation details or sensitive request context if not redacted consistently. | `../../../reference/apps/api/v2/src/filters`, `../../../reference/apps/api/v2/src/modules/auth/oauth2/filters/oauth2-http-exception.filter.ts` | Error responses and logs use allowlisted fields. Internals, credentials, tokens, and provider payloads are not returned or logged. |
| SEC-010 | Medium | Credential-bearing integration state can leak if database rows or provider structs are returned directly. | `../../../reference/packages/app-store`, `../../../reference/packages/features/credentials`, `../../../reference/apps/api/v2/src/modules/conferencing`, `../../../reference/apps/api/v2/src/modules/stripe` | Public app metadata and private credential state are separate types. API responses are explicit allowlists. |

## Reviewer Checklist

- Every high-risk route has an owner, policy decision, and negative authorization test.
- Every secret field is classified as input-only, show-once, internal-only, or public.
- Every token or authorization code has expiry, audience/client binding, revocation behavior, and replay tests.
- Every cron or job trigger has non-query-string authentication.
- Every log path has redaction tests using realistic secret field names.
- Every booking write has idempotency and side-effect tests.
- Every approved security break is mirrored in the source-neutral security regression controls.

## Required Source-Neutral Outputs

The implementation team should receive:

- security invariant list;
- security regression matrix;
- auth and authorization test fixtures;
- redaction test fixtures;
- OAuth replay fixtures;
- cron authentication fixtures;
- booking write abuse-case fixtures.
