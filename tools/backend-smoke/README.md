# Backend Smoke Tool

This tool proves the starter Go backend can satisfy the current fixture replay contracts.

It:

- captures expected fixture outputs from the synthetic mock API;
- reviews the captured outputs and writes schema snapshots in a temporary directory;
- starts the Go API service;
- replays the captured API v2 auth and booking lifecycle fixtures against Go.

```bash
node tools/backend-smoke/smoke-test.mjs
```

The smoke test uses synthetic credentials only.
