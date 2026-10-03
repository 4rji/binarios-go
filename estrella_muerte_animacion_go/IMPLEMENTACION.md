# Cómo se implementó

## Punto de integración

La aplicación de origen tiene un módulo en `todo/go.mod` y su entrada en
`main.go`. Su `main` crea un programa Bubble Tea con `tea.WithAltScreen()`.
La única inserción en ese archivo fue importar el paquete local y llamar
`splash.Play()` antes de `tea.NewProgram(...)`.

El splash no depende de la aplicación ni usa sus datos. Antes de dibujar,
comprueba stdout con `term.IsTerminal` y obtiene las dimensiones con
`term.GetSize`. Se reutilizaron `golang.org/x/term v0.36.0`,
`github.com/muesli/termenv v0.16.0` y `github.com/charmbracelet/x/ansi v0.11.6`,
que ya existían en el proyecto como dependencias indirectas; solo pasaron a ser
directas. No se agregó otra TUI.

## Splash autónomo

Archivos necesarios:

```text
internal/splash/splash.go
internal/splash/splash_test.go
internal/splash/color.go
```

API pública:

```go
func Play()
```

`Play()` bloquea durante el splash. Si no puede animar, retorna inmediatamente
sin escribir nada. Si recibe `SIGINT`, `SIGTERM`, `SIGHUP` o `SIGQUIT`, limpia el
terminal, elimina su manejador y reenvía la señal; el menú no continúa. También
incluye una salida de respaldo cuando el sistema no permite reenviar la señal.
Como cualquier código de usuario, no puede interceptar `SIGKILL`.

### Color de fondo del terminal

El splash no pinta ningún fondo. Para que la estrella surja del color que tenga
el terminal (negro, azul, crema…), `terminalBackground()` en `color.go`
pregunta ese color una sola vez con `termenv` (secuencia OSC 11, seguida de una
consulta de cursor que garantiza respuesta aunque el terminal no soporte OSC
11). Si no hay respuesta, o con `TERM=tmux*`/`screen*`, termenv devuelve el
color indicado por `COLORFGBG` o, en su defecto, negro. La consulta pone el terminal sin eco durante unos milisegundos y
lee la respuesta; por eso debe hacerse antes de que otra parte del programa
(por ejemplo Bubble Tea) lea la entrada. `Play()` la hace al empezar y
`NewFade()`/`NewBackdrop()` reutilizan el resultado.

`tint(bg, nivel)` mueve el fondo hacia `highlight` (`#dae1e6`) según un nivel de
0 a 1. En fondos claros mueve hacia `darkness`, para que la estrella sea
oscura y visible. El nivel se cuantiza en 63 pasos (`shades`) para que celdas
vecinas compartan secuencias de color.

### Geometría y luz

`deathStar()` genera una cuadrícula de 64 por 30 celdas. Las coordenadas se
normalizan para formar una esfera, teniendo en cuenta que las celdas suelen ser
más altas que anchas. El dibujo incluye:

- Borde punteado, placas, meridianos y bandas de paneles.
- Dos filas oscuras de trinchera con bordes y pequeños puertos.
- Plato desplazado arriba y a la derecha, con borde, anillos, ocho radios y
  emisor central.

`frame()` conserva los caracteres y cambia sus colores. Una campana gaussiana
recorre la esfera en diagonal y una onda sinusoidal tenue añade variación.
La luminosidad de cada celda depende de sus coordenadas, su material y el
tiempo, y se convierte en color con `tint()` sobre el fondo del terminal. Solo
emite secuencias `38;2;r;g;b`; las celdas vacías son espacios sin fondo.

La visibilidad global usa `smoothstep`, que suaviza el comienzo y el final:

| Fase | Tiempo aproximado |
| --- | --- |
| Aparición desde el fondo | 0–360 ms |
| Barrido de luz | 324–1440 ms |
| Asentamiento en el brillo del backdrop | 1404–1800 ms |

Las fases se solapan ligeramente para que el movimiento no se detenga. El
último frame es exactamente el que dibuja `Backdrop` (nivel igual al brillo
propio de cada celda multiplicado por `backdropLevel = 0.18`), así que la
estrella no salta al pasar del splash a la TUI.

### Manejo del terminal

- Entra una vez en una pantalla alternativa temporal y oculta el cursor, sin
  establecer color de fondo.
- Dibuja cada fila con posiciones absolutas `CSI fila;columna H`.
- No escribe saltos de línea ni limpia toda la pantalla durante los frames.
- Deja un margen de seguridad; necesita al menos 66 por 32 celdas.
- Revisa el tamaño durante la animación y termina si cambia.
- En el `defer`, restablece colores y borra solo su rectángulo con `CSI n X`,
  recortando el borrado al tamaño actual del terminal.
- Sale de la pantalla alternativa y vuelve a mostrar el cursor.
- Aparte de la consulta del color de fondo, no activa modos de entrada ni lee
  teclas. La TUI conserva la responsabilidad sobre sus propios modos.

