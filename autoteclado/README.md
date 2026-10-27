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
autoteclado -f notas.txt
autoteclado -p
```

## Uso

```sh
# Por defecto: ventana falsa en la terminal que "teclea" un script de Python
# (escaneo de red con nmap) en ráfagas de 10 caracteres con pausas de 20 s,
# moviendo el ratón en cada pausa y repitiendo en bucle. Ctrl+C sale sin
# guardar nada.
go run .

# Teclear solo el contenido de un archivo con el teclado real y terminar
go run . -f notas.txt -t 5

# macOS: teclear el texto copiado, a velocidad máxima, después de 3 segundos
go run . -p

# Archivo a velocidad máxima
go run . -f notas.txt -r
```

Flags:

| flag    | por defecto | descripción                                        |
|---------|-------------|----------------------------------------------------|
| `-f`    | —           | archivo (*file*) cuyo contenido se teclea; al terminar, sale |
| `-p`    | desactivado | teclea el texto del portapapeles de macOS, a velocidad máxima por defecto |
| `-t`    | `3`         | tiempo de espera inicial en **segundos**, para enfocar la ventana (con `-f` o `-p`) |
| `-r`    | automático con `-p` | velocidad máxima, sin pausas añadidas entre teclas |
| `-r1`…`-r9` | —       | reduce la velocidad respecto a `-r` en pasos del 10 % |
| `-min`  | `45`        | pausa mínima entre teclas, en **milisegundos**; menor pausa = más velocidad |
| `-max`  | `500`       | pausa máxima entre teclas, en **milisegundos**; debe ser mayor o igual que `-min` |

La ayuda (`./autoteclado -h`) usa **cian** para títulos y opciones, y distingue
los niveles más rápidos en **verde**, intermedios en **cian** y lentos en **amarillo**.
La cuenta atrás aparece en amarillo, la confirmación en verde y los errores
de validación o escritura en rojo. Estos mensajes desactivan los colores al
redirigir su salida, con `TERM=dumb` o con `NO_COLOR=1`:

```sh
NO_COLOR=1 ./autoteclado -h
```

### Portapapeles de macOS: `-p`

Copia el texto con **Cmd+C**, ejecuta el comando y haz clic en la ventana donde
quieres escribir durante los **3 segundos** de cuenta atrás:

```sh
./autoteclado -p
```

Lee el texto mediante `pbpaste` antes de la cuenta atrás y luego lo teclea con
pulsaciones reales, a velocidad máxima. Conserva los espacios, tabulaciones y
saltos de línea del contenido; los saltos de línea se envían como **Enter**.
Si el portapapeles está vacío o no contiene texto, muestra un error.

```sh
# Esperar 5 segundos en lugar de 3
./autoteclado -p -t 5

# Escribir un 10 % más lento que la velocidad máxima
./autoteclado -p -r1

# Escribir un 20 % más lento
./autoteclado -p -r2

# Escribir a la mitad de velocidad, después de 5 segundos
./autoteclado -p -t 5 -r5
```

Elige una fuente de texto: **`-f archivo` o `-p`**. `-p` está disponible en
macOS; `-f` también funciona en las demás plataformas admitidas.

### Velocidades predefinidas: `-r`, `-r1`…`-r9`

`-r` escribe tan rápido como permita el envío de teclas, sin pausas añadidas.
Cada nivel reduce la velocidad en 10 puntos porcentuales respecto a `-r`:

| Opción | Velocidad respecto a `-r` | Reducción | Tiempo aproximado si `-r` tarda 1 s |
|--------|--------------------------|-----------|-----------------------------------|
| `-r`   | 100 % | 0 %  | 1 s |
| `-r1`  | 90 %  | 10 % | 1,11 s |
| `-r2`  | 80 %  | 20 % | 1,25 s |
| `-r3`  | 70 %  | 30 % | 1,43 s |
| `-r4`  | 60 %  | 40 % | 1,67 s |
| `-r5`  | 50 %  | 50 % | 2 s |
| `-r6`  | 40 %  | 60 % | 2,5 s |
| `-r7`  | 30 %  | 70 % | 3,33 s |
| `-r8`  | 20 %  | 80 % | 5 s |
| `-r9`  | 10 %  | 90 % | 10 s |

El programa mide cuánto tarda en enviar las teclas y calcula las pausas para
aproximarse a la proporción elegida. La velocidad real depende del sistema y
de la aplicación que recibe las teclas. La cuenta atrás de `-t` se suma aparte.

```sh
# Archivo a velocidad máxima, después de 3 segundos
./autoteclado -f notas.txt -r

