package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Estructuras para mantener el estado de la aplicación
type AppState struct {
	app        *tview.Application
	procPane   *tview.TextView
	tailPane   *tview.TextView
	logPane    *tview.TextView
	configPane *tview.Form
	flex       *tview.Flex
	target     string
	scanDir    string
	htmlPath   string
	scanOpts   *ScanOptions
}

// Configura la UI de la aplicación
func setupUI() *AppState {
	state := &AppState{
		scanOpts: &ScanOptions{},
	}

	// Crear aplicación tview
	state.app = tview.NewApplication()

	// Configurar estilos globales
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDarkBlue
	tview.Styles.ContrastBackgroundColor = tcell.ColorDarkBlue
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorDarkBlue
	tview.Styles.BorderColor = tcell.ColorGreen
	tview.Styles.TitleColor = tcell.ColorGreen
	tview.Styles.GraphicsColor = tcell.ColorLightCyan
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.SecondaryTextColor = tcell.ColorLightGrey

	// Crear paneles
	state.procPane = tview.NewTextView()
	state.procPane.SetTitle("nmap processes (top)")
	state.procPane.SetBorder(true)
	state.procPane.SetBackgroundColor(tcell.ColorDarkBlue)

	state.tailPane = tview.NewTextView()
	state.tailPane.SetTitle("ports.nmap (tail)")
	state.tailPane.SetBorder(true)
	state.tailPane.SetBackgroundColor(tcell.ColorDarkBlue)

	state.logPane = tview.NewTextView()
	state.logPane.SetTitle("scan log")
	state.logPane.SetBorder(true)
	state.logPane.SetBackgroundColor(tcell.ColorDarkBlue)
	state.logPane.SetChangedFunc(func() {
		state.logPane.ScrollToEnd()
		state.app.Draw()
	})

	// Crear panel de configuración
	state.configPane = tview.NewForm()
	state.configPane.SetTitle("Scan Configuration")
	state.configPane.SetBorder(true)
	state.configPane.SetBackgroundColor(tcell.ColorDarkBlue)

	// Variable para mostrar el comando nmap
	cmdTextView := tview.NewTextView().SetDynamicColors(true)
	cmdTextView.SetBorder(false)

	// Función para construir el comando nmap
	updateCmd := func() {
		cmd := buildNmapCommandPreview(state)
		cmdTextView.SetText("[yellow]nmap " + cmd)
	}

	// Agregar campos de configuración
	state.configPane.AddDropDown("Scan Type", []string{"TCP SYN (-sS)", "TCP Connect (-sT)", "UDP (-sU)", "Custom"}, 0, func(option string, index int) {
		switch index {
		case 0:
			state.scanOpts.ScanType = "sS"
		case 1:
			state.scanOpts.ScanType = "sT"
		case 2:
			state.scanOpts.ScanType = "sU"
		case 3:
			state.scanOpts.ScanType = ""
		}
		updateCmd()
	})

	state.configPane.AddDropDown("Timing Template", []string{"Paranoid (T0)", "Sneaky (T1)", "Polite (T2)", "Normal (T3)", "Aggressive (T4)", "Insane (T5)"}, 3, func(option string, index int) {
		state.scanOpts.Timing = fmt.Sprintf("T%d", index)
		updateCmd()
	})

	state.configPane.AddInputField("Top Ports", "1000", 10, nil, func(text string) {
		state.scanOpts.TopPorts = text
		updateCmd()
	})

	state.configPane.AddInputField("Custom Flags", "", 30, nil, func(text string) {
		state.scanOpts.CustomFlags = text
		updateCmd()
	})

	state.configPane.AddButton("Start Scan", func() {
		showNmapCommandModal(state)
	})

	// Agregar el comando nmap como preview
	state.configPane.AddFormItem(cmdTextView)
	updateCmd()

	// Crear layout 2x2
	topFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(state.configPane, 0, 1, false).
		AddItem(state.procPane, 0, 1, false)

	bottomFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(state.logPane, 0, 1, false).
		AddItem(state.tailPane, 0, 1, false)

	state.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topFlex, 0, 1, false).
		AddItem(bottomFlex, 0, 1, false)
	state.flex.SetBackgroundColor(tcell.ColorDarkBlue)

	return state
}

