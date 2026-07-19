# OpenStack Lab Portal

Proyecto npm para un portal de laboratorios OpenStack que corre como web app y como app de escritorio con Electron.

## Estructura

```text
lab-portal/
├── backend/         # API Express y proveedor mock
├── frontend/        # React + Vite
├── electron/        # Shell Electron seguro
├── heat-templates/  # Plantillas Heat iniciales
└── package.json     # npm workspaces y scripts comunes
```

## Requisitos

- Node.js 22.12 o superior.
- npm.
- OpenStack no es necesario para el primer MVP; el backend arranca con proveedor `mock`.

## Instalacion

```bash
cd lab-portal
npm install
```

## Desarrollo

API y web en paralelo:

```bash
npm run dev
```

Solo API:

```bash
npm run dev:api
```

Solo web:

```bash
npm run dev:web
```

Electron en modo desarrollo, despues de levantar la web:

```bash
npm run dev:desktop
```

Electron usa el API configurado por `LAB_API_URL` o, si no se define, `http://127.0.0.1:3001`.
Para desarrollo normal puedes correr todo con:

```bash
npm run dev
```

Y en otra terminal abrir la ventana Electron:

```bash
npm run dev:desktop
```

Por defecto, la API y Vite escuchan en `0.0.0.0`, para que sean accesibles desde otros equipos de la red. Abre la app con:

```bash
http://<IP_DEL_SERVIDOR>:5173
```

## Build

```bash
npm run build:web
npm run build:desktop
```

El paquete de escritorio queda en `dist-desktop/`. La app empaquetada carga la UI construida en
`frontend/dist/` y llama al API en `LAB_API_URL` o `http://127.0.0.1:3001`. Antes de usar la app
empaquetada, deja el API corriendo:

```bash
npm run start:api
```

## Configuracion

Variables utiles:

```bash
PORT=3001
HOST=0.0.0.0
LAB_PROVIDER=mock
LAB_OPENSTACK_DEPLOYMENT_MODE=auto
CORS_ORIGIN=http://localhost:5173,http://127.0.0.1:5173
LAB_PORTAL_URL=http://127.0.0.1:5173
LAB_API_URL=http://127.0.0.1:3001
```

Para permitir llamadas directas a la API desde cualquier origen durante desarrollo, usa `CORS_ORIGIN=*`. No uses ese valor en produccion.

La app no llama OpenStack desde el frontend ni desde Electron. Para usar OpenStack, el backend se autentica con Keystone usando una application credential y crea stacks en Heat:

```bash
LAB_PROVIDER=openstack
OS_AUTH_URL=http://172.16.101.32/identity/v3
OS_REGION_NAME=RegionOne
OS_PROJECT_ID=<project-id>
OS_APPLICATION_CREDENTIAL_ID=<application-credential-id>
OS_APPLICATION_CREDENTIAL_SECRET=<application-credential-secret>
```

`LAB_OPENSTACK_DEPLOYMENT_MODE=auto` intenta crear stacks con Heat. Si Keystone no publica Heat/orchestration en
el catalogo de servicios, el backend cae a despliegue directo con Nova para laboratorios de una sola VM que tengan
parametros `image`, `flavor` y `network`. Tambien puedes forzarlo con:

```bash
LAB_OPENSTACK_DEPLOYMENT_MODE=nova
```

Si Heat existe pero no esta en el catalogo, define `LAB_HEAT_ENDPOINT`. Para despliegue directo Nova, usa
`LAB_NOVA_NETWORK=<tenant-network-name-or-id>` cuando la plantilla/laboratorio no provee un parametro `network`.
El provider tambien acepta estos parametros comunes para plantillas Heat:

```bash
LAB_HEAT_IMAGE=<glance-image-name-or-id>
LAB_HEAT_FLAVOR=<nova-flavor-name-or-id>
LAB_HEAT_KEY_NAME=<keypair-name>
LAB_HEAT_EXTERNAL_NETWORK=<external-network-name-or-id>
```

Puedes sobreescribirlos por plataforma o laboratorio con variables como `LAB_LINUX_IMAGE`,
`LAB_WINDOWS_FLAVOR` o `LAB_CCDC_WKST_UBUNTU_24_PARAM_IMAGE`. Para una prueba rapida con la
plantilla deployable incluida:

```bash
LAB_CCDC_WKST_UBUNTU_24_HEAT_TEMPLATE=heat-templates/single-linux.yaml
```

Nota: `heat-templates/mini-ccdc.yaml` sigue siendo un placeholder multi-VM. Para crear maquinas reales,
apunta el laboratorio a `single-linux.yaml` o reemplaza `mini-ccdc.yaml` por una plantilla Heat completa.

## Siguiente fase

1. Persistir usuarios y despliegues en SQLite.
2. Validar plantillas Heat desde una allowlist mas estricta.
3. Agregar limpieza por TTL y limites por usuario.
