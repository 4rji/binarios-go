# Integración en un modelo Bubble Tea existente

Estos son fragmentos de inserción. Conserva los campos, comandos y mensajes del
modelo real. No reemplaces sus archivos completos con estos ejemplos.

Los pasos 2 a 5 agregan el fade-in (`fade.go`). Los pasos 6 y 7 agregan la
estrella fija de fondo (`backdrop.go`) y son opcionales.

## 1. Entrada

Importa el paquete local con el módulo real y ejecuta el splash antes de iniciar
la TUI:

```go
import "example.com/tu-proyecto/internal/splash"

func main() {
    splash.Play()
    p := tea.NewProgram(initialModel(), tea.WithAltScreen())
    // Conserva p.Run(), los errores y el tratamiento del modelo final.
}
```

`initialModel()` debe evaluarse antes de `p.Run()`, como en este ejemplo: allí se
crean el fade y el backdrop, que pueden consultar el color de fondo al
terminal. Si se crearan dentro de `Init`, `Update` o un comando, la respuesta
del terminal llegaría a Bubble Tea como teclas.

## 2. Estado del modelo

Agrega un campo al struct existente:

```go
menuFade splash.Fade
```

En su constructor, conserva todos los valores iniciales y agrega:

```go
menuFade: splash.NewFade(),
```

## 3. Comienza cuando estén listos los datos

En el caso que termina la carga inicial, después de guardar los datos y activar
el menú:

```go
cmd := m.menuFade.Start()
return m, cmd
```

Si ese caso ya devuelve otro comando, conserva ambos:

```go
return m, tea.Batch(comandoExistente, m.menuFade.Start())
```

## 4. Actualiza los frames

Agrega al switch de `Update`:

```go
case splash.FadeTick:
    cmd := m.menuFade.Update(msg)
    return m, cmd
```

Usa el nombre real de la variable del type switch; aquí se llama `msg`. No
cambies los demás mensajes, el manejo de teclas ni los comandos existentes.

## 5. Envuelve la vista

Renombra el cuerpo original de `View()` a un método privado, por ejemplo
`view()`, sin alterar su contenido. La nueva `View()` queda así:

```go
func (m Model) View() string {
    return m.menuFade.View(m.view())
}
```

Si el modelo usa receptores con puntero, conserva ese tipo de receptor. El
modelo final continúa siendo el mismo tipo; no requiere envolverlo en otro
modelo ni cambiar las comprobaciones de tipo después de `p.Run()`.

## 6. Estrella fija de fondo (opcional)

Requiere `tea.WithAltScreen()` y que el modelo guarde el tamaño de la ventana
(`tea.WindowSizeMsg`); si todavía no lo hace, agrega:

```go
case tea.WindowSizeMsg:
    m.width, m.height = msg.Width, msg.Height
```

Agrega el campo y su valor inicial junto al fade:

```go
backdrop splash.Backdrop
```

```go
backdrop: splash.NewBackdrop(),
```

## 7. Aplica el backdrop después del fade

El orden importa: el backdrop va por fuera, para que la estrella se quede quieta
mientras el menú aparece a su alrededor.

```go
func (m Model) View() string {
    view := m.menuFade.View(m.view())
    // Vistas con imágenes de terminal (chafa, sixel, kitty) o más altas que la
    // pantalla deben devolverse sin backdrop.
    if m.mode == modoConImagenes {
        return view
    }
    return m.backdrop.View(view, m.width, m.height)
}
```

Sustituye `m.mode == modoConImagenes` por la condición real del proyecto, o
elimina ese `if` si ninguna vista muestra imágenes.

## 8. Comprueba la integración

La vista debe permanecer invisible (del color del terminal) hasta completar la
carga inicial. Después se revela en 900 ms, sin bloquear las teclas. Al acabar
tiene exactamente el texto y los colores originales. No debe haber ticks
adicionales después de ese fade.

Con el backdrop, la estrella debe quedarse en la posición exacta en la que
terminó el splash, tenue, detrás del texto. Prueba también con un fondo de
terminal azul o claro: no debe aparecer ningún recuadro oscuro.

Ejecuta `gofmt`, las pruebas, `go vet` y la compilación desde el módulo correcto.
