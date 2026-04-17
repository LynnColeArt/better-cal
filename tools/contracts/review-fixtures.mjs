#!/usr/bin/env node
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);
const contractsRoot = path.join(projectRoot, "contracts");
const defaultFixtureFiles = {
  inputs: ["request.redacted.json"],
  outputs: ["response.json", "capture-metadata.json"],
  schemas: ["request.schema.json", "response.schema.json"],
};

function parseArgs(argv) {
  const args = {
    approveIds: [],
    approveAllCaptured: false,
    dryRun: false,
    fixtures: [],
    fixturesRoot: path.join(contractsRoot, "fixtures"),
    writeSchemas: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--approve") args.approveIds.push(argv[++index]);
    else if (arg === "--approve-all-captured") args.approveAllCaptured = true;
    else if (arg === "--dry-run") args.dryRun = true;
    else if (arg === "--fixture") args.fixtures.push(argv[++index]);
    else if (arg === "--fixtures-root") args.fixturesRoot = argv[++index];
    else if (arg === "--help") args.help = true;
    else if (arg === "--manifest") args.manifest = argv[++index];
    else if (arg === "--status") args.status = argv[++index];
    else if (arg === "--write-schemas") args.writeSchemas = true;
    else throw new Error(`Unknown argument: ${arg}`);
  }

  return args;
}

function printHelp() {
  console.log(`Usage:
  node review-fixtures.mjs --manifest <manifest.json> [--write-schemas] [--approve <fixture-id>]

Options:
  --manifest             Fixture set manifest path.
  --fixtures-root        Captured fixture root. Defaults to contracts/fixtures.
  --fixture              Fixture id to review. Can be repeated. Defaults to all fixtures.
  --write-schemas        Infer request.schema.json and response.schema.json from captured payloads.
  --approve              Promote one captured fixture in the manifest. Can be repeated.
  --approve-all-captured Promote every captured fixture in the selected set.
  --status               Approval status. Defaults to accepted, except security-break fixtures stay security-break.
  --dry-run              Report changes without writing schema files or the manifest.
`);
}

function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function writeJson(file, data) {
  writeFileSync(file, `${JSON.stringify(data, null, 2)}\n`);
}

function rel(file) {
  return path.relative(projectRoot, file);
}

function fixtureDir(root, setId, fixtureId) {
  return path.resolve(root, setId, fixtureId);
}

function secretFieldNames() {
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

function pathMatches(patterns, pathParts) {
  const key = pathParts.at(-1)?.toLowerCase();
  if (key && patterns.has(key)) return true;
  const candidates = pathCandidates(pathParts);
  return [...patterns].some((pattern) =>
    candidates.some((candidate) => pathMatchesPattern(candidate, pattern))
  );
}

function isRedactedValue(value) {
  return typeof value === "string" && (value === "<redacted>" || value.startsWith("<unstable:"));
}

function hasMaterialValue(value) {
  if (value === null || value === undefined) return false;
  if (typeof value === "string") return value.trim().length > 0 && !isRedactedValue(value);
  if (typeof value === "number" || typeof value === "boolean") return true;
  if (Array.isArray(value)) return value.some((item) => hasMaterialValue(item));
  if (typeof value === "object") return Object.values(value).some((item) => hasMaterialValue(item));
  return false;
}

function findSecretLeaks(value, secretNames, pathParts = []) {
  const leaks = [];
  const key = pathParts.at(-1)?.toLowerCase();
  if (key && secretNames.has(key) && hasMaterialValue(value)) leaks.push(pathParts.join("."));
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      leaks.push(...findSecretLeaks(item, secretNames, [...pathParts, String(index)]));
    });
  } else if (value && typeof value === "object") {
    for (const [childKey, childValue] of Object.entries(value)) {
      leaks.push(...findSecretLeaks(childValue, secretNames, [...pathParts, childKey]));
    }
  }
  return leaks;
}

function findRedactionLeaks(value, redactionPaths, pathParts = []) {
  const leaks = [];
  if (pathParts.length > 0 && pathMatches(redactionPaths, pathParts) && hasMaterialValue(value)) {
    leaks.push(pathParts.join("."));
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      leaks.push(...findRedactionLeaks(item, redactionPaths, [...pathParts, String(index)]));
    });
  } else if (value && typeof value === "object") {
    for (const [childKey, childValue] of Object.entries(value)) {
      leaks.push(...findRedactionLeaks(childValue, redactionPaths, [...pathParts, childKey]));
    }
  }
  return leaks;
}

function primitiveSchema(type) {
  return { type };
}

