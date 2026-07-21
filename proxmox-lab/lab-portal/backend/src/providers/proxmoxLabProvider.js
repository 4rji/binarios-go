import { randomUUID } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { WebSocket } from "ws";
import { findEnabledLab, labs } from "../catalog/labs.js";

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const nowIso = () => new Date().toISOString();
const maxTimerDelayMs = 2_147_483_647;

function addMinutes(minutes) {
  return new Date(Date.now() + minutes * 60 * 1000).toISOString();
}

function envKeySuffix(value) {
  return String(value || "")
    .toUpperCase()
    .replace(/[^A-Z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

function firstEnv(env, names) {
  for (const name of names) {
    if (env[name] !== undefined && env[name] !== "") return env[name];
  }
  return undefined;
}

function requiredEnv(env, names) {
  const value = firstEnv(env, names);
  if (value !== undefined) return value;

  const error = new Error(`Missing required environment variable: ${names.join(" or ")}`);
  error.status = 500;
  throw error;
}

function readBool(value, fallback = false) {
  if (value === undefined || value === null || value === "") return fallback;
  return ["1", "true", "yes", "on"].includes(String(value).toLowerCase());
}

function compactObject(value) {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) => entry !== undefined && entry !== null && entry !== "")
  );
}

function deploymentVisibleTo(user, deployment) {
  return user.role === "admin" || deployment.user.id === user.id;
}

