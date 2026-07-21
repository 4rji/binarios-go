import assert from "node:assert/strict";
import { existsSync, mkdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { isAllowedCorsOrigin, parseAllowedOrigins } from "../src/cors.js";
import { listDownloadableResources } from "../src/catalog/resources.js";
import {
  clearLoginFailures,
  getLoginRateLimitStatus,
  loginRateLimitKey,
  recordFailedLogin,
  resetLoginRateLimits
} from "../src/loginRateLimit.js";
import { MockLabProvider } from "../src/providers/mockLabProvider.js";
import { ProxmoxClient, ProxmoxLabProvider } from "../src/providers/proxmoxLabProvider.js";
import { authenticateUser, getPublicUsers, publicUser } from "../src/users.js";

const user = { id: "user-havi", username: "havi", role: "student" };
const testStateDir = resolve(tmpdir(), `lab-portal-provider-tests-${process.pid}`);
mkdirSync(testStateDir, { recursive: true });

async function test(name, fn) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    console.error(`not ok - ${name}`);
    throw error;
  }
}

function jsonResponse(payload, { status = 200, headers = {} } = {}) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json",
      ...headers
    }
  });
}

function createProxmoxFetchRecorder() {
  const requests = [];

  return {
    requests,
    fetchImpl: async (url, options = {}) => {
      const parsedUrl = new URL(String(url));
      const body = options.body
        ? Object.fromEntries(new URLSearchParams(String(options.body)))
        : null;
      const request = {
        url: String(url),
        path: parsedUrl.pathname,
        method: options.method || "GET",
        headers: options.headers || {},
        body
      };
      requests.push(request);

      if (request.path.endsWith("/api2/json/cluster/nextid")) {
        return jsonResponse({ data: "101" });
      }

      if (request.path.endsWith("/api2/json/version")) {
        return jsonResponse({ data: { version: "8.2.0" } });
      }

      if (request.path.endsWith("/api2/json/nodes/pve/status")) {
        return jsonResponse({
          data: {
            cpu: 0.25,
            cpuinfo: { cpus: 16 },
            memory: { total: 68719476736, used: 17179869184 },
            rootfs: { total: 1073741824000, used: 268435456000 }
          }
        });
      }

      if (request.path.endsWith("/api2/json/nodes/pve/qemu/9000/clone")) {
        return jsonResponse({ data: "UPID:pve:clone" });
      }

      if (request.path.endsWith("/api2/json/nodes/pve/qemu/101/config")) {
        return jsonResponse({ data: null });
      }

      if (request.path.endsWith("/api2/json/nodes/pve/qemu/101/status/start")) {
        return jsonResponse({ data: "UPID:pve:start" });
      }

      if (request.path.endsWith("/api2/json/nodes/pve/qemu/101/status/current")) {
        return jsonResponse({ data: { vmid: 101, name: "ecom-1-havi", status: "running" } });
      }

      if (request.path.includes("/api2/json/nodes/pve/tasks/") && request.path.endsWith("/status")) {
        return jsonResponse({ data: { status: "stopped", exitstatus: "OK" } });
      }

      return jsonResponse({ error: `unhandled ${request.method} ${request.url}` }, { status: 404 });
    }
  };
}

await test("mock provider creates a queued deployment for an enabled lab", () => {
  const provider = new MockLabProvider();
  const deployment = provider.deployLab(user, "ccdc-wkst-ubuntu-24");

  assert.equal(deployment.status, "queued");
  assert.equal(deployment.lab.id, "ccdc-wkst-ubuntu-24");
  assert.match(deployment.resourceName, /^lab-havi-ccdc-wkst-ubuntu-24-/);
  assert.equal(deployment.lab.defaultTtlMinutes, 120);
});

await test("mock provider hides another user's deployment", () => {
  const provider = new MockLabProvider();
  const deployment = provider.deployLab(user, "ccdc-wkst-ubuntu-24");
  const otherUser = { id: "user-other", username: "other", role: "student" };

  assert.equal(provider.getDeployment(otherUser, deployment.id), null);
});

await test("mock provider expires overdue deployments", () => {
  const provider = new MockLabProvider();
  const deployment = provider.deployLab(user, "ccdc-wkst-ubuntu-24");
  provider.deployments.set(deployment.id, {
    ...deployment,
    expiresAt: new Date(Date.now() - 1000).toISOString()
  });

  const deployments = provider.listDeployments(user);
  assert.equal(deployments.find((candidate) => candidate.id === deployment.id)?.status, "deleting");
});

