# Cómo se implementó

## Punto de integración

La aplicación de origen tiene un módulo en `todo/go.mod` y su entrada en
`main.go`. Su `main` crea un programa Bubble Tea con `tea.WithAltScreen()`.
La única inserción en ese archivo fue importar el paquete local y llamar
`splash.Play()` antes de `tea.NewProgram(...)`.

El splash no depende de la aplicación ni usa sus datos. Antes de dibujar,
comprueba stdout con `term.IsTerminal` y obtiene las dimensiones con
`term.GetSize`. Se reutilizó `golang.org/x/term v0.36.0`, que ya existía en el
proyecto; solo pasó de dependencia indirecta a directa. No se agregó otra TUI.

## Splash autónomo

Archivos necesarios:

```text
internal/splash/splash.go
internal/splash/splash_test.go
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
La luminosidad de cada celda depende de sus coordenadas, su material y el tiempo.
La paleta tiene 32 niveles y usa secuencias `38;2;r;g;b`.

La visibilidad global usa `smoothstep`, que suaviza el comienzo y el final:

| Fase | Tiempo aproximado |
| --- | --- |
| Aparición desde la oscuridad | 0–360 ms |
| Barrido de luz | 324–1440 ms |
| Fade-out | 1404–1800 ms |

Las fases se solapan ligeramente para que el movimiento no se detenga. El fondo
oscuro usa RGB `(10, 5, 21)`, el mismo color base que el menú de origen.

### Manejo del terminal

- Entra una vez en una pantalla alternativa temporal y oculta el cursor.
- Dibuja cada fila con posiciones absolutas `CSI fila;columna H`.
- No escribe saltos de línea ni limpia toda la pantalla durante los frames.
- Deja un margen de seguridad; necesita al menos 66 por 32 celdas.
- Revisa el tamaño durante la animación y termina si cambia.
- En el `defer`, restablece colores y borra solo su rectángulo con `CSI n X`,
  recortando el borrado al tamaño actual del terminal.
- Sale de la pantalla alternativa y vuelve a mostrar el cursor.
- No activa modo raw ni cambia flags de entrada. La TUI conserva la
  responsabilidad sobre sus propios modos de terminal.

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

El fade dura 900 ms a 30 FPS. Antes de que haya datos mantiene oscura la vista de
carga. Durante el fade transforma únicamente los colores SGR, incluyendo RGB,
256 colores, 16 colores y restablecimientos. Conserva texto, tamaño, estilos y
los demás escapes. Al terminar devuelve exactamente la vista original y deja
de programar ticks. La navegación sigue disponible durante toda la transición.

El valor cero de `Fade` no modifica las vistas. `NewFade()` habilita la
transición únicamente si el terminal cumple las mismas condiciones del splash.

## Qué ajustar para otro proyecto

- El import del paquete local debe usar el módulo real de su `go.mod`.
- Coloca `Play()` antes de su primera salida, sin mover flags ni comandos.
- Para el fade del menú, adapta los nombres del modelo y del mensaje de datos.
- Si tiene un fondo distinto, ajusta `darkness` en `fade.go`, el fondo de
  `enterScreen` y el primer color de `palette()` en `splash.go` de forma coherente.
- `width`, `height`, `duration` y `fps` están en `splash.go`; `menuDuration` está
  en `fade.go`. Si cambias dimensiones, actualiza sus pruebas y documentación.
- En un proyecto sin Bubble Tea, lleva solo los dos archivos del splash.

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
cambiar tamaño, límites del frame, desplazamiento de luz y conservación de
colores y contenido durante el fade-in.
