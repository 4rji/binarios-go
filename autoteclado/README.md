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
# Por defecto: ventana falsa en la terminal que "teclea" un script de Python
# (escaneo de red con nmap) en ráfagas de 10 caracteres con pausas de 20 s,
# moviendo el ratón en cada pausa y repitiendo en bucle. Ctrl+C sale sin
# guardar nada.
go run .

# Teclear solo el contenido de un archivo con el teclado real y terminar
go run . -t notas.txt
```

Flags:

| flag    | por defecto | descripción                                        |
|---------|-------------|----------------------------------------------------|
| `-t`    | —           | archivo a teclear con el teclado real; si se indica, solo teclea y sale |
| `-min`  | `45`        | retardo mínimo entre teclas dentro de cada ráfaga (ms) |
| `-max`  | `500`       | retardo máximo entre teclas dentro de cada ráfaga (ms) |
| `-lead` | `3`         | segundos de margen antes de empezar (solo con `-t`, para enfocar la ventana) |

### Modo por defecto (ventana falsa)

Sin flags dibuja una ventana con caracteres de caja dentro de la propia
terminal y va "tecleando" dentro un script de Python de ejemplo:

- Teclea en **ráfagas de 10 caracteres** y luego hace una **pausa de 20 s**.
- En cada pausa **mueve el ratón real** a un punto aleatorio (esto sí usa
  robotgo, así que en macOS necesita permiso de Accesibilidad para el ratón).
- Al terminar el script **vuelve a empezar** en bucle.
- En el borde inferior muestra un **contador de segundos** (`[ 42s ]`) que se
  actualiza cada segundo, incluso durante las pausas.
- Usa el buffer de pantalla alternativo: al salir con **Ctrl+C** la terminal
  queda igual que antes, **sin guardar nada** ni dejar rastro en el historial.

El tecleo dentro de la ventana es simulado (texto dibujado en pantalla); no
envía pulsaciones al teclado real.

### Modo `-t` (teclado real)

Durante los segundos de `-lead` tienes que hacer clic en la ventana donde
quieres que escriba (editor, terminal, navegador…). Luego teclea el contenido
del archivo con el teclado real y termina.

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
