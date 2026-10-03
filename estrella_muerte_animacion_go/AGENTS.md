# Instrucciones para el LLM

## Objetivo

Integrar el código de este kit en una aplicación Go existente con cambios
mínimos. El resultado debe mostrar automáticamente una Estrella de la Muerte
hecha con caracteres al arrancar: aparición desde el fondo del terminal, onda de
luz y asentamiento en un brillo muy tenue. Si la aplicación ya usa Bubble Tea,
agregar el fade-in opcional del menú cuando sus datos estén listos y, si el
usuario quiere la estrella fija, el backdrop que la mantiene detrás de la vista.

No reemplaces el programa con el ejemplo. Este directorio contiene componentes
reutilizables, no la aplicación de destino.

## Antes de modificar archivos

1. Lee las instrucciones del repositorio de destino y cualquier `AGENTS.md`
   aplicable. Revisa el estado de Git y conserva los cambios del usuario.
2. Busca `go.mod`, archivos Go, `func main`, `cmd/`, Makefiles y pruebas. En un
   monorepo, identifica el módulo concreto y ejecuta sus comandos dentro de él.
3. Identifica el punto que produce la primera salida visible o inicia la TUI.
4. Identifica las dependencias de terminal, ANSI y TUI que ya estén disponibles.
5. Lee `IMPLEMENTACION.md` y el código del kit. Elige integración de splash
   únicamente, splash más fade-in, o splash más fade-in y backdrop.

## Implementación

- Copia siempre `internal/splash/splash.go`, `splash_test.go` y `color.go` al
  paquete `internal/splash` del destino. No hagas imports hacia este kit ni
  hacia `todo`.
- Si ya existe ese paquete, integra los archivos sin sobrescribir su contenido.
- El splash necesita `golang.org/x/term` (tamaño y detección del terminal) y
  `github.com/muesli/termenv` (consulta del color de fondo). Reutiliza las
  versiones instaladas; si usa Lip Gloss, `termenv` ya está como indirecta. Las
  versiones probadas son `x/term v0.36.0`, `termenv v0.16.0`,
  `charmbracelet/x/ansi v0.11.6` y `bubbletea v1.3.10`, con Go 1.24.2.
  Comprueba la compatibilidad antes de agregarlas a un proyecto con Go más
  antiguo. Si ejecutas `go mod tidy`, revisa que no elimine ni cambie
  dependencias ajenas a la integración.
- Agrega una llamada a `splash.Play()` antes de la salida normal. Cambia el
  import de los ejemplos al módulo real declarado en `go.mod`.
- Conserva argumentos, flags, códigos de salida, carga de datos, navegación,
  ejecución de comandos y tratamiento de errores de la aplicación existente.
- Si ya hay manejadores de señales, coordina el splash con ellos: su `Play()`
  restaura el terminal y reenvía las señales de terminación. No dupliques una
  política de cierre incompatible con la del destino.
- Para una CLI sin Bubble Tea, no copies `fade*.go` ni `backdrop*.go` y no
  agregues Bubble Tea. El splash por sí solo no lo utiliza.
- Si ya usa Bubble Tea, copia también `fade.go` y `fade_test.go` y adapta los
  cambios de `ejemplos/bubbletea.md` al modelo real. Para la estrella fija,
  copia además `backdrop.go` y `backdrop_test.go`. Ambos efectos son visuales y
  no deben bloquear mensajes, teclas ni comandos.
- Crea `splash.NewFade()` y `splash.NewBackdrop()` en el constructor del modelo,
  antes de `p.Run()`. Pueden consultar el color de fondo al terminal; si esa
  consulta ocurriera con Bubble Tea leyendo la entrada, la respuesta llegaría
  como teclas falsas.
- Aplica el backdrop sobre la vista ya pasada por el fade:
  `m.backdrop.View(m.menuFade.View(m.view()), m.width, m.height)`.
- Usa el backdrop solo en vistas a pantalla completa (`tea.WithAltScreen()`),
  que empiezan en la primera fila. No lo apliques a vistas con imágenes de
  terminal (chafa, sixel, kitty graphics) ni a vistas más altas que la pantalla.
- No copies `main.go`, `model.go` ni estilos de la aplicación de origen. Los
  ejemplos muestran puntos de inserción, no archivos que debas reemplazar.

## Errores que no deben repetirse

Estos fallos ya ocurrieron en `todo` y las pruebas del kit los detectan. No los
reintroduzcas al adaptar colores o estilos al proyecto de destino:

