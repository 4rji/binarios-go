# Integración en un modelo Bubble Tea existente

Estos son fragmentos de inserción. Conserva los campos, comandos y mensajes del
modelo real. No reemplaces sus archivos completos con estos ejemplos.

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

## 6. Comprueba la integración

La vista debe permanecer oscura hasta completar la carga inicial. Después se
revela en 900 ms, sin bloquear las teclas. Al acabar tiene exactamente el texto
y los colores originales. No debe haber ticks adicionales después de ese fade.

Ejecuta `gofmt`, las pruebas, `go vet` y la compilación desde el módulo correcto.
