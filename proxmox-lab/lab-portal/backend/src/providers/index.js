import { MockLabProvider } from "./mockLabProvider.js";
import { ProxmoxLabProvider } from "./proxmoxLabProvider.js";

export function createLabProvider() {
  const providerName = process.env.LAB_PROVIDER || "mock";

  if (providerName === "mock") {
    return new MockLabProvider();
  }

  if (providerName === "proxmox") {
    return new ProxmoxLabProvider();
  }

  throw new Error(`Unsupported LAB_PROVIDER: ${providerName}`);
}
