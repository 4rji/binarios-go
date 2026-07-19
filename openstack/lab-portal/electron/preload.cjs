const { contextBridge } = require("electron");

contextBridge.exposeInMainWorld("labPortal", {
  apiBaseUrl: process.env.LAB_API_URL || process.env.LAB_API_BASE_URL || "http://127.0.0.1:3001",
  platform: process.platform
});
