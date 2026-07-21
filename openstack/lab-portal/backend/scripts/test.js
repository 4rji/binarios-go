import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { MockLabProvider } from "../src/providers/mockLabProvider.js";
import {
  buildHeatParameters,
  extractTemplateParameterNames,
  OpenStackHeatClient,
  OpenStackHeatProvider
} from "../src/providers/openStackHeatProvider.js";
import { authenticateUser, getPublicUsers, publicUser } from "../src/users.js";

const user = { id: "user-havi", username: "havi", role: "student" };
const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

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

function createOpenStackFetchRecorder() {
  const requests = [];
  const catalog = [
    {
      type: "compute",
      name: "nova",
      endpoints: [{ interface: "public", region: "RegionOne", url: "http://openstack.example.local/compute/v2.1" }]
    },
    {
      type: "image",
      name: "glance",
      endpoints: [{ interface: "public", region: "RegionOne", url: "http://openstack.example.local/image" }]
    },
    {
      type: "network",
      name: "neutron",
      endpoints: [{ interface: "public", region: "RegionOne", url: "http://openstack.example.local/networking" }]
    }
  ];

  return {
    requests,
    fetchImpl: async (url, options = {}) => {
      const request = {
        url: String(url),
        method: options.method || "GET",
        body: options.body ? JSON.parse(options.body) : null
      };
      requests.push(request);

      if (request.url.endsWith("/identity/v3/auth/tokens")) {
        return jsonResponse({
          token: {
            expires_at: "2099-01-01T00:00:00Z",
            project: { id: "project-id" },
            catalog
          }
        }, { status: 201, headers: { "x-subject-token": "token-id" } });
      }

      if (request.url.endsWith("/image/v2/images?name=cirros")) {
        return jsonResponse({ images: [{ id: "image-id", name: "cirros" }] });
      }

      if (request.url.endsWith("/compute/v2.1/project-id/flavors/detail")) {
        return jsonResponse({ flavors: [{ id: "flavor-id", name: "m1.tiny" }] });
      }

      if (request.url.endsWith("/compute/v2.1/project-id/limits")) {
        return jsonResponse({
          limits: {
            absolute: {
              maxTotalCores: 32,
              totalCoresUsed: 6,
              maxTotalRAMSize: 65536,
              totalRAMUsed: 12288,
              maxTotalInstances: 20,
              totalInstancesUsed: 3
            }
          }
        });
      }

      if (request.url.endsWith("/compute/v2.1/project-id/os-hypervisors/statistics")) {
        return jsonResponse({
          hypervisor_statistics: {
            vcpus: 64,
            vcpus_used: 10,
            memory_mb: 131072,
            memory_mb_used: 32768,
            local_gb: 2000,
            local_gb_used: 280,
            running_vms: 4
          }
        });
      }

      if (request.url.endsWith("/networking/v2.0/networks?name=private")) {
        return jsonResponse({ networks: [{ id: "network-id", name: "private" }] });
      }

      if (request.url.endsWith("/compute/v2.1/project-id/servers") && request.method === "POST") {
        return jsonResponse({ server: { id: "server-id", name: request.body.server.name } }, { status: 202 });
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
  assert.match(deployment.heatStackName, /^lab-havi-ccdc-wkst-ubuntu-24-/);
});

await test("mock provider hides another user's deployment", () => {
  const provider = new MockLabProvider();
  const deployment = provider.deployLab(user, "ccdc-wkst-ubuntu-24");
  const otherUser = { id: "user-other", username: "other", role: "student" };

  assert.equal(provider.getDeployment(otherUser, deployment.id), null);
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

await test("public users never include passwords", () => {
  const users = getPublicUsers();
  const usernames = users.map((candidate) => candidate.username);

  assert.ok(usernames.includes("havi"));
  assert.ok(usernames.includes("connor"));
  assert.ok(users.every((candidate) => !("password" in candidate)));
});

await test("heat template parameter names are extracted from yaml", () => {
  const templateSource = readFileSync(resolve(repositoryRoot, "heat-templates/single-linux.yaml"), "utf8");

  assert.deepEqual(extractTemplateParameterNames(templateSource), [
    "image",
    "flavor",
    "key_name",
    "external_network"
  ]);
});

await test("smoke template does not require an ssh key", () => {
  const templateSource = readFileSync(resolve(repositoryRoot, "heat-templates/single-cirros.yaml"), "utf8");

  assert.deepEqual(extractTemplateParameterNames(templateSource), [
    "image",
    "flavor",
    "network"
  ]);
});

await test("openstack heat parameters come from scoped environment variables", () => {
  const templateSource = readFileSync(resolve(repositoryRoot, "heat-templates/single-linux.yaml"), "utf8");
  const lab = {
    id: "ccdc-wkst-ubuntu-24",
    slug: "ccdc-wkst-ubuntu-24",
    platform: "Linux"
  };

  assert.deepEqual(buildHeatParameters({
    env: {
      LAB_LINUX_IMAGE: "ubuntu-24.04",
      LAB_HEAT_FLAVOR: "m1.small",
      LAB_HEAT_KEY_NAME: "lab-key",
      LAB_HEAT_EXTERNAL_NETWORK: "public",
      LAB_HEAT_PARAM_UNKNOWN: "ignored"
    },
    lab,
    templateSource
  }), {
    image: "ubuntu-24.04",
    flavor: "m1.small",
    key_name: "lab-key",
    external_network: "public"
  });
});

await test("splunk lab uses Oracle Linux direct Nova parameters", () => {
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_CCDC_SPLUNK_PARAM_IMAGE: "Oracle Linux 9",
      LAB_CCDC_SPLUNK_PARAM_FLAVOR: "splunk.large",
      LAB_CCDC_SPLUNK_PARAM_NETWORK: "private"
    }
  });
  const lab = provider.getLab("ccdc-splunk");
  const templateSource = readFileSync(resolve(repositoryRoot, lab.heatTemplatePath), "utf8");

  assert.equal(lab.heatTemplatePath, "heat-templates/single-cirros.yaml");
  assert.deepEqual(buildHeatParameters({
    env: provider.env,
    lab,
    templateSource
  }), {
    image: "Oracle Linux 9",
    flavor: "splunk.large",
    network: "private"
  });
});

await test("ecom lab uses existing Glance image and custom flavor defaults", () => {
  const provider = new OpenStackHeatProvider({ env: {} });
  const lab = provider.getLab("ccdc-ecom-ubuntu-24");
  const templateSource = readFileSync(resolve(repositoryRoot, lab.heatTemplatePath), "utf8");

  assert.equal(lab.heatTemplatePath, "heat-templates/single-linux.yaml");
  assert.deepEqual(lab.resources, {
    vcpus: 3,
    ramMb: 4096,
    diskGb: 50,
    networks: 2,
    servers: 1
  });
  assert.deepEqual(buildHeatParameters({
    env: provider.env,
    lab,
    templateSource
  }), {
    image: "ecom",
    flavor: "ecom-3c-4g-50g"
  });
});

await test("ecom lab reuses the Splunk network for direct Nova deploys", async () => {
  let directCreateRequest = null;
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_OPENSTACK_DEPLOYMENT_MODE: "nova",
      LAB_CCDC_SPLUNK_PARAM_NETWORK: "private",
      LAB_STACK_POLL_INTERVAL_MS: "60000"
    },
    client: {
      async createDirectServer(request) {
        directCreateRequest = request;
        return { id: "server-id", name: request.serverName };
      }
    }
  });
  const deployment = provider.deployLab(user, "ccdc-ecom-ubuntu-24");
  const originalInfo = console.info;
  console.info = () => {};
  try {
    await new Promise((resolve) => setTimeout(resolve, 0));
  } finally {
    console.info = originalInfo;
  }

  const next = provider.getDeployment(user, deployment.id);
  assert.equal(next.serverId, "server-id");
  assert.deepEqual(directCreateRequest.parameters, {
    image: "ecom",
    flavor: "ecom-3c-4g-50g",
    network: "private"
  });
});

