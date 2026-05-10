# Security Baseline

The replacement must be safer than the reference while preserving legitimate client behavior. Compatibility does not require preserving known unsafe behavior.

The baseline defines what must be true. [Security Regression Controls](security-regression-controls.md) defines the tests, manifests, and gates that prove those properties remain true.

## Identity

- Resolve users by immutable ids, not mutable emails.
- Treat session update payloads as untrusted input.
- Bind profile, organization, and team context server-side.
- Do not accept client-supplied role, organization, profile, or user ids without authorization.
- Preserve impersonation only with explicit audit events and policy checks.

## Authentication

- Support required legacy auth inputs at the boundary: bearer API keys, platform OAuth client id and secret headers, platform access tokens, third-party access tokens, and session-backed calls.
- Store secrets hashed or encrypted as appropriate.
- Compare secrets using constant-time comparison.
- Accept plaintext secrets only at creation or verification boundaries.
- Never return existing secrets after creation.
- Validate token issuer, audience, expiry, subject, and revocation state.

## Authorization

- Centralize policy checks for user, profile, organization, team, system admin, platform OAuth client, and PBAC-style booking access.
- Treat metadata-only decorators or annotations as documentation unless enforced by middleware or service policy.
- Require explicit permission checks for platform OAuth permissions.
- Deny by default.
- Add audit events for privileged writes and impersonated actions.

## Secrets And Credentials

- Do not expose credential payloads, provider refresh tokens, platform client secrets, webhook secrets, or encryption keys.
- Redact secrets before logging, tracing, metrics, errors, and fixture output.
- Keep provider credentials behind service ports.
- Rotate or migrate legacy plaintext secrets to hashed or encrypted storage.
- Separate public app metadata from private credential state.

## Logging And Observability

- Structured logs must redact known secret fields.
- Avoid logging full request bodies by default.
- Log authentication failures without credential values.
- Include request id, principal id, auth method, route, status, and denial reason where safe.
- Capture security-relevant audit events in durable storage.

## Webhooks

- Verify inbound provider signatures using raw-body bytes.
- Sign outbound webhook payloads consistently.
- Protect webhook secrets from read APIs.
- Add replay protection where provider protocol supports it.
- Preserve retry semantics while preventing duplicate side effects.

## Cron And Jobs

- Do not accept cron secrets in query strings.
- Prefer signed headers, private network ingress, or scheduler identity.
- Make jobs idempotent.
- Lock repeated jobs where duplicate execution is unsafe.
- Record job attempts, failures, and dead-letter state.

## API Responses

- Use explicit allowlists for response fields.
- Never return credential secret fields.
- Preserve public response envelopes unless a security break is approved.
- Return stable validation errors without leaking internals.
- Preserve null versus omitted fields where clients depend on it.

## Database And State

- Keep aggregate writes transactional for booking, OAuth, credential, webhook, and payment operations.
- Enforce uniqueness for public ids and idempotency keys.
- Treat public identifiers as high-entropy where they authorize public flows.
- Avoid exposing integer ids where public routes use opaque identifiers.
- Preserve audit and evidence records.

## Security Break Ledger

| Unsafe Behavior | Replacement Behavior | Compatibility Handling |
| --- | --- | --- |
| Client-supplied session update fields can alter identity-sensitive values | Resolve identity and profile context server-side | UI/session fixtures must assert server-owned identity wins |
| Platform OAuth client secrets are returned after creation | Show secret once; store only hash after migration | API clients must rotate or record secret at creation |
| Role metadata exists without enforced authorization | Enforce policy in middleware and service layer | Security-break fixtures expect denials |
| Cron secrets appear in query strings | Use signed headers or scheduler identity | Keep route path but change auth contract intentionally |
| Secrets can appear in stringified logs | Redact before logging | Fixture/log tests assert redaction |
| Authorization codes can be consumed non-atomically | Consume codes transactionally once | Replay tests assert second exchange fails |

## Gate Before Booking And OAuth Writes

Before implementing booking writes or OAuth token writes, the replacement must have:

- principal resolver;
- policy engine;
- secret store;
- redacting logger;
- audit event writer;
- transaction helper;
- fixture runner for security-break cases.