1. **Recuadro negro alrededor de la estrella.** El splash pintaba su propio
   fondo (`48;2;10;5;21`). En terminales con otro color de fondo se veía un
   rectángulo oscuro. El splash solo debe emitir `38;2;r;g;b` y `0`; nunca
   códigos de fondo (`48;...`, `40`–`47`, `100`–`107`).
2. **Colores calculados desde un negro fijo.** Los niveles oscuros se veían como
   puntos sucios en fondos de color o claros. Los colores se mezclan con el
   fondo real del terminal mediante `tint()`, y en fondos claros la estrella se
   oscurece en vez de aclararse.
3. **Franja oscura al iniciar el menú.** El fade partía de `#0a0515` y daba ese
   fondo al texto sin estilo. Ahora parte del color del terminal y el fondo por
   defecto del texto queda como `49`.
4. **Estrella que desaparecía.** El splash terminaba en oscuridad. Ahora termina
   exactamente en el brillo del backdrop, para que la estrella siga en su sitio.

Si el proyecto necesita otra apariencia, cambia `backdropLevel`, `highlight` o
`darkness`, pero no agregues un fondo.

## Requisitos que debes conservar

- Duración del splash de aproximadamente 1,5–2 segundos; esta versión usa 1,8.
- Tamaño de la figura entre 50 y 70 columnas; esta versión usa 64 por 30 filas.
- Forma circular, plato superior derecho y trinchera horizontal reconocibles.
- Caracteres de terminal, sin imágenes, videos, fuentes descargadas ni assets.
- Colores RGB de 24 bits y brillo calculado por posición y tiempo.
- Ningún fondo pintado: el fondo del terminal debe verse a través de la figura.
- Colores mezclados con el fondo real del terminal, consultado una sola vez.
- Onda suave que atraviesa la figura, sin parpadeo global.
- Aparición progresiva y final en el brillo tenue del backdrop.
- Aproximadamente 25–35 FPS; esta versión usa 30.
- Cursor oculto durante el splash y restaurado al finalizar.
- Limpieza mediante `defer`, incluso ante errores, panics y señales manejadas.
- Pantalla previa preservada, sin saltos de línea ni scroll entre frames.
- Redibujado en el mismo lugar; no borres toda la pantalla en cada frame.
- Borrado limitado al área del splash y ajustado al tamaño actual del terminal.
- Sin animación cuando stdout esté redirigido, el terminal sea pequeño o
  `TERM=dumb`.
- Fade-in opcional del menú de 900 ms, con su contenido y colores finales
  originales intactos.
- Backdrop opcional que solo reemplaza espacios y conserva texto, fondos y
  estilos de la vista.

## Validación

1. Formatea los archivos Go modificados con `gofmt`.
2. Dentro del módulo de destino, ejecuta `go test ./...` y `go vet ./...`.
3. Compila con el comando habitual del proyecto o su Makefile.
4. Prueba en un terminal de al menos 66 por 32: comprueba el plato, la trinchera,
   las tres fases, el cursor y la continuación de la aplicación.
5. Prueba con un fondo de terminal que no sea negro (por ejemplo azul) y con un
   tema claro: no debe aparecer ningún recuadro y, en el tema claro, la estrella
   debe verse oscura. En tmux puede simularse con
   `tmux set -g window-style 'bg=#1e3a5f'` y ejecutando la aplicación con
   `TERM=xterm-256color`, porque termenv no consulta el fondo con
   `TERM=tmux*` o `screen*` (en ese caso usa `COLORFGBG` o negro).
6. Comprueba salida redirigida, terminal pequeño, cambio de tamaño y Ctrl+C
   durante el splash. Tras Ctrl+C, el menú no debe empezar a ejecutarse.
7. Si integraste el fade-in, comprueba que empieza cuando hay datos, que el menú
   conserva sus colores al terminar y que las teclas funcionan durante el fade.
8. Si integraste el backdrop, comprueba que la estrella queda en la misma
   posición en la que terminó el splash, que el texto se lee encima y que no
   aparecen caracteres extraños al escribir en el buscador.
9. Corrige los problemas introducidos y describe los cambios y verificaciones.

No hagas commits, despliegues ni modificaciones fuera del alcance de la
integración salvo que el usuario lo pida. No conviertas este kit en otra TUI
ni reorganices la aplicación para acomodarlo.