await test("openstack client can build password auth payload", () => {
  const client = new OpenStackHeatClient({
    env: {
      OS_AUTH_URL: "http://openstack.example.local/identity/v3",
      OS_AUTH_TYPE: "password",
      OS_USERNAME: "ccdc",
      OS_PASSWORD: "secret",
      OS_PROJECT_NAME: "ccdc",
      OS_USER_DOMAIN_NAME: "Default",
      OS_PROJECT_DOMAIN_NAME: "Default"
    }
  });

  assert.deepEqual(client.authPayload(client.readConfig()), {
    auth: {
      identity: {
        methods: ["password"],
        password: {
          user: {
            name: "ccdc",
            password: "secret",
            domain: { name: "Default" }
          }
        }
      },
      scope: {
        project: {
          name: "ccdc",
          domain: { name: "Default" }
        }
      }
    }
  });
});

await test("openstack client accepts cli application credential auth type", () => {
  const client = new OpenStackHeatClient({
    env: {
      OS_AUTH_URL: "http://openstack.example.local/identity/v3",
      OS_AUTH_TYPE: "v3applicationcredential",
      OS_APPLICATION_CREDENTIAL_ID: "credential-id",
      OS_APPLICATION_CREDENTIAL_SECRET: "credential-secret"
    }
  });

  assert.deepEqual(client.authPayload(client.readConfig()), {
    auth: {
      identity: {
        methods: ["application_credential"],
        application_credential: {
          id: "credential-id",
          secret: "credential-secret"
        }
      }
    }
  });
});

