import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import cors from "cors";
import express from "express";
import { WebSocketServer } from "ws";
import { findDownloadableResource, listDownloadableResources } from "./catalog/resources.js";
import { isAllowedCorsOrigin, parseAllowedOrigins } from "./cors.js";
import {
  clearLoginFailures,
  getLoginRateLimitStatus,
  loginRateLimitKey,
  readLoginRateLimitConfig,
  recordFailedLogin
} from "./loginRateLimit.js";
import { createLabProvider } from "./providers/index.js";
import {
  authenticateUser,
  createSession,
  deleteSession,
  getPublicUsers,
  getSessionUser,
  publicUser,
  readAuthToken
} from "./users.js";

function loadLocalEnv() {
  const currentDir = dirname(fileURLToPath(import.meta.url));
  const envPath = resolve(currentDir, "../../.env");
  if (!existsSync(envPath)) {
    console.warn("[api] .env not found; using shell environment and defaults", { envPath });
    return;
  }

  for (const line of readFileSync(envPath, "utf8").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;

    const separatorIndex = trimmed.indexOf("=");
    if (separatorIndex === -1) continue;

    const key = trimmed.slice(0, separatorIndex).trim();
    let value = trimmed.slice(separatorIndex + 1).trim();
    if (
      (value.startsWith("\"") && value.endsWith("\""))
      || (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }

    process.env[key] = value;
  }
  console.info("[api] loaded local environment", { envPath });
}

loadLocalEnv();

const app = express();
const provider = createLabProvider();
const port = Number(process.env.PORT || 3001);
const host = process.env.HOST || "0.0.0.0";
const allowedOrigins = parseAllowedOrigins(process.env.CORS_ORIGIN);
const loginRateLimitConfig = readLoginRateLimitConfig();

app.use(express.json());
app.use((req, res, next) => {
  const startedAt = Date.now();
  res.on("finish", () => {
    const shouldLog = req.method !== "GET" || res.statusCode >= 400;
    if (!shouldLog) return;

    console.info("[api] request", {
      method: req.method,
      path: req.originalUrl,
      status: res.statusCode,
      durationMs: Date.now() - startedAt,
      user: req.user?.username
    });
  });
  next();
});
app.use(cors({
  origin(origin, callback) {
    if (isAllowedCorsOrigin(origin, { allowedOrigins })) {
      callback(null, true);
      return;
    }
    callback(new Error("CORS origin denied"));
  }
}));

function currentUser(req, _res, next) {
  const user = getSessionUser(readAuthToken(req));
  if (!user) {
    const error = new Error("Authentication required");
    error.status = 401;
    next(error);
    return;
  }

  req.user = publicUser(user);
  next();
}

function asyncRoute(handler) {
  return async (req, res, next) => {
    try {
      await handler(req, res, next);
    } catch (error) {
      next(error);
    }
  };
}

app.get("/api/health", (_req, res) => {
  res.json({
    ok: true,
    provider: process.env.LAB_PROVIDER || "mock",
    time: new Date().toISOString()
  });
});

app.post("/api/auth/login", (req, res) => {
  const rateLimitKey = loginRateLimitKey({
    ip: req.ip,
    username: req.body?.username
  });
  const rateLimitStatus = getLoginRateLimitStatus(rateLimitKey, loginRateLimitConfig);
  if (rateLimitStatus.limited) {
    res.set("Retry-After", String(rateLimitStatus.retryAfterSeconds));
    res.status(429).json({
      error: "Too many failed login attempts. Try again later.",
      retryAfterSeconds: rateLimitStatus.retryAfterSeconds
    });
    return;
  }

  const user = authenticateUser(req.body?.username, req.body?.password);
  if (!user) {
    const failedStatus = recordFailedLogin(rateLimitKey, loginRateLimitConfig);
    if (failedStatus.limited) {
      res.set("Retry-After", String(failedStatus.retryAfterSeconds));
      res.status(429).json({
        error: "Too many failed login attempts. Try again later.",
        retryAfterSeconds: failedStatus.retryAfterSeconds
      });
      return;
    }

    res.status(401).json({ error: "Invalid username or password" });
    return;
  }

  clearLoginFailures(rateLimitKey);
  res.json({ user: publicUser(user), token: createSession(user) });
});

app.post("/api/auth/logout", (req, res) => {
  deleteSession(readAuthToken(req));
  res.status(204).send();
});

app.get("/api/me", currentUser, (req, res) => {
  res.json({ user: req.user });
});

app.get("/api/labs", currentUser, (_req, res) => {
  res.json({ labs: provider.listLabs() });
});

app.get("/api/labs/:labId", currentUser, (req, res) => {
  const lab = provider.getLab(req.params.labId);
  if (!lab) {
    res.status(404).json({ error: "Lab not found" });
    return;
  }
  res.json({ lab });
});

app.post("/api/deployments", currentUser, asyncRoute((req, res) => {
  console.info("[api] deployment requested", {
    user: req.user.username,
    labId: req.body?.labId,
    provider: process.env.LAB_PROVIDER || "mock"
  });
  const deployment = provider.deployLab(req.user, req.body?.labId);
  console.info("[api] deployment accepted", {
    deploymentId: deployment.id,
    resourceName: deployment.resourceName,
    status: deployment.status,
    provider: deployment.provider
  });
  res.status(201).json({ deployment });
}));

app.get("/api/deployments", currentUser, (req, res) => {
  res.json({ deployments: provider.listDeployments(req.user) });
});

app.get("/api/deployments/:deploymentId", currentUser, (req, res) => {
  const deployment = provider.getDeployment(req.user, req.params.deploymentId);
  if (!deployment) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }
  res.json({ deployment });
});

