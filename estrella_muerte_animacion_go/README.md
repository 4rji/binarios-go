# Estrella de la Muerte: animación para Go CLI

Kit reutilizable del splash implementado en `todo`. Incluye el código completo,
las pruebas y las instrucciones para integrarlo en otro proyecto sin reescribir
su lógica. La aplicación original continúa usando `internal/splash`; este kit
contiene copias independientes para transportar a otro repositorio.

La Estrella de la Muerte aparece desde la oscuridad, recibe una onda de luz y
desaparece con un fade-out. Después, si el proyecto ya usa Bubble Tea, su menú
puede aparecer gradualmente con el fade-in opcional.

## Archivos

```text
estrella_muerte_animacion_go/
├── AGENTS.md                    # instrucciones para el LLM que lo integrará
├── IMPLEMENTACION.md            # decisiones, API y pasos completos
├── README.md
├── internal/splash/
│   ├── splash.go                # splash autónomo; no depende de Bubble Tea
│   ├── splash_test.go           # pruebas del splash y limpieza del terminal
│   ├── fade.go                  # fade-in opcional para Bubble Tea
│   └── fade_test.go             # pruebas de colores y transición del menú
└── ejemplos/
    ├── cli.go.example           # ejemplo para una CLI normal
    └── bubbletea.md             # cambios mínimos en un modelo existente
```

## Integración rápida

1. Lee [AGENTS.md](AGENTS.md) y revisa el proyecto de destino.
2. Copia `splash.go` y `splash_test.go` a su carpeta `internal/splash/`.
3. Reutiliza su dependencia `golang.org/x/term`, o agrega una versión compatible.
4. Importa `<módulo-del-proyecto>/internal/splash` y llama `splash.Play()` al
   principio de su punto de entrada, antes de la salida o del inicio de la TUI.
5. Si ya usa Bubble Tea y quieres el fade-in del menú, copia también `fade.go`
   y `fade_test.go` y sigue [el ejemplo de integración](ejemplos/bubbletea.md).
6. Ejecuta `gofmt`, `go test ./...`, `go vet ./...` y la compilación habitual
   desde el módulo correcto.

Este kit es material para copiar e integrar, no un módulo Go independiente.
Los ejemplos usan la extensión `.example` para no crear nuevos ejecutables
durante las pruebas del repositorio.

## Comportamiento

- Silueta de 64 columnas por 30 filas, con plato superior derecho, anillos,
  radios, paneles, luces y trinchera ecuatorial.
- Splash de 1,8 segundos a 30 FPS: aparición, onda de luz y fade-out.
- Colores ANSI RGB de 24 bits, con 32 niveles de brillo.
- Fade-in opcional del menú de 900 ms, sin bloquear la navegación.
- Se omiten ambas transiciones si stdout no es un terminal interactivo,
  `TERM=dumb` o el terminal mide menos de 66 columnas por 32 filas.
- El splash se detiene si cambia el tamaño del terminal.
- Usa una pantalla alternativa temporal, posiciones absolutas y borrado
  limitado al rectángulo de la animación. Conserva la pantalla anterior.
- Restaura pantalla, cursor y colores antes de continuar o terminar por una
  señal manejable. No cambia el modo de entrada ni lee teclas durante el splash.

Para usarlo con un LLM, copia la carpeta al proyecto y envíale:

> Integra el kit `estrella_muerte_animacion_go` en esta aplicación. Lee primero
> su `AGENTS.md` y `IMPLEMENTACION.md`. Inspecciona el punto de entrada y las
> dependencias antes de editar. Conserva el comportamiento actual. Agrega el
> fade-in del menú solamente si el proyecto ya usa Bubble Tea. Ejecuta las
> verificaciones indicadas al terminar.
