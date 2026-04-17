#!/usr/bin/env node
import path from "node:path";
import {
  buildUrl,
  collectEnvPlaceholders,
  contractsRoot,
  fixtureDir,
  headerObject,
  normalize,
  parseResponseBody,
  projectRoot,
  readJson,
  redact,
  renderEnv,
  secretFieldNames,
} from "../fixture-capture/capture-fixtures.mjs";

function parseArgs(argv) {
  const args = {
    fixtures: [],
    fixturesRoot: path.join(contractsRoot, "fixtures"),
    includeNeedsCapture: false,
    includeSecurityBreak: false,
    maxDiffs: 20,
    templateRoot: path.join(contractsRoot, "fixtures"),
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--base-url") args.baseUrl = argv[++index];
    else if (arg === "--fixture") args.fixtures.push(argv[++index]);
    else if (arg === "--fixtures-root") args.fixturesRoot = argv[++index];
    else if (arg === "--help") args.help = true;
    else if (arg === "--include-needs-capture") args.includeNeedsCapture = true;
    else if (arg === "--include-security-break") args.includeSecurityBreak = true;
    else if (arg === "--manifest") args.manifest = argv[++index];
    else if (arg === "--max-diffs") args.maxDiffs = Number(argv[++index]);
    else if (arg === "--template-root") args.templateRoot = argv[++index];
    else throw new Error(`Unknown argument: ${arg}`);
  }

  return args;
}

function printHelp() {
  console.log(`Usage:
  node replay-fixtures.mjs --manifest <manifest.json> --base-url <url>

Options:
  --manifest               Fixture set manifest path.
  --base-url               Replacement server base URL, for example http://localhost:8080.
  --fixture                Fixture id to replay. Can be repeated. Defaults to accepted fixtures.
  --fixtures-root          Expected fixture root. Defaults to contracts/fixtures.
  --template-root          Request template root. Defaults to contracts/fixtures.
  --include-needs-capture  Replay captured draft fixtures. Intended for local smoke tests only.
  --include-security-break Replay security-break fixtures with captured expected payloads.
  --max-diffs              Maximum mismatch lines per fixture. Defaults to 20.
`);
}

function hasStatus(args, fixture) {
  if (fixture.status === "accepted") return true;
  if (args.includeNeedsCapture && fixture.status === "needs-capture") return true;
  return args.includeSecurityBreak && fixture.status === "security-break";
}

function selectedFixtures(args, manifest) {
  const selected = args.fixtures.length > 0 ? new Set(args.fixtures) : null;
  const fixtureById = new Map((manifest.fixtures ?? []).map((fixture) => [fixture.id, fixture]));
  const unknown = [...(selected ?? [])].filter((fixtureId) => !fixtureById.has(fixtureId));
  if (unknown.length > 0) throw new Error(`Unknown fixture id(s): ${unknown.join(", ")}`);
  return manifest.fixtures.filter((fixture) => (selected ? selected.has(fixture.id) : hasStatus(args, fixture)));
}

function display(value) {
  const text = JSON.stringify(value);
  if (text === undefined) return String(value);
  return text.length > 160 ? `${text.slice(0, 157)}...` : text;
}

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function diffJson(expected, actual, pathParts = ["$"], diffs = []) {
  if (Object.is(expected, actual)) return diffs;
  if (Array.isArray(expected) || Array.isArray(actual)) {
    if (!Array.isArray(expected) || !Array.isArray(actual)) {
      diffs.push(`${pathParts.join(".")}: expected ${display(expected)}, got ${display(actual)}`);
      return diffs;
    }
    if (expected.length !== actual.length) {
      diffs.push(`${pathParts.join(".")}: expected array length ${expected.length}, got ${actual.length}`);
    }
    const max = Math.max(expected.length, actual.length);
    for (let index = 0; index < max; index += 1) {
      if (!(index in expected)) diffs.push(`${pathParts.join(".")}.${index}: unexpected ${display(actual[index])}`);
      else if (!(index in actual)) diffs.push(`${pathParts.join(".")}.${index}: missing ${display(expected[index])}`);
      else diffJson(expected[index], actual[index], [...pathParts, String(index)], diffs);
    }
    return diffs;
  }
  if (isObject(expected) || isObject(actual)) {
    if (!isObject(expected) || !isObject(actual)) {
      diffs.push(`${pathParts.join(".")}: expected ${display(expected)}, got ${display(actual)}`);
      return diffs;
    }
    const keys = [...new Set([...Object.keys(expected), ...Object.keys(actual)])].sort();
    for (const key of keys) {
      if (!(key in expected)) diffs.push(`${pathParts.join(".")}.${key}: unexpected ${display(actual[key])}`);
      else if (!(key in actual)) diffs.push(`${pathParts.join(".")}.${key}: missing ${display(expected[key])}`);
      else diffJson(expected[key], actual[key], [...pathParts, key], diffs);
    }
    return diffs;
  }
  diffs.push(`${pathParts.join(".")}: expected ${display(expected)}, got ${display(actual)}`);
  return diffs;
}