await test("openstack client can summarize server resource usage", async () => {
  const recorder = createOpenStackFetchRecorder();
  const client = new OpenStackHeatClient({
    env: {
      OS_AUTH_URL: "http://openstack.example.local/identity/v3",
      OS_AUTH_TYPE: "v3applicationcredential",
      OS_APPLICATION_CREDENTIAL_ID: "credential-id",
      OS_APPLICATION_CREDENTIAL_SECRET: "credential-secret"
    },
    fetchImpl: recorder.fetchImpl
  });

  const summary = await client.resourceSummary();

  assert.equal(summary.source, "hypervisor-statistics");
  assert.deepEqual(summary.resources.find((metric) => metric.key === "vcpus"), {
    key: "vcpus",
    label: "vCPU",
    used: 10,
    total: 64,
    available: 54,
    unit: "cores"
  });
  assert.deepEqual(summary.resources.find((metric) => metric.key === "disk"), {
    key: "disk",
    label: "Disk",
    used: 280,
    total: 2000,
    available: 1720,
    unit: "GB"
  });
});

await test("openstack client can create a Nova server without Heat", async () => {
  const recorder = createOpenStackFetchRecorder();
  const client = new OpenStackHeatClient({
    env: {
      OS_AUTH_URL: "http://openstack.example.local/identity/v3",
      OS_AUTH_TYPE: "v3applicationcredential",
      OS_APPLICATION_CREDENTIAL_ID: "credential-id",
      OS_APPLICATION_CREDENTIAL_SECRET: "credential-secret"
    },
    fetchImpl: recorder.fetchImpl
  });

  const server = await client.createDirectServer({
    serverName: "lab-havi-cirros",
    parameters: {
      image: "cirros",
      flavor: "m1.tiny",
      network: "private"
    },
    metadata: {
      lab_portal: "true"
    }
  });
  const createRequest = recorder.requests.find((request) => (
    request.method === "POST" && request.url.endsWith("/compute/v2.1/project-id/servers")
  ));

  assert.equal(server.id, "server-id");
  assert.deepEqual(createRequest.body.server, {
    name: "lab-havi-cirros",
    imageRef: "image-id",
    flavorRef: "flavor-id",
    networks: [{ uuid: "network-id" }],
    metadata: { lab_portal: "true" }
  });
});

