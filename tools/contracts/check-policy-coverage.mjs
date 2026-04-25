#!/usr/bin/env node
import { readFileSync } from "node:fs";
import path from "node:path";

export const projectRoot = path.resolve(new URL("../..", import.meta.url).pathname);

function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function rel(file) {
  return path.relative(projectRoot, file);
}

function implementedRoutes(serverSource) {
  const routes = [];
  const routePattern = /s\.mux\.HandleFunc\("([A-Z]+)\s+([^"]+)",\s*s\.([A-Za-z0-9_]+)\)/g;
  for (const match of serverSource.matchAll(routePattern)) {
    routes.push({
      method: match[1],
      path: match[2],
      handler: match[3],
    });
  }
  return routes;
}

function policyConstants(policySource) {
  const constants = new Map();
  const constantPattern = /(\w+)\s+Policy\s+=\s+"([^"]+)"/g;
  for (const match of policySource.matchAll(constantPattern)) {
    constants.set(match[2], match[1]);
  }
  return constants;
}

function findFunctionBody(source, handlerName) {
  const signature = `func (s *Server) ${handlerName}(`;
  const signatureIndex = source.indexOf(signature);
  if (signatureIndex < 0) return null;

  const bodyStart = source.indexOf("{", signatureIndex);
  if (bodyStart < 0) return null;

  let depth = 0;
  for (let index = bodyStart; index < source.length; index += 1) {
    const char = source[index];
    if (char === "{") depth += 1;
    else if (char === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(bodyStart + 1, index);
    }
  }

  return null;
}

function routeKey(method, routePath) {
  return `${method} ${routePath}`;
}

function authModeOrder(mode) {
  return {
    none: 0,
    "api-key": 1,
    "oauth-access-token": 2,
    "platform-client-secret": 3,
    "oauth-client": 4,
    session: 5,
    "platform-access-token": 6,
    "public-booking-token": 7,
    "public-pkce-client": 8,
  }[mode] ?? 99;
}

function sortAuthModes(modes) {
  return [...modes].sort((left, right) => authModeOrder(left) - authModeOrder(right) || left.localeCompare(right));
}

function implementedAuthModes(handlerBody) {
  const modes = new Set();
  if (handlerBody.includes("authenticateAPIKeyOrOAuthAccessToken(")) {
    modes.add("api-key");
    modes.add("oauth-access-token");
  } else if (handlerBody.includes("authenticateAPIKey(")) {
    modes.add("api-key");
  }
  if (handlerBody.includes("VerifyPlatformClientContext(")) {
    modes.add("platform-client-secret");
  }
  if (handlerBody.includes("OAuthClientContext(") && handlerBody.includes("PolicyOAuth2TokenExchange")) {
    modes.add("oauth-client");
  }
  return sortAuthModes(modes);
}

function isPublicPolicy(policy) {
  return policy && Array.isArray(policy.authModes) && policy.authModes.length === 1 && policy.authModes[0] === "none";
}

const implementedContractModes = new Set(["api-key", "oauth-access-token", "platform-client-secret", "oauth-client"]);

