import { randomUUID } from "node:crypto";
import { findEnabledLab, labs } from "../catalog/labs.js";

const nowIso = () => new Date().toISOString();
const maxTimerDelayMs = 2_147_483_647;

function addMinutes(minutes) {
  return new Date(Date.now() + minutes * 60 * 1000).toISOString();
}

function buildOutputs(lab) {
  return [
    { key: "machine", value: lab.name, sensitive: false },
    { key: "platform", value: lab.platform || "unknown", sensitive: false },
    { key: "lan", value: lab.network?.lan || "pending", sensitive: false },
    { key: "public", value: lab.network?.public || "pending", sensitive: false },
    { key: "access", value: (lab.accessMethods || []).join(", "), sensitive: false },
    { key: "primary_user", value: lab.credentials?.[0]?.username || "pending", sensitive: false }
  ];
}

export class MockLabProvider {
  constructor() {
    this.deployments = new Map();
    this.expiryTimers = new Map();
  }

  listLabs() {
    return labs.filter((lab) => lab.enabled);
  }

  getLab(labId) {
    return findEnabledLab(labId);
  }

  listDeployments(user) {
    this.expireDueDeployments();
    return [...this.deployments.values()]
      .filter((deployment) => user.role === "admin" || deployment.user.id === user.id)
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  getDeployment(user, deploymentId) {
    this.expireDueDeployments();
    const deployment = this.deployments.get(deploymentId);
    if (!deployment) return null;
    if (user.role !== "admin" && deployment.user.id !== user.id) return null;
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
    const shortId = id.slice(0, 8);
    const resourceName = `lab-${user.username}-${lab.slug}-${shortId}`;
    const deployment = {
      id,
      lab,
      user,
      provider: "mock",
      resourceName,
      status: "queued",
      outputs: [],
      lastError: null,
      expiresAt: addMinutes(lab.defaultTtlMinutes),
      createdAt: nowIso(),
      updatedAt: nowIso(),
      deletedAt: null
    };

    this.deployments.set(id, deployment);
    this.scheduleDeploymentExpiry(id);
    console.info("[mock] deployment queued", {
      deploymentId: id,
      user: user.username,
      labId: lab.id,
      resourceName
    });

    const creatingTimer = setTimeout(() => {
      console.info("[mock] deployment creating", { deploymentId: id, resourceName });
      this.updateStatus(id, "creating");
    }, 400);
    const readyTimer = setTimeout(() => {
      const current = this.deployments.get(id);
      if (!current || current.status === "deleted" || current.status === "deleting") return;
      console.info("[mock] deployment ready", { deploymentId: id, resourceName });
      this.deployments.set(id, {
        ...current,
        status: "ready",
        outputs: buildOutputs(lab),
        updatedAt: nowIso()
      });
    }, 2200);
    creatingTimer.unref?.();
    readyTimer.unref?.();

    return deployment;
  }

  destroyDeployment(user, deploymentId) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment) return null;
    if (user.role !== "admin" && deployment.user.id !== user.id) return null;
    if (["deleted", "deleting"].includes(deployment.status)) return deployment;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "deleting",
      updatedAt: nowIso()
    });
    this.clearDeploymentExpiry(deploymentId);

    const deleteTimer = setTimeout(() => {
      const current = this.deployments.get(deploymentId);
      if (!current) return;
      this.deployments.set(deploymentId, {
        ...current,
        status: "deleted",
        deletedAt: nowIso(),
        updatedAt: nowIso()
      });
      this.clearDeploymentExpiry(deploymentId);
    }, 900);
    deleteTimer.unref?.();

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
    return next;
  }

  getHealth() {
    return {
      status: "mock",
      reachable: true
    };
  }

  getResourceSummary() {
    const activeDeployments = [...this.deployments.values()]
      .filter((deployment) => !["deleted", "failed"].includes(deployment.status));
    const used = activeDeployments.reduce((totals, deployment) => ({
      vcpus: totals.vcpus + (deployment.lab?.resources?.vcpus || 0),
      ramMb: totals.ramMb + (deployment.lab?.resources?.ramMb || 0),
      diskGb: totals.diskGb + (deployment.lab?.resources?.diskGb || 0),
      instances: totals.instances + 1
    }), { vcpus: 0, ramMb: 0, diskGb: 0, instances: 0 });

    return {
      status: "mock",
      source: "mock-capacity",
      resources: [
        { key: "vcpus", label: "vCPU", used: used.vcpus, total: 32, available: 32 - used.vcpus, unit: "cores" },
        { key: "ram", label: "RAM", used: used.ramMb, total: 65536, available: 65536 - used.ramMb, unit: "MB" },
        { key: "disk", label: "Disk", used: used.diskGb, total: 1000, available: 1000 - used.diskGb, unit: "GB" },
        { key: "instances", label: "Instances", used: used.instances, total: 20, available: 20 - used.instances, unit: "VMs" }
      ],
      warnings: []
    };
  }

  updateStatus(deploymentId, status) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted" || deployment.status === "deleting") return;
    this.deployments.set(deploymentId, {
      ...deployment,
      status,
      updatedAt: nowIso()
    });
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
      this.expireDeployment(deploymentId);
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
        this.expireDeployment(deployment.id);
      }
    }
  }

  expireDeployment(deploymentId) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || ["deleted", "deleting"].includes(deployment.status)) return;
    this.destroyDeployment(deployment.user, deploymentId);
  }
}