# El mismo archivo al 80 % de velocidad, después de 5 segundos
./autoteclado -f notas.txt -t 5 -r2
```

Se elige **un solo nivel**. Los niveles requieren `-f` o `-p` y no se combinan
con `-min`/`-max`. Con `-p` se aplica `-r` automáticamente, salvo que elijas un
nivel o pausas manuales. Con `-f`, sin opciones de velocidad, se usan las
pausas de 45 a 500 ms.

### Ajuste manual: `-min` y `-max`

`-min` y `-max` controlan la velocidad mediante la **pausa después de cada
tecla**. El programa elige una pausa aleatoria entre esos dos valores:
**cuanto menores sean, más rápido escribe**. 1000 milisegundos (ms) equivalen
a 1 segundo.

| Ritmo de ejemplo | Opciones | Pausa por tecla |
|-----------------|----------|-----------------|
| Rápido | `-min 10 -max 30` | entre 0,01 y 0,03 segundos |
| Medio | `-min 50 -max 150` | entre 0,05 y 0,15 segundos |
| Lento | `-min 250 -max 500` | entre 0,25 y 0,5 segundos |
| Predeterminado con `-f` | sin opciones de velocidad | entre 0,045 y 0,5 segundos |

Ejemplos con el ejecutable compilado en esta carpeta:

```sh
# Rápido: espera 5 segundos y luego teclea el archivo con pausas de 10 a 30 ms
./autoteclado -f notas.txt -t 5 -min 10 -max 30

# Velocidad media
./autoteclado -f notas.txt -t 5 -min 50 -max 150

# Lento
./autoteclado -f notas.txt -t 5 -min 250 -max 500

# Ritmo constante: siempre una pausa de 100 ms (0,1 segundos) por tecla
./autoteclado -f notas.txt -t 5 -min 100 -max 100

# Portapapeles con pausas manuales
./autoteclado -p -min 50 -max 150
```

`-t 5` solo da **5 segundos antes de empezar**, para hacer clic en el destino;
no cambia la velocidad de escritura. `-t 0` empieza inmediatamente. El tiempo
total también incluye el procesamiento de las teclas por el sistema. Si una
consola web pierde caracteres a velocidad rápida, aumenta las pausas.

En el modo de ventana falsa, `-min` y `-max` controlan las pausas dentro de
cada ráfaga; la pausa de 20 segundos entre ráfagas sigue siendo fija.

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

### Modo `-f` (teclado real)

Durante los segundos de `-t` tienes que hacer clic en la ventana donde
quieres que escriba (editor, terminal, navegador…). Luego teclea el contenido
del archivo con el teclado real y termina.

Las letras, números, espacios y símbolos ASCII se envían como pulsaciones de
teclas. Esto permite usarlos en consolas web que leen los códigos del teclado
(por ejemplo, consolas remotas), donde el envío de texto Unicode en macOS podía
producir `aaaaa` en lugar del contenido del archivo. Los saltos de línea siguen
enviándose como **Enter**, las tabulaciones como **Tab** y se ignora `\r` en
archivos CRLF. Si RobotGo devuelve un error al pulsar una tecla, el programa
se detiene y lo muestra.

Para los símbolos, RobotGo usa combinaciones de un teclado **US**; usa esa
distribución tanto en el equipo local como en la consola remota. Los caracteres
fuera de ASCII, como `ñ` y las vocales con tilde, siguen usando inserción Unicode
y requieren que el destino la admita; no se garantiza su entrada en consolas
que solo aceptan códigos de tecla.

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
