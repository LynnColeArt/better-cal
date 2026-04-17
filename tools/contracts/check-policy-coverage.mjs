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

function isPublicPolicy(policy) {
  return policy && Array.isArray(policy.authModes) && policy.authModes.length === 1 && policy.authModes[0] === "none";
}

export function checkPolicyCoverage() {
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

    if (isPublicPolicy(policy)) continue;

    const policyConstant = constantsByPolicyID.get(registryRoute.policy);
    if (!policyConstant) {
      problems.push(`${rel(policySourcePath)}:${registryRoute.policy}: missing authz policy constant`);
      continue;
    }

    const handlerBody = findFunctionBody(handlersSource, route.handler);
    if (handlerBody === null) {
      problems.push(`${rel(handlersPath)}:${route.handler}: missing handler function for ${key}`);
      continue;
    }

    const policyReference = `authz.${policyConstant}`;
    if (!handlerBody.includes("s.authorize(") || !handlerBody.includes(policyReference)) {
      problems.push(
        `${rel(handlersPath)}:${route.handler}: ${key} must enforce ${registryRoute.policy} with ${policyReference}`
      );
    }
  }

  return problems;
}

export function runPolicyCoverageCli() {
  const problems = checkPolicyCoverage();
  if (problems.length > 0) {
    console.error(problems.join("\n"));
    process.exit(1);
  }
  console.log("OK: implemented backend routes have registry policies and handler-side policy checks");
}

if (import.meta.url === `file://${process.argv[1]}`) {
  runPolicyCoverageCli();
}
