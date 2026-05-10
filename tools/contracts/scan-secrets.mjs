#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

export const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);
const contractsRoot = path.join(projectRoot, "contracts");

const generatedFixtureFiles = new Set([
  "request.redacted.json",
  "response.json",
  "capture-metadata.json",
]);

const fixtureDenyLiterals = [
  "cal_test_valid_mock",
  "mock-platform-secret",
  "invalid-fixture-secret",
  "fixture-attendee@example.test",
  "unauthorized-fixture@example.test",
];

function parseArgs(argv) {
  const args = {
    denyLiterals: [],
    paths: [],
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--deny-literal") args.denyLiterals.push(argv[++index]);
    else if (arg === "--help") args.help = true;
    else if (arg === "--path") args.paths.push(argv[++index]);
    else throw new Error(`Unknown argument: ${arg}`);
  }

  return args;
}

function printHelp() {
  console.log(`Usage:
  node scan-secrets.mjs [--path <file-or-dir>] [--deny-literal <value>]

Options:
  --path          File or directory to scan. Defaults to generated fixture artifacts in contracts/fixtures.
  --deny-literal  Literal value that must never appear in scanned artifacts. Can be repeated.
`);
}

function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function rel(file) {
  return path.relative(projectRoot, file);
}

function walk(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) out.push(...walk(full));
    else out.push(full);
  }
  return out.sort();
}

function existingFiles(scanPaths) {
  const files = [];
  for (const item of scanPaths) {
    if (!existsSync(item)) continue;
    const stat = statSync(item);
    if (stat.isDirectory()) files.push(...walk(item));
    else files.push(item);
  }
  return files.sort();
}

function defaultScanFiles() {
  return existingFiles([path.join(contractsRoot, "fixtures")]).filter((file) =>
    generatedFixtureFiles.has(path.basename(file))
  );
}

function shouldScanFile(file, explicitPaths) {
  if (explicitPaths.length === 0) return generatedFixtureFiles.has(path.basename(file));
  if (generatedFixtureFiles.has(path.basename(file))) return true;
  return /\.(log|out|err|txt)$/i.test(file);
}

function secretFieldNames() {
  const registry = readJson(path.join(contractsRoot, "registries", "secrets.json"));
  return new Set(registry.secrets.flatMap((secret) => secret.fieldNames.map((name) => name.toLowerCase())));
}

function deniedLiterals(extraLiterals) {
  const values = [
    ...fixtureDenyLiterals,
    process.env.CALDIY_API_KEY,
    process.env.CALDIY_PLATFORM_CLIENT_SECRET,
    ...extraLiterals,
  ];
  return [...new Set(values.filter((value) => typeof value === "string" && value.length > 0))];
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function isAllowedRedaction(value) {
  return value === "<redacted>" || (typeof value === "string" && value.startsWith("<unstable:"));
}

function scanText(file, text, secretNames, literals) {
  const problems = [];
  for (const literal of literals) {
    if (text.includes(literal)) {
      problems.push(`${rel(file)}: denied literal ${JSON.stringify(literal)} found`);
    }
  }

  const keyPattern = [...secretNames].map(escapeRegExp).sort((a, b) => b.length - a.length).join("|");
  const assignmentPattern = new RegExp(
    `(^|[^A-Za-z0-9_-])["']?(${keyPattern})["']?\\s*[:=]\\s*["']?(?:\\[([^\\]]+)\\]|([^"',\\s}]+))`,
    "gi"
  );
  for (const match of text.matchAll(assignmentPattern)) {
    const value = match[3] ?? match[4];
    if (!isAllowedRedaction(value)) {
      problems.push(`${rel(file)}: secret-like assignment ${match[2]} is not redacted`);
    }
  }

  return problems;
}

function displayPath(pathParts) {
  return pathParts.length === 0 ? "$" : pathParts.join(".");
}

function scanJsonValue(file, value, secretNames, pathParts = []) {
  const problems = [];
  if (Array.isArray(value)) {
    for (let index = 0; index < value.length; index += 1) {
      problems.push(...scanJsonValue(file, value[index], secretNames, [...pathParts, String(index)]));
    }
    return problems;
  }
  if (!value || typeof value !== "object") return problems;

  for (const [key, childValue] of Object.entries(value)) {
    const childPath = [...pathParts, key];
    if (secretNames.has(key.toLowerCase()) && !isAllowedRedaction(childValue)) {
      problems.push(`${rel(file)}:${displayPath(childPath)}: secret-like field is not redacted`);
      continue;
    }
    problems.push(...scanJsonValue(file, childValue, secretNames, childPath));
  }
  return problems;
}

function scanFile(file, secretNames, literals) {
  const text = readFileSync(file, "utf8");
  const problems = scanText(file, text, secretNames, literals);

  if (file.endsWith(".json")) {
    try {
      problems.push(...scanJsonValue(file, JSON.parse(text), secretNames));
    } catch (error) {
      problems.push(`${rel(file)}: invalid JSON: ${error.message}`);
    }
  }

  return problems;
}

export function scanSecrets(options = {}) {
  const explicitPaths = (options.paths ?? []).map((item) => path.resolve(item));
  const files =
    explicitPaths.length === 0
      ? defaultScanFiles()
      : existingFiles(explicitPaths).filter((file) => shouldScanFile(file, explicitPaths));
  const secretNames = secretFieldNames();
  const literals = deniedLiterals(options.denyLiterals ?? []);
  const problems = [];

  for (const file of files) {
    problems.push(...scanFile(file, secretNames, literals));
  }

  return { files, problems };
}

export function runSecretScanCli(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  if (args.help) {
    printHelp();
    return;
  }

  const { files, problems } = scanSecrets({
    denyLiterals: args.denyLiterals,
    paths: args.paths,
  });

  if (problems.length > 0) {
    console.error(problems.join("\n"));
    process.exit(1);
  }

  console.log(`OK: secret scanner checked ${files.length} generated fixture/log artifact(s)`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  runSecretScanCli();
}
