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

  assert.deepEqual(usernames, ["havi", "connor"]);
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

await test("cirros smoke template does not require an ssh key", () => {
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

await test("openstack provider falls back to Nova when Heat is missing in auto mode", async () => {
  let directCreateRequest = null;
  const provider = new OpenStackHeatProvider({
    env: {
      LAB_CIRROS_SMOKE_TEST_PARAM_IMAGE: "cirros",
      LAB_HEAT_FLAVOR: "m1.tiny",
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
    image: "cirros",
    flavor: "m1.tiny",
    network: "private"
  });
});