## Fade-in del menú con Bubble Tea

Archivos adicionales, solo para proyectos que ya usan Bubble Tea:

```text
internal/splash/fade.go
internal/splash/fade_test.go
```

API:

```go
func NewFade() Fade
func (f *Fade) Start() tea.Cmd
func (f *Fade) Update(tick FadeTick) tea.Cmd
func (f Fade) View(view string) string
```

El modelo guarda un `splash.Fade`. Al recibir los datos iniciales, llama
`Start()` y devuelve su comando. Cuando recibe `splash.FadeTick`, actualiza la
transición y devuelve el siguiente comando. Su `View()` pasa la vista original
por `menuFade.View(...)`.

El fade dura 900 ms a 30 FPS. Antes de que haya datos mantiene la vista de carga
del mismo color que el fondo del terminal, es decir, invisible. Durante el fade
mezcla cada color SGR desde ese fondo hasta su valor final, incluyendo RGB, 256
colores, 16 colores y restablecimientos. El texto sin color de fondo conserva
el del terminal (`49`), por lo que no aparecen franjas oscuras. Conserva texto,
tamaño, estilos y los demás escapes. Al terminar devuelve exactamente la vista
original y deja de programar ticks. La navegación sigue disponible durante
toda la transición.

El valor cero de `Fade` no modifica las vistas. `NewFade()` habilita la
transición únicamente si el terminal cumple las mismas condiciones del splash.

## Estrella de fondo con Bubble Tea

Archivos adicionales, opcionales, para mantener la estrella después del splash:

```text
internal/splash/backdrop.go
internal/splash/backdrop_test.go
```

API:

```go
func NewBackdrop() Backdrop
func (b Backdrop) View(view string, columns, rows int) string
```

`View()` centra la estrella en una pantalla de `columns × rows`, en la misma
posición que el splash, y la dibuja solo en las celdas que la vista deja como
espacios. Recorre la vista con `ansi.DecodeSequence`, sigue el color de primer
plano, el de fondo y el video inverso de cada celda, y:

- Calcula el color del carácter con `tint()` sobre el fondo de esa celda, de
  modo que se ve igual de tenue sobre cualquier fila, seleccionada o no.
- Restaura el color de primer plano original antes del siguiente texto.
- No toca celdas con texto ni espacios en video inverso (el cursor de un input).
- Si la vista es más corta que la estrella, agrega líneas en blanco hasta su
  última fila, sin superar la altura de la pantalla, y respeta el salto final.

El valor cero de `Backdrop` no modifica las vistas. `NewBackdrop()` lo habilita
en terminales interactivos; si la pantalla mide menos de 66 por 32, `View()`
devuelve la vista intacta. Solo funciona con vistas a pantalla completa
(`tea.WithAltScreen()`), porque supone que la vista empieza en la primera fila.
En `todo` no se aplica a la vista de detalle, que muestra imágenes con `chafa`.

## Qué ajustar para otro proyecto

- El import del paquete local debe usar el módulo real de su `go.mod`.
- Coloca `Play()` antes de su primera salida, sin mover flags ni comandos.
- Para el fade del menú, adapta los nombres del modelo y del mensaje de datos.
- Intensidad de la estrella fija: `backdropLevel` en `splash.go`; más bajo es
  más tenue.
- Color de la estrella: `highlight` en `color.go`. `darkness` es el tono hacia
  el que se oscurece en temas claros y la base de la paleta original.
- No agregues un color de fondo a `enterScreen` ni a los frames: el fondo del
  terminal debe verse siempre. La prueba `assertOnlyGlyphColors` lo comprueba.
- `width`, `height`, `duration` y `fps` están en `splash.go`; `menuDuration` está
  en `fade.go`. Si cambias dimensiones, actualiza sus pruebas y documentación.
- En un proyecto sin Bubble Tea, lleva solo `splash.go`, `splash_test.go` y
  `color.go`.

## Verificación en el destino

Desde el módulo correspondiente, después de copiar e integrar:

```sh
gofmt -w internal/splash/*.go
# Incluye también main.go y los archivos del modelo que hayas editado.
go test ./...
go vet ./...
go build ./...
```

Si el proyecto usa un Makefile o una entrada en `cmd/`, utiliza sus comandos de
compilación habituales. Este kit incluye pruebas de tamaño insuficiente,
stdout redirigido, errores de escritura, panic, interrupciones, limpieza al
cambiar tamaño, límites del frame, desplazamiento de luz, ausencia de fondo
pintado, aparición desde fondos oscuros, de color y claros, conservación de
colores y contenido durante el fade-in, coincidencia del último frame con el
backdrop, y conservación de texto, estilos, cursor y altura con el backdrop.

Este kit es material para copiar e integrar, no un módulo Go independiente.
Los ejemplos usan la extensión `.example` para no crear nuevos ejecutables
durante las pruebas del repositorio.
