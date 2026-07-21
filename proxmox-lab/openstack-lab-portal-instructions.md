# OpenStack Lab Portal Instructions for Codex

## Goal

Build a cross-platform lab launcher that can run as both a web application and an Electron desktop application. The app should let users log in, choose a lab from a catalog, deploy the assigned OpenStack resources, view status and connection details, and destroy or reset the lab when finished.

This is intended for future CCDC-style labs made of multiple VMs, networks, routers, security groups, and optional access helpers such as SSH, RDP, noVNC, or Apache Guacamole.

## Key Decision

Use Electron only as a client shell. Do not store OpenStack credentials, admin tokens, Heat templates with secrets, or cloud API logic inside the Electron renderer. The backend must own all OpenStack access.

Recommended shape:

```text
Web browser or Electron app
        |
        v
Backend API
        |
        v
OpenStack APIs: Keystone, Heat, Nova, Neutron, Glance
```

## Recommended Stack

- Frontend: React + Vite, or plain HTML/CSS/JS if the first MVP should stay very small.
- Desktop: Electron with `electron-builder`.
- Backend: Node.js/Express or FastAPI. Prefer FastAPI if Python OpenStack SDK is easier to use.
- Database: SQLite for MVP, PostgreSQL later.
- OpenStack automation: Heat templates first. Terraform can be evaluated later.
- VM access: start with IP/port display; add Guacamole/noVNC later.

## Suggested Repository Layout

```text
lab-portal/
├── backend/
│   ├── app/
│   ├── templates/
│   ├── config/
│   └── README.md
├── frontend/
│   ├── src/
│   └── package.json
├── electron/
│   ├── main.js
│   └── preload.js
├── heat-templates/
│   ├── single-linux.yaml
│   └── mini-ccdc.yaml
├── package.json
└── README.md
```

## MVP User Flow

1. User opens the web app or Electron app.
2. User logs in.
3. User sees a catalog of labs.
4. User clicks Deploy.
5. Backend creates a Heat stack using a unique stack name, such as `lab-user123-mini-ccdc`.
6. UI shows stack status: queued, creating, ready, failed, deleting.
7. UI shows VM names, fixed IPs, floating IPs when present, usernames, and connection buttons.
8. User can destroy the lab.
9. Backend automatically destroys expired labs based on TTL.

## MVP Screens

- Login
- Lab catalog
- Lab detail
- Active lab status
- Admin view for users, active stacks, errors, quotas, and cleanup

Avoid exposing Horizon directly to end users. Horizon is useful for administrators, but the goal is a simple "select lab, deploy, use, destroy" experience.

## Backend Responsibilities

- Authenticate users.
- Authorize which labs each user may deploy.
- Enforce quotas:
  - maximum active labs per user
  - maximum runtime
  - maximum vCPU/RAM/disk per user or team
- Create Heat stacks.
- Poll or receive stack status.
- Store lab deployment records.
- Destroy expired stacks.
- Return only safe connection details to the client.
- Keep OpenStack credentials in server-side environment variables or secret storage.

## Electron Responsibilities

- Load the same frontend used by the web app.
- Support two modes:
  - remote mode: `win.loadURL("https://labs.example.com")`
  - local/dev mode: `win.loadURL("http://localhost:5173")` or `win.loadFile("frontend/dist/index.html")`
- Use secure Electron defaults:
  - `nodeIntegration: false`
  - `contextIsolation: true`
  - use a small `preload.js` only if native desktop features are required
- Never call OpenStack directly from Electron.

## Example Root `package.json`

```json
{
  "main": "electron/main.js",
  "scripts": {
    "dev:web": "vite --host 0.0.0.0",
    "dev:desktop": "electron .",
    "build:web": "vite build",
    "build:desktop": "vite build && electron-builder",
    "serve-api": "python3 -m backend.app"
  },
  "devDependencies": {
    "electron": "^34.5.6",
    "electron-builder": "^25.1.8",
    "vite": "^6.0.0"
  }
}
```

Adjust versions when implementing. Check current package versions before installing.

## Suggested API Endpoints

```text
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/me

GET    /api/labs
GET    /api/labs/:labId

POST   /api/deployments
GET    /api/deployments
GET    /api/deployments/:deploymentId
POST   /api/deployments/:deploymentId/stop
POST   /api/deployments/:deploymentId/start
POST   /api/deployments/:deploymentId/reset
DELETE /api/deployments/:deploymentId

GET    /api/admin/deployments
GET    /api/admin/users
GET    /api/admin/openstack/health
```