await test("dev auth accepts configured student credentials", () => {
  const authenticated = authenticateUser("havi", "metro123");

  assert.deepEqual(publicUser(authenticated), {
    id: "user-havi",
    username: "havi",
    role: "student"
  });
});

await test("dev auth rejects invalid credentials", () => {
  assert.equal(authenticateUser("havi", "wrong-password"), null);
  assert.equal(authenticateUser("missing-user", "metro123"), null);
  assert.equal(authenticateUser("demo", "demo"), null);
  assert.equal(authenticateUser("admin", "admin"), null);
});

await test("login rate limit locks repeated failures for one ip and username", () => {
  resetLoginRateLimits();
  const config = { maxAttempts: 3, lockoutMs: 60_000, windowMs: 300_000 };
  const key = loginRateLimitKey({ ip: "192.0.2.10", username: "Havi " });

  assert.equal(recordFailedLogin(key, { ...config, now: 1_000 }).limited, false);
  assert.equal(recordFailedLogin(key, { ...config, now: 2_000 }).limited, false);
  const locked = recordFailedLogin(key, { ...config, now: 3_000 });

  assert.equal(locked.limited, true);
  assert.equal(locked.retryAfterSeconds, 60);
  assert.deepEqual(getLoginRateLimitStatus(key, { ...config, now: 4_000 }), {
    limited: true,
    retryAfterSeconds: 59
  });
});

await test("login rate limit resets after successful login or expired timeout", () => {
  resetLoginRateLimits();
  const config = { maxAttempts: 2, lockoutMs: 30_000, windowMs: 300_000 };
  const key = loginRateLimitKey({ ip: "192.0.2.20", username: "havi" });

  assert.equal(recordFailedLogin(key, { ...config, now: 1_000 }).limited, false);
  clearLoginFailures(key);
  assert.equal(getLoginRateLimitStatus(key, { ...config, now: 2_000 }).limited, false);

  assert.equal(recordFailedLogin(key, { ...config, now: 3_000 }).limited, false);
  assert.equal(recordFailedLogin(key, { ...config, now: 4_000 }).limited, true);
  assert.equal(getLoginRateLimitStatus(key, { ...config, now: 35_000 }).limited, false);
});

await test("public users never include passwords", () => {
  const users = getPublicUsers();
  const usernames = users.map((candidate) => candidate.username);

  assert.ok(usernames.includes("havi"));
  assert.ok(usernames.includes("connor"));
  assert.ok(users.every((candidate) => !("password" in candidate)));
});

await test("download catalog exposes the four Mastering books", () => {
  const resources = listDownloadableResources();
  const ids = resources.map((resource) => resource.id);

  assert.equal(resources.length, 4);
  assert.deepEqual(ids.sort(), [
    "mastering-linux-security-hardening-epub",
    "mastering-linux-security-hardening-pdf",
    "mastering-windows-security-hardening-epub",
    "mastering-windows-security-hardening-pdf"
  ]);
  assert.ok(resources.every((resource) => resource.available));
  assert.ok(resources.every((resource) => resource.sizeBytes > 0));
});

