package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[VaultEvent]("vault:state")
}

func main() {
	vaultService, err := NewVaultService("")
	if err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{
		Name:        "NukeVault",
		Description: "Disposable encrypted laboratory vault",
		Services: []application.Service{
			application.NewService(vaultService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	vaultService.setEmitter(func(name string, data any) {
		app.Event.Emit(name, data)
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "NukeVault Lab",
		Width:  1000,
		Height: 680,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(5, 7, 12),
		URL:              "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
