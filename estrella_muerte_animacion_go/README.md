# Estrella de la Muerte: animación para Go CLI

Kit reutilizable del splash implementado en `todo`. Incluye el código completo,
las pruebas y las instrucciones para integrarlo en otro proyecto sin reescribir
su lógica. La aplicación original continúa usando `internal/splash`; este kit
contiene copias independientes para transportar a otro repositorio.

La Estrella de la Muerte surge del fondo del propio terminal, recibe una onda de
luz y se queda como un fondo muy tenue. Solo se colorean los caracteres: el
fondo del terminal se ve a través de la figura, así que no aparece ningún
recuadro negro en temas de color, y en temas claros la estrella se dibuja
oscura. Si el proyecto ya usa Bubble Tea, su menú puede aparecer gradualmente
(fade-in) y la estrella puede quedarse detrás del menú (backdrop).

## Archivos

```text
estrella_muerte_animacion_go/
├── AGENTS.md                    # instrucciones para el LLM que lo integrará
├── IMPLEMENTACION.md            # decisiones, API y pasos completos
├── README.md
├── internal/splash/
│   ├── splash.go                # splash autónomo; no depende de Bubble Tea
│   ├── splash_test.go           # pruebas del splash, sus colores y la limpieza
│   ├── color.go                 # fondo del terminal y mezcla de colores (siempre)
│   ├── fade.go                  # fade-in opcional para Bubble Tea
│   ├── fade_test.go             # pruebas de colores y transición del menú
│   ├── backdrop.go              # estrella tenue opcional detrás de la vista
│   └── backdrop_test.go         # pruebas del backdrop
└── ejemplos/
    ├── cli.go.example           # ejemplo para una CLI normal
    └── bubbletea.md             # cambios mínimos en un modelo existente
```

## Qué copiar

| Proyecto | Archivos | Dependencias |
| --- | --- | --- |
| CLI sin Bubble Tea | `splash.go`, `splash_test.go`, `color.go` | `golang.org/x/term`, `github.com/muesli/termenv` |
| Bubble Tea, solo fade-in | lo anterior + `fade.go`, `fade_test.go` | + `github.com/charmbracelet/bubbletea` |
| Bubble Tea con estrella de fondo | lo anterior + `backdrop.go`, `backdrop_test.go` | + `github.com/charmbracelet/x/ansi` |

`termenv` y `x/ansi` ya llegan como dependencias indirectas con Lip Gloss y
Bubble Tea; en ese caso solo pasan a ser directas.

## Integración rápida

1. Lee [AGENTS.md](AGENTS.md) y revisa el proyecto de destino.
2. Copia los archivos de la tabla a su carpeta `internal/splash/`.
3. Reutiliza sus dependencias de terminal, o agrega versiones compatibles.
4. Importa `<módulo-del-proyecto>/internal/splash` y llama `splash.Play()` al
   principio de su punto de entrada, antes de la salida o del inicio de la TUI.
5. Si ya usa Bubble Tea, sigue [el ejemplo de integración](ejemplos/bubbletea.md)
   para el fade-in y, si quieres la estrella fija, el backdrop.
6. Ejecuta `gofmt`, `go test ./...`, `go vet ./...` y la compilación habitual
   desde el módulo correcto. Pruébalo en un terminal con fondo de color o claro:
   no debe verse ningún recuadro alrededor de la estrella.

## Comportamiento

- Silueta de 64 columnas por 30 filas, con plato superior derecho, anillos,
  radios, paneles, luces y trinchera ecuatorial.
- Splash de 1,8 segundos a 30 FPS: aparición desde el fondo, onda de luz y
  asentamiento en el brillo tenue del backdrop.
- Colores ANSI RGB de 24 bits, mezclados con el fondo real del terminal, que se
  consulta una sola vez al arrancar (OSC 11). Si no responde, se usa la
  variable `COLORFGBG` o, en su defecto, negro.
- Nunca pinta el fondo: solo colorea los caracteres.
- Fade-in opcional del menú de 900 ms desde el color del terminal, sin bloquear
  la navegación.
- Backdrop opcional: la estrella sigue en el mismo sitio, muy tenue, ocupando
  solo las celdas vacías; el texto de la vista queda siempre encima.
- Se omiten los efectos si stdout no es un terminal interactivo, `TERM=dumb` o
  el terminal mide menos de 66 columnas por 32 filas.
- El splash se detiene si cambia el tamaño del terminal.
- Usa una pantalla alternativa temporal, posiciones absolutas y borrado
  limitado al rectángulo de la animación. Conserva la pantalla anterior.
- Restaura pantalla, cursor y colores antes de continuar o terminar por una
  señal manejable. Aparte de la consulta inicial del color de fondo, no lee
  teclas ni cambia el modo de entrada.

Para usarlo con un LLM, copia la carpeta al proyecto y envíale:

> Integra el kit `estrella_muerte_animacion_go` en esta aplicación. Lee primero
> su `AGENTS.md` y `IMPLEMENTACION.md`. Inspecciona el punto de entrada y las
> dependencias antes de editar. Conserva el comportamiento actual. Agrega el
> fade-in y el backdrop solamente si el proyecto ya usa Bubble Tea. Ejecuta las
> verificaciones indicadas al terminar.
