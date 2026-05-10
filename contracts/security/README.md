# Security Contract Artifacts

Generated security reports and review artifacts will live here.

Current security checks:

- `node tools/contracts/check-policy-coverage.mjs` validates that implemented backend routes have registry policies, handler-side policy checks, and auth-mode coverage for currently implemented auth modes.
- `node tools/contracts/check-policy-coverage.mjs --report` prints the compact route/auth-mode matrix captured in `route-auth-mode-coverage.md`.
- `node tools/contracts/scan-secrets.mjs` validates generated fixture artifacts and supplied log paths do not contain unredacted secret-like fields or known fixture credentials.

Current reports:

- `route-auth-mode-coverage.md`;

Planned files:

- secret scanner reports;
- redaction scanner reports;
- approved security-break ledger exports.
