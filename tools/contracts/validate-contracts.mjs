#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { checkPolicyCoverage } from "./check-policy-coverage.mjs";
import { scanSecrets } from "./scan-secrets.mjs";

const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);
const contractsRoot = path.join(projectRoot, "contracts");
const specRoot = path.join(projectRoot, "docs", "spec");

function walk(dir, predicate) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) out.push(...walk(full, predicate));
    else if (predicate(full)) out.push(full);
  }
  return out.sort();
}

function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function rel(file) {
  return path.relative(projectRoot, file);
}

const problems = [];

const jsonFiles = walk(contractsRoot, (file) => file.endsWith(".json"));
const parsed = new Map();

for (const file of jsonFiles) {
  try {
    parsed.set(file, readJson(file));
  } catch (error) {
    problems.push(`${rel(file)}: invalid JSON: ${error.message}`);
  }
}

for (const [file, data] of parsed) {
  if (typeof data.$schema === "string" && !data.$schema.startsWith("http")) {
    const schemaPath = path.resolve(path.dirname(file), data.$schema);
    if (!existsSync(schemaPath)) problems.push(`${rel(file)}: missing $schema ${data.$schema}`);
  }
}

const routes = readJson(path.join(contractsRoot, "registries", "routes.json")).routes;
const policies = new Set(readJson(path.join(contractsRoot, "registries", "policies.json")).policies.map((p) => p.id));
const fixtures = new Set();

for (const manifestFile of walk(path.join(contractsRoot, "manifests"), (file) => file.endsWith(".json"))) {
  const manifest = readJson(manifestFile);
  for (const fixture of manifest.fixtures ?? []) fixtures.add(fixture.id);
}

for (const route of routes) {
  if (!policies.has(route.policy)) problems.push(`routes.json:${route.id}: missing policy ${route.policy}`);
  for (const fixtureRef of route.fixtureRefs) {
    if (!fixtures.has(fixtureRef)) problems.push(`routes.json:${route.id}: missing fixture ${fixtureRef}`);
  }
}

const sourcePathPattern = /(\.\.\/\.\.\/\.\.\/reference|reference\/|packages\/|apps\/)/;
for (const file of [...walk(contractsRoot, () => true), ...walk(specRoot, () => true)]) {
  if (sourcePathPattern.test(readFileSync(file, "utf8"))) {
    problems.push(`${rel(file)}: source path reference found in source-neutral area`);
  }
}

problems.push(...checkPolicyCoverage());
problems.push(...scanSecrets().problems);

if (problems.length > 0) {
  console.error(problems.join("\n"));
  process.exit(1);
}

console.log(
  `OK: ${jsonFiles.length} JSON files, schema refs, route refs, source-neutral checks, policy coverage, and secret scan passed`
);