function coverageContext() {
  const routesPath = path.join(projectRoot, "contracts", "registries", "routes.json");
  const policiesPath = path.join(projectRoot, "contracts", "registries", "policies.json");
  const serverPath = path.join(projectRoot, "backend", "internal", "httpapi", "server.go");
  const handlersPath = path.join(projectRoot, "backend", "internal", "httpapi", "handlers.go");
  const policySourcePath = path.join(projectRoot, "backend", "internal", "authz", "policy.go");

  const routeRegistry = readJson(routesPath).routes ?? [];
  const policyRegistry = readJson(policiesPath).policies ?? [];
  const serverSource = readFileSync(serverPath, "utf8");
  const handlersSource = readFileSync(handlersPath, "utf8");
  const authzPolicySource = readFileSync(policySourcePath, "utf8");

  const registryByRoute = new Map(routeRegistry.map((route) => [routeKey(route.method, route.path), route]));
  const policiesByID = new Map(policyRegistry.map((policy) => [policy.id, policy]));
  const constantsByPolicyID = policyConstants(authzPolicySource);
  const problems = [];
  const rows = [];

  for (const route of implementedRoutes(serverSource)) {
    const key = routeKey(route.method, route.path);
    const registryRoute = registryByRoute.get(key);
    if (!registryRoute) {
      problems.push(`${rel(serverPath)}:${key}: implemented route is missing from contracts/registries/routes.json`);
      continue;
    }

    const policy = policiesByID.get(registryRoute.policy);
    if (!policy) {
      problems.push(`${rel(routesPath)}:${registryRoute.id}: policy ${registryRoute.policy} is missing from policies.json`);
      continue;
    }

    const handlerBody = findFunctionBody(handlersSource, route.handler);
    if (handlerBody === null) {
      problems.push(`${rel(handlersPath)}:${route.handler}: missing handler function for ${key}`);
      continue;
    }

    const implementedModes = implementedAuthModes(handlerBody);
    rows.push({
      route: key,
      id: registryRoute.id,
      handler: route.handler,
      policy: registryRoute.policy,
      policyAuthModes: sortAuthModes(policy.authModes ?? []),
      implementedAuthModes: implementedModes,
    });

    if (isPublicPolicy(policy)) {
      if (implementedModes.length > 0) {
        problems.push(
          `${rel(handlersPath)}:${route.handler}: ${key} is registered as public but implements auth modes ${implementedModes.join(", ")}`
        );
      }
      if (handlerBody.includes("s.authorize(")) {
        problems.push(`${rel(handlersPath)}:${route.handler}: ${key} is registered as public but performs policy authorization`);
      }
      continue;
    }

    const policyConstant = constantsByPolicyID.get(registryRoute.policy);
    if (!policyConstant) {
      problems.push(`${rel(policySourcePath)}:${registryRoute.policy}: missing authz policy constant`);
      continue;
    }

    const policyReference = `authz.${policyConstant}`;
    if (!handlerBody.includes("s.authorize(") || !handlerBody.includes(policyReference)) {
      problems.push(
        `${rel(handlersPath)}:${route.handler}: ${key} must enforce ${registryRoute.policy} with ${policyReference}`
      );
    }

    const policyAuthModes = new Set(policy.authModes ?? []);
    if (implementedModes.length === 0) {
      problems.push(`${rel(handlersPath)}:${route.handler}: ${key} has no recognized implemented auth mode`);
    }
    for (const mode of implementedModes) {
      if (!policyAuthModes.has(mode)) {
        problems.push(
          `${rel(handlersPath)}:${route.handler}: ${key} implements ${mode} but ${registryRoute.policy} authModes does not list it`
        );
      }
    }
    for (const mode of implementedContractModes) {
      if (policyAuthModes.has(mode) && !implementedModes.includes(mode)) {
        problems.push(
          `${rel(handlersPath)}:${route.handler}: ${registryRoute.policy} lists ${mode} but ${key} does not implement it`
        );
      }
    }
  }

  return { problems, rows };
}

export function checkPolicyCoverage() {
  return coverageContext().problems;
}

export function policyCoverageRows() {
  return coverageContext().rows;
}

export function formatPolicyCoverageReport(rows = policyCoverageRows()) {
  const header = "| Route | Handler | Policy | Registry auth modes | Implemented auth modes |";
  const separator = "| --- | --- | --- | --- | --- |";
  const lines = rows.map((row) => {
    const policyModes = row.policyAuthModes.length > 0 ? row.policyAuthModes.join(", ") : "(none)";
    const implementedModes = row.implementedAuthModes.length > 0 ? row.implementedAuthModes.join(", ") : "(none)";
    return `| \`${row.route}\` | \`${row.handler}\` | \`${row.policy}\` | ${policyModes} | ${implementedModes} |`;
  });
  return [header, separator, ...lines].join("\n");
}

export function runPolicyCoverageCli() {
  const { problems, rows } = coverageContext();
  const wantsReport = process.argv.includes("--report");
  if (wantsReport) {
    console.log(formatPolicyCoverageReport(rows));
  }
  if (problems.length > 0) {
    console.error(problems.join("\n"));
    process.exit(1);
  }
  if (!wantsReport) {
    console.log("OK: implemented backend routes have registry policies, handler-side policy checks, and auth-mode coverage");
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  runPolicyCoverageCli();
}
