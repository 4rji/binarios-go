import { randomUUID } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { dirname, isAbsolute, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { findEnabledLab, labs } from "../catalog/labs.js";

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const nowIso = () => new Date().toISOString();

function addMinutes(minutes) {
  return new Date(Date.now() + minutes * 60 * 1000).toISOString();
}

function envKeySuffix(value) {
  return String(value || "")
    .toUpperCase()
    .replace(/[^A-Z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

function compactObject(value) {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) => entry !== undefined && entry !== null && entry !== "")
  );
}

function readBool(value, fallback = false) {
  if (value === undefined || value === null || value === "") return fallback;
  return ["1", "true", "yes", "on"].includes(String(value).toLowerCase());
}

function readJsonObject(value, envName) {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value);
    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
      throw new Error("value must be a JSON object");
    }
    return parsed;
  } catch (error) {
    const wrapped = new Error(`${envName} must be valid JSON object: ${error.message}`);
    wrapped.status = 500;
    throw wrapped;
  }
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

function optionalDomain(env, names) {
  const domain = firstEnv(env, names);
  return domain ? { name: domain } : undefined;
}

function normalizeAuthType(authType) {
  const normalized = String(authType || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "");

  if (["password", "v3password"].includes(normalized)) return "password";
  if (["applicationcredential", "v3applicationcredential"].includes(normalized)) {
    return "application_credential";
  }

  return authType;
}

function joinUrl(baseUrl, path) {
  return `${baseUrl.replace(/\/+$/g, "")}/${path.replace(/^\/+/g, "")}`;
}

function normalizeAuthUrl(authUrl) {
  const trimmed = authUrl.replace(/\/+$/g, "");
  return trimmed.endsWith("/v3") ? trimmed : `${trimmed}/v3`;
}

function normalizeInterfaceName(interfaceName) {
  return String(interfaceName || "public").replace(/URL$/i, "").toLowerCase();
}

function serviceEndpointFromCatalog(catalog, serviceType, interfaceName, regionName) {
  const services = catalog.filter((service) => service.type === serviceType || service.name === serviceType);
  const exact = services
    .flatMap((service) => service.endpoints || [])
    .find((endpoint) => endpoint.interface === interfaceName && endpoint.region === regionName);
  if (exact) return exact.url;

  const sameRegion = services
    .flatMap((service) => service.endpoints || [])
    .find((endpoint) => endpoint.region === regionName);
  if (sameRegion) return sameRegion.url;

  const sameInterface = services
    .flatMap((service) => service.endpoints || [])
    .find((endpoint) => endpoint.interface === interfaceName);
  if (sameInterface) return sameInterface.url;

  return services.flatMap((service) => service.endpoints || [])[0]?.url;
}

function normalizeHeatEndpoint(endpointUrl, projectId) {
  const substituted = endpointUrl
    .replace(/%\((tenant_id|project_id)\)s/g, projectId)
    .replace(/\{(tenant_id|project_id)\}/g, projectId)
    .replace(/\/+$/g, "");

  if (/\/v1\/[^/]+$/i.test(substituted)) return substituted;
  if (/\/v1$/i.test(substituted)) return `${substituted}/${projectId}`;
  return `${substituted}/v1/${projectId}`;
}

function normalizeComputeEndpoint(endpointUrl, projectId) {
  const substituted = endpointUrl
    .replace(/%\((tenant_id|project_id)\)s/g, projectId)
    .replace(/\{(tenant_id|project_id)\}/g, projectId)
    .replace(/\/+$/g, "");

  if (/\/v2(?:\.1)?\/[^/]+$/i.test(substituted)) return substituted;
  if (/\/v2(?:\.1)?$/i.test(substituted)) return `${substituted}/${projectId}`;
  return `${substituted}/v2.1/${projectId}`;
}

function normalizeImageEndpoint(endpointUrl) {
  const substituted = endpointUrl.replace(/\/+$/g, "");

  if (/\/v2$/i.test(substituted)) return substituted;
  return `${substituted}/v2`;
}

function normalizeNetworkEndpoint(endpointUrl) {
  const substituted = endpointUrl.replace(/\/+$/g, "");

  if (/\/v2\.0$/i.test(substituted)) return substituted;
  return `${substituted}/v2.0`;
}

function normalizeDeploymentMode(value) {
  const normalized = String(value || "auto").toLowerCase();
  if (["heat", "nova", "auto"].includes(normalized)) return normalized;
  return "auto";
}

function sanitizeStackPart(value) {
  return String(value || "lab")
    .toLowerCase()
    .replace(/[^a-z0-9_.-]+/g, "-")
    .replace(/^[^a-z]+/g, "")
    .replace(/-+$/g, "")
    .slice(0, 70) || "lab";
}

function stackNameFor(user, lab, shortId) {
  const username = sanitizeStackPart(user.username);
  const slug = sanitizeStackPart(lab.slug || lab.id);
  return `lab-${username}-${slug}-${shortId}`.slice(0, 255);
}

function safeResolveTemplatePath(templatePath) {
  const absolutePath = resolve(repositoryRoot, templatePath);
  const relativePath = relative(repositoryRoot, absolutePath);

  if (isAbsolute(templatePath) || relativePath.startsWith("..") || isAbsolute(relativePath)) {
    const error = new Error("Heat template path must stay inside the repository");
    error.status = 500;
    throw error;
  }

  if (!existsSync(absolutePath)) {
    const error = new Error(`Heat template not found: ${templatePath}`);
    error.status = 500;
    throw error;
  }

  return absolutePath;
}

function extractJsonTemplateParameters(templateSource) {
  const trimmed = templateSource.trim();
  if (!trimmed.startsWith("{")) return null;

  try {
    const parsed = JSON.parse(trimmed);
    return Object.keys(parsed.parameters || {});
  } catch {
    return null;
  }
}