await test("openstack client can let Nova choose a default network", async () => {
  const recorder = createOpenStackFetchRecorder();
  const client = new OpenStackHeatClient({
    env: {
      OS_AUTH_URL: "http://openstack.example.local/identity/v3",
      OS_AUTH_TYPE: "v3applicationcredential",
      OS_APPLICATION_CREDENTIAL_ID: "credential-id",
      OS_APPLICATION_CREDENTIAL_SECRET: "credential-secret"
    },
    fetchImpl: recorder.fetchImpl
  });

  await client.createDirectServer({
    serverName: "lab-havi-cirros",
    parameters: {
      image: "cirros",
      flavor: "m1.tiny"
    }
  });
  const createRequest = recorder.requests.find((request) => (
    request.method === "POST" && request.url.endsWith("/compute/v2.1/project-id/servers")
  ));
  const networkRequests = recorder.requests.filter((request) => request.url.includes("/networking/v2.0/networks"));

  assert.equal(networkRequests.length, 0);
  assert.deepEqual(createRequest.body.server, {
    name: "lab-havi-cirros",
    imageRef: "image-id",
    flavorRef: "flavor-id",
    metadata: {}
  });
});

await test("openstack provider falls back to Nova when Heat is missing in auto mode", async () => {
  let directCreateRequest = null;
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_CIRROS_SMOKE_TEST_PARAM_IMAGE: "Debian 13 Trixie",
      LAB_HEAT_FLAVOR: "m1.small",
      LAB_CIRROS_SMOKE_TEST_PARAM_NETWORK: "private",
      LAB_STACK_POLL_INTERVAL_MS: "60000"
    },
    client: {
      async createStack() {
        const error = new Error("Keystone catalog did not include an orchestration endpoint; set LAB_HEAT_ENDPOINT");
        error.code = "OPENSTACK_HEAT_ENDPOINT_MISSING";
        throw error;
      },
      async createDirectServer(request) {
        directCreateRequest = request;
        return { id: "server-id", name: request.serverName };
      }
    }
  });
  const lab = provider.getLab("cirros-smoke-test");
  const deployment = {
    id: "deployment-id",
    lab,
    user,
    provider: "openstack",
    openStackEngine: "heat",
    heatStackName: "lab-havi-cirros-smoke-test",
    heatStackId: null,
    serverId: null,
    status: "queued",
    outputs: [],
    lastError: null,
    expiresAt: "2099-01-01T00:00:00Z",
    createdAt: "2099-01-01T00:00:00Z",
    updatedAt: "2099-01-01T00:00:00Z",
    deletedAt: null
  };

  provider.deployments.set(deployment.id, deployment);
  const originalInfo = console.info;
  console.info = () => {};
  try {
    await provider.createStackForDeployment(deployment.id, deployment.heatStackName);
  } finally {
    console.info = originalInfo;
  }

  const next = provider.getDeployment(user, deployment.id);
  assert.equal(next.openStackEngine, "nova");
  assert.equal(next.serverId, "server-id");
  assert.deepEqual(directCreateRequest.parameters, {
    image: "Debian 13 Trixie",
    flavor: "m1.small",
    network: "private"
  });
});

await test("openstack provider can override the Nova smoke test server name", async () => {
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_OPENSTACK_DEPLOYMENT_MODE: "nova",
      LAB_CIRROS_SMOKE_TEST_SERVER_NAME: "vm-prueba",
      LAB_CIRROS_SMOKE_TEST_PARAM_IMAGE: "Debian 13 Trixie",
      LAB_CIRROS_SMOKE_TEST_PARAM_FLAVOR: "m1.small",
      LAB_CIRROS_SMOKE_TEST_PARAM_NETWORK: "private",
      LAB_STACK_POLL_INTERVAL_MS: "60000"
    },
    client: {
      async createDirectServer(request) {
        return { id: "server-id", name: request.serverName };
      }
    }
  });

  const originalInfo = console.info;
  console.info = () => {};
  try {
    const deployment = provider.deployLab(user, "cirros-smoke-test");
    await new Promise((resolve) => setTimeout(resolve, 0));
    const next = provider.getDeployment(user, deployment.id);

    assert.equal(next.heatStackName, "vm-prueba");
    assert.equal(next.serverId, "server-id");
    assert.equal(next.outputs.find((output) => output.key === "vm_name")?.value, "vm-prueba");
  } finally {
    console.info = originalInfo;
  }
});

