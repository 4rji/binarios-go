# Trafico CLI

Colección de utilidades de tráfico HTTP originales del repositorio [4rji/binarios-go](https://github.com/4rji/binarios-go/tree/7431ee012e137b465931deff513c1c34e66bcfc4/trafico), adaptadas para instalarse fácilmente con `go install`.

## Requisitos

- Go 1.21 o superior.

## Instalación

Cada binario se instala por separado apuntando al subpaquete correspondiente:

```bash
# Simulador configurable para un objetivo concreto
go install github.com/4rji/binarios-go/trafico/cmd/trafico@latest

# Simulador HTTPS con dominios predefinidos
go install github.com/4rji/binarios-go/trafico/cmd/traficoS@latest
```

Los ejecutables se copian en tu `GOBIN` (o `GOPATH/bin` si `GOBIN` no está definido).

Si necesitas fijar una revisión concreta, sustituye `@latest` por el tag o commit deseado, por ejemplo `@7431ee012e13`.

## Uso

### `trafico`

```
trafico [objetivo]
```

- `objetivo` es opcional y puede ser un dominio, IP o URL. Si no incluye esquema, se antepone `http://`.
- Al iniciar, la herramienta solicita duración de la sesión (minutos), retraso entre peticiones (segundos) y número de workers concurrentes.
- Mientras esté activa, imprime las URLs solicitadas hasta que transcurra el tiempo indicado o se cierre manualmente.

### `traficoS`

```
traficoS
```

- No acepta argumentos. En cada petición elige de forma aleatoria uno de los dominios HTTPS preconfigurados.
- Solicita duración, retraso y workers igual que `trafico`.

## Desarrollo local

```bash
# Ejecutar trafico directamente desde el repositorio
go run ./cmd/trafico

# Ejecutar traficoS
go run ./cmd/traficoS

# Compilar todos los comandos disponibles en el módulo
go build ./...
```

## Licencia

Se mantiene la misma licencia que el repositorio original. Revisa el archivo `LICENSE` del proyecto raíz para más detalles.