function inferValueSchema(value) {
  if (value === null) return primitiveSchema("null");
  if (Array.isArray(value)) {
    return {
      type: "array",
      items: value.length === 0 ? {} : mergeSchemas(value.map((item) => inferValueSchema(item))),
    };
  }
  if (typeof value === "object") {
    const keys = Object.keys(value).sort();
    return {
      type: "object",
      additionalProperties: true,
      required: keys,
      properties: Object.fromEntries(keys.map((key) => [key, inferValueSchema(value[key])])),
    };
  }
  if (Number.isInteger(value)) return primitiveSchema("integer");
  if (typeof value === "number") return primitiveSchema("number");
  if (typeof value === "boolean") return primitiveSchema("boolean");
  if (typeof value === "string") {
    if (value === "<redacted>") {
      return { type: "string", description: "Redacted by the fixture capture pipeline." };
    }
    if (value.startsWith("<unstable:") && value.endsWith(">")) {
      return { type: "string", description: "Normalized unstable value." };
    }
    return primitiveSchema("string");
  }
  return {};
}

function typeKey(schema) {
  return Array.isArray(schema.type) ? schema.type.join("|") : schema.type;
}

function mergeTypeValues(first, second) {
  return [...new Set([first, second].flat().filter(Boolean))].sort();
}

function mergeSchemas(schemas) {
  return schemas.reduce((current, next) => mergeSchema(current, next), {});
}

function mergeSchema(first, second) {
  if (Object.keys(first).length === 0) return second;
  if (Object.keys(second).length === 0) return first;
  if (typeKey(first) !== typeKey(second)) {
    return { type: mergeTypeValues(first.type, second.type) };
  }
  if (first.type === "object") {
    const keys = [...new Set([...Object.keys(first.properties ?? {}), ...Object.keys(second.properties ?? {})])].sort();
    const firstRequired = new Set(first.required ?? []);
    const secondRequired = new Set(second.required ?? []);
    const required = keys.filter((key) => firstRequired.has(key) && secondRequired.has(key));
    const properties = {};
    for (const key of keys) {
      if (first.properties?.[key] && second.properties?.[key]) {
        properties[key] = mergeSchema(first.properties[key], second.properties[key]);
      } else {
        properties[key] = first.properties?.[key] ?? second.properties?.[key];
      }
    }
    return { type: "object", additionalProperties: true, required, properties };
  }
  if (first.type === "array") {
    return { type: "array", items: mergeSchema(first.items ?? {}, second.items ?? {}) };
  }
  return first;
}

function rootSchema(value, title) {
  return {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    title,
    ...inferValueSchema(value),
  };
}

function readCapturedJson(file, result, label) {
  if (!existsSync(file)) {
    result.missing.push(label);
    return undefined;
  }
  try {
    return readJson(file);
  } catch (error) {
    result.problems.push(`${rel(file)}: invalid JSON: ${error.message}`);
    return undefined;
  }
}

function inspectFixture({ args, manifest, fixture, secretNames }) {
  const dir = fixtureDir(args.fixturesRoot, manifest.id, fixture.id);
  const result = {
    dir,
    fixture,
    generatedSchemas: undefined,
    hasSchemas: false,
    missing: [],
    problems: [],
    willHaveSchemas: false,
  };
  const requestFile = path.join(dir, "request.redacted.json");
  const responseFile = path.join(dir, "response.json");
  const metadataFile = path.join(dir, "capture-metadata.json");

  const request = readCapturedJson(requestFile, result, "request.redacted.json");
  const response = readCapturedJson(responseFile, result, "response.json");
  const metadata = readCapturedJson(metadataFile, result, "capture-metadata.json");
  result.captured = result.missing.length === 0 && result.problems.length === 0;

  if (request !== undefined && response !== undefined) {
    const redactionPaths = new Set((fixture.redactions ?? []).map((item) => item.toLowerCase()));
    const secretLeaks = [
      ...findSecretLeaks(request, secretNames),
      ...findSecretLeaks(response, secretNames),
    ];
    const redactionLeaks = [
      ...findRedactionLeaks(request, redactionPaths),
      ...findRedactionLeaks(response, redactionPaths),
    ];
    for (const leak of secretLeaks) result.problems.push(`${fixture.id}: unredacted secret field at ${leak}`);
    for (const leak of redactionLeaks) result.problems.push(`${fixture.id}: unredacted fixture redaction at ${leak}`);
  }

  if (metadata !== undefined) {
    if (metadata.fixtureId !== fixture.id) {
      result.problems.push(`${fixture.id}: metadata fixtureId is ${metadata.fixtureId}`);
    }
    if (metadata.manifestId !== manifest.id) {
      result.problems.push(`${fixture.id}: metadata manifestId is ${metadata.manifestId}`);
    }
    if (
      response?.status !== undefined &&
      Array.isArray(metadata.expectStatus) &&
      metadata.expectStatus.length > 0 &&
      !metadata.expectStatus.includes(response.status)
    ) {
      result.problems.push(`${fixture.id}: response status ${response.status} is outside metadata expectStatus`);
    }
  }

  result.hasSchemas = defaultFixtureFiles.schemas.every((file) => existsSync(path.join(dir, file)));
  if (args.writeSchemas && result.captured && request !== undefined && response !== undefined) {
    result.generatedSchemas = {
      "request.schema.json": rootSchema(request, `${fixture.id} request`),
      "response.schema.json": rootSchema(response, `${fixture.id} response`),
    };
  }
  result.willHaveSchemas = result.hasSchemas || Boolean(result.generatedSchemas);
  return result;
}

