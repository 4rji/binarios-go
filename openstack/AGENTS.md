# Repository Guidelines

## Project Structure & Module Organization

This repository contains the OpenStack Lab Portal project under `lab-portal/`. The root `openstack-lab-portal-instructions.md` is the product and architecture brief.

Inside `lab-portal/`:

- `backend/`: Express API, mock lab provider, catalog data, and tests.
- `frontend/`: React + Vite web app source in `frontend/src/`.
- `electron/`: Electron main and preload scripts. Electron must only load the UI; do not add OpenStack API logic here.
- `heat-templates/`: Heat template files such as `single-linux.yaml`.
- `package.json`: npm workspace scripts for API, web, Electron, tests, and builds.

Generated folders such as `node_modules/`, `frontend/dist/`, and `dist-desktop/` should not be committed.

## Build, Test, and Development Commands

Run commands from `lab-portal/` unless noted otherwise.

- `npm install`: install workspace dependencies and update `package-lock.json`.
- `npm run dev`: run API and Vite web app together.
- `npm run dev:api`: run the Express API on `127.0.0.1:3001`.
- `npm run dev:web`: run Vite on `127.0.0.1:5173` when available.
- `npm run dev:desktop`: start Electron, loading the dev UI.
- `npm test`: run backend smoke tests.
- `npm run build:web`: build the React app.
- `npm run build:desktop`: build the web app and package Electron.

## Coding Style & Naming Conventions

Use modern JavaScript ES modules. Use two-space indentation, semicolons, and descriptive camelCase names. React components use PascalCase, for example `StatusPill`. Backend provider methods should be action-oriented, for example `deployLab()`.

Do not place OpenStack credentials, admin tokens, Heat secrets, or cloud API calls in `frontend/` or `electron/`.

## Testing Guidelines

Backend tests use `backend/scripts/test.js`. Add focused assertions for provider behavior and API-facing logic. Test names should describe behavior, such as `mock provider hides another user's deployment`.

Before submitting changes, run:

```bash
cd lab-portal
npm test
npm run build:web
```

## Commit & Pull Request Guidelines

Recent git history does not establish a meaningful convention. Use short, imperative commit messages such as `Add mock deployment lifecycle`.

Pull requests should include a summary, verification commands, linked issues if any, and screenshots for UI changes. Call out security-sensitive changes.

## Security & Configuration Tips

Default development uses `LAB_PROVIDER=mock`. Keep production CORS narrow, validate lab IDs server-side, and deploy Heat templates only from an allowlist. Use environment variables or secret storage for OpenStack credentials.
