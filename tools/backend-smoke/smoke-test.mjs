#!/usr/bin/env node
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import net from "node:net";
import path from "node:path";
import { spawn } from "node:child_process";
import { createMockApiV2Server } from "../fixture-capture/mock-api-v2-server.mjs";

const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);
const fixtureEnv = {
  CALDIY_API_KEY: "cal_test_valid_mock",
  CALDIY_OAUTH_CLIENT_ID: "mock-oauth-client",
  CALDIY_PLATFORM_CLIENT_ID: "mock-platform-client",
  CALDIY_PLATFORM_CLIENT_SECRET: "mock-platform-secret",
};

function listen(server) {
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") reject(new Error("Unexpected server address"));
      else resolve(`http://127.0.0.1:${address.port}`);
    });
  });
}

function close(server) {
  return new Promise((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
  });
}

function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        server.close();
        reject(new Error("Unexpected free-port address"));
        return;
      }
      const port = address.port;
      server.close((error) => (error ? reject(error) : resolve(port)));
    });
  });
}

function runTool(args, env = {}, cwd = projectRoot) {
  return runCommand(process.execPath, args, env, cwd);
}

function runCommand(command, args, env = {}, cwd = projectRoot) {
  return new Promise((resolve) => {
    const child = spawn(command, args, {
      cwd,
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
    fixtureEnv
  );
}

function runReview(outputRoot, manifestName) {
  return runTool([
    path.join(projectRoot, "tools", "contracts", "review-fixtures.mjs"),
    "--manifest",
    path.join(projectRoot, "contracts", "manifests", manifestName),
    "--fixtures-root",
    outputRoot,
    "--write-schemas",
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
    fixtureEnv
  );
}

function buildBackend(binaryPath) {
  return runCommand("go", ["build", "-o", binaryPath, "./cmd/api"], {}, path.join(projectRoot, "backend"));
}

function startGoBackend(port, binaryPath) {
  const child = spawn(binaryPath, [], {
    cwd: path.join(projectRoot, "backend"),
    env: {
      ...process.env,
      ...fixtureEnv,
      HOST: "127.0.0.1",
      PORT: String(port),
      CALDIY_REQUEST_ID: "mock-request-id",
    },
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
  return { child, logs: () => `${stdout}${stderr}` };
}

async function waitForHealth(baseUrl, backend) {
  for (let attempt = 0; attempt < 80; attempt += 1) {
    if (backend.child.exitCode !== null) {
      throw new Error(`Go backend exited before health check passed:\n${backend.logs()}`);
    }
    try {
      const response = await fetch(`${baseUrl}/health`);
      if (response.status === 200) return;
    } catch {
      // Retry until the server is ready.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`Timed out waiting for Go backend:\n${backend.logs()}`);
}

async function stopBackend(backend) {
  if (!backend || backend.child.exitCode !== null) return;
  await new Promise((resolve) => {
    const timer = setTimeout(() => {
      if (backend.child.exitCode === null) backend.child.kill("SIGKILL");
      resolve();
    }, 2000);
    backend.child.once("close", () => {
      clearTimeout(timer);
      resolve();
    });
    backend.child.kill("SIGTERM");
  });
}

function assertOK(result, label) {
  if (result.status === 0) return;
  process.stdout.write(result.stdout);
  process.stderr.write(result.stderr);
  throw new Error(`${label} exited with status ${result.status}`);
}

let mockServer;
let backend;
const outputRoot = mkdtempSync(path.join(tmpdir(), "better-cal-backend-smoke-"));

try {
  mockServer = createMockApiV2Server();
  const mockBaseUrl = await listen(mockServer);
  assertOK(await runCapture(mockBaseUrl, outputRoot, "api-v2-auth.json"), "auth capture");
  assertOK(await runCapture(mockBaseUrl, outputRoot, "booking-lifecycle.json"), "booking capture");
  assertOK(await runReview(outputRoot, "api-v2-auth.json"), "auth review");
  assertOK(await runReview(outputRoot, "booking-lifecycle.json"), "booking review");
  await close(mockServer);
  mockServer = undefined;

  const port = await freePort();
  const goBaseUrl = `http://127.0.0.1:${port}`;
  const backendBinary = path.join(outputRoot, "better-cal-api");
  assertOK(await buildBackend(backendBinary), "backend build");
  backend = startGoBackend(port, backendBinary);
  await waitForHealth(goBaseUrl, backend);

  const authReplay = await runReplay(goBaseUrl, outputRoot, "api-v2-auth.json");
  const bookingReplay = await runReplay(goBaseUrl, outputRoot, "booking-lifecycle.json");
  assertOK(authReplay, "auth replay");
  assertOK(bookingReplay, "booking replay");

  console.log(authReplay.stdout.trim());
  console.log(bookingReplay.stdout.trim());
  console.log(`OK: Go backend replay smoke test used ${outputRoot}`);
} finally {
  if (mockServer?.listening) await close(mockServer);
  await stopBackend(backend);
}
