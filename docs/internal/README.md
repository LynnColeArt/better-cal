# Internal Docs

These notes are for the backend replacement effort. The goal is a Go service that can stand in for the current backend while the Next.js frontend keeps rendering the product.

Read these in order:

1. [Feature Inventory](feature-inventory.md)
2. [Route Inventory](route-inventory.md)
3. [Backend Contracts](backend-contracts.md)
4. [Data Model Contracts](data-model-contracts.md)
5. [Drop-In Compatibility Strategy](drop-in-compatibility-strategy.md)
6. [Security Audit Ledger](security-audit-ledger.md)
7. [Project Plan](project-plan.md)

The reference implementation lives in `../../../reference`. These docs point to source landmarks rather than trying to duplicate every field from the existing implementation.

For implementation-safe material, use the source-neutral spec pack in `../spec`. The `internal` docs are for reviewers who are allowed to inspect the reference implementation.

## Working Assumption

The first replacement backend should be compatibility-first:

- preserve current endpoints, methods, auth inputs, response envelopes, status codes, webhook payloads, and database-visible side effects;
- fix known security defects as intentional contract breaks only where the current behavior is unsafe;
- keep Next.js as the frontend and, if useful, as a thin compatibility bridge during migration;
- move domain behavior into Go services behind compatibility adapters.
