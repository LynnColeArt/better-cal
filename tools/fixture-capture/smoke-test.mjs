#!/usr/bin/env node
import { mkdtempSync, readFileSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import { createMockApiV2Server } from "./mock-api-v2-server.mjs";

const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);

function listen(server) {
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") reject(new Error("Unexpected mock server address"));
      else resolve(`http://127.0.0.1:${address.port}`);
    });
  });
}

function close(server) {
  return new Promise((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
  });
}

function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function assertCaptured(outputRoot, manifestId, fixtureId, expectedStatus) {
  const dir = path.join(outputRoot, manifestId, fixtureId);
  statSync(path.join(dir, "request.redacted.json"));
  statSync(path.join(dir, "response.json"));
  statSync(path.join(dir, "capture-metadata.json"));
  statSync(path.join(dir, "request.schema.json"));
  statSync(path.join(dir, "response.schema.json"));

  const requestText = readFileSync(path.join(dir, "request.redacted.json"), "utf8");
  const responseText = readFileSync(path.join(dir, "response.json"), "utf8");
  const response = readJson(path.join(dir, "response.json"));
  assert(response.status === expectedStatus, `${fixtureId}: expected ${expectedStatus}, got ${response.status}`);
  assert(!requestText.includes("cal_test_valid_mock"), `${fixtureId}: API key was not redacted`);
  assert(!requestText.includes("mock-platform-secret"), `${fixtureId}: platform secret was not redacted`);
  assert(!requestText.includes("fixture-attendee@example.test"), `${fixtureId}: attendee email was not redacted`);
  assert(!responseText.includes("fixture-attendee@example.test"), `${fixtureId}: response attendee email was not redacted`);
  assert(!requestText.includes("unauthorized-fixture@example.test"), `${fixtureId}: unauthorized email was not redacted`);
  assert(!responseText.includes("unauthorized-fixture@example.test"), `${fixtureId}: response unauthorized email was not redacted`);
  assert(!requestText.includes("unavailable-slot@example.test"), `${fixtureId}: unavailable slot email was not redacted`);
  assert(!responseText.includes("unavailable-slot@example.test"), `${fixtureId}: response unavailable slot email was not redacted`);
  assert(!requestText.includes("super-secret-fixture"), `${fixtureId}: secret-bearing metadata was not redacted`);
  assert(!responseText.includes("super-secret-fixture"), `${fixtureId}: response echoed secret-bearing metadata`);
}

