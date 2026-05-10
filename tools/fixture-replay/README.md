# Fixture Replay Tool

This tool replays captured fixture intents against a replacement server and compares the normalized response to the approved fixture output.

It executes `request.template.json` files, not `request.redacted.json` files. The redacted request remains review evidence; the template remains the executable fixture input.

## Replay Accepted Fixtures

```bash
CALDIY_API_KEY=cal_test_example \
node tools/fixture-replay/replay-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --base-url http://localhost:8080
```

By default, only `accepted` fixtures are replayed.

## Replay One Fixture

```bash
CALDIY_API_KEY=cal_test_example \
node tools/fixture-replay/replay-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --fixture api-v2-auth.api-key.success \
  --base-url http://localhost:8080
```

## Draft Fixture Smoke Runs

Use `--include-needs-capture` only for local smoke tests where captured output lives in a temporary fixture root:

```bash
node tools/fixture-replay/replay-fixtures.mjs \
  --manifest contracts/manifests/api-v2-auth.json \
  --base-url http://localhost:5555 \
  --fixtures-root /tmp/caldiy-fixture-smoke-example \
  --include-needs-capture
```

The comparator applies the same redaction and unstable-field normalization as fixture capture, then reports the first mismatched JSON paths.
