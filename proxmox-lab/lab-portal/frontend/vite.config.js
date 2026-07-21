import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

function loadRootEnv() {
  const envPath = resolve(process.cwd(), "../.env");
  if (!existsSync(envPath)) return;

  for (const line of readFileSync(envPath, "utf8").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;

    const separatorIndex = trimmed.indexOf("=");
    if (separatorIndex === -1) continue;

    const key = trimmed.slice(0, separatorIndex).trim();
    if (process.env[key] !== undefined) continue;

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

function httpsOptions() {
  const keyPath = process.env.WEB_HTTPS_KEY_PATH;
  const certPath = process.env.WEB_HTTPS_CERT_PATH;
  if (process.env.WEB_PORT === "443" && (!keyPath || !certPath)) {
    throw new Error("WEB_PORT=443 requires WEB_HTTPS_KEY_PATH and WEB_HTTPS_CERT_PATH");
  }
  if (!keyPath || !certPath) return undefined;

  return {
    key: readFileSync(resolve(process.cwd(), "..", keyPath)),
    cert: readFileSync(resolve(process.cwd(), "..", certPath))
  };
}

loadRootEnv();

const webPort = Number(process.env.WEB_PORT || 5173);

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    allowedHosts: ["ccdclab.4rji.com"],
    port: webPort,
    strictPort: true,
    https: httpsOptions(),
    proxy: {
      "/api": {
        target: "http://127.0.0.1:3001",
        ws: true
      }
    }
  }
});