function requestBody(rendered) {
  return rendered.body === undefined || rendered.body === null ? undefined : JSON.stringify(rendered.body);
}

async function replayFixture({ args, manifest, fixture, secretNames }) {
  const sourceDir = fixtureDir(args.templateRoot, manifest.id, fixture.id);
  const expectedDir = fixtureDir(args.fixturesRoot, manifest.id, fixture.id);
  const templatePath = path.join(sourceDir, "request.template.json");
  const expectedPath = path.join(expectedDir, "response.json");

  let template;
  try {
    template = readJson(templatePath);
  } catch {
    return { fixture, skipped: true, message: "missing request.template.json" };
  }
  if ((template.captureMode ?? "http") === "manual") {
    return { fixture, skipped: true, message: "manual fixture skipped" };
  }

  let expected;
  try {
    expected = readJson(expectedPath);
  } catch {
    return { fixture, failed: true, diffs: [`missing expected response.json at ${path.relative(projectRoot, expectedPath)}`] };
  }

  const missingEnv = [...collectEnvPlaceholders(template)].filter((name) => process.env[name] === undefined);
  if (missingEnv.length > 0) {
    return { fixture, failed: true, diffs: [`missing environment variables: ${missingEnv.join(", ")}`] };
  }

  const rendered = renderEnv(template);
  const url = buildUrl(args.baseUrl, rendered);
  const body = requestBody(rendered);
  const headers = { ...(rendered.headers ?? {}) };
  if (body && !Object.keys(headers).some((key) => key.toLowerCase() === "content-type")) {
    headers["content-type"] = "application/json";
  }

  const response = await fetch(url, {
    method: rendered.method,
    headers,
    body,
  });
  const responseBody = await parseResponseBody(response);
  const redactionPaths = new Set((fixture.redactions ?? []).map((item) => item.toLowerCase()));
  const unstableFields = new Set(fixture.unstableFields ?? []);
  const actual = normalize(
    redact(
      {
        status: response.status,
        statusText: response.statusText,
        headers: headerObject(response.headers),
        body: responseBody,
      },
      redactionPaths,
      secretNames
    ),
    unstableFields
  );

  const diffs = diffJson(expected, actual).slice(0, args.maxDiffs);
  return { fixture, failed: diffs.length > 0, diffs };
}

export async function runReplayFixturesCli(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  if (args.help) {
    printHelp();
    return;
  }
  if (!args.manifest) throw new Error("--manifest is required");
  if (!args.baseUrl) throw new Error("--base-url is required");
  if (!Number.isInteger(args.maxDiffs) || args.maxDiffs < 1) throw new Error("--max-diffs must be a positive integer");

  const manifest = readJson(path.resolve(args.manifest));
  const fixtures = selectedFixtures(args, manifest);
  if (fixtures.length === 0) {
    console.log(`${manifest.id}: no replayable fixtures selected`);
    return;
  }

  const secretNames = secretFieldNames();
  const results = [];
  for (const fixture of fixtures) {
    results.push(await replayFixture({ args, manifest, fixture, secretNames }));
  }

  const failures = [];
  for (const result of results) {
    if (result.skipped) {
      console.log(`${result.fixture.id}: ${result.message}`);
    } else if (result.failed) {
      console.log(`${result.fixture.id}: mismatch`);
      for (const diff of result.diffs) console.log(`  ${diff}`);
      failures.push(result.fixture.id);
    } else {
      console.log(`${result.fixture.id}: ok`);
    }
  }

  if (failures.length > 0) {
    throw new Error(`fixture replay failed for ${failures.join(", ")}`);
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await runReplayFixturesCli();
}
