import { MockLabProvider } from "./mockLabProvider.js";
import { OpenStackHeatProvider } from "./openStackHeatProvider.js";

export function createLabProvider() {
  const providerName = process.env.LAB_PROVIDER || "mock";

  if (providerName === "mock") {
    return new MockLabProvider();
  }

  if (providerName === "openstack") {
    return new OpenStackHeatProvider();
  }

  throw new Error(`Unsupported LAB_PROVIDER: ${providerName}`);
}
