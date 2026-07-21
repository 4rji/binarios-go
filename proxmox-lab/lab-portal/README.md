# Proxmox Lab Portal

Npm project for a Proxmox lab portal that runs as a web app and as an Electron desktop app.

## Structure

```text
lab-portal/
├── backend/      # Express API, mock provider, Proxmox provider, and tests
├── frontend/     # React + Vite
├── electron/     # Safe Electron shell
└── package.json  # npm workspaces and common scripts
```

## Requirements

- Node.js 22.12 or newer.
- npm.
- Proxmox VE API token for real deployments.

## Installation

```bash
cd lab-portal
npm install
```

## Development

```bash
npm run dev
npm run dev:api
npm run dev:web
npm run dev:desktop
```

To expose the web UI on standard ports, use:

```bash
npm run dev
npm run dev:80
npm run dev:443
```

`npm run dev` uses HTTP port `80` by default. Ports `80` and `443` usually require elevated permissions on macOS/Linux. For HTTPS on `443`, set `WEB_HTTPS_KEY_PATH` and `WEB_HTTPS_CERT_PATH` in `.env`; paths are relative to `lab-portal/`.

Electron uses the API configured by `LAB_API_URL` or, if unset, `http://127.0.0.1:3001`.

By default, the API and Vite listen on `0.0.0.0` so they are reachable from other machines on the network:

```bash
http://<SERVER_IP>:5173
```

## Build

```bash
npm run build:web
npm run build:desktop
```

The desktop package is written to `dist-desktop/`. The packaged app loads the built UI from
`frontend/dist/` and calls the API at `LAB_API_URL` or `http://127.0.0.1:3001`.

## Configuration

Useful variables:

```bash
PORT=3001
HOST=0.0.0.0
LAB_PROVIDER=mock
CORS_ORIGIN=http://localhost,http://127.0.0.1,http://localhost:5173,http://127.0.0.1:5173,http://ccdclab.4rji.com,https://ccdclab.4rji.com
LAB_PORTAL_URL=http://127.0.0.1
LAB_API_URL=http://127.0.0.1:3001
WEB_PORT=80
# WEB_HTTPS_KEY_PATH=certs/localhost-key.pem
# WEB_HTTPS_CERT_PATH=certs/localhost-cert.pem
```

To allow direct API calls from any origin during development, use `CORS_ORIGIN=*`. Do not use that value in production.

## Proxmox Provider

The app deploys temporary Proxmox VMs by cloning a template VM and deleting the clone when the lab is destroyed or reaches its two-hour TTL:

```bash
LAB_PROVIDER=proxmox
PROXMOX_API_URL=https://proxmox.example.local:8006
PROXMOX_NODE=pve
PROXMOX_API_USER=<api-user@realm>
PROXMOX_API_TOKEN_NAME=<token-name>
PROXMOX_API_TOKEN_VALUE=<token-secret>
PROXMOX_TEMPLATE_VMID=<template-vmid>
PROXMOX_CLONE_FULL=false
PROXMOX_TLS_INSECURE=true
```

Use `LAB_<LAB_SLUG>_PROXMOX_TEMPLATE_VMID` to map individual labs to different Proxmox templates, for example:

```bash
LAB_CCDC_WKST_UBUNTU_24_PROXMOX_TEMPLATE_VMID=9100
LAB_CCDC_WEBMAIL_FEDORA_42_PROXMOX_TEMPLATE_VMID=109
LAB_CCDC_SPLUNK_PROXMOX_TEMPLATE_VMID=104
```

The backend uses the API token for clone/start/stop/reset/delete. For embedded console access, it creates a temporary Proxmox VNC proxy ticket and bridges the noVNC WebSocket through the portal so users do not need to log in to the Proxmox web UI.

## Verification

```bash
npm test
npm run build:web
```