function approvalStatus(fixture, requestedStatus) {
  if (requestedStatus) return requestedStatus;
  return fixture.status === "security-break" || fixture.comparison === "security-break" ? "security-break" : "accepted";
}

function reportResult(result) {
  if (result.captured) {
    const schemaText = result.willHaveSchemas ? "schemas ready" : "schemas missing";
    console.log(`${result.fixture.id}: captured, ${schemaText}`);
    return;
  }
  console.log(`${result.fixture.id}: missing ${result.missing.join(", ")}`);
}

const args = parseArgs(process.argv.slice(2));
if (args.help) {
  printHelp();
  process.exit(0);
}
if (!args.manifest) throw new Error("--manifest is required");
if (args.status && !["accepted", "amended", "security-break"].includes(args.status)) {
  throw new Error("--status must be accepted, amended, or security-break");
}

const manifestPath = path.resolve(args.manifest);
const manifest = readJson(manifestPath);
const selected = args.fixtures.length > 0 ? new Set(args.fixtures) : null;
const approveIds = new Set(args.approveIds);
const fixtureById = new Map((manifest.fixtures ?? []).map((fixture) => [fixture.id, fixture]));
const unknownFixtures = [...(selected ?? [])].filter((fixtureId) => !fixtureById.has(fixtureId));
const unknownApprovals = [...approveIds].filter((fixtureId) => !fixtureById.has(fixtureId));
if (unknownFixtures.length > 0) throw new Error(`Unknown fixture id(s): ${unknownFixtures.join(", ")}`);
if (unknownApprovals.length > 0) throw new Error(`Unknown approval fixture id(s): ${unknownApprovals.join(", ")}`);

const fixtures = manifest.fixtures.filter((fixture) => !selected || selected.has(fixture.id));
const secretNames = secretFieldNames();
const results = fixtures.map((fixture) => inspectFixture({ args, manifest, fixture, secretNames }));

for (const result of results) reportResult(result);

const problems = results.flatMap((result) => result.problems);
const approvalProblems = [];
const approvalResults = results.filter(
  (result) => approveIds.has(result.fixture.id) || (args.approveAllCaptured && result.captured)
);

for (const result of approvalResults) {
  if (!result.captured) approvalProblems.push(`${result.fixture.id}: cannot approve until capture files exist`);
  if (!result.willHaveSchemas) approvalProblems.push(`${result.fixture.id}: cannot approve until schema snapshots exist`);
}

if (problems.length > 0 || approvalProblems.length > 0) {
  console.error([...problems, ...approvalProblems].join("\n"));
  process.exit(1);
}

if (args.writeSchemas) {
  for (const result of results) {
    if (!result.generatedSchemas || args.dryRun) continue;
    for (const [file, schema] of Object.entries(result.generatedSchemas)) {
      writeJson(path.join(result.dir, file), schema);
    }
  }
}

let manifestChanged = false;
for (const result of approvalResults) {
  result.fixture.status = approvalStatus(result.fixture, args.status);
  result.fixture.inputs = [...defaultFixtureFiles.inputs];
  result.fixture.outputs = [...defaultFixtureFiles.outputs];
  result.fixture.schemas = [...defaultFixtureFiles.schemas];
  manifestChanged = true;
  console.log(`${result.fixture.id}: approved as ${result.fixture.status}`);
}

if (manifestChanged) {
  if (args.dryRun) console.log(`DRY RUN: manifest would be updated at ${rel(manifestPath)}`);
  else writeJson(manifestPath, manifest);
}

if (args.writeSchemas && args.dryRun) console.log("DRY RUN: schema snapshots were not written");