export function extractTemplateParameterNames(templateSource) {
  const jsonParameters = extractJsonTemplateParameters(templateSource);
  if (jsonParameters) return jsonParameters;

  const names = [];
  const lines = templateSource.split(/\r?\n/);
  let inParameters = false;
  let parameterIndent = null;

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;

    const indent = line.length - line.trimStart().length;

    if (!inParameters) {
      if (indent === 0 && /^parameters:\s*(?:#.*)?$/.test(trimmed)) {
        inParameters = true;
      }
      continue;
    }

    if (indent === 0 && /^[A-Za-z0-9_.-]+:/.test(trimmed)) break;
    if (indent === 0) continue;

    const keyMatch = line.slice(indent).match(/^([A-Za-z0-9_.-]+):\s*(?:#.*)?$/);
    if (!keyMatch) continue;

    if (parameterIndent === null) {
      parameterIndent = indent;
    }

    if (indent === parameterIndent) {
      names.push(keyMatch[1]);
    }
  }

  return names;
}

function parameterAliasEnvNames(parameterName) {
  const parameterKey = envKeySuffix(parameterName);
  const aliases = {
    IMAGE: ["LAB_HEAT_IMAGE", "OS_IMAGE"],
    IMAGE_ID: ["LAB_HEAT_IMAGE", "OS_IMAGE"],
    IMAGE_NAME: ["LAB_HEAT_IMAGE", "OS_IMAGE"],
    FLAVOR: ["LAB_HEAT_FLAVOR", "OS_FLAVOR"],
    FLAVOR_ID: ["LAB_HEAT_FLAVOR", "OS_FLAVOR"],
    FLAVOR_NAME: ["LAB_HEAT_FLAVOR", "OS_FLAVOR"],
    INSTANCE_TYPE: ["LAB_HEAT_FLAVOR", "OS_FLAVOR"],
    KEY: ["LAB_HEAT_KEY_NAME", "OS_KEY_NAME"],
    KEY_NAME: ["LAB_HEAT_KEY_NAME", "OS_KEY_NAME"],
    SSH_KEY_NAME: ["LAB_HEAT_KEY_NAME", "OS_KEY_NAME"],
    EXTERNAL_NETWORK: ["LAB_HEAT_EXTERNAL_NETWORK", "OS_EXTERNAL_NETWORK"],
    PUBLIC_NETWORK: ["LAB_HEAT_EXTERNAL_NETWORK", "OS_EXTERNAL_NETWORK"],
    FLOATING_NETWORK: ["LAB_HEAT_EXTERNAL_NETWORK", "OS_EXTERNAL_NETWORK"]
  };

  return aliases[parameterKey] || [];
}

export function buildHeatParameters({ env, lab, templateSource }) {
  const parameterNames = extractTemplateParameterNames(templateSource);
  const labKey = envKeySuffix(lab.slug || lab.id);
  const platformKey = envKeySuffix(lab.platform);
  const globalParameters = readJsonObject(env.LAB_HEAT_PARAMETERS, "LAB_HEAT_PARAMETERS");
  const labParameters = readJsonObject(env[`LAB_${labKey}_HEAT_PARAMETERS`], `LAB_${labKey}_HEAT_PARAMETERS`);

  return Object.fromEntries(
    parameterNames
      .map((name) => {
        const parameterKey = envKeySuffix(name);
        const value = labParameters[name]
          ?? labParameters[parameterKey]
          ?? globalParameters[name]
          ?? globalParameters[parameterKey]
          ?? firstEnv(env, [
            `LAB_${labKey}_PARAM_${parameterKey}`,
            `LAB_${platformKey}_PARAM_${parameterKey}`,
            `LAB_HEAT_PARAM_${parameterKey}`,
            ...parameterAliasEnvNames(name).flatMap((alias) => [
              `LAB_${labKey}_${alias.replace(/^LAB_HEAT_/, "")}`,
              `LAB_${platformKey}_${alias.replace(/^LAB_HEAT_/, "")}`,
              alias
            ])
          ]);

        return [name, value];
      })
      .filter(([, value]) => value !== undefined && value !== "")
  );
}

function heatOutputsToPortalOutputs(outputs = []) {
  return outputs.map((output) => ({
    key: output.output_key || output.key,
    value: output.output_value ?? output.value,
    description: output.description || "",
    sensitive: false
  })).filter((output) => output.key);
}

function stackStatusToPortalStatus(stackStatus) {
  switch (stackStatus) {
    case "CREATE_COMPLETE":
    case "RESUME_COMPLETE":
    case "UPDATE_COMPLETE":
    case "CHECK_COMPLETE":
      return "ready";
    case "CREATE_IN_PROGRESS":
    case "INIT_IN_PROGRESS":
    case "ADOPT_IN_PROGRESS":
      return "creating";
    case "SUSPEND_COMPLETE":
      return "stopped";
    case "SUSPEND_IN_PROGRESS":
    case "RESUME_IN_PROGRESS":
    case "UPDATE_IN_PROGRESS":
    case "CHECK_IN_PROGRESS":
      return "resetting";
    case "DELETE_IN_PROGRESS":
      return "deleting";
    case "DELETE_COMPLETE":
      return "deleted";
    case "CREATE_FAILED":
    case "UPDATE_FAILED":
    case "ROLLBACK_FAILED":
    case "ROLLBACK_COMPLETE":
    case "DELETE_FAILED":
    case "SUSPEND_FAILED":
    case "RESUME_FAILED":
    case "CHECK_FAILED":
    case "ADOPT_FAILED":
      return "failed";
    default:
      return "queued";
  }
}

function serverStatusToPortalStatus(serverStatus) {
  switch (String(serverStatus || "").toUpperCase()) {
    case "ACTIVE":
      return "ready";
    case "BUILD":
    case "REBUILD":
    case "SCHEDULING":
      return "creating";
    case "SHUTOFF":
    case "SHELVED":
    case "SHELVED_OFFLOADED":
      return "stopped";
    case "PAUSED":
    case "SUSPENDED":
      return "stopped";
    case "DELETED":
      return "deleted";
    case "ERROR":
      return "failed";
    case "REBOOT":
    case "HARD_REBOOT":
    case "PASSWORD":
    case "RESIZE":
    case "REVERT_RESIZE":
    case "VERIFY_RESIZE":
      return "resetting";
    default:
      return "queued";
  }
}

function shouldPollStatus(status) {
  return ["queued", "creating", "resetting", "deleting"].includes(status);
}

function findConsoleUrl(outputs) {
  const consoleOutput = outputs.find((output) => (
    ["console_url", "novnc_console_url", "web_console_url"].includes(String(output.key).toLowerCase())
  ));
  if (consoleOutput?.value) return consoleOutput.value;

  const consoleUrlsOutput = outputs.find((output) => String(output.key).toLowerCase() === "console_urls");
  return consoleUrlsOutput?.value?.novnc || "";
}

function findOutputValue(outputs, key) {
  return outputs.find((output) => String(output.key).toLowerCase() === key)?.value;
}

class OpenStackHttpError extends Error {
  constructor(message, { status, body }) {
    super(message);
    this.name = "OpenStackHttpError";
    this.status = status >= 400 && status < 600 ? status : 500;
    this.openStackStatus = status;
    this.body = body;
  }
}

function stringifyErrorBody(body) {
  if (!body) return "";
  if (typeof body === "string") return body;
  try {
    return JSON.stringify(body);
  } catch {
    return String(body);
  }
}

function deploymentErrorMessage(error) {
  const bodyText = stringifyErrorBody(error.body);
  const statusText = error.openStackStatus ? `OpenStack HTTP ${error.openStackStatus}` : "";
  return [error.message, statusText, bodyText].filter(Boolean).join(" - ");
}

function missingHeatEndpointError() {
  const error = new Error("Keystone catalog did not include an orchestration endpoint; set LAB_HEAT_ENDPOINT");
  error.code = "OPENSTACK_HEAT_ENDPOINT_MISSING";
  return error;
}

function isMissingHeatEndpointError(error) {
  return error?.code === "OPENSTACK_HEAT_ENDPOINT_MISSING"
    || error?.message === "Keystone catalog did not include an orchestration endpoint; set LAB_HEAT_ENDPOINT";
}

function firstAddress(addresses = {}) {
  for (const networkAddresses of Object.values(addresses)) {
    const address = networkAddresses?.find((candidate) => candidate?.addr)?.addr;
    if (address) return address;
  }

  return "";
}

function directServerOutputs(server, consoleUrl = "") {
  return [
    { key: "server_id", value: server?.id, description: "Nova server ID", sensitive: false },
    { key: "vm_name", value: server?.name, description: "Server name", sensitive: false },
    { key: "status", value: server?.status, description: "Nova server status", sensitive: false },
    { key: "fixed_ip", value: firstAddress(server?.addresses), description: "First fixed IP", sensitive: false },
    { key: "addresses", value: server?.addresses, description: "Nova addresses", sensitive: false },
    { key: "novnc_console_url", value: consoleUrl, description: "noVNC console", sensitive: false }
  ].filter((output) => output.value !== undefined && output.value !== null && output.value !== "");
}

export class OpenStackHeatClient {
  constructor({ env = process.env, fetchImpl = fetch } = {}) {
    this.env = env;
    this.fetch = fetchImpl;
    this.token = null;
  }

  readConfig() {
    const authType = normalizeAuthType(firstEnv(this.env, ["OS_AUTH_TYPE"]) || (
      firstEnv(this.env, ["OS_APPLICATION_CREDENTIAL_ID"]) ? "application_credential" : "password"
    ));

    return {
      authType,
      authUrl: normalizeAuthUrl(requiredEnv(this.env, ["OS_AUTH_URL"])),
      applicationCredentialId: firstEnv(this.env, ["OS_APPLICATION_CREDENTIAL_ID"]),
      applicationCredentialSecret: firstEnv(this.env, ["OS_APPLICATION_CREDENTIAL_SECRET"]),
      username: firstEnv(this.env, ["OS_USERNAME"]),
      password: firstEnv(this.env, ["OS_PASSWORD"]),
      userDomainName: firstEnv(this.env, ["OS_USER_DOMAIN_NAME"]) || "Default",
      projectName: firstEnv(this.env, ["OS_PROJECT_NAME", "OS_TENANT_NAME"]),
      projectDomainName: firstEnv(this.env, ["OS_PROJECT_DOMAIN_NAME"]) || "Default",
      projectId: firstEnv(this.env, ["OS_PROJECT_ID", "OS_TENANT_ID"]),
      regionName: firstEnv(this.env, ["OS_REGION_NAME"]) || "RegionOne",
      interfaceName: normalizeInterfaceName(firstEnv(this.env, ["OS_INTERFACE", "OS_ENDPOINT_TYPE"])),
      heatEndpoint: firstEnv(this.env, ["LAB_HEAT_ENDPOINT", "OS_ORCHESTRATION_URL"]),
      imageEndpoint: firstEnv(this.env, ["LAB_IMAGE_ENDPOINT", "OS_IMAGE_URL"]),
      networkEndpoint: firstEnv(this.env, ["LAB_NETWORK_ENDPOINT", "OS_NETWORK_URL"])
    };
  }

  authPayload(config) {
    if (config.authType === "password") {
      if (!config.username || !config.password) {
        throw new Error("Missing required environment variable: OS_USERNAME or OS_PASSWORD");
      }

      const projectScope = config.projectId
        ? { id: config.projectId }
        : {
          name: requiredEnv(this.env, ["OS_PROJECT_NAME", "OS_TENANT_NAME"]),
          domain: optionalDomain(this.env, ["OS_PROJECT_DOMAIN_NAME"])
        };

      return {
        auth: {
          identity: {
            methods: ["password"],
            password: {
              user: compactObject({
                name: config.username,
                password: config.password,
                domain: { name: config.userDomainName }
              })
            }
          },
          scope: {
            project: projectScope
          }
        }
      };
    }

    if (!config.applicationCredentialId || !config.applicationCredentialSecret) {
      throw new Error(
        "Missing required environment variable: OS_APPLICATION_CREDENTIAL_ID or OS_APPLICATION_CREDENTIAL_SECRET"
      );
    }

    return {
      auth: {
        identity: {
          methods: ["application_credential"],
          application_credential: {
            id: config.applicationCredentialId,
            secret: config.applicationCredentialSecret
          }
        }
      }
    };
  }

  async authenticate() {
    const config = this.readConfig();
    const cachedToken = this.token;
    if (cachedToken && Date.parse(cachedToken.expiresAt) - Date.now() > 300_000) {
      return cachedToken;
    }

    const response = await this.fetch(joinUrl(config.authUrl, "auth/tokens"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(this.authPayload(config))
    });

    const { body, text } = await parseResponseBody(response);
    if (!response.ok) {
      throw new OpenStackHttpError(`Keystone authentication failed (${response.status})`, {
        status: response.status,
        body: body || text
      });
    }

    const tokenId = response.headers.get("x-subject-token");
    if (!tokenId) {
      throw new Error("Keystone did not return X-Subject-Token");
    }

    const projectId = config.projectId || body?.token?.project?.id;
    if (!projectId) {
      throw new Error("Keystone token did not include a project id; set OS_PROJECT_ID");
    }

    this.token = {
      id: tokenId,
      expiresAt: body?.token?.expires_at || new Date(Date.now() + 60 * 60 * 1000).toISOString(),
      catalog: body?.token?.catalog || [],
      projectId,
      regionName: config.regionName,
      interfaceName: config.interfaceName,
      heatEndpoint: config.heatEndpoint,
      imageEndpoint: config.imageEndpoint,
      networkEndpoint: config.networkEndpoint
    };

    return this.token;
  }

  async heatEndpoint() {
    const token = await this.authenticate();
    const catalogEndpoint = token.heatEndpoint
      || serviceEndpointFromCatalog(token.catalog, "orchestration", token.interfaceName, token.regionName)
      || serviceEndpointFromCatalog(token.catalog, "heat", token.interfaceName, token.regionName);

    if (!catalogEndpoint) {
      throw missingHeatEndpointError();
    }

    return normalizeHeatEndpoint(catalogEndpoint, token.projectId);
  }

  async computeEndpoint() {
    const token = await this.authenticate();
    const catalogEndpoint = serviceEndpointFromCatalog(token.catalog, "compute", token.interfaceName, token.regionName)
      || serviceEndpointFromCatalog(token.catalog, "nova", token.interfaceName, token.regionName);

    if (!catalogEndpoint) {
      throw new Error("Keystone catalog did not include a compute endpoint");
    }

    return normalizeComputeEndpoint(catalogEndpoint, token.projectId);
  }

  async imageEndpoint() {
    const token = await this.authenticate();
    const catalogEndpoint = token.imageEndpoint
      || serviceEndpointFromCatalog(token.catalog, "image", token.interfaceName, token.regionName)
      || serviceEndpointFromCatalog(token.catalog, "glance", token.interfaceName, token.regionName);

    if (!catalogEndpoint) {
      throw new Error("Keystone catalog did not include an image endpoint");
    }

    return normalizeImageEndpoint(catalogEndpoint);
  }

  async networkEndpoint() {
    const token = await this.authenticate();
    const catalogEndpoint = token.networkEndpoint
      || serviceEndpointFromCatalog(token.catalog, "network", token.interfaceName, token.regionName)
      || serviceEndpointFromCatalog(token.catalog, "neutron", token.interfaceName, token.regionName);

    if (!catalogEndpoint) {
      throw new Error("Keystone catalog did not include a network endpoint");
    }

    return normalizeNetworkEndpoint(catalogEndpoint);
  }

  async request(path, options = {}) {
    const token = await this.authenticate();
    const heatEndpoint = await this.heatEndpoint();
    const response = await this.fetch(joinUrl(heatEndpoint, path), {
      ...options,
      headers: compactObject({
        "X-Auth-Token": token.id,
        "Content-Type": options.body ? "application/json" : undefined,
        Accept: "application/json",
        ...(options.headers || {})
      })
    });
    const { body, text } = await parseResponseBody(response);

    if (!response.ok) {
      throw new OpenStackHttpError(`OpenStack Heat request failed (${response.status})`, {
        status: response.status,
        body: body || text
      });
    }

    return body;
  }

  async computeRequest(path, options = {}) {
    const token = await this.authenticate();
    const computeEndpoint = await this.computeEndpoint();
    const response = await this.fetch(joinUrl(computeEndpoint, path), {
      ...options,
      headers: compactObject({
        "X-Auth-Token": token.id,
        "Content-Type": options.body ? "application/json" : undefined,
        Accept: "application/json",
        ...(options.headers || {})
      })
    });
    const { body, text } = await parseResponseBody(response);

    if (!response.ok) {
      throw new OpenStackHttpError(`OpenStack Nova request failed (${response.status})`, {
        status: response.status,
        body: body || text
      });
    }

    return body;
  }

  async imageRequest(path, options = {}) {
    const token = await this.authenticate();
    const imageEndpoint = await this.imageEndpoint();
    const response = await this.fetch(joinUrl(imageEndpoint, path), {
      ...options,
      headers: compactObject({
        "X-Auth-Token": token.id,
        "Content-Type": options.body ? "application/json" : undefined,
        Accept: "application/json",
        ...(options.headers || {})
      })
    });
    const { body, text } = await parseResponseBody(response);

    if (!response.ok) {
      throw new OpenStackHttpError(`OpenStack Glance request failed (${response.status})`, {
        status: response.status,
        body: body || text
      });
    }

    return body;
  }

  async networkRequest(path, options = {}) {
    const token = await this.authenticate();
    const networkEndpoint = await this.networkEndpoint();
    const response = await this.fetch(joinUrl(networkEndpoint, path), {
      ...options,
      headers: compactObject({
        "X-Auth-Token": token.id,
        "Content-Type": options.body ? "application/json" : undefined,
        Accept: "application/json",
        ...(options.headers || {})
      })
    });
    const { body, text } = await parseResponseBody(response);

    if (!response.ok) {
      throw new OpenStackHttpError(`OpenStack Neutron request failed (${response.status})`, {
        status: response.status,
        body: body || text
      });
    }

    return body;
  }

  async resolveImageId(identifier) {
    if (!identifier) throw new Error("Nova direct deployment requires an image parameter");

    const byName = await this.imageRequest(`/images?name=${encodeURIComponent(identifier)}`);
    const image = byName.images?.find((candidate) => candidate.name === identifier || candidate.id === identifier);
    if (image?.id) return image.id;

    try {
      const byId = await this.imageRequest(`/images/${encodeURIComponent(identifier)}`);
      if (byId.image?.id) return byId.image.id;
      if (byId.id) return byId.id;
    } catch (error) {
      if (error.openStackStatus !== 404) throw error;
    }

    throw new Error(`Glance image not found: ${identifier}`);
  }

  async resolveFlavorId(identifier) {
    if (!identifier) throw new Error("Nova direct deployment requires a flavor parameter");

    const body = await this.computeRequest("/flavors/detail");
    const flavor = body.flavors?.find((candidate) => candidate.name === identifier || candidate.id === identifier);
    if (flavor?.id) return flavor.id;

    throw new Error(`Nova flavor not found: ${identifier}`);
  }

  async resolveNetworkId(identifier) {
    if (!identifier) {
      throw new Error("Nova direct deployment requires a network parameter or LAB_NOVA_NETWORK");
    }

    const byName = await this.networkRequest(`/networks?name=${encodeURIComponent(identifier)}`);
    const network = byName.networks?.find((candidate) => candidate.name === identifier || candidate.id === identifier);
    if (network?.id) return network.id;

    try {
      const byId = await this.networkRequest(`/networks/${encodeURIComponent(identifier)}`);
      if (byId.network?.id) return byId.network.id;
    } catch (error) {
      if (error.openStackStatus !== 404) throw error;
    }

    throw new Error(`Neutron network not found: ${identifier}`);
  }

  async createStack({ stackName, templateSource, parameters, tags, timeoutMins, disableRollback }) {
    const body = await this.request("/stacks", {
      method: "POST",
      body: JSON.stringify(compactObject({
        stack_name: stackName,
        template: templateSource,
        parameters,
        timeout_mins: timeoutMins,
        disable_rollback: disableRollback,
        tags
      }))
    });

    return body.stack;
  }

  async showStack(stackName, stackId) {
    return (await this.request(`/stacks/${encodeURIComponent(stackName)}/${encodeURIComponent(stackId)}`)).stack;
  }

  async deleteStack(stackName, stackId) {
    await this.request(`/stacks/${encodeURIComponent(stackName)}/${encodeURIComponent(stackId)}`, {
      method: "DELETE"
    });
  }

  async stackAction(stackName, stackId, action) {
    await this.request(`/stacks/${encodeURIComponent(stackName)}/${encodeURIComponent(stackId)}/actions`, {
      method: "POST",
      body: JSON.stringify({ [action]: null })
    });
  }

  async createDirectServer({ serverName, parameters, metadata = {} }) {
    const [imageRef, flavorRef, networkId] = await Promise.all([
      this.resolveImageId(parameters.image || parameters.image_id || parameters.image_name),
      this.resolveFlavorId(parameters.flavor || parameters.flavor_id || parameters.flavor_name || parameters.instance_type),
      this.resolveNetworkId(parameters.network || parameters.network_id || parameters.network_name || parameters.private_network)
    ]);

    const body = await this.computeRequest("/servers", {
      method: "POST",
      body: JSON.stringify({
        server: compactObject({
          name: serverName,
          imageRef,
          flavorRef,
          key_name: parameters.key_name || parameters.ssh_key_name,
          networks: [{ uuid: networkId }],
          metadata: compactObject(metadata)
        })
      })
    });

    return body.server;
  }

  async showServer(serverId) {
    return (await this.computeRequest(`/servers/${encodeURIComponent(serverId)}`)).server;
  }

  async deleteServer(serverId) {
    await this.computeRequest(`/servers/${encodeURIComponent(serverId)}`, {
      method: "DELETE"
    });
  }

  async serverAction(serverId, action) {
    await this.computeRequest(`/servers/${encodeURIComponent(serverId)}/action`, {
      method: "POST",
      body: JSON.stringify({ [action]: null })
    });
  }

  async createNoVncConsole(serverId) {
    try {
      const body = await this.computeRequest(`/servers/${encodeURIComponent(serverId)}/remote-consoles`, {
        method: "POST",
        headers: { "OpenStack-API-Version": "compute 2.6" },
        body: JSON.stringify({
          remote_console: {
            protocol: "vnc",
            type: "novnc"
          }
        })
      });
      return body?.remote_console?.url || "";
    } catch (error) {
      const body = await this.computeRequest(`/servers/${encodeURIComponent(serverId)}/action`, {
        method: "POST",
        body: JSON.stringify({
          "os-getVNCConsole": {
            type: "novnc"
          }
        })
      });
      return body?.console?.url || "";
    }
  }

  async health() {
    const token = await this.authenticate();
    const heatEndpoint = await this.heatEndpoint();
    await this.request("/stacks?limit=1");
    return {
      status: "openstack",
      reachable: true,
      authenticated: true,
      projectId: token.projectId,
      region: token.regionName,
      interface: token.interfaceName,
      heatEndpoint
    };
  }

  async novaHealth({ heatError = "" } = {}) {
    const token = await this.authenticate();
    const [computeEndpoint, imageEndpoint, networkEndpoint] = await Promise.all([
      this.computeEndpoint(),
      this.imageEndpoint(),
      this.networkEndpoint()
    ]);
    await Promise.all([
      this.computeRequest("/flavors/detail"),
      this.imageRequest("/images?limit=1"),
      this.networkRequest("/networks?limit=1")
    ]);

    return {
      status: "openstack-nova",
      reachable: true,
      authenticated: true,
      projectId: token.projectId,
      region: token.regionName,
      interface: token.interfaceName,
      computeEndpoint,
      imageEndpoint,
      networkEndpoint,
      heatEndpoint: null,
      heatError
    };
  }
}

async function parseResponseBody(response) {
  const text = await response.text();
  if (!text) return { body: null, text };

  try {
    return { body: JSON.parse(text), text };
  } catch {
    return { body: null, text };
  }
}

function deploymentVisibleTo(user, deployment) {
  return user.role === "admin" || deployment.user.id === user.id;
}

export class OpenStackHeatProvider {
  constructor({ env = process.env, client = new OpenStackHeatClient({ env }) } = {}) {
    this.env = env;
    this.client = client;
    this.deployments = new Map();
    this.pollIntervalMs = Number(env.LAB_STACK_POLL_INTERVAL_MS || 5000);
    this.createTimeoutMins = Number(env.LAB_HEAT_TIMEOUT_MINS || 60);
    this.disableRollback = readBool(env.LAB_HEAT_DISABLE_ROLLBACK, false);
    this.deploymentMode = normalizeDeploymentMode(
      firstEnv(env, ["LAB_OPENSTACK_DEPLOYMENT_MODE", "LAB_OPENSTACK_DEPLOY_MODE"])
    );
  }

  listLabs() {
    return labs.filter((lab) => lab.enabled);
  }

  getLab(labId) {
    return findEnabledLab(labId);
  }

  listDeployments(user) {
    return [...this.deployments.values()]
      .filter((deployment) => deploymentVisibleTo(user, deployment))
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  getDeployment(user, deploymentId) {
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
    const stackName = stackNameFor(user, lab, id.slice(0, 8));
    const deployment = {
      id,
      lab,
      user,
      provider: "openstack",
      openStackEngine: this.deploymentMode === "nova" ? "nova" : "heat",
      heatStackName: stackName,
      heatStackId: null,
      serverId: null,
      status: "queued",
      outputs: [],
      lastError: null,
      expiresAt: addMinutes(lab.defaultTtlMinutes),
      createdAt: nowIso(),
      updatedAt: nowIso(),
      deletedAt: null
    };

    this.deployments.set(id, deployment);
    const createPromise = this.deploymentMode === "nova"
      ? this.createNovaServerForDeployment(id, stackName)
      : this.createStackForDeployment(id, stackName);

    createPromise.catch((error) => this.failDeployment(id, error));
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

    const deletePromise = deployment.openStackEngine === "nova" || deployment.serverId
      ? this.deleteNovaServerForDeployment(deploymentId, deployment.serverId)
      : this.deleteStackForDeployment(deploymentId, deployment.heatStackName, deployment.heatStackId);

    deletePromise
      .catch((error) => this.failDeployment(deploymentId, error));

    return this.deployments.get(deploymentId);
  }

  stopDeployment(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;
    if (deployment.openStackEngine === "nova" || deployment.serverId) {
      if (!deployment.serverId) return deployment;

      this.deployments.set(deploymentId, {
        ...deployment,
        status: "resetting",
        updatedAt: nowIso()
      });

      this.client.serverAction(deployment.serverId, "os-stop")
        .then(() => this.scheduleStatusPoll(deploymentId, this.pollIntervalMs))
        .catch((error) => this.failDeployment(deploymentId, error));

      return this.deployments.get(deploymentId);
    }

    if (!deployment.heatStackId) return deployment;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "resetting",
      updatedAt: nowIso()
    });

    this.client.stackAction(deployment.heatStackName, deployment.heatStackId, "suspend")
      .then(() => this.scheduleStatusPoll(deploymentId, this.pollIntervalMs))
      .catch((error) => this.failDeployment(deploymentId, error));

    return this.deployments.get(deploymentId);
  }

  startDeployment(user, deploymentId) {
    const deployment = this.getDeployment(user, deploymentId);
    if (!deployment) return null;
    if (deployment.openStackEngine === "nova" || deployment.serverId) {
      if (!deployment.serverId) return deployment;

      this.deployments.set(deploymentId, {
        ...deployment,
        status: "resetting",
        updatedAt: nowIso()
      });

      this.client.serverAction(deployment.serverId, "os-start")
        .then(() => this.scheduleStatusPoll(deploymentId, this.pollIntervalMs))
        .catch((error) => this.failDeployment(deploymentId, error));

      return this.deployments.get(deploymentId);
    }

    if (!deployment.heatStackId) return deployment;

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "resetting",
      updatedAt: nowIso()
    });

    this.client.stackAction(deployment.heatStackName, deployment.heatStackId, "resume")
      .then(() => this.scheduleStatusPoll(deploymentId, this.pollIntervalMs))
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
      lastError: null,
      updatedAt: nowIso()
    });

    const resetPromise = deployment.openStackEngine === "nova" || deployment.serverId
      ? this.resetNovaServerForDeployment(deploymentId, deployment.serverId)
      : this.resetStackForDeployment(deploymentId, deployment.heatStackName, deployment.heatStackId);

    resetPromise
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
    return next;
  }

  async getHealth() {
    try {
      if (this.deploymentMode === "nova") {
        return await this.client.novaHealth();
      }

      return await this.client.health();
    } catch (error) {
      if (this.deploymentMode === "auto" && isMissingHeatEndpointError(error)) {
        try {
          return await this.client.novaHealth({ heatError: error.message });
        } catch (novaError) {
          return {
            status: "openstack-nova",
            reachable: false,
            authenticated: false,
            error: deploymentErrorMessage(novaError),
            heatError: error.message
          };
        }
      }

      return {
        status: "openstack",
        reachable: false,
        authenticated: false,
        error: error.message
      };
    }
  }

  async createStackForDeployment(deploymentId, expectedStackName = null) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted" || deployment.status === "deleting") return;
    if (expectedStackName && deployment.heatStackName !== expectedStackName) return;

    const templatePath = this.resolveLabTemplatePath(deployment.lab);
    const templateSource = readFileSync(templatePath, "utf8");
    const parameters = buildHeatParameters({ env: this.env, lab: deployment.lab, templateSource });
    const stackName = deployment.heatStackName;

    console.info(`[openstack] creating stack ${stackName}`, {
      deploymentId,
      labId: deployment.lab.id,
      templatePath,
      parameters
    });

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "creating",
      updatedAt: nowIso()
    });

    let stack;
    try {
      stack = await this.client.createStack({
        stackName,
        templateSource,
        parameters,
        tags: `lab-portal,lab:${deployment.lab.id},user:${deployment.user.id}`,
        timeoutMins: this.createTimeoutMins,
        disableRollback: this.disableRollback
      });
    } catch (error) {
      if (this.deploymentMode === "auto" && isMissingHeatEndpointError(error)) {
        console.info(`[openstack] Heat endpoint missing; creating Nova server ${stackName}`, {
          deploymentId,
          labId: deployment.lab.id
        });
        await this.createNovaServerForDeployment(deploymentId, expectedStackName);
        return;
      }

      throw error;
    }

    const stackId = stack?.id || null;
    if (!stackId) {
      throw new Error("Heat create stack response did not include stack.id");
    }

    const current = this.deployments.get(deploymentId);
    if (
      !current
      || current.heatStackName !== stackName
      || current.status === "deleted"
      || current.status === "deleting"
    ) {
      await this.client.deleteStack(stackName, stackId).catch(() => {});
      return;
    }

    this.deployments.set(deploymentId, {
      ...current,
      heatStackId: stackId,
      updatedAt: nowIso()
    });
    console.info(`[openstack] stack create accepted ${stackName}`, { deploymentId, stackId });
    this.scheduleStatusPoll(deploymentId, this.pollIntervalMs);
  }

  async createNovaServerForDeployment(deploymentId, expectedServerName = null) {
    const deployment = this.deployments.get(deploymentId);
    if (!deployment || deployment.status === "deleted" || deployment.status === "deleting") return;
    if (expectedServerName && deployment.heatStackName !== expectedServerName) return;

    const templatePath = this.resolveLabTemplatePath(deployment.lab);
    const templateSource = readFileSync(templatePath, "utf8");
    const templateParameters = buildHeatParameters({ env: this.env, lab: deployment.lab, templateSource });
    const networkOverride = firstEnv(this.env, ["LAB_NOVA_NETWORK", "LAB_HEAT_NETWORK"]);
    const parameters = networkOverride
      ? { ...templateParameters, network: networkOverride }
      : templateParameters;
    const serverName = deployment.heatStackName;

    console.info(`[openstack] creating Nova server ${serverName}`, {
      deploymentId,
      labId: deployment.lab.id,
      templatePath,
      parameters
    });

    this.deployments.set(deploymentId, {
      ...deployment,
      openStackEngine: "nova",
      status: "creating",
      updatedAt: nowIso()
    });

    const server = await this.client.createDirectServer({
      serverName,
      parameters,
      metadata: {
        lab_portal: "true",
        lab_portal_deployment_id: deploymentId,
        lab_id: deployment.lab.id,
        user_id: deployment.user.id
      }
    });
    const serverId = server?.id || null;
    if (!serverId) {
      throw new Error("Nova create server response did not include server.id");
    }

    const current = this.deployments.get(deploymentId);
    if (
      !current
      || current.heatStackName !== serverName
      || current.status === "deleted"
      || current.status === "deleting"
    ) {
      await this.client.deleteServer(serverId).catch(() => {});
      return;
    }

    this.deployments.set(deploymentId, {
      ...current,
      openStackEngine: "nova",
      serverId,
      outputs: directServerOutputs({ id: serverId, name: serverName, status: "BUILD" }),
      updatedAt: nowIso()
    });
    console.info(`[openstack] Nova server create accepted ${serverName}`, { deploymentId, serverId });
    this.scheduleStatusPoll(deploymentId, this.pollIntervalMs);
  }

  resolveLabTemplatePath(lab) {
    const labKey = envKeySuffix(lab.slug || lab.id);
    const templatePath = firstEnv(this.env, [
      `LAB_${labKey}_HEAT_TEMPLATE`,
      "LAB_HEAT_TEMPLATE_PATH"
    ]) || lab.heatTemplatePath;

    return safeResolveTemplatePath(templatePath);
  }

  async resetStackForDeployment(deploymentId, oldStackName, oldStackId) {
    if (oldStackId) {
      await this.client.deleteStack(oldStackName, oldStackId);
      await this.waitForStackDelete(oldStackName, oldStackId);
    }

    const current = this.deployments.get(deploymentId);
    if (!current || current.status === "deleted" || current.status === "deleting") return;

    const nextStackName = stackNameFor(current.user, current.lab, randomUUID().slice(0, 8));
    this.deployments.set(deploymentId, {
      ...current,
      heatStackName: nextStackName,
      heatStackId: null,
      serverId: null,
      status: "queued",
      updatedAt: nowIso()
    });

    await this.createStackForDeployment(deploymentId, nextStackName);
  }

  async resetNovaServerForDeployment(deploymentId, oldServerId) {
    if (oldServerId) {
      await this.client.deleteServer(oldServerId);
      await this.waitForServerDelete(oldServerId);
    }

    const current = this.deployments.get(deploymentId);
    if (!current || current.status === "deleted" || current.status === "deleting") return;

    const nextServerName = stackNameFor(current.user, current.lab, randomUUID().slice(0, 8));
    this.deployments.set(deploymentId, {
      ...current,
      heatStackName: nextServerName,
      heatStackId: null,
      serverId: null,
      openStackEngine: "nova",
      status: "queued",
      updatedAt: nowIso()
    });

    await this.createNovaServerForDeployment(deploymentId, nextServerName);
  }

  async deleteStackForDeployment(deploymentId, stackName, stackId) {
    if (stackId) {
      await this.client.deleteStack(stackName, stackId);
      await this.waitForStackDelete(stackName, stackId);
    }

    const current = this.deployments.get(deploymentId);
    if (!current) return;
    this.deployments.set(deploymentId, {
      ...current,
      status: "deleted",
      deletedAt: nowIso(),
      updatedAt: nowIso()
    });
  }

  async deleteNovaServerForDeployment(deploymentId, serverId) {
    if (serverId) {
      await this.client.deleteServer(serverId);
      await this.waitForServerDelete(serverId);
    }

    const current = this.deployments.get(deploymentId);
    if (!current) return;
    this.deployments.set(deploymentId, {
      ...current,
      status: "deleted",
      deletedAt: nowIso(),
      updatedAt: nowIso()
    });
  }

  async waitForStackDelete(stackName, stackId) {
    for (let attempt = 0; attempt < 120; attempt += 1) {
      try {
        const stack = await this.client.showStack(stackName, stackId);
        if (stack?.stack_status === "DELETE_COMPLETE") return;
        if (stack?.stack_status === "DELETE_FAILED") {
          throw new Error(stack.stack_status_reason || "Stack delete failed");
        }
      } catch (error) {
        if (error.openStackStatus === 404) return;
        throw error;
      }

      await new Promise((resolvePromise) => setTimeout(resolvePromise, this.pollIntervalMs));
    }

    throw new Error("Timed out waiting for stack deletion");
  }

  async waitForServerDelete(serverId) {
    for (let attempt = 0; attempt < 120; attempt += 1) {
      try {
        const server = await this.client.showServer(serverId);
        if (server?.status === "DELETED") return;
      } catch (error) {
        if (error.openStackStatus === 404) return;
        throw error;
      }

      await new Promise((resolvePromise) => setTimeout(resolvePromise, this.pollIntervalMs));
    }

    throw new Error("Timed out waiting for server deletion");
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
    if (!deployment || deployment.status === "deleted") return;

    if (deployment.openStackEngine === "nova" || deployment.serverId) {
      await this.refreshNovaDeploymentStatus(deploymentId, deployment);
      return;
    }

    if (!deployment.heatStackId) return;

    const stack = await this.client.showStack(deployment.heatStackName, deployment.heatStackId);
    const outputs = heatOutputsToPortalOutputs(stack.outputs);
    const status = stackStatusToPortalStatus(stack.stack_status);
    const consoleUrl = findConsoleUrl(outputs)
      || (
        status === "ready" && findOutputValue(outputs, "server_id")
          ? await this.client.createNoVncConsole(findOutputValue(outputs, "server_id")).catch(() => "")
          : ""
      );
    const nextOutputs = consoleUrl && !findConsoleUrl(outputs)
      ? [...outputs, { key: "novnc_console_url", value: consoleUrl, description: "noVNC console", sensitive: false }]
      : outputs;
    const next = {
      ...deployment,
      status,
      outputs: nextOutputs,
      consoleUrl,
      lastError: status === "failed" ? stack.stack_status_reason || null : null,
      deletedAt: status === "deleted" ? nowIso() : deployment.deletedAt,
      updatedAt: nowIso()
    };

    this.deployments.set(deploymentId, next);
    if (status !== deployment.status || status === "failed") {
      console.info(`[openstack] stack ${deployment.heatStackName} status ${stack.stack_status}`, {
        deploymentId,
        portalStatus: status,
        reason: stack.stack_status_reason || ""
      });
    }
    if (shouldPollStatus(status)) {
      this.scheduleStatusPoll(deploymentId, this.pollIntervalMs);
    }
  }

  async refreshNovaDeploymentStatus(deploymentId, deployment) {
    if (!deployment.serverId) return;

    const server = await this.client.showServer(deployment.serverId);
    const status = serverStatusToPortalStatus(server.status);
    const consoleUrl = status === "ready"
      ? await this.client.createNoVncConsole(deployment.serverId).catch(() => deployment.consoleUrl || "")
      : deployment.consoleUrl || "";
    const outputs = directServerOutputs(server, consoleUrl);
    const next = {
      ...deployment,
      status,
      outputs,
      consoleUrl,
      lastError: status === "failed" ? server.fault?.message || server["OS-EXT-STS:task_state"] || "Nova server entered ERROR" : null,
      deletedAt: status === "deleted" ? nowIso() : deployment.deletedAt,
      updatedAt: nowIso()
    };

    this.deployments.set(deploymentId, next);
    if (status !== deployment.status || status === "failed") {
      console.info(`[openstack] Nova server ${deployment.heatStackName} status ${server.status}`, {
        deploymentId,
        portalStatus: status,
        reason: next.lastError || ""
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

    console.error(`[openstack] deployment failed ${deployment.heatStackName}`, {
      deploymentId,
      labId: deployment.lab.id,
      status: error.openStackStatus || null,
      message,
      body: error.body || null
    });

    this.deployments.set(deploymentId, {
      ...deployment,
      status: "failed",
      lastError: message,
      updatedAt: nowIso()
    });
  }
}
