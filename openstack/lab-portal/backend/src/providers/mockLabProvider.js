import { randomUUID } from "node:crypto";
import { findEnabledLab, labs } from "../catalog/labs.js";

const nowIso = () => new Date().toISOString();

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
  }

  listLabs() {
    return labs.filter((lab) => lab.enabled);
  }

  getLab(labId) {
    return findEnabledLab(labId);
  }

  listDeployments(user) {
    return [...this.deployments.values()]
      .filter((deployment) => user.role === "admin" || deployment.user.id === user.id)
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  getDeployment(user, deploymentId) {
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
    const stackName = `lab-${user.username}-${lab.slug}-${shortId}`;
    const deployment = {
      id,
      lab,
      user,
      provider: "mock",
      heatStackName: stackName,
      heatStackId: null,
      status: "queued",
      outputs: [],
      lastError: null,
      expiresAt: addMinutes(lab.defaultTtlMinutes),
      createdAt: nowIso(),
      updatedAt: nowIso(),
      deletedAt: null
    };

    this.deployments.set(id, deployment);

    const creatingTimer = setTimeout(() => this.updateStatus(id, "creating"), 400);
    const readyTimer = setTimeout(() => {
      const current = this.deployments.get(id);
      if (!current || current.status === "deleted" || current.status === "deleting") return;
      this.deployments.set(id, {
        ...current,
        status: "ready",
        heatStackId: `mock-stack-${shortId}`,
        outputs: buildOutputs(lab),
        updatedAt: nowIso()
      });
    }, 2200);
    creatingTimer.unref?.();
    readyTimer.unref?.();

    return deployment;
  }

  destroyDeployment(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "deleting",
      updatedAt: nowIso()
    });

    const deleteTimer = setTimeout(() => {
      const current = this.deployments.get(deploymentId);
      if (!current) return;
      this.deployments.set(deploymentId, {
        ...current,
        status: "deleted",
        deletedAt: nowIso(),
        updatedAt: nowIso()
      });
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

  updateStatus(deploymentId, status) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted" || deployment.status === "deleting") return;
    this.deployments.set(deploymentId, {
      ...deployment,
      status,
      updatedAt: nowIso()
    });
  }
}