app.post("/api/deployments/:deploymentId/console-session", currentUser, asyncRoute(async (req, res) => {
  if (!provider.createConsoleSession) {
    res.status(404).json({ error: "Console sessions are not supported by this provider" });
    return;
  }

  const session = await provider.createConsoleSession(req.user, req.params.deploymentId);
  if (!session) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }

  res.status(201).json({ session });
}));

app.get("/api/resources", currentUser, asyncRoute(async (_req, res) => {
  const summary = provider.getResourceSummary
    ? await provider.getResourceSummary()
    : { status: process.env.LAB_PROVIDER || "mock", source: "unavailable", resources: [], warnings: [] };
  res.json(summary);
}));

app.get("/api/downloads", currentUser, (_req, res) => {
  res.json({ resources: listDownloadableResources() });
});

app.get("/api/downloads/:resourceId", currentUser, (req, res) => {
  const resource = findDownloadableResource(req.params.resourceId);
  if (!resource || !resource.available) {
    res.status(404).json({ error: "Resource not found" });
    return;
  }

  res.download(resource.filePath, resource.fileName);
});

app.post("/api/deployments/:deploymentId/stop", currentUser, asyncRoute(async (req, res) => {
  const deployment = provider.stopDeployment
    ? await provider.stopDeployment(req.user, req.params.deploymentId)
    : provider.setLifecycleStatus(req.user, req.params.deploymentId, "stopped");
  if (!deployment) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }
  res.json({ deployment });
}));

app.post("/api/deployments/:deploymentId/start", currentUser, asyncRoute(async (req, res) => {
  const deployment = provider.startDeployment
    ? await provider.startDeployment(req.user, req.params.deploymentId)
    : provider.setLifecycleStatus(req.user, req.params.deploymentId, "ready");
  if (!deployment) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }
  res.json({ deployment });
}));

app.post("/api/deployments/:deploymentId/reset", currentUser, asyncRoute(async (req, res) => {
  const deployment = provider.resetDeployment
    ? await provider.resetDeployment(req.user, req.params.deploymentId)
    : provider.setLifecycleStatus(req.user, req.params.deploymentId, "resetting");
  if (!deployment) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }
  if (!provider.resetDeployment) {
    setTimeout(() => provider.setLifecycleStatus(req.user, req.params.deploymentId, "ready"), 1200);
  }
  res.json({ deployment });
}));

app.delete("/api/deployments/:deploymentId", currentUser, asyncRoute(async (req, res) => {
  const deployment = await provider.destroyDeployment(req.user, req.params.deploymentId);
  if (!deployment) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }
  res.json({ deployment });
}));

app.get("/api/admin/deployments", currentUser, (req, res) => {
  if (req.user.role !== "admin") {
    res.status(403).json({ error: "Admin role required" });
    return;
  }
  res.json({ deployments: provider.listDeployments(req.user) });
});

app.get("/api/admin/users", currentUser, (req, res) => {
  if (req.user.role !== "admin") {
    res.status(403).json({ error: "Admin role required" });
    return;
  }
  res.json({ users: getPublicUsers() });
});

app.use((error, _req, res, _next) => {
  const status = error.status || 500;
  console.error(error);
  res.status(status).json({
    error: error.message || "Internal server error",
    details: error.body
  });
});

const server = app.listen(port, host, () => {
  const providerName = process.env.LAB_PROVIDER || "mock";
  console.log(`Proxmox Lab Portal API listening on http://${host}:${port}`);
  console.info("[api] runtime configuration", {
    provider: providerName,
    corsOrigins: process.env.CORS_ORIGIN || "development defaults"
  });
  if (providerName === "mock") {
    console.warn("[api] LAB_PROVIDER=mock is simulation mode; no Proxmox VMs will be created");
  }
});

const consoleWebSocketServer = new WebSocketServer({ noServer: true });

server.on("upgrade", (req, socket, head) => {
  const url = new URL(req.url || "/", "http://127.0.0.1");
  const match = /^\/api\/proxmox\/console-sessions\/([^/]+)\/ws$/.exec(url.pathname);
  if (!match || !provider.proxyConsoleWebSocket) {
    socket.destroy();
    return;
  }

  consoleWebSocketServer.handleUpgrade(req, socket, head, (webSocket) => {
    provider.proxyConsoleWebSocket(match[1], webSocket, req);
  });
});

server.on("error", (error) => {
  console.error(`Proxmox Lab Portal API failed to start: ${error.message}`);
  process.exit(1);
});
