# Instrucciones para el LLM

## Objetivo

Integrar el código de este kit en una aplicación Go existente con cambios
mínimos. El resultado debe mostrar automáticamente una Estrella de la Muerte
hecha con caracteres al arrancar: aparición desde la oscuridad, onda de luz y
fade-out. Si la aplicación ya usa Bubble Tea, agregar el fade-in opcional del
menú cuando sus datos estén listos.

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
   únicamente o splash más fade-in, según el proyecto.

## Implementación

- Copia `internal/splash/splash.go` y `splash_test.go` al paquete
  `internal/splash` del destino. No hagas imports hacia este kit ni hacia `todo`.
- Si ya existe ese paquete, integra los archivos sin sobrescribir su contenido.
- El splash necesita `golang.org/x/term`. Reutiliza una versión instalada o una
  dependencia de terminal equivalente si resulta más apropiado. Evita duplicar
  dependencias para detección y tamaño del terminal.
- La versión usada aquí es `golang.org/x/term v0.36.0`, en un proyecto Go 1.24.2.
  Comprueba la compatibilidad antes de agregarla a un proyecto con Go más antiguo.
- Agrega una llamada a `splash.Play()` antes de la salida normal. Cambia el
  import de los ejemplos al módulo real declarado en `go.mod`.
- Conserva argumentos, flags, códigos de salida, carga de datos, navegación,
  ejecución de comandos y tratamiento de errores de la aplicación existente.
- Si ya hay manejadores de señales, coordina el splash con ellos: su `Play()`
  restaura el terminal y reenvía las señales de terminación. No dupliques una
  política de cierre incompatible con la del destino.
- Para una CLI sin Bubble Tea, no copies `fade.go` ni `fade_test.go` y no agregues
  Bubble Tea. El splash por sí solo no lo utiliza.
- Si ya usa Bubble Tea, copia los cuatro archivos y adapta los cambios del
  ejemplo `ejemplos/bubbletea.md` al modelo real. El fade-in es visual y no debe
  bloquear mensajes, teclas ni comandos.
- No copies `main.go`, `model.go` ni estilos de la aplicación de origen. Los
  ejemplos muestran puntos de inserción, no archivos que debas reemplazar.

## Requisitos que debes conservar

- Duración del splash de aproximadamente 1,5–2 segundos; esta versión usa 1,8.
- Tamaño de la figura entre 50 y 70 columnas; esta versión usa 64 por 30 filas.
- Forma circular, plato superior derecho y trinchera horizontal reconocibles.
- Caracteres de terminal, sin imágenes, videos, fuentes descargadas ni assets.
- Colores RGB de 24 bits y brillo calculado por posición y tiempo.
- Onda suave que atraviesa la figura, sin parpadeo global.
- Aparición progresiva y fade-out completos.
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

## Validación

1. Formatea los archivos Go modificados con `gofmt`.
2. Dentro del módulo de destino, ejecuta `go test ./...` y `go vet ./...`.
3. Compila con el comando habitual del proyecto o su Makefile.
4. Prueba en un terminal de al menos 66 por 32: comprueba el plato, la trinchera,
   las tres fases, el cursor y la continuación de la aplicación.
5. Comprueba salida redirigida, terminal pequeño, cambio de tamaño y Ctrl+C
   durante el splash. Tras Ctrl+C, el menú no debe empezar a ejecutarse.
6. Si integraste el fade-in, comprueba que empieza cuando hay datos, que el menú
   conserva sus colores al terminar y que las teclas funcionan durante el fade.
7. Corrige los problemas introducidos y describe los cambios y verificaciones.

No hagas commits, despliegues ni modificaciones fuera del alcance de la
integración salvo que el usuario lo pida. No conviertas este kit en otra TUI
ni reorganices la aplicación para acomodarlo.
