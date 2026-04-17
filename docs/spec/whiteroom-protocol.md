# Whiteroom Protocol

The goal is to build a drop-in replacement without translating the reference implementation line by line. Compatibility should come from public behavior, accepted specs, and fixtures.

## Roles

| Role | May Inspect Reference Source | May Write Replacement Code | Outputs |
| --- | --- | --- | --- |
| Reference reviewer | Yes | No | Observations, behavior notes, route lists, risk notes |
| Spec author | Yes, if also acting as reviewer; otherwise no | No | Source-neutral specs, fixture manifests, open questions |
| Implementation engineer | No | Yes | Go services, Next.js frontend, tests against accepted specs |
| Compatibility QA | No for source; yes for running reference fixtures | No production code | Fixture capture, replay, diff reports |
| Security owner | As needed | No production code unless separately assigned | Approved security breaks and security baseline changes |

One person can fill multiple roles only if the project explicitly accepts that as a reduced whiteroom boundary. For stricter separation, implementation engineers should not read source-informed material at all.

## Allowed Implementation Inputs

Implementation engineers may use:

- documents in this `spec/` directory;
- accepted fixture manifests and fixture payloads;
- accepted database contract artifacts such as normalized DDL, public enum lists, and migration requirements;
- public protocol names: URLs, HTTP methods, headers, query keys, request fields, response fields, enum string values, cookie names, and webhook trigger names;
- public third-party API documentation for providers.

## Prohibited Implementation Inputs

Implementation engineers should not use:

- reference source files;
- source-informed notes outside this spec pack;
- copied code snippets;
- internal class names, function names, repository names, local helper names, or comments from the reference implementation;
- tests copied from the reference implementation;
- algorithm descriptions that reveal source structure rather than externally observable behavior.

Public product terms are allowed. For example, `bookingUid`, `cal-api-version`, `Authorization`, and `WebhookTriggerEvents` values are public contract terms when they appear in API payloads or external behavior.

## Spec Authoring Rules

Specs should describe behavior in terms of:

- caller-visible inputs and outputs;
- authorization outcomes;
- state transitions;
- externally observable side effects;
- timing and idempotency rules;
- accepted security breaks.

Specs should avoid:

- source file paths;
- private framework concepts;
- implementation-specific object graphs;
- exact old algorithms unless they are externally required behavior;
- old dependency names unless the dependency is part of the public protocol.

## Question Flow

When implementation needs clarification:

1. The implementation engineer asks a behavior question in source-neutral terms.
2. The spec author answers from existing accepted specs and fixtures when possible.
3. If the answer needs new investigation, the reference reviewer captures behavior from the reference system.
4. The spec author updates the relevant spec or fixture manifest.
5. Implementation proceeds only from the updated spec or accepted fixture.

Questions should not be answered by pasting source snippets or summarizing private code structure.

## Fixture Flow

Fixtures are the main bridge between compatibility and whiteroom discipline.

1. Capture behavior from a running reference environment.
2. Redact secrets and unstable identifiers.
3. Normalize timestamps, generated ids, provider request ids, and other non-deterministic fields.
4. Record the fixture manifest and approval state.
5. Replay the same fixture against the replacement.
6. Diff using strict, tolerant, or security-break comparison rules defined in [Fixture Harness](fixture-harness.md).

## Security Break Flow

Some reference behavior must not be preserved. A security break is allowed only when:

- the unsafe behavior is named;
- the replacement behavior is specified;
- affected clients or UI flows are listed;
- migration or compatibility handling is described;
- regression tests assert the safer behavior.

The security owner approves security breaks before implementation.

## Contamination Handling

If implementation work accidentally uses prohibited material:

1. Stop work on the affected area.
2. Record what material was exposed and which files or decisions may be affected.
3. Replace affected code or tests using only accepted specs and fixtures.
4. Add a short contamination note to the implementation review.
5. Resume only after the spec author and security owner agree the issue is contained.

## Definition Of Ready

A domain is ready for implementation when it has:

- route or procedure inventory;
- accepted request and response shapes;
- auth and authorization matrix;
- persistence/state transition rules;
- side-effect rules;
- fixture coverage;
- security break list;
- unresolved gaps marked with owners.

## Definition Of Done

A domain is done when:

- replacement behavior passes accepted fixtures;
- security-break tests pass;
- required side effects are asserted;
- logs redact secrets;
- rollback behavior exists for production rollout;
- the spec is updated to reflect the replacement as the new source of truth.
