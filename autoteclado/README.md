# autoteclado

Simula tecleo y movimiento de ratón "humano" (retardos aleatorios) con
[robotgo](https://github.com/go-vgo/robotgo).

## Instalación desde GitHub

Necesitas Go 1.25.0 o posterior y las dependencias de compilación de tu sistema
indicadas en la sección **Build** (robotgo usa CGO).

```sh
go install github.com/4rji/binarios-go/autoteclado@latest
```

La ruta incluye `/autoteclado` porque este repositorio contiene varias
herramientas, cada una con su propio módulo Go.

Go instala el ejecutable en `GOBIN` si está configurado, o en `$(go env GOPATH)/bin`.
Para usarlo desde cualquier directorio, añade esa carpeta a tu `PATH`. Si usas
la ubicación predeterminada, puedes hacerlo para la sesión actual con:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

```sh
autoteclado -h
autoteclado -t notas.txt
```

## Uso

```sh
# Modo aleatorio: escribe texto aleatorio y mueve el ratón sin parar (Ctrl+C para salir)
go run .

# Teclear solo el contenido de un archivo y terminar (no mueve el ratón)
go run . -t notas.txt
```

Flags:

| flag    | por defecto | descripción                                        |
|---------|-------------|----------------------------------------------------|
| `-t`    | —           | archivo a teclear; si se indica, solo teclea y sale |
| `-min`  | `45`        | retardo mínimo entre teclas (ms)                   |
| `-max`  | `170`       | retardo máximo entre teclas (ms)                   |
| `-lead` | `3`         | segundos de margen antes de empezar (para enfocar la ventana) |

Durante los segundos de `-lead` tienes que hacer clic en la ventana donde
quieres que escriba (editor, terminal, navegador…).

## Build

```sh
go build -o autoteclado .
```

### macOS

1. `xcode-select --install` (Command Line Tools, para CGO/CoreGraphics).
2. Da permiso a la app que lanza el binario en
   **Ajustes del Sistema → Privacidad y Seguridad → Accesibilidad**
   (añade tu Terminal / iTerm / el ejecutable). Sin esto el tecleo y el
   movimiento del ratón **no hacen nada** y no sale error claro.
3. Apple Silicon compila nativo en arm64; no cruces de plataforma (CGO).

### Linux (X11)

Necesita los headers de desarrollo de X11:

```sh
# Debian/Ubuntu
sudo apt install gcc libc6-dev libx11-dev xorg-dev libxtst-dev xclip libxkbcommon-dev
```

### Windows

Compila con un toolchain con CGO (p. ej. mingw-w64/TDM-GCC).