await test("default CORS allows localhost and private Vite dev origins", () => {
  const allowedOrigins = parseAllowedOrigins();

  assert.equal(isAllowedCorsOrigin("http://localhost", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://127.0.0.1", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://192.168.8.239", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("https://localhost", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("https://127.0.0.1", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("https://192.168.8.239", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://localhost:5173", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://127.0.0.1:5173", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://192.168.8.239:5173", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://10.0.0.12:5173", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://172.16.5.20:5173", { allowedOrigins }), true);
  assert.equal(isAllowedCorsOrigin("http://203.0.113.20:5173", { allowedOrigins }), false);
  assert.equal(isAllowedCorsOrigin("https://192.168.8.239:5173", { allowedOrigins }), false);
  assert.equal(isAllowedCorsOrigin("http://203.0.113.20", { allowedOrigins }), false);
  assert.equal(isAllowedCorsOrigin("http://192.168.8.239:4173", { allowedOrigins }), false);
});

await test("production CORS only allows configured origins", () => {
  const allowedOrigins = parseAllowedOrigins("https://portal.example.com");

  assert.equal(isAllowedCorsOrigin("https://portal.example.com", { allowedOrigins, nodeEnv: "production" }), true);
  assert.equal(isAllowedCorsOrigin("http://192.168.8.239:5173", { allowedOrigins, nodeEnv: "production" }), false);
});

await test("proxmox client sends API token auth and form encoded clone request", async () => {
  const recorder = createProxmoxFetchRecorder();
  const env = {
    PROXMOX_API_URL: "https://proxmox.example.local:8006",
    PROXMOX_API_USER: "lab@pve",
    PROXMOX_API_TOKEN_NAME: "portal",
    PROXMOX_API_TOKEN_VALUE: "secret"
  };
  const client = new ProxmoxClient({ env, fetchImpl: recorder.fetchImpl });

  await client.cloneVm({
    sourceNode: "pve",
    templateVmid: 9000,
    newid: 101,
    name: "lab-test",
    full: false
  });

  const cloneRequest = recorder.requests.find((request) => request.path.endsWith("/qemu/9000/clone"));
  assert.equal(cloneRequest.headers.Authorization, "PVEAPIToken=lab@pve!portal=secret");
  assert.deepEqual(cloneRequest.body, {
    newid: "101",
    name: "lab-test",
    full: "0"
  });
});

await test("proxmox provider clones a template and exposes a console URL", async () => {
  const recorder = createProxmoxFetchRecorder();
  const env = {
    PROXMOX_API_URL: "https://proxmox.example.local:8006",
    PROXMOX_NODE: "pve",
    PROXMOX_API_USER: "lab@pve",
    PROXMOX_API_TOKEN_NAME: "portal",
    PROXMOX_API_TOKEN_VALUE: "secret",
    PROXMOX_TEMPLATE_VMID: "9000",
    PROXMOX_TASK_POLL_INTERVAL_MS: "1",
    PROXMOX_VM_POLL_INTERVAL_MS: "60000"
  };
  const provider = new ProxmoxLabProvider({
    env,
    client: new ProxmoxClient({ env, fetchImpl: recorder.fetchImpl }),
    statePath: null
  });

  const originalInfo = console.info;
  console.info = () => {};
  try {
    const deployment = provider.deployLab(user, "ccdc-ecom-ubuntu-24");
    for (let tick = 0; tick < 5; tick += 1) {
      await new Promise((resolve) => setTimeout(resolve, 0));
    }
    const next = provider.getDeployment(user, deployment.id);

    assert.equal(next.provider, "proxmox");
    assert.equal(next.resourceName, "ecom-1-havi");
    assert.equal(next.proxmoxVmid, 101);
    assert.equal(next.status, "ready");
    assert.match(next.consoleUrl, /^https:\/\/proxmox\.example\.local:8006\/\?console=kvm&novnc=1&vmid=101/);
    assert.equal(next.outputs.find((output) => output.key === "novnc_url")?.value, next.consoleUrl);
  } finally {
    console.info = originalInfo;
  }
});

await test("proxmox provider deploys Windows catalog labs from template VMID 107", async () => {
  let cloneRequest = null;
  const client = {
    readConfig() {
      return { apiUrl: "https://proxmox.example.local:8006" };
    },
    async getNextVmId() {
      return "202";
    },
    async cloneVm(params) {
      cloneRequest = params;
      return "UPID:pve:clone";
    },
    async waitForTask() {
      return null;
    },
    async updateVmConfig() {
      return null;
    },
    async startVm() {
      return "UPID:pve:start";
    },
    async getVmStatus() {
      return { vmid: 202, name: "ftp-1-havi", status: "running" };
    }
  };
  const provider = new ProxmoxLabProvider({
    env: {
      PROXMOX_NODE: "pve",
      PROXMOX_VM_POLL_INTERVAL_MS: "60000"
    },
    client,
    statePath: null
  });
  const windowsLabs = provider.listLabs().filter((lab) => lab.category === "windows");

  assert.ok(windowsLabs.length > 0);
  assert.ok(windowsLabs.every((lab) => lab.proxmoxTemplateVmid === 107));

  const deployment = provider.deployLab(user, "ccdc-ftp-server-2022");
  for (let tick = 0; tick < 5; tick += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  const next = provider.getDeployment(user, deployment.id);

  assert.equal(next.proxmoxTemplateVmid, 107);
  assert.equal(cloneRequest.templateVmid, 107);
});

await test("proxmox provider creates a temporary console websocket session", async () => {
  const provider = new ProxmoxLabProvider({
    env: {
      PROXMOX_CONSOLE_SESSION_TTL_MS: "60000"
    },
    client: {
      async createVmConsoleProxy(node, vmid) {
        assert.equal(node, "lorox");
        assert.equal(vmid, 112);
        return {
          port: 5900,
          ticket: "PVEVNC:ticket"
        };
      }
    },
    statePath: null
  });
  const deployment = {
    id: "deployment-id",
    lab: provider.getLab("ccdc-ecom-ubuntu-24"),
    user,
    provider: "proxmox",
    resourceName: "ecom",
    serverId: null,
    proxmoxSourceNode: "lorox",
    proxmoxNode: "lorox",
    proxmoxTemplateVmid: 104,
    proxmoxVmid: 112,
    proxmoxVmName: "ecom",
    status: "ready",
    outputs: [],
    consoleUrl: "https://proxmox.example.local",
    lastError: null,
    expiresAt: "2099-01-01T00:00:00Z",
    createdAt: "2099-01-01T00:00:00Z",
    updatedAt: "2099-01-01T00:00:00Z",
    deletedAt: null
  };

  provider.deployments.set(deployment.id, deployment);
  const session = await provider.createConsoleSession(user, deployment.id);

  assert.match(session.id, /^[0-9a-f-]+$/);
  assert.match(session.websocketPath, new RegExp(`^/api/proxmox/console-sessions/${session.id}/ws$`));
  assert.equal(session.password, "PVEVNC:ticket");
  assert.ok(session.expiresAt);
});

await test("proxmox provider deletes app deployment when VM is already missing", async () => {
  const missingVmError = new Error("Configuration file 'nodes/lorox/qemu-server/112.conf' does not exist\n");
  const provider = new ProxmoxLabProvider({
    client: {
      async getVmStatus() {
        throw missingVmError;
      },
      async deleteVm() {
        throw missingVmError;
      },
      async waitForTask() {
        return null;
      }
    },
    statePath: null
  });
  const deployment = {
    id: "missing-vm-deployment",
    lab: provider.getLab("ccdc-ecom-ubuntu-24"),
    user,
    provider: "proxmox",
    resourceName: "ecom-1-havi",
    serverId: null,
    proxmoxSourceNode: "lorox",
    proxmoxNode: "lorox",
    proxmoxTemplateVmid: 104,
    proxmoxVmid: 112,
    proxmoxVmName: "ecom-1-havi",
    status: "failed",
    outputs: [],
    consoleUrl: "",
    lastError: missingVmError.message,
    expiresAt: "2099-01-01T00:00:00Z",
    createdAt: "2099-01-01T00:00:00Z",
    updatedAt: "2099-01-01T00:00:00Z",
    deletedAt: null
  };

  provider.deployments.set(deployment.id, deployment);
  const deleted = await provider.destroyDeployment(user, deployment.id);

  assert.equal(deleted.status, "deleted");
  assert.ok(deleted.deletedAt);
});

await test("proxmox provider reloads persisted deployments", () => {
  const statePath = resolve(testStateDir, "deployments.json");
  if (existsSync(statePath)) rmSync(statePath);

  const firstProvider = new ProxmoxLabProvider({
    env: {
      PROXMOX_NODE: "lorox",
      PROXMOX_TEMPLATE_VMID: "104"
    },
    client: {},
    statePath
  });
  const deployment = {
    id: "deployment-id",
    lab: firstProvider.getLab("ccdc-ecom-ubuntu-24"),
    user,
    provider: "proxmox",
    resourceName: "ecom-1-havi",
    serverId: null,
    proxmoxSourceNode: "lorox",
    proxmoxNode: "lorox",
    proxmoxTemplateVmid: 104,
    proxmoxVmid: 112,
    proxmoxVmName: "ecom-1-havi",
    status: "ready",
    outputs: [],
    consoleUrl: "https://proxmox.example.local",
    lastError: null,
    expiresAt: "2099-01-01T00:00:00Z",
    createdAt: "2099-01-01T00:00:00Z",
    updatedAt: "2099-01-01T00:00:00Z",
    deletedAt: null
  };
  firstProvider.deployments.set(deployment.id, deployment);
  firstProvider.saveDeployments();

  const secondProvider = new ProxmoxLabProvider({
    env: {
      PROXMOX_NODE: "lorox",
      PROXMOX_TEMPLATE_VMID: "104"
    },
    client: {},
    statePath
  });
  const restored = secondProvider.getDeployment(user, deployment.id);

  assert.equal(restored.id, deployment.id);
  assert.equal(restored.lab.id, "ccdc-ecom-ubuntu-24");
  assert.equal(restored.resourceName, "ecom-1-havi");
  assert.equal(restored.proxmoxVmid, 112);
});
