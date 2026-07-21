# OpenStack Lab Portal

Npm project for an OpenStack lab portal that runs as a web app and as an Electron desktop app.

## Structure

```text
lab-portal/
├── backend/         # Express API and mock provider
├── frontend/        # React + Vite
├── electron/        # Safe Electron shell
├── heat-templates/  # Initial Heat templates
└── package.json     # npm workspaces and common scripts
```

## Requirements

- Node.js 22.12 or newer.
- npm.
- OpenStack is not required for the first MVP; the backend starts with the `mock` provider.

## Installation

```bash
cd lab-portal
npm install
```

## Development

API and web together:

```bash
npm run dev
```

API only:

```bash
npm run dev:api
```

Web only:

```bash
npm run dev:web
```

Electron in development mode, after starting the web app:

```bash
npm run dev:desktop
```

Electron uses the API configured by `LAB_API_URL` or, if unset, `http://127.0.0.1:3001`.
For normal development, you can run everything with:

```bash
npm run dev
```

Then open the Electron window in another terminal:

```bash
npm run dev:desktop
```

By default, the API and Vite listen on `0.0.0.0` so they are reachable from other machines on the network. Open the app with:

```bash
http://<SERVER_IP>:5173
```

## Build

```bash
npm run build:web
npm run build:desktop
```

The desktop package is written to `dist-desktop/`. The packaged app loads the built UI from
`frontend/dist/` and calls the API at `LAB_API_URL` or `http://127.0.0.1:3001`. Before using the
packaged app, keep the API running:

```bash
npm run start:api
```

## Configuration

Useful variables:

```bash
PORT=3001
HOST=0.0.0.0
LAB_PROVIDER=mock
LAB_OPENSTACK_DEPLOYMENT_MODE=auto
CORS_ORIGIN=http://localhost:5173,http://127.0.0.1:5173
LAB_PORTAL_URL=http://127.0.0.1:5173
LAB_API_URL=http://127.0.0.1:3001
```

To allow direct API calls from any origin during development, use `CORS_ORIGIN=*`. Do not use that value in production.

The app does not call OpenStack from the frontend or Electron. To use OpenStack, the backend authenticates with Keystone using an application credential and creates stacks in Heat:

```bash
LAB_PROVIDER=openstack
OS_AUTH_URL=http://172.16.101.32/identity/v3
OS_REGION_NAME=RegionOne
OS_PROJECT_ID=<project-id>
OS_APPLICATION_CREDENTIAL_ID=<application-credential-id>
OS_APPLICATION_CREDENTIAL_SECRET=<application-credential-secret>
```

`LAB_OPENSTACK_DEPLOYMENT_MODE=auto` tries to create stacks with Heat. If Keystone does not publish Heat/orchestration in
the service catalog, the backend falls back to direct Nova deployment for single-VM labs that provide
`image`, `flavor`, and `network` parameters. You can also force it with:

```bash
LAB_OPENSTACK_DEPLOYMENT_MODE=nova
```

If Heat exists but is not in the catalog, set `LAB_HEAT_ENDPOINT`. For direct Nova deployment, use
`LAB_NOVA_NETWORK=<tenant-network-name-or-id>` when the template/lab does not provide a `network` parameter.
The provider also accepts these common parameters for Heat templates:

```bash
LAB_HEAT_IMAGE=<glance-image-name-or-id>
LAB_HEAT_FLAVOR=<nova-flavor-name-or-id>
LAB_HEAT_KEY_NAME=<keypair-name>
LAB_HEAT_EXTERNAL_NETWORK=<external-network-name-or-id>
```

You can override them by platform or lab with variables such as `LAB_LINUX_IMAGE`,
`LAB_WINDOWS_FLAVOR`, or `LAB_CCDC_WKST_UBUNTU_24_PARAM_IMAGE`. For a quick test with the
included deployable template:

```bash
LAB_CCDC_WKST_UBUNTU_24_HEAT_TEMPLATE=heat-templates/single-linux.yaml
```

The Ecom Ubuntu 24 lab is wired to `heat-templates/single-linux.yaml` and defaults to the
`ecom` Glance image and `ecom-3c-4g-50g` Nova flavor. The image must already exist in Glance;
create the flavor if it is not present:

```bash
openstack flavor create ecom-3c-4g-50g --vcpus 3 --ram 4096 --disk 50
```

Note: `heat-templates/mini-ccdc.yaml` is still a multi-VM placeholder. To create real machines,
point the lab to `single-linux.yaml` or replace `mini-ccdc.yaml` with a complete Heat template.

## Next Phase

1. Persist users and deployments in SQLite.
2. Validate Heat templates from a stricter allowlist.
3. Add TTL cleanup and per-user limits.
