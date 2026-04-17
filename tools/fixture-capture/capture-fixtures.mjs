#!/usr/bin/env node
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

export const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);
export const contractsRoot = path.join(projectRoot, "contracts");

function parseArgs(argv) {
  const args = {
    dryRun: false,
    fixtures: [],
    templateRoot: path.join(contractsRoot, "fixtures"),
    outputRoot: path.join(contractsRoot, "fixtures"),
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--dry-run") args.dryRun = true;
    else if (arg === "--manifest") args.manifest = argv[++index];
    else if (arg === "--base-url") args.baseUrl = argv[++index];
    else if (arg === "--fixture") args.fixtures.push(argv[++index]);
    else if (arg === "--template-root") args.templateRoot = argv[++index];
    else if (arg === "--output-root") args.outputRoot = argv[++index];
    else if (arg === "--help") args.help = true;
    else throw new Error(`Unknown argument: ${arg}`);
  }

  return args;
}

function printHelp() {
  console.log(`Usage:
  node capture-fixtures.mjs --manifest <manifest.json> --base-url <url> [--fixture <id>] [--dry-run]

Options:
  --manifest     Fixture set manifest path.
  --base-url     Server base URL, for example http://localhost:5555.
  --fixture      Fixture id to capture. Can be repeated. Defaults to all fixtures in the manifest.
  --template-root Request template root. Defaults to contracts/fixtures.
  --output-root  Fixture output root. Defaults to contracts/fixtures.
  --dry-run      Validate templates and environment variables without making HTTP requests.
`);
}

export function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

export function collectEnvPlaceholders(value, names = new Set()) {
  if (typeof value === "string") {
    for (const match of value.matchAll(/\$\{([A-Z0-9_]+)\}/g)) names.add(match[1]);
  } else if (Array.isArray(value)) {
    for (const item of value) collectEnvPlaceholders(item, names);
  } else if (value && typeof value === "object") {
    for (const item of Object.values(value)) collectEnvPlaceholders(item, names);
  }
  return names;
}

export function renderEnv(value) {
  if (typeof value === "string") {
    return value.replaceAll(/\$\{([A-Z0-9_]+)\}/g, (_match, name) => {
      const replacement = process.env[name];
      if (replacement === undefined) throw new Error(`Missing environment variable ${name}`);
      return replacement;
    });
  }
  if (Array.isArray(value)) return value.map((item) => renderEnv(item));
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, renderEnv(item)]));
  }
  return value;
}

function applyPathParams(pathTemplate, params = {}) {
  let out = pathTemplate;
  for (const [key, value] of Object.entries(params)) {
    out = out.replaceAll(`{${key}}`, encodeURIComponent(String(value)));
  }
  return out;
}

export function buildUrl(baseUrl, template) {
  const pathWithParams = applyPathParams(template.path, template.pathParams);
  const url = new URL(pathWithParams, baseUrl);
  for (const [key, value] of Object.entries(template.query ?? {})) {
    if (Array.isArray(value)) {
      for (const item of value) url.searchParams.append(key, String(item));
    } else if (value !== null && value !== undefined) {
      url.searchParams.set(key, String(value));
    }
  }
  return url;
}

export function headerObject(headers) {
  const ignoredHeaders = new Set(["connection", "date", "keep-alive", "transfer-encoding"]);
  const out = {};
  headers.forEach((value, key) => {
    if (ignoredHeaders.has(key.toLowerCase())) return;
    out[key] = value;
  });
  return out;
}

export function secretFieldNames() {
  const registry = readJson(path.join(contractsRoot, "registries", "secrets.json"));
  return new Set(registry.secrets.flatMap((secret) => secret.fieldNames.map((name) => name.toLowerCase())));
}

function pathCandidates(pathParts) {
  return [
    pathParts.join(".").toLowerCase(),
    pathParts.filter((part) => !/^\d+$/.test(part)).join(".").toLowerCase(),
  ];
}

function pathMatchesPattern(candidate, pattern) {
  return candidate === pattern || candidate.endsWith(`.${pattern}`);
}

function shouldRedactPath(pathParts, redactionPaths, secretNames) {
  const key = pathParts.at(-1)?.toLowerCase();
  if (key && (secretNames.has(key) || redactionPaths.has(key))) return true;
  const candidates = pathCandidates(pathParts);
  return [...redactionPaths].some((pattern) =>
    candidates.some((candidate) => pathMatchesPattern(candidate, pattern))
  );
}

export function redact(value, redactionPaths, secretNames, pathParts = []) {
  if (shouldRedactPath(pathParts, redactionPaths, secretNames)) return "<redacted>";
  if (Array.isArray(value)) return value.map((item, index) => redact(item, redactionPaths, secretNames, [...pathParts, String(index)]));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([childKey, childValue]) => [
        childKey,
        redact(childValue, redactionPaths, secretNames, [...pathParts, childKey]),
      ])
    );
  }
  return value;
}

