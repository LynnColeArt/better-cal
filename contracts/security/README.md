# Security Contract Artifacts

Generated security reports and review artifacts will live here.

Current security checks:

- `node tools/contracts/check-policy-coverage.mjs` validates that implemented backend routes have registry policies and handler-side policy checks.
- `node tools/contracts/scan-secrets.mjs` validates generated fixture artifacts and supplied log paths do not contain unredacted secret-like fields or known fixture credentials.

Planned files:

- route policy coverage reports;
- secret scanner reports;
- redaction scanner reports;
- approved security-break ledger exports.