function runTool(args, env = {}) {
  return new Promise((resolve) => {
    const child = spawn(process.execPath, args, {
      cwd: projectRoot,
      env: { ...process.env, ...env },
      stdio: ["ignore", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });
    child.on("close", (status) => resolve({ status, stdout, stderr }));
  });
}

function runCapture(baseUrl, outputRoot, manifestName) {
  return runTool(
    [
      path.join(projectRoot, "tools", "fixture-capture", "capture-fixtures.mjs"),
      "--manifest",
      path.join(projectRoot, "contracts", "manifests", manifestName),
      "--base-url",
      baseUrl,
      "--output-root",
      outputRoot,
    ],
    {
      CALDIY_API_KEY: "cal_test_valid_mock",
      CALDIY_OAUTH_CLIENT_ID: "mock-oauth-client",
      CALDIY_PLATFORM_CLIENT_ID: "mock-platform-client",
      CALDIY_PLATFORM_CLIENT_SECRET: "mock-platform-secret",
    }
  );
}

function runReview(outputRoot, manifestName, extraArgs = []) {
  return runTool([
    path.join(projectRoot, "tools", "contracts", "review-fixtures.mjs"),
    "--manifest",
    path.join(projectRoot, "contracts", "manifests", manifestName),
    "--fixtures-root",
    outputRoot,
    ...extraArgs,
  ]);
}

function runReplay(baseUrl, outputRoot, manifestName) {
  return runTool(
    [
      path.join(projectRoot, "tools", "fixture-replay", "replay-fixtures.mjs"),
      "--manifest",
      path.join(projectRoot, "contracts", "manifests", manifestName),
      "--base-url",
      baseUrl,
      "--fixtures-root",
      outputRoot,
      "--include-needs-capture",
    ],
    {
      CALDIY_API_KEY: "cal_test_valid_mock",
      CALDIY_OAUTH_CLIENT_ID: "mock-oauth-client",
      CALDIY_PLATFORM_CLIENT_ID: "mock-platform-client",
      CALDIY_PLATFORM_CLIENT_SECRET: "mock-platform-secret",
    }
  );
}

function runSecretScan(outputRoot) {
  return runTool([
    path.join(projectRoot, "tools", "contracts", "scan-secrets.mjs"),
    "--path",
    outputRoot,
  ]);
}

let server = createMockApiV2Server();
const outputRoot = mkdtempSync(path.join(tmpdir(), "caldiy-fixture-smoke-"));

try {
  const baseUrl = await listen(server);
  const authResult = await runCapture(baseUrl, outputRoot, "api-v2-auth.json");
  const bookingResult = await runCapture(baseUrl, outputRoot, "booking-lifecycle.json");
  const slotsResult = await runCapture(baseUrl, outputRoot, "slots.json");
  const authReviewResult = await runReview(outputRoot, "api-v2-auth.json", ["--write-schemas"]);
  const bookingReviewResult = await runReview(outputRoot, "booking-lifecycle.json", ["--write-schemas"]);
  const slotsReviewResult = await runReview(outputRoot, "slots.json", ["--write-schemas"]);
  const authApprovalDryRun = await runReview(outputRoot, "api-v2-auth.json", ["--approve-all-captured", "--dry-run"]);
  const bookingApprovalDryRun = await runReview(outputRoot, "booking-lifecycle.json", ["--approve-all-captured", "--dry-run"]);
  const slotsApprovalDryRun = await runReview(outputRoot, "slots.json", ["--approve-all-captured", "--dry-run"]);
  const secretScanResult = await runSecretScan(outputRoot);
  await close(server);
  server = createMockApiV2Server();
  const replayBaseUrl = await listen(server);
  const authReplayResult = await runReplay(replayBaseUrl, outputRoot, "api-v2-auth.json");
  const bookingReplayResult = await runReplay(replayBaseUrl, outputRoot, "booking-lifecycle.json");
  const slotsReplayResult = await runReplay(replayBaseUrl, outputRoot, "slots.json");
  const results = [
    authResult,
    bookingResult,
    slotsResult,
    authReviewResult,
    bookingReviewResult,
    slotsReviewResult,
    authApprovalDryRun,
    bookingApprovalDryRun,
    slotsApprovalDryRun,
    secretScanResult,
    authReplayResult,
    bookingReplayResult,
    slotsReplayResult,
  ];
  const combinedStdout = results.map((result) => result.stdout).join("");
  const combinedStderr = results.map((result) => result.stderr).join("");
  const failedStatus = results.find((result) => result.status !== 0)?.status ?? 0;

  if (failedStatus !== 0) {
    process.stdout.write(combinedStdout);
    process.stderr.write(combinedStderr);
    throw new Error(`fixture smoke command exited with status ${failedStatus}`);
  }

  assertCaptured(outputRoot, "api-v2-auth", "api-v2-auth.api-key.success", 200);
  assertCaptured(outputRoot, "api-v2-auth", "api-v2-auth.api-key.invalid", 401);
  assertCaptured(outputRoot, "api-v2-auth", "api-v2-auth.oauth2-client-metadata.success", 200);
  assertCaptured(outputRoot, "api-v2-auth", "api-v2-auth.oauth2-client-metadata.not-found", 404);
  assertCaptured(outputRoot, "api-v2-auth", "api-v2-auth.platform-client.success", 200);
  assertCaptured(outputRoot, "api-v2-auth", "api-v2-auth.platform-client.invalid-secret", 401);

  assertCaptured(outputRoot, "booking-lifecycle", "booking.create.personal-basic", 201);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.create.duplicate-idempotency-key", 200);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.read.by-uid", 200);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.cancel.owner", 200);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.reschedule.owner", 200);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.create.unauthorized-user-denied", 403);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.create.secret-field-denied", 400);
  assertCaptured(outputRoot, "booking-lifecycle", "booking.create.unavailable-slot-denied", 400);
  assertCaptured(outputRoot, "slots", "slots.read.personal-basic", 200);

  console.log(combinedStdout.trim());
  console.log(`OK: fixture capture smoke test wrote redacted output to ${outputRoot}`);
} finally {
  if (server.listening) await close(server);
}