export function normalize(value, unstableFields, pathParts = []) {
  const dotted = pathParts.join(".");
  const key = pathParts.at(-1);
  if (key && (unstableFields.has(key) || unstableFields.has(dotted))) return `<unstable:${key}>`;
  if (Array.isArray(value)) return value.map((item, index) => normalize(item, unstableFields, [...pathParts, String(index)]));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([childKey, childValue]) => [
        childKey,
        normalize(childValue, unstableFields, [...pathParts, childKey]),
      ])
    );
  }
  return value;
}

export async function parseResponseBody(response) {
  const text = await response.text();
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json") && text.length > 0) {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }
  return text;
}

export function fixtureDir(root, setId, fixtureId) {
  return path.resolve(root, setId, fixtureId);
}

export async function captureFixture({ args, manifest, fixture, secretNames }) {
  const sourceDir = fixtureDir(args.templateRoot, manifest.id, fixture.id);
  const outputDir = fixtureDir(args.outputRoot, manifest.id, fixture.id);
  const templatePath = path.join(sourceDir, "request.template.json");

  let template;
  try {
    template = readJson(templatePath);
  } catch {
    console.log(`${fixture.id}: missing request.template.json`);
    return { skipped: true };
  }

  const placeholders = [...collectEnvPlaceholders(template)].sort();
  const missingEnv = placeholders.filter((name) => process.env[name] === undefined);
  const captureMode = template.captureMode ?? "http";

  if (args.dryRun) {
    const missingText = missingEnv.length > 0 ? ` missing env: ${missingEnv.join(", ")}` : " env ok";
    console.log(`${fixture.id}: ${captureMode}${missingText}`);
    return { skipped: true };
  }

  if (captureMode === "manual") {
    console.log(`${fixture.id}: manual fixture skipped`);
    return { skipped: true };
  }

  if (missingEnv.length > 0) {
    throw new Error(`${fixture.id}: missing environment variables: ${missingEnv.join(", ")}`);
  }

  const rendered = renderEnv(template);
  const url = buildUrl(args.baseUrl, rendered);
  const body = rendered.body === undefined || rendered.body === null ? undefined : JSON.stringify(rendered.body);
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
  const expectedStatuses = rendered.expectStatus ?? [];
  if (expectedStatuses.length > 0 && !expectedStatuses.includes(response.status)) {
    throw new Error(
      `${fixture.id}: expected status ${expectedStatuses.join(" or ")}, got ${response.status}`
    );
  }

  const redactionPaths = new Set((fixture.redactions ?? []).map((item) => item.toLowerCase()));
  const unstableFields = new Set(fixture.unstableFields ?? []);

  const requestRecord = redact(
    {
      method: rendered.method,
      url: `${url.pathname}${url.search}`,
      headers,
      body: rendered.body ?? null,
    },
    redactionPaths,
    secretNames
  );

  const responseRecord = normalize(
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

  mkdirSync(outputDir, { recursive: true });
  writeFileSync(path.join(outputDir, "request.redacted.json"), `${JSON.stringify(requestRecord, null, 2)}\n`);
  writeFileSync(path.join(outputDir, "response.json"), `${JSON.stringify(responseRecord, null, 2)}\n`);
  writeFileSync(
    path.join(outputDir, "capture-metadata.json"),
    `${JSON.stringify(
      {
        fixtureId: fixture.id,
        manifestId: manifest.id,
        capturedAt: new Date().toISOString(),
        baseUrl: args.baseUrl,
        templatePath: path.relative(projectRoot, templatePath),
        expectStatus: rendered.expectStatus ?? [],
        comparison: fixture.comparison,
        securityBreaks: fixture.securityBreaks,
      },
      null,
      2
    )}\n`
  );

  console.log(`${fixture.id}: captured ${response.status}`);
  return { captured: true };
}

export async function runCaptureFixturesCli(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  if (args.help) {
    printHelp();
    return;
  }
  if (!args.manifest) throw new Error("--manifest is required");
  if (!args.baseUrl) throw new Error("--base-url is required");

  const manifest = readJson(path.resolve(args.manifest));
  const selected = args.fixtures.length > 0 ? new Set(args.fixtures) : null;
  const fixtures = manifest.fixtures.filter((fixture) => !selected || selected.has(fixture.id));
  if (selected && fixtures.length !== selected.size) {
    const found = new Set(fixtures.map((fixture) => fixture.id));
    const missing = [...selected].filter((fixtureId) => !found.has(fixtureId));
    throw new Error(`Unknown fixture id(s): ${missing.join(", ")}`);
  }

  const secretNames = secretFieldNames();
  for (const fixture of fixtures) {
    await captureFixture({ args, manifest, fixture, secretNames });
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await runCaptureFixturesCli();
}