await test("openstack provider destroys all Nova servers matching a fixed deployment name", async () => {
  const deletedServerIds = [];
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_OPENSTACK_DEPLOYMENT_MODE: "nova",
      LAB_STACK_POLL_INTERVAL_MS: "60000"
    },
    client: {
      async listServersByName(serverName) {
        assert.equal(serverName, "vm-prueba");
        return [
          { id: "tracked-server-id", name: "vm-prueba" },
          { id: "orphan-server-id", name: "vm-prueba" }
        ];
      },
      async deleteServer(serverId) {
        deletedServerIds.push(serverId);
      },
      async showServer() {
        const error = new Error("not found");
        error.openStackStatus = 404;
        throw error;
      }
    }
  });
  const lab = provider.getLab("cirros-smoke-test");
  const deployment = {
    id: "deployment-id",
    lab,
    user,
    provider: "openstack",
    openStackEngine: "nova",
    heatStackName: "vm-prueba",
    heatStackId: null,
    serverId: "tracked-server-id",
    status: "ready",
    outputs: [],
    lastError: null,
    expiresAt: "2099-01-01T00:00:00Z",
    createdAt: "2099-01-01T00:00:00Z",
    updatedAt: "2099-01-01T00:00:00Z",
    deletedAt: null
  };

  provider.deployments.set(deployment.id, deployment);
  const next = await provider.destroyDeployment(user, deployment.id);

  assert.equal(next.status, "deleted");
  assert.deepEqual(deletedServerIds.sort(), ["orphan-server-id", "tracked-server-id"]);
});

await test("openstack provider treats Nova start on active server as already ready", async () => {
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_OPENSTACK_DEPLOYMENT_MODE: "nova",
      LAB_STACK_POLL_INTERVAL_MS: "60000"
    },
    client: {
      async serverAction(serverId, action) {
        assert.equal(serverId, "server-id");
        assert.equal(action, "os-start");
        const error = new Error("OpenStack Nova request failed (409)");
        error.openStackStatus = 409;
        error.body = {
          conflictingRequest: {
            code: 409,
            message: "Cannot 'start' instance server-id while it is in vm_state active"
          }
        };
        throw error;
      },
      async showServer(serverId) {
        assert.equal(serverId, "server-id");
        return {
          id: "server-id",
          name: "vm-prueba",
          status: "ACTIVE",
          addresses: {}
        };
      },
      async createNoVncConsole() {
        return "https://console.example.local";
      }
    }
  });
  const deployment = {
    id: "deployment-id",
    lab: provider.getLab("cirros-smoke-test"),
    user,
    provider: "openstack",
    openStackEngine: "nova",
    heatStackName: "vm-prueba",
    heatStackId: null,
    serverId: "server-id",
    status: "stopped",
    outputs: [],
    lastError: null,
    expiresAt: "2099-01-01T00:00:00Z",
    createdAt: "2099-01-01T00:00:00Z",
    updatedAt: "2099-01-01T00:00:00Z",
    deletedAt: null
  };

  provider.deployments.set(deployment.id, deployment);
  const originalError = console.error;
  console.error = () => {};
  try {
    provider.startDeployment(user, deployment.id);
    await new Promise((resolve) => setTimeout(resolve, 0));
    const next = provider.getDeployment(user, deployment.id);

    assert.equal(next.status, "ready");
    assert.equal(next.lastError, null);
  } finally {
    console.error = originalError;
  }
});