function proxmoxLabShortName(lab) {
  return String(lab.name || lab.slug || lab.id)
    .toLowerCase()
    .replace(/ubuntu|linux|windows|server|workstation|wkst|[0-9]+/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    || envKeySuffix(lab.slug || lab.id).toLowerCase().replace(/_/g, "-");
}

function proxmoxNameFor(env, user, lab, number = null) {
  const labKey = envKeySuffix(lab.slug || lab.id);
  const fixedName = firstEnv(env, [
    `LAB_${labKey}_PROXMOX_VM_NAME`,
    `LAB_${labKey}_SERVER_NAME`
  ]);
  if (fixedName) return fixedName;

  const safeUser = envKeySuffix(user.username).toLowerCase().replace(/_/g, "-");
  const safeLab = proxmoxLabShortName(lab);
  return `${safeLab}-${number || 1}-${safeUser}`;
}

function portalStatusFromProxmox(status) {
  switch (String(status || "").toLowerCase()) {
    case "running":
      return "ready";
    case "stopped":
      return "stopped";
    case "paused":
    case "suspended":
      return "stopped";
    default:
      return "queued";
  }
}

function shouldPollStatus(status) {
  return ["queued", "creating", "resetting", "deleting"].includes(status);
}

function proxmoxConsoleUrl({ env, apiUrl, node, vmid, name }) {
  const baseUrl = String(firstEnv(env, ["PROXMOX_CONSOLE_URL_BASE", "PROXMOX_WEB_URL", "PROXMOX_API_URL"]) || apiUrl)
    .replace(/\/+$/, "");
  const url = new URL("/", baseUrl);
  url.searchParams.set("console", "kvm");
  url.searchParams.set("novnc", "1");
  url.searchParams.set("vmid", String(vmid));
  url.searchParams.set("vmname", name);
  url.searchParams.set("node", node);
  url.searchParams.set("resize", "scale");
  return url.toString();
}

function deploymentOutputs({ deployment, vm = {}, consoleUrl = "" }) {
  return [
    { key: "vm_name", value: deployment.proxmoxVmName, sensitive: false },
    { key: "proxmox_vmid", value: String(deployment.proxmoxVmid || ""), sensitive: false },
    { key: "proxmox_node", value: deployment.proxmoxNode, sensitive: false },
    { key: "platform", value: deployment.lab.platform || "unknown", sensitive: false },
    { key: "status", value: vm.status || deployment.status, sensitive: false },
    { key: "novnc_url", value: consoleUrl, description: "Proxmox noVNC console", sensitive: false }
  ];
}

function deploymentErrorMessage(error) {
  if (!error) return "Unknown Proxmox error";
  if (error.body?.message) return error.body.message;
  if (error.body?.errors) return JSON.stringify(error.body.errors);
  if (typeof error.body === "string") return error.body;
  return error.message || String(error);
}

function isMissingProxmoxVmError(error) {
  if (error?.proxmoxStatus === 404) return true;

  const message = deploymentErrorMessage(error);
  return /configuration file .*qemu-server\/\d+\.conf.*does not exist/i.test(message)
    || /no such vm/i.test(message)
    || /vm \d+ does not exist/i.test(message);
}

function defaultStatePath(env) {
  return resolve(repositoryRoot, env.LAB_STATE_PATH || "backend/data/deployments.json");
}

function cleanDeploymentForStorage(deployment) {
  return {
    id: deployment.id,
    lab: {
      id: deployment.lab?.id
    },
    user: deployment.user,
    provider: deployment.provider,
    resourceName: deployment.resourceName,
    serverId: deployment.serverId,
    proxmoxSourceNode: deployment.proxmoxSourceNode,
    proxmoxNode: deployment.proxmoxNode,
    proxmoxTemplateVmid: deployment.proxmoxTemplateVmid,
    proxmoxVmid: deployment.proxmoxVmid,
    proxmoxVmName: deployment.proxmoxVmName,
    status: deployment.status,
    outputs: deployment.outputs || [],
    consoleUrl: deployment.consoleUrl || "",
    lastError: deployment.lastError || null,
    expiresAt: deployment.expiresAt,
    createdAt: deployment.createdAt,
    updatedAt: deployment.updatedAt,
    deletedAt: deployment.deletedAt || null
  };
}

function restoreDeploymentFromStorage(stored) {
  const lab = findEnabledLab(stored.lab?.id || stored.labId);
  if (!lab) return null;
  return {
    id: stored.id,
    resourceName: stored.resourceName || stored.proxmoxVmName,
    user: stored.user,
    provider: stored.provider || "proxmox",
    serverId: stored.serverId || null,
    proxmoxSourceNode: stored.proxmoxSourceNode,
    proxmoxNode: stored.proxmoxNode,
    proxmoxTemplateVmid: stored.proxmoxTemplateVmid,
    proxmoxVmid: stored.proxmoxVmid,
    proxmoxVmName: stored.proxmoxVmName,
    status: stored.status,
    outputs: stored.outputs || [],
    consoleUrl: stored.consoleUrl || "",
    lastError: stored.lastError || null,
    expiresAt: stored.expiresAt,
    createdAt: stored.createdAt,
    updatedAt: stored.updatedAt,
    deletedAt: stored.deletedAt || null,
    lab
  };
}

function parseProxmoxResponse(payload) {
  if (!payload || typeof payload !== "object") return payload;
  if ("data" in payload) return payload.data;
  return payload;
}

function formBody(params) {
  const body = new URLSearchParams();
  for (const [key, value] of Object.entries(compactObject(params))) {
    if (typeof value === "boolean") {
      body.set(key, value ? "1" : "0");
    } else {
      body.set(key, String(value));
    }
  }
  return body;
}

class ProxmoxHttpError extends Error {
  constructor(message, { status, body }) {
    super(message);
    this.name = "ProxmoxHttpError";
    this.status = status >= 400 && status < 600 ? status : 500;
    this.proxmoxStatus = status;
    this.body = body;
  }
}

export class ProxmoxClient {
  constructor({ env = process.env, fetchImpl = fetch } = {}) {
    this.env = env;
    this.fetchImpl = fetchImpl;
  }

  readConfig() {
    const apiUrl = requiredEnv(this.env, ["PROXMOX_API_URL", "PVE_API_URL"]).replace(/\/+$/, "");
    const apiUser = requiredEnv(this.env, ["PROXMOX_API_USER", "PVE_API_USER"]);
    const tokenName = requiredEnv(this.env, ["PROXMOX_API_TOKEN_NAME", "PVE_API_TOKEN_NAME"]);
    const tokenValue = requiredEnv(this.env, ["PROXMOX_API_TOKEN_VALUE", "PVE_API_TOKEN_VALUE"]);

    if (readBool(firstEnv(this.env, ["PROXMOX_TLS_INSECURE"]), false)) {
      process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
    }

    return {
      apiUrl,
      apiUser,
      tokenName,
      tokenValue
    };
  }

  authHeader(config = this.readConfig()) {
    return `PVEAPIToken=${config.apiUser}!${config.tokenName}=${config.tokenValue}`;
  }

  async request(path, { method = "GET", params = {}, config = this.readConfig() } = {}) {
    const url = new URL(`/api2/json${path}`, config.apiUrl);
    const headers = {
      Authorization: this.authHeader(config)
    };
    const options = { method, headers };

    if (method === "GET" || method === "DELETE") {
      for (const [key, value] of Object.entries(compactObject(params))) {
        url.searchParams.set(key, String(value));
      }
    } else {
      headers["Content-Type"] = "application/x-www-form-urlencoded";
      options.body = formBody(params);
    }

    const response = await this.fetchImpl(url, options);
    const contentType = response.headers?.get?.("content-type") || "";
    const body = contentType.includes("application/json")
      ? await response.json()
      : await response.text();

    if (!response.ok) {
      throw new ProxmoxHttpError(`Proxmox API request failed (${response.status})`, {
        status: response.status,
        body
      });
    }

    return parseProxmoxResponse(body);
  }

  getNextVmId() {
    return this.request("/cluster/nextid");
  }

  getVersion() {
    return this.request("/version");
  }

  getNodeStatus(node) {
    return this.request(`/nodes/${encodeURIComponent(node)}/status`);
  }

  cloneVm({ sourceNode, templateVmid, newid, name, full, storage, pool, targetNode, description }) {
    return this.request(`/nodes/${encodeURIComponent(sourceNode)}/qemu/${encodeURIComponent(templateVmid)}/clone`, {
      method: "POST",
      params: {
        newid,
        name,
        full,
        storage,
        pool,
        target: targetNode && targetNode !== sourceNode ? targetNode : undefined,
        description
      }
    });
  }

  updateVmConfig(node, vmid, config) {
    return this.request(`/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}/config`, {
      method: "PUT",
      params: config
    });
  }

  startVm(node, vmid) {
    return this.request(`/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}/status/start`, {
      method: "POST"
    });
  }

  stopVm(node, vmid) {
    return this.request(`/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}/status/stop`, {
      method: "POST"
    });
  }

  getVmStatus(node, vmid) {
    return this.request(`/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}/status/current`);
  }

  deleteVm(node, vmid) {
    return this.request(`/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}`, {
      method: "DELETE",
      params: {
        purge: 1,
        "destroy-unreferenced-disks": 1
      }
    });
  }

  getTaskStatus(node, upid) {
    return this.request(`/nodes/${encodeURIComponent(node)}/tasks/${encodeURIComponent(upid)}/status`);
  }

  createVmConsoleProxy(node, vmid) {
    return this.request(`/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}/vncproxy`, {
      method: "POST",
      params: {
        websocket: 1
      }
    });
  }

  consoleWebSocketUrl({ node, vmid, port, vncticket }) {
    const config = this.readConfig();
    const url = new URL(`/api2/json/nodes/${encodeURIComponent(node)}/qemu/${encodeURIComponent(vmid)}/vncwebsocket`, config.apiUrl);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.searchParams.set("port", String(port));
    url.searchParams.set("vncticket", vncticket);
    return url.toString();
  }

  webSocketOptions(config = this.readConfig()) {
    return {
      headers: {
        Authorization: this.authHeader(config)
      },
      rejectUnauthorized: !readBool(firstEnv(this.env, ["PROXMOX_TLS_INSECURE"]), false)
    };
  }

  async waitForTask(node, upid, {
    pollIntervalMs = Number(this.env.PROXMOX_TASK_POLL_INTERVAL_MS || 1500),
    timeoutMs = Number(this.env.PROXMOX_TASK_TIMEOUT_MS || 300000)
  } = {}) {
    if (!upid) return null;

    const startedAt = Date.now();
    while (Date.now() - startedAt < timeoutMs) {
      const task = await this.getTaskStatus(node, upid);
      if (task?.status === "stopped") {
        if (!task.exitstatus || task.exitstatus === "OK") return task;
        throw new Error(`Proxmox task failed: ${task.exitstatus}`);
      }

      await new Promise((resolve) => setTimeout(resolve, pollIntervalMs));
    }

    throw new Error(`Timed out waiting for Proxmox task ${upid}`);
  }
}

export class ProxmoxLabProvider {
  constructor({ env = process.env, client = new ProxmoxClient({ env }), statePath = defaultStatePath(env) } = {}) {
    this.env = env;
    this.client = client;
    this.deployments = new Map();
    this.consoleSessions = new Map();
    this.expiryTimers = new Map();
    this.statePath = statePath;
    this.pollIntervalMs = Number(env.PROXMOX_VM_POLL_INTERVAL_MS || 5000);
    this.consoleSessionTtlMs = Number(env.PROXMOX_CONSOLE_SESSION_TTL_MS || 120000);
    this.loadDeployments();
    this.scheduleAllDeploymentExpirations();
    console.info("[proxmox] provider initialized", {
      statePath: this.statePath || "disabled",
      restoredDeployments: this.deployments.size,
      pollIntervalMs: this.pollIntervalMs
    });
  }

  listLabs() {
    return labs.filter((lab) => lab.enabled);
  }

  getLab(labId) {
    return findEnabledLab(labId);
  }

  listDeployments(user) {
    this.expireDueDeployments();
    this.refreshPersistedDeploymentsForUser(user);
    return [...this.deployments.values()]
      .filter((deployment) => deploymentVisibleTo(user, deployment))
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  getDeployment(user, deploymentId) {
    this.expireDueDeployments();
    const deployment = this.deployments.get(deploymentId);
    if (!deployment) return null;
    if (!deploymentVisibleTo(user, deployment)) return null;
    return deployment;
  }

  deployLab(user, labId) {
    const lab = findEnabledLab(labId);
    if (!lab) {
      const error = new Error("Unknown or disabled lab");
      error.status = 404;
      throw error;
    }

    const id = randomUUID();
    const labKey = envKeySuffix(lab.slug || lab.id);
    const sourceNode = requiredEnv(this.env, [`LAB_${labKey}_PROXMOX_NODE`, "PROXMOX_NODE"]);
    const targetNode = firstEnv(this.env, [`LAB_${labKey}_PROXMOX_TARGET_NODE`, "PROXMOX_TARGET_NODE"]) || sourceNode;
    const templateEnvNames = [
      `LAB_${labKey}_PROXMOX_TEMPLATE_VMID`,
      `LAB_${envKeySuffix(lab.platform)}_PROXMOX_TEMPLATE_VMID`,
      "PROXMOX_TEMPLATE_VMID"
    ];
    const templateVmid = firstEnv(this.env, templateEnvNames) ?? lab.proxmoxTemplateVmid;
    if (templateVmid === undefined || templateVmid === null || templateVmid === "") {
      const error = new Error(`Missing required environment variable: ${templateEnvNames.join(" or ")}`);
      error.status = 500;
      throw error;
    }
    const vmName = proxmoxNameFor(this.env, user, lab, this.nextDeploymentNumber(user, lab));
    const deployment = {
      id,
      lab,
      user,
      provider: "proxmox",
      resourceName: vmName,
      serverId: null,
      proxmoxSourceNode: sourceNode,
      proxmoxNode: targetNode,
      proxmoxTemplateVmid: Number(templateVmid),
      proxmoxVmid: null,
      proxmoxVmName: vmName,
      status: "queued",
      outputs: [],
      consoleUrl: "",
      lastError: null,
      expiresAt: addMinutes(lab.defaultTtlMinutes),
      createdAt: nowIso(),
      updatedAt: nowIso(),
      deletedAt: null
    };

    this.deployments.set(id, deployment);
    this.saveDeployments();
    this.scheduleDeploymentExpiry(id);
    console.info("[proxmox] deployment queued", {
      deploymentId: id,
      user: user.username,
      labId: lab.id,
      resourceName: vmName,
      sourceNode,
      targetNode,
      templateVmid: Number(templateVmid)
    });
    this.createVmForDeployment(id, vmName)
      .catch((error) => this.failDeployment(id, error));
    return deployment;
  }

  async destroyDeployment(user, deploymentId) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment) return null;
    if (!deploymentVisibleTo(user, deployment)) return null;
    if (["deleted", "deleting"].includes(deployment.status)) return deployment;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "deleting",
      updatedAt: nowIso()
    });
    this.saveDeployments();
    this.clearDeploymentExpiry(deploymentId);

    try {
      await this.deleteVmForDeployment(deploymentId, deployment.proxmoxNode, deployment.proxmoxVmid);
    } catch (error) {
      this.failDeployment(deploymentId, error);
    }

    return this.deployments.get(deploymentId);
  }

  stopDeployment(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;
    if (!deployment.proxmoxVmid) return deployment;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "resetting",
      updatedAt: nowIso()
    });
    this.saveDeployments();

    this.client.stopVm(deployment.proxmoxNode, deployment.proxmoxVmid)
      .then((upid) => this.client.waitForTask(deployment.proxmoxNode, upid))
      .then(() => this.refreshDeploymentStatus(deploymentId))
      .catch((error) => this.failDeployment(deploymentId, error));

    return this.deployments.get(deploymentId);
  }

  startDeployment(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;
    if (!deployment.proxmoxVmid) return deployment;
    if (deployment.status === "ready") return deployment;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "resetting",
      updatedAt: nowIso()
    });
    this.saveDeployments();

    this.client.startVm(deployment.proxmoxNode, deployment.proxmoxVmid)
      .then((upid) => this.client.waitForTask(deployment.proxmoxNode, upid))
      .then(() => this.refreshDeploymentStatus(deploymentId))
      .catch((error) => this.failDeployment(deploymentId, error));

    return this.deployments.get(deploymentId);
  }

  resetDeployment(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "resetting",
      outputs: [],
      consoleUrl: "",
      lastError: null,
      updatedAt: nowIso()
    });
    this.saveDeployments();

    this.resetVmForDeployment(deploymentId)
      .catch((error) => this.failDeployment(deploymentId, error));

    return this.deployments.get(deploymentId);
  }

  setLifecycleStatus(user, deploymentId, status) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;

    const next = {
      ...deployment,
      status,
      updatedAt: nowIso()
    };
    this.deployments.set(deploymentId, next);
    this.saveDeployments();
    return next;
  }

  nextDeploymentNumber(user, lab) {
    const prefix = `${proxmoxLabShortName(lab)}-`;
    const suffix = `-${envKeySuffix(user.username).toLowerCase().replace(/_/g, "-")}`;
    const usedNumbers = new Set();

    for (const deployment of this.deployments.values()) {
      if (deployment.user?.id !== user.id) continue;
      if (deployment.lab?.id !== lab.id) continue;
      if (["deleted", "deleting", "failed"].includes(deployment.status)) continue;

      const name = String(deployment.proxmoxVmName || deployment.resourceName || "");
      if (!name.startsWith(prefix) || !name.endsWith(suffix)) continue;

      const number = Number(name.slice(prefix.length, -suffix.length));
      if (Number.isInteger(number)) usedNumbers.add(number);
    }

    for (let number = 1; number <= 20; number += 1) {
      if (!usedNumbers.has(number)) return number;
    }

    const error = new Error(`No free Proxmox VM slot for ${lab.name}; destroy an existing deployment first`);
    error.status = 409;
    throw error;
  }

  loadDeployments() {
    if (!this.statePath || !existsSync(this.statePath)) return;

    try {
      const payload = JSON.parse(readFileSync(this.statePath, "utf8"));
      for (const stored of payload.deployments || []) {
        const deployment = restoreDeploymentFromStorage(stored);
        if (deployment) this.deployments.set(deployment.id, deployment);
      }
    } catch (error) {
      console.error("[proxmox] failed to load persisted deployments", {
        statePath: this.statePath,
        error: error.message
      });
    }
  }

  saveDeployments() {
    if (!this.statePath) return;

    const directory = dirname(this.statePath);
    mkdirSync(directory, { recursive: true });
    const payload = {
      version: 1,
      deployments: [...this.deployments.values()].map(cleanDeploymentForStorage)
    };
    const tempPath = `${this.statePath}.${process.pid}.tmp`;
    writeFileSync(tempPath, JSON.stringify(payload, null, 2));
    renameSync(tempPath, this.statePath);
  }

  refreshPersistedDeploymentsForUser(user) {
    for (const deployment of this.deployments.values()) {
      if (!deploymentVisibleTo(user, deployment)) continue;
      if (!deployment.proxmoxVmid || ["deleted", "deleting"].includes(deployment.status)) continue;
      if (shouldPollStatus(deployment.status)) continue;

      this.refreshDeploymentStatus(deployment.id).catch((error) => {
        console.warn("[proxmox] failed to refresh persisted deployment", {
          deploymentId: deployment.id,
          proxmoxVmid: deployment.proxmoxVmid,
          error: deploymentErrorMessage(error)
        });
      });
    }
  }

  async getHealth() {
    try {
      const config = this.client.readConfig();
      const node = firstEnv(this.env, ["PROXMOX_NODE"]);
      const [version, nodeStatus] = await Promise.all([
        this.client.getVersion(),
        node ? this.client.getNodeStatus(node).catch(() => null) : Promise.resolve(null)
      ]);

      return {
        status: "proxmox",
        reachable: true,
        apiUrl: config.apiUrl,
        version: version?.version || version?.release || "unknown",
        node,
        nodeOnline: nodeStatus ? true : undefined
      };
    } catch (error) {
      return {
        status: "proxmox",
        reachable: false,
        error: deploymentErrorMessage(error)
      };
    }
  }

  async getResourceSummary() {
    const node = requiredEnv(this.env, ["PROXMOX_NODE"]);
    try {
      const status = await this.client.getNodeStatus(node);
      const totalCpu = Number(status?.cpuinfo?.cpus || 0);
      const cpuRatio = Number(status?.cpu);
      const usedCpu = totalCpu && Number.isFinite(cpuRatio)
        ? Math.round(totalCpu * cpuRatio * 100) / 100
        : 0;
      const totalRam = Math.round(Number(status?.memory?.total || 0) / 1024 / 1024);
      const usedRam = Math.round(Number(status?.memory?.used || 0) / 1024 / 1024);
      const totalDisk = Math.round(Number(status?.rootfs?.total || 0) / 1024 / 1024 / 1024);
      const usedDisk = Math.round(Number(status?.rootfs?.used || 0) / 1024 / 1024 / 1024);
      const activeInstances = [...this.deployments.values()]
        .filter((deployment) => !["deleted", "failed"].includes(deployment.status)).length;

      return {
        status: "proxmox",
        source: `proxmox-node:${node}`,
        resources: [
          { key: "vcpus", label: "vCPU", used: usedCpu, total: totalCpu, available: Math.max(0, totalCpu - usedCpu), unit: "cores" },
          { key: "ram", label: "RAM", used: usedRam, total: totalRam, available: Math.max(0, totalRam - usedRam), unit: "MB" },
          { key: "disk", label: "Disk", used: usedDisk, total: totalDisk, available: Math.max(0, totalDisk - usedDisk), unit: "GB" },
          { key: "instances", label: "Instances", used: activeInstances, total: null, available: null, unit: "VMs" }
        ],
        warnings: []
      };
    } catch (error) {
      return {
        status: "proxmox",
        source: "unavailable",
        resources: [],
        warnings: [deploymentErrorMessage(error)]
      };
    }
  }

  async createConsoleSession(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;
    if (!deployment.proxmoxVmid) {
      const error = new Error("Deployment does not have a Proxmox VM yet");
      error.status = 409;
      throw error;
    }

    const proxy = await this.client.createVmConsoleProxy(deployment.proxmoxNode, deployment.proxmoxVmid);
    const sessionId = randomUUID();
    const expiresAtMs = Date.now() + this.consoleSessionTtlMs;
    const session = {
      id: sessionId,
      deploymentId,
      userId: user.id,
      node: deployment.proxmoxNode,
      vmid: deployment.proxmoxVmid,
      port: proxy.port,
      vncticket: proxy.ticket,
      createdAt: nowIso(),
      expiresAt: new Date(expiresAtMs).toISOString(),
      websocketPath: `/api/proxmox/console-sessions/${sessionId}/ws`
    };

    this.consoleSessions.set(sessionId, session);
    const cleanupTimer = setTimeout(() => this.consoleSessions.delete(sessionId), this.consoleSessionTtlMs);
    cleanupTimer.unref?.();

    return {
      id: session.id,
      expiresAt: session.expiresAt,
      password: session.vncticket,
      websocketPath: session.websocketPath
    };
  }

  proxyConsoleWebSocket(sessionId, clientSocket) {
    const session = this.consoleSessions.get(sessionId);
    if (!session) {
      clientSocket.close(1008, "Console session not found or expired");
      return;
    }

    if (new Date(session.expiresAt).getTime() <= Date.now()) {
      this.consoleSessions.delete(sessionId);
      clientSocket.close(1008, "Console session expired");
      return;
    }

    const upstreamUrl = this.client.consoleWebSocketUrl(session);
    const upstreamSocket = new WebSocket(upstreamUrl, this.client.webSocketOptions());
    let closed = false;

    const closeBoth = (code = 1000, reason = "") => {
      if (closed) return;
      closed = true;
      this.consoleSessions.delete(sessionId);
      if (clientSocket.readyState === WebSocket.OPEN || clientSocket.readyState === WebSocket.CONNECTING) {
        clientSocket.close(code, reason);
      }
      if (upstreamSocket.readyState === WebSocket.OPEN || upstreamSocket.readyState === WebSocket.CONNECTING) {
        upstreamSocket.close();
      }
    };

    upstreamSocket.on("open", () => {
      clientSocket.on("message", (data, isBinary) => {
        if (upstreamSocket.readyState === WebSocket.OPEN) {
          upstreamSocket.send(data, { binary: isBinary });
        }
      });
    });

    upstreamSocket.on("message", (data, isBinary) => {
      if (clientSocket.readyState === WebSocket.OPEN) {
        clientSocket.send(data, { binary: isBinary });
      }
    });

    upstreamSocket.on("close", () => closeBoth());
    upstreamSocket.on("error", (error) => {
      console.error("[proxmox] console websocket failed", {
        sessionId,
        deploymentId: session.deploymentId,
        error: error.message
      });
      closeBoth(1011, "Proxmox console websocket failed");
    });
    clientSocket.on("close", () => closeBoth());
    clientSocket.on("error", () => closeBoth(1011, "Portal console websocket failed"));
  }

  async createVmForDeployment(deploymentId, expectedVmName = null) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted" || deployment.status === "deleting") return;
    if (expectedVmName && deployment.proxmoxVmName !== expectedVmName) return;

    const fullClone = readBool(firstEnv(this.env, ["PROXMOX_CLONE_FULL"]), false);
    const storage = firstEnv(this.env, [
      `LAB_${envKeySuffix(deployment.lab.slug || deployment.lab.id)}_PROXMOX_STORAGE`,
      "PROXMOX_STORAGE"
    ]);
    const pool = firstEnv(this.env, ["PROXMOX_POOL"]);

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "creating",
      updatedAt: nowIso()
    });
    this.saveDeployments();
    console.info("[proxmox] deployment creating", {
      deploymentId,
      labId: deployment.lab.id,
      resourceName: deployment.proxmoxVmName,
      sourceNode: deployment.proxmoxSourceNode,
      targetNode: deployment.proxmoxNode,
      templateVmid: deployment.proxmoxTemplateVmid
    });

    const vmid = Number(await this.client.getNextVmId());
    console.info("[proxmox] allocated VM id", {
      deploymentId,
      resourceName: deployment.proxmoxVmName,
      vmid
    });
    const description = `lab-portal deployment ${deploymentId} for ${deployment.user.username}/${deployment.lab.id}`;
    console.info(`[proxmox] cloning template ${deployment.proxmoxTemplateVmid} to VM ${vmid}`, {
      deploymentId,
      labId: deployment.lab.id,
      sourceNode: deployment.proxmoxSourceNode,
      targetNode: deployment.proxmoxNode,
      fullClone
    });

    const cloneUpid = await this.client.cloneVm({
      sourceNode: deployment.proxmoxSourceNode,
      templateVmid: deployment.proxmoxTemplateVmid,
      newid: vmid,
      name: deployment.proxmoxVmName,
      full: fullClone,
      storage,
      pool,
      targetNode: deployment.proxmoxNode,
      description
    });
    await this.client.waitForTask(deployment.proxmoxSourceNode, cloneUpid);
    console.info("[proxmox] clone completed", {
      deploymentId,
      vmid,
      resourceName: deployment.proxmoxVmName
    });

    const current = this.deployments.get(deploymentId);
    if (!current || current.status === "deleted" || current.status === "deleting") {
      await this.client.deleteVm(deployment.proxmoxNode, vmid).catch(() => {});
      return;
    }

    await this.configureVm(current, vmid);
    console.info("[proxmox] VM configured", {
      deploymentId,
      vmid,
      resourceName: current.proxmoxVmName
    });

    this.deployments.set(deploymentId, {
      ...current,
      proxmoxVmid: vmid,
      updatedAt: nowIso()
    });
    this.saveDeployments();

    console.info("[proxmox] starting VM", {
      deploymentId,
      vmid,
      resourceName: deployment.proxmoxVmName
    });
    const startUpid = await this.client.startVm(deployment.proxmoxNode, vmid);
    await this.client.waitForTask(deployment.proxmoxNode, startUpid);
    await this.refreshDeploymentStatus(deploymentId);
  }

  async configureVm(deployment, vmid) {
    const applyResources = readBool(firstEnv(this.env, ["PROXMOX_APPLY_LAB_RESOURCES"]), false);
    const config = compactObject({
      description: `lab-portal deployment ${deployment.id} for ${deployment.user.username}/${deployment.lab.id}`,
      cores: applyResources ? deployment.lab.resources?.vcpus : undefined,
      memory: applyResources ? deployment.lab.resources?.ramMb : undefined
    });

    if (Object.keys(config).length === 0) return;
    await this.client.updateVmConfig(deployment.proxmoxNode, vmid, config);
  }

  async resetVmForDeployment(deploymentId) {
    const current = this.deployments.get(deploymentId);
    if (!current || current.status === "deleted" || current.status === "deleting") return;

    await this.deleteVmForDeployment(deploymentId, current.proxmoxNode, current.proxmoxVmid, { markDeleted: false });

    const nextName = current.proxmoxVmName || current.resourceName;
    const next = this.deployments.get(deploymentId);
    if (!next || next.status === "deleted" || next.status === "deleting") return;

    this.deployments.set(deploymentId, {
      ...next,
      resourceName: nextName,
      proxmoxVmName: nextName,
      proxmoxVmid: null,
      status: "queued",
      updatedAt: nowIso()
    });
    this.saveDeployments();

    await this.createVmForDeployment(deploymentId, nextName);
  }

  async deleteVmForDeployment(deploymentId, node, vmid, { markDeleted = true } = {}) {
    if (vmid) {
      const vm = await this.client.getVmStatus(node, vmid).catch((error) => {
        if (isMissingProxmoxVmError(error)) return null;
        throw error;
      });
      if (vm?.status === "running") {
        const stopUpid = await this.client.stopVm(node, vmid).catch((error) => {
          if (isMissingProxmoxVmError(error)) return null;
          throw error;
        });
        await this.client.waitForTask(node, stopUpid).catch((error) => {
          if (isMissingProxmoxVmError(error)) return null;
          throw error;
        });
      }

      const deleteUpid = await this.client.deleteVm(node, vmid).catch((error) => {
        if (isMissingProxmoxVmError(error)) return null;
        throw error;
      });
      await this.client.waitForTask(node, deleteUpid).catch((error) => {
        if (isMissingProxmoxVmError(error)) return null;
        throw error;
      });
    }

    const current = this.deployments.get(deploymentId);
    if (!current) return;
    this.deployments.set(deploymentId, {
      ...current,
      status: markDeleted ? "deleted" : "queued",
      proxmoxVmid: markDeleted ? current.proxmoxVmid : null,
      deletedAt: markDeleted ? nowIso() : current.deletedAt,
      updatedAt: nowIso()
    });
    if (markDeleted) this.clearDeploymentExpiry(deploymentId);
    this.saveDeployments();
  }

  scheduleAllDeploymentExpirations() {
    for (const deployment of this.deployments.values()) {
      this.scheduleDeploymentExpiry(deployment.id);
    }
  }

  scheduleDeploymentExpiry(deploymentId) {
    this.clearDeploymentExpiry(deploymentId);
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || ["deleted", "deleting"].includes(deployment.status)) return;

    const expiresAtMs = new Date(deployment.expiresAt).getTime();
    if (!Number.isFinite(expiresAtMs)) return;

    const delayMs = Math.min(maxTimerDelayMs, Math.max(0, expiresAtMs - Date.now()));
    const timer = setTimeout(() => {
      const current = this.deployments.get(deploymentId);
      const currentExpiresAtMs = new Date(current?.expiresAt).getTime();
      if (Number.isFinite(currentExpiresAtMs) && currentExpiresAtMs > Date.now()) {
        this.scheduleDeploymentExpiry(deploymentId);
        return;
      }
      this.expireDeployment(deploymentId).catch((error) => {
        console.warn("[proxmox] failed to expire deployment", {
          deploymentId,
          error: deploymentErrorMessage(error)
        });
      });
    }, delayMs);
    timer.unref?.();
    this.expiryTimers.set(deploymentId, timer);
  }

  clearDeploymentExpiry(deploymentId) {
    const timer = this.expiryTimers.get(deploymentId);
    if (timer) clearTimeout(timer);
    this.expiryTimers.delete(deploymentId);
  }

  expireDueDeployments() {
    const now = Date.now();
    for (const deployment of this.deployments.values()) {
      const expiresAtMs = new Date(deployment.expiresAt).getTime();
      if (
        Number.isFinite(expiresAtMs)
        && expiresAtMs <= now
        && !["deleted", "deleting"].includes(deployment.status)
      ) {
        this.expireDeployment(deployment.id).catch((error) => {
          console.warn("[proxmox] failed to expire due deployment", {
            deploymentId: deployment.id,
            error: deploymentErrorMessage(error)
          });
        });
      }
    }
  }

  async expireDeployment(deploymentId) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || ["deleted", "deleting"].includes(deployment.status)) return;
    await this.destroyDeployment(deployment.user, deploymentId);
  }

  scheduleStatusPoll(deploymentId, delayMs) {
    const timer = setTimeout(() => {
      this.refreshDeploymentStatus(deploymentId)
        .catch((error) => this.failDeployment(deploymentId, error));
    }, delayMs);
    timer.unref?.();
  }

  async refreshDeploymentStatus(deploymentId) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted" || !deployment.proxmoxVmid) return;

    const vm = await this.client.getVmStatus(deployment.proxmoxNode, deployment.proxmoxVmid);
    const status = portalStatusFromProxmox(vm.status);
    const consoleUrl = proxmoxConsoleUrl({
      env: this.env,
      apiUrl: this.client.readConfig().apiUrl,
      node: deployment.proxmoxNode,
      vmid: deployment.proxmoxVmid,
      name: deployment.proxmoxVmName
    });
    const next = {
      ...deployment,
      status,
      outputs: deploymentOutputs({ deployment, vm, consoleUrl }),
      consoleUrl,
      lastError: null,
      updatedAt: nowIso()
    };

    this.deployments.set(deploymentId, next);
    this.saveDeployments();
    if (status !== deployment.status) {
      console.info(`[proxmox] VM ${deployment.proxmoxVmid} status ${vm.status}`, {
        deploymentId,
        portalStatus: status
      });
    }
    if (shouldPollStatus(status)) {
      this.scheduleStatusPoll(deploymentId, this.pollIntervalMs);
    }
  }

  failDeployment(deploymentId, error) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted") return;
    const message = deploymentErrorMessage(error);
    console.error(`[proxmox] deployment failed ${deployment.proxmoxVmName}`, {
      deploymentId,
      labId: deployment.lab.id,
      error: message
    });
    this.deployments.set(deploymentId, {
      ...deployment,
      status: "failed",
      lastError: message,
      updatedAt: nowIso()
    });
    this.saveDeployments();
  }
}
