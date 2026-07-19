# backd

Herramienta CLI para listar conexiones TCP establecidas y mostrar la información del proceso responsable (PID, usuario, ruta del ejecutable, etc.). Está escrita en Go y utiliza [`gopsutil`](https://github.com/shirou/gopsutil).

## Instalación

```bash
go install github.com/4rji/binarios-go/backd@latest
```

El binario quedará disponible en `$(go env GOPATH)/bin/backd`. Añade esa ruta a tu `PATH` (si aún no lo está) y ejecútalo con privilegios de superusuario para ver datos de todos los procesos:

```bash
sudo backd
```

> **Requisitos**: Go 1.22 o superior. En sistemas modernos `GO111MODULE` ya no es necesario.

## Otros binarios incluidos

El repositorio contiene variantes que puedes instalar especificando el subdirectorio correspondiente:

- Monitor en modo daemon que escribe en `backd.log`:
  ```bash
  go install github.com/4rji/binarios-go/backd/cmd/backd-daemon@latest
  ```
- Monitor TCP/UDP con filtros adicionales:
  ```bash
  go install github.com/4rji/binarios-go/backd/cmd/backd-udp@latest
  ```
- Versión interactiva (legacy) con menú textual:
  ```bash
  go install github.com/4rji/binarios-go/backd/cmd/backde-legacy@latest
  ```

Cada variante mantiene la funcionalidad original; solo se reorganizó su ubicación para que `go install` funcione correctamente desde GitHub.

### Uso de backde-legacy

```bash
sudo backde-legacy -2
```

El número indica el intervalo de refresco en segundos (también acepta `-r/--refresh`). En la vista de actividad:

- `r` + Enter: regresar al menú
- `q` + Enter: salir
- `k` + Enter: terminar el proceso seleccionado

Si usas `go run`, pasa los flags con `--`:

```bash
go run ./cmd/backde-legacy -- -2
```

## Desarrollo local

```bash
git clone https://github.com/4rji/binarios-go.git
cd binarios-go/backd

go mod init tu/modulo
go get github.com/shirou/gopsutil/v3/net
go get github.com/shirou/gopsutil/v3/process

go run main.go

Para backde-legacy desde main:
go run ./cmd/backde-legacy/ -2 (2 seconds)
go run ./cmd/backd-daemon
go run ./cmd/backd-udp


```

Si quieres ejecutar alguna variante directamente, cambia al directorio `cmd/<variante>` y usa `go run .`.
