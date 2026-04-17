# Source-Neutral Spec Pack

This directory is the implementation-safe contract pack for the whiteroom reimplementation. It describes externally visible behavior, security requirements, fixture expectations, and migration decisions without depending on source paths or implementation details from the reference app.

Read these in order:

1. [Whiteroom Protocol](whiteroom-protocol.md)
2. [Compatibility Matrix](compatibility-matrix.md)
3. [Data Structure Contracts](data-structure-contracts.md)
4. [Fixture Harness](fixture-harness.md)
5. [Security Baseline](security-baseline.md)
6. [Security Regression Controls](security-regression-controls.md)
7. [Migration And Cutover Plan](migration-cutover-plan.md)
8. [Implementation Scaffold](implementation-scaffold.md)
9. [API v2 Auth Spec](api-v2-auth-spec.md)
10. [Booking Lifecycle Spec](booking-lifecycle-spec.md)

## Rule Of Use

The implementation team may use these specs, accepted fixture files, approved database contract artifacts, and public protocol names such as paths, headers, enum values, and response fields.

The implementation team should not inspect source-informed notes, source code, copied snippets, internal class names, internal function names, or algorithms from the reference app.

Machine-readable contract artifacts live in [Contract Artifacts](../../contracts/README.md).

## Spec Status

These docs are initial working specs. A spec becomes implementation-ready when it has:

- accepted route or procedure behavior;
- accepted data structure and persistence contracts;
- accepted auth and authorization rules;
- accepted request and response fixtures;
- accepted state transition or side-effect fixtures for writes;
- listed security breaks;
- an owner for unresolved gaps.