## Suggested Data Model

```text
users
- id
- username
- password_hash or external_auth_id
- role
- created_at

labs
- id
- slug
- name
- description
- heat_template_path
- default_ttl_minutes
- enabled

deployments
- id
- user_id
- lab_id
- openstack_project_id
- heat_stack_name
- heat_stack_id
- status
- expires_at
- created_at
- deleted_at
- last_error

deployment_outputs
- id
- deployment_id
- key
- value
- sensitive
```

## Heat Template Expectations

Each lab should be defined as a Heat stack. A lab can include:

- Neutron networks
- Neutron subnets
- routers
- security groups
- Nova servers
- ports with fixed IPs
- floating IPs when needed
- cloud-init user data
- outputs with IPs and connection info

Example lab topology:

```text
mini-ccdc
├── dmz-net
│   ├── webmail
│   └── webserver
├── internal-net
│   ├── db
│   └── workstation
└── router
```

## OpenStack Strategy

For the first MVP, use one OpenStack project and one Heat stack per user deployment. This is simpler to build.

Later, consider one OpenStack project per user or team for stronger isolation and cleaner quotas.

Naming convention:

```text
lab-{username}-{lab_slug}-{short_random_id}
```

Example:

```text
lab-natasha-mini-ccdc-a8f32c
```

## Security Requirements

- Do not expose OpenStack admin credentials to the frontend or Electron app.
- Validate lab IDs and template paths on the backend. Do not accept arbitrary template paths from users.
- Use server-side allowlists for deployable labs.
- Use per-user quotas and TTL cleanup.
- Log stack creation, deletion, and errors.
- Treat lab output values as potentially sensitive.
- Do not store plaintext VM passwords unless unavoidable; prefer generated credentials stored securely and shown once.
- Keep CORS narrow in production.

## Development Phases

### Phase 1: Minimal Prototype

- Build a backend with hardcoded lab catalog.
- Add one simple Heat template for a single Ubuntu VM.
- Add a frontend with catalog, deploy button, status polling, and destroy button.
- Add Electron wrapper that loads the frontend.

### Phase 2: Real Lab Support

- Add mini CCDC Heat template with multiple VMs and networks.
- Add database-backed deployments.
- Add TTL cleanup worker.
- Add better status and error reporting.

### Phase 3: User Experience

- Add login and roles.
- Add connection buttons for SSH/RDP/web.
- Add Guacamole or noVNC integration.
- Add admin dashboard.

### Phase 4: Multi-User Hardening

- Add OpenStack project-per-team mode.
- Add quota enforcement at both app and OpenStack levels.
- Add audit logs.
- Add backup and restore for app database.
- Add production deployment docs.

## Local Development Notes

The implementation should support development without touching a real OpenStack cloud by providing a mock OpenStack provider. This lets the frontend and Electron app be built before OpenStack credentials are ready.

Provider abstraction:

```text
LabProvider
- list_labs()
- deploy_lab(user, lab_id)
- get_deployment(deployment_id)
- destroy_deployment(deployment_id)
```

Implementations:

```text
MockLabProvider
OpenStackHeatProvider
```

## What Codex Should Do First

When implementation starts:

1. Create a new project directory instead of modifying existing unrelated scripts.
2. Scaffold frontend, backend, and Electron wrapper.
3. Implement mock deployment mode first.
4. Add the OpenStack provider behind a configuration flag.
5. Add one Heat template only after the UI/API flow works.
6. Write a short README with setup, run, and build commands.

## Commands To Aim For

```bash
# Backend
cd backend
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload

# Frontend
npm install
npm run dev:web

# Electron
npm run dev:desktop

# Build desktop app
npm run build:desktop
```

## Open Questions For Future Implementation

- Should the backend be Python/FastAPI or Node/Express?
- Should users authenticate locally, through LDAP, or through Keycloak/OIDC?
- Should labs be isolated by Heat stack only, by OpenStack project, or by project per team?
- Should console access be via Guacamole, noVNC, direct RDP/SSH links, or all of them?
- Should the first production target be a single-node OpenStack lab or a multi-node OpenStack server?

