# API v2 Auth Spec

This spec describes the source-neutral authentication contract for API v2 and platform OAuth behavior.

## Scope

In scope:

- API key authentication.
- Platform OAuth client id and secret headers.
- Platform access token authentication.
- Third-party access token authentication where supported.
- Session-backed API v2 calls.
- OAuth2 client metadata, token exchange, and refresh.
- Atoms OAuth client metadata.

Out of scope:

- Web UI tRPC session protocol.
- Provider-specific OAuth callback details.
- User-facing login UI.

## Public Inputs

| Input | Location | Notes |
| --- | --- | --- |
| `Authorization: Bearer <token>` | HTTP header | Used for API keys, platform access tokens, and other bearer tokens. |
| `x-cal-client-id` | HTTP header | Platform OAuth client id. |
| `x-cal-secret-key` | HTTP header | Platform OAuth client secret. Accepted for verification, never returned after creation. |
| `cal-api-version` | HTTP header | Selects versioned API behavior where routes require it. |
| session cookie | HTTP cookie | Used only where API v2 allows session-backed calls. |
| `Origin` | HTTP header | Used for platform access-token origin checks where applicable. |

## Principal Types

The auth layer resolves one of these principals:

- user principal;
- profile principal;
- organization principal;
- team principal;
- platform OAuth client principal;
- managed user principal;
- system principal for approved internal operations.

Each resolved principal must include:

- immutable subject id;
- auth method;
- granted permissions;
- organization or team scope when applicable;
- impersonation actor when applicable.

## Auth Method Precedence

The replacement must preserve the accepted precedence observed by fixtures. Until fixtures lock exact precedence, use this default:

1. Valid bearer token.
2. Valid platform OAuth client id and secret headers.
3. Valid session-backed identity where the route supports sessions.
4. Reject.

When multiple credential types are present, fixtures must decide whether the replacement uses the first valid credential, rejects ambiguity, or follows legacy precedence.

## OAuth2 Client Metadata

Routes:

- `GET /v2/auth/oauth2/clients/{clientId}`
- `GET /v2/atoms/auth/oauth2/clients/{clientId}`

Behavior:

- Return client metadata for valid client ids.
- Return not-found or invalid-client errors for unknown client ids.
- Do not include client secrets.
- Preserve response envelope and status from accepted fixtures.
- Atoms metadata may return a reduced public shape.

## OAuth2 Token Endpoint

Route:

- `POST /v2/auth/oauth2/token`

Content types:

- `application/json`
- `application/x-www-form-urlencoded`

Supported grants:

- authorization code exchange;
- refresh token exchange.

Required behavior:

- Validate client id.
- Validate client secret for confidential clients.
- Validate PKCE verifier for public clients where required.
- Validate redirect URI for authorization code exchange where applicable.
- Consume authorization codes atomically.
- Rotate or update refresh tokens according to accepted fixtures.
- Return token fields and expiry fields using accepted response names.
- Return OAuth-compatible error shapes from accepted fixtures.

Security requirements:

- Store token secrets hashed or encrypted.
- Do not log token values.
- Deny expired, revoked, reused, or mismatched codes.
- Deny refresh token reuse when fixtures or security policy require rotation.

## API Key Authentication

Required behavior:

- Accept bearer API keys on routes that support API keys.
- Hash and compare keys server-side.
- Resolve the owning user or organization scope.
- Enforce route permissions after authentication.
- Return invalid credential errors without disclosing whether the key exists.
- Show plaintext API keys only at creation or refresh time.

## Platform OAuth Client Credentials

Required behavior:

- Accept `x-cal-client-id` and `x-cal-secret-key` for routes that support platform OAuth client credentials.
- Resolve the owning organization and permission set.
- Enforce owner/admin/member policy where route behavior requires it.
- Deny inactive, deleted, or unapproved clients.
- Preserve legitimate client behavior without returning existing secrets.

Approved security break:

- Existing client secrets must not be returned by list, get, update, or delete responses. Secret values may be shown once at creation or rotation.

## Platform Access Tokens

Required behavior:

- Accept bearer access tokens on routes that support platform access tokens.
- Validate expiry, issuer, subject, token type, and client binding.
- Enforce origin checks where the token contract requires allowed origins.
- Enforce permission bitsets before route execution.
- Preserve managed-user context when tokens represent managed users.

## Session-Backed Calls

Required behavior:

- Use immutable user id from trusted session state.
- Re-resolve profile, organization, and membership context server-side.
- Reject mutated client-provided identity fields.
- Preserve existing session-backed routes where fixtures show support.

Approved security break:

- Session update payloads cannot change identity, email, role, organization, profile, or user id without server-side validation.

## Denial Outcomes

Fixtures must lock exact status and response envelopes for:

- missing credentials;
- malformed credentials;
- expired token;
- revoked token;
- invalid API key;
- invalid platform client id;
- invalid platform secret;
- insufficient permission;
- invalid origin;
- inactive client;
- unknown managed user;
- reused authorization code.

## Required Fixtures

Critical fixtures:

- API key success.
- API key invalid.
- Platform client id and secret success.
- Platform client invalid secret.
- Platform access token success.
- Platform access token expired.
- Platform access token invalid origin.
- Session-backed success.
- Session identity mutation attempt.
- OAuth2 client metadata success and not found.
- OAuth2 authorization code exchange success.
- OAuth2 authorization code replay denied.
- OAuth2 refresh success.
- OAuth2 refresh denied after revocation.

## Open Decisions

- Exact precedence when bearer token and platform client headers are both present.
- Whether ambiguous mixed credentials should be rejected for safer behavior.
- Whether refresh token rotation is mandatory for every refresh or only for selected clients.
