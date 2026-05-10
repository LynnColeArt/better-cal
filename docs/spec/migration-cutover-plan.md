# Migration And Cutover Plan

The replacement should become the backend of record one surface at a time. Avoid a big-bang rewrite. Compatibility, observability, and rollback matter more than early internal elegance.

## Migration Principles

- Reuse the existing production data contract first.
- Keep external routes stable.
- Prefer read shadowing before read cutover.
- Prefer audit-only write comparison before write cutover.
- Move high-side-effect domains last.
- Keep rollback switches per route group.
- Treat approved security breaks as planned migrations, not accidental drift.

## Environments

| Environment | Purpose |
| --- | --- |
| Local fixture environment | Replay accepted fixtures quickly during development. |
| Compatibility test environment | Run full fixture suite against reference and replacement. |
| Shadow environment | Send production-like read traffic to replacement without serving its response. |
| Canary environment | Serve selected low-risk traffic through replacement. |
| Production cutover | Route accepted domains to replacement with rollback controls. |

## Routing Strategy

Use route-level routing controls:

- legacy only;
- shadow replacement only;
- replacement read, legacy fallback;
- replacement write for canary cohort;
- replacement primary;
- replacement disabled.

Routing decisions should be observable by route, auth method, tenant, user cohort, and API version.

## Phase 0: Contract Capture

Required:

- source-neutral specs accepted;
- fixture manifests accepted;
- data structure contract artifacts accepted;
- security breaks approved;
- database contract artifact accepted;
- provider mocks available;
- rollback criteria defined.

No production traffic moves in this phase.

## Phase 1: Read Shadowing

Run replacement reads in parallel with legacy reads.

Compare:

- status;
- response shape;
- selected headers;
- authorization outcome;
- latency;
- redaction behavior.

Do not serve replacement responses yet. Record diffs and classify them as expected, fixture gap, bug, or approved security break.

## Phase 2: Read Cutover

Serve low-risk reads from the replacement.

Start with:

- health;
- current-user reads;
- timezone and feature reads;
- provider metadata reads;
- public event type reads after fixtures pass.

Keep fallback to legacy for transient replacement failures if the route is safe to retry.

## Phase 3: Write Shadowing

For write paths, do not double-write to external providers. Instead:

- run replacement validation in audit-only mode;
- compute intended database changes without committing where possible;
- compare intended side effects with legacy side effects;
- record mismatches.

Provider calls should use mocks or dry-run adapters during shadowing.

## Phase 4: Low-Risk Write Canary

Move writes with limited blast radius first:

- profile preference updates;
- schedule create/update/delete after availability fixtures pass;
- event type metadata updates after response fixtures pass;
- webhook configuration writes;
- API key refresh after auth fixtures pass.

Canary by internal tenant or selected users first.

## Phase 5: High-Risk Write Canary

Move booking, OAuth, credential, calendar, conferencing, and payment writes only after:

- fixture coverage is complete;
- provider mocks pass;
- idempotency tests pass;
- rollback plan is tested;
- security-break tests pass;
- operator dashboards exist.

These domains should cut over by narrow cohorts and route groups.

## Data Strategy

Initial state:

- use the existing database-visible contract;
- add new tables only for replacement-owned infrastructure, such as migration tracking, audit expansion, or worker state;
- avoid destructive schema changes until after compatibility cutover.

Data structure migration:

- keep public DTOs stable while internal structs evolve;
- keep public enum strings stable;
- keep public identifiers stable;
- backfill new derived fields before routes depend on them;
- use compatibility views or adapters if internal schema changes before public cutover;
- run schema and state fixtures before and after every migration.

Secret migration:

- support legacy plaintext verification only long enough to rotate or hash secrets;
- hash secrets on successful verification where safe;
- show new secrets only once;
- record migration progress per client or credential.

Schema cleanup:

- defer renames and shape simplification until the replacement owns traffic;
- use additive migrations before subtractive migrations;
- keep rollback windows short and explicit.

## Rollback Strategy

Reads:

- route back to legacy;
- discard replacement cache entries if needed.

Writes:

- rollback is domain-specific and harder;
- preserve idempotency keys;
- record provider side-effect ids;
- avoid partial rollback of provider calls without compensating actions;
- keep legacy write path available until replacement writes are stable.

Jobs:

- use job ownership locks so legacy and replacement do not process the same job unsafely;
- keep a way to pause replacement workers;
- preserve dead-letter queues for replay.

## Observability Gates

Each cutover needs dashboards for:

- request count, latency, and error rate by route and version;
- auth denials by reason;
- fixture drift in shadow mode;
- database write failures;
- provider failures;
- webhook delivery failures;
- queue depth and dead letters;
- security-break events;
- rollback activations.

## Cutover Gate Checklist

- Accepted fixtures pass.
- Security-break tests pass.
- Shadow diff rate is below the approved threshold.
- Operators can identify whether a request used legacy or replacement.
- Rollback switch has been tested.
- Data migration impact is understood.
- Provider side effects are idempotent or compensatable.
- On-call notes exist.
