import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import cors from "cors";
import express from "express";
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
  if (!existsSync(envPath)) return;

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
}

loadLocalEnv();

const app = express();
const provider = createLabProvider();
const port = Number(process.env.PORT || 3001);
const host = process.env.HOST || "0.0.0.0";
const allowedOrigins = (process.env.CORS_ORIGIN || "http://localhost:5173,http://127.0.0.1:5173")
  .split(",")
  .map((origin) => origin.trim());

app.use(express.json());
app.use(cors({
  origin(origin, callback) {
    if (!origin || allowedOrigins.includes("*") || allowedOrigins.includes(origin)) {
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
  const user = authenticateUser(req.body?.username, req.body?.password);
  if (!user) {
    res.status(401).json({ error: "Invalid username or password" });
    return;
  }

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
  const deployment = provider.deployLab(req.user, req.body?.labId);
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

app.delete("/api/deployments/:deploymentId", currentUser, (req, res) => {
  const deployment = provider.destroyDeployment(req.user, req.params.deploymentId);
  if (!deployment) {
    res.status(404).json({ error: "Deployment not found" });
    return;
  }
  res.json({ deployment });
});

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

app.get("/api/admin/openstack/health", currentUser, asyncRoute(async (req, res) => {
  if (req.user.role !== "admin") {
    res.status(403).json({ error: "Admin role required" });
    return;
  }
  const health = provider.getHealth
    ? await provider.getHealth()
    : { status: process.env.LAB_PROVIDER || "mock", reachable: true };
  res.json(health);
}));

app.use((error, _req, res, _next) => {
  const status = error.status || 500;
  console.error(error);
  res.status(status).json({
    error: error.message || "Internal server error",
    openStackStatus: error.openStackStatus,
    details: error.body
  });
});

const server = app.listen(port, host, () => {
  console.log(`OpenStack Lab Portal API listening on http://${host}:${port}`);
});

server.on("error", (error) => {
  console.error(`OpenStack Lab Portal API failed to start: ${error.message}`);
  process.exit(1);
});