// Muestra un modal para editar y ejecutar el comando nmap
func showNmapCommandModal(state *AppState) {
	// Construir el comando nmap completo
	nmapCommand := "nmap " + buildNmapCommandPreview(state)
	// Reemplazar placeholders con el target real
	nmapCommand = strings.ReplaceAll(nmapCommand, "<hosts.txt>", state.target)
	nmapCommand = strings.ReplaceAll(nmapCommand, "<ports.nmap>", "ports.nmap")

	// Crear el campo de entrada para editar el comando
	inputField := tview.NewInputField().
		SetLabel("nmap command: ").
		SetText(nmapCommand).
		SetFieldWidth(0)
	inputField.SetBorder(true)
	inputField.SetTitle("nmap to execute")
	inputField.SetBackgroundColor(tcell.ColorDarkBlue)

	// Crear botones
	executeBtn := tview.NewButton("Execute").SetSelectedFunc(func() {
		// Obtener el comando editado
		editedCommand := inputField.GetText()

		// Extraer los argumentos del comando (quitar "nmap " del inicio)
		if strings.HasPrefix(editedCommand, "nmap ") {
			editedCommand = editedCommand[5:]
		}

		// Volver a la UI principal
		state.app.SetRoot(state.flex, true)

		// Iniciar el escaneo con el comando personalizado
		startScanWithCustomCommand(state, editedCommand)
	})
	executeBtn.SetBackgroundColor(tcell.ColorGreen)
	executeBtn.SetLabelColor(tcell.ColorBlack)

	cancelBtn := tview.NewButton("Cancel").SetSelectedFunc(func() {
		state.app.SetRoot(state.flex, true)
	})
	cancelBtn.SetBackgroundColor(tcell.ColorRed)
	cancelBtn.SetLabelColor(tcell.ColorWhite)

	// Crear layout del modal
	buttonsRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(executeBtn, 0, 1, false).
		AddItem(tview.NewBox(), 1, 0, false). // Espaciador
		AddItem(cancelBtn, 0, 1, false)

	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 0, 1, false). // Espaciador superior
		AddItem(inputField, 3, 0, true).
		AddItem(buttonsRow, 3, 0, false).
		AddItem(tview.NewBox(), 0, 1, false) // Espaciador inferior

	// Configurar manejo de teclas
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			state.app.SetRoot(state.flex, true)
			return nil
		case tcell.KeyEnter:
			// Si estamos en el input field, ejecutar
			if state.app.GetFocus() == inputField {
				// Obtener el comando editado
				editedCommand := inputField.GetText()

				// Extraer los argumentos del comando (quitar "nmap " del inicio)
				if strings.HasPrefix(editedCommand, "nmap ") {
					editedCommand = editedCommand[5:]
				}

				// Volver a la UI principal
				state.app.SetRoot(state.flex, true)

				// Iniciar el escaneo con el comando personalizado
				startScanWithCustomCommand(state, editedCommand)
				return nil
			}
		case tcell.KeyTab:
			// Cambiar foco entre elementos
			if state.app.GetFocus() == inputField {
				state.app.SetFocus(executeBtn)
			} else if state.app.GetFocus() == executeBtn {
				state.app.SetFocus(cancelBtn)
			} else {
				state.app.SetFocus(inputField)
			}
			return nil
		}
		return event
	})

	// Establecer foco inicial en el campo de entrada
	state.app.SetRoot(modal, true).SetFocus(inputField)
}

// Inicia el escaneo con un comando nmap personalizado editado por el usuario
func startScanWithCustomCommand(state *AppState, customCommand string) {
	go func() {
		// Configurar directorios de salida
		ts := time.Now().Format("20060102_150405")
		state.scanDir = "test_" + ts
		os.MkdirAll(state.scanDir, 0755)
		state.htmlPath = state.scanDir + "/report.html"

		// Obtener información de red
		hostIP, _ := run("sh", "-c", `ifconfig en0 | grep "inet " | awk '{print $2}'`)
		gateway, _ := run("sh", "-c", `route -n get default | grep gateway | awk '{print $2}'`)

		// Escribir mensaje de log
		fmt.Printf("[!] Executing custom nmap command: nmap %s\n", customCommand)

		// Realizar descubrimiento de hosts si el comando no lo incluye ya
		if !strings.Contains(customCommand, "-sn") && !strings.Contains(customCommand, "-iL") {
			performHostDiscovery(state)
		}

		// Ejecutar el comando nmap personalizado
		// Dividir el comando en argumentos individuales
		args := strings.Fields(customCommand)

		// Si el comando no incluye archivo de salida, agregarlo
		if !strings.Contains(customCommand, "-oN") && !strings.Contains(customCommand, "-oG") && !strings.Contains(customCommand, "-oX") {
			args = append(args, "-oN", state.scanDir+"/ports.nmap")
		}

		// Si el comando no especifica target y no usa -iL, usar el target actual
		if !strings.Contains(customCommand, "-iL") && !strings.Contains(customCommand, state.target) {
			args = append(args, state.target)
		}

		// Ejecutar nmap con los argumentos personalizados
		run("nmap", args...)

		// Generar reporte
		var hostsData []byte
		var portsData []byte

		if _, err := os.Stat(state.scanDir + "/hosts.txt"); err == nil {
			hostsData, _ = ioutil.ReadFile(state.scanDir + "/hosts.txt")
		} else {
			// Si no hay archivo hosts.txt, crear uno con el target actual
			hostsData = []byte(state.target + "\n")
		}

		if _, err := os.Stat(state.scanDir + "/ports.nmap"); err == nil {
			portsData, _ = ioutil.ReadFile(state.scanDir + "/ports.nmap")
		} else {
			portsData = []byte("No output file found.\n")
		}

		htmlContent := generateHTMLReport(state, hostIP, gateway, hostsData, portsData)
		ioutil.WriteFile(state.htmlPath, []byte(htmlContent), 0644)

		// Mostrar popup con resultados
		showCompletionPopup(state)
	}()
}
