package main

import (
	"log"

	"github.com/Weit145/local-sender/frontend"
	"github.com/Weit145/local-sender/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	path, err := desktop.ConfigPath()
	if err != nil {
		log.Fatal(err)
	}
	var app *application.App
	service := desktop.New(path, func(state desktop.State) { app.Event.Emit("sender:state", state) })
	app = application.New(application.Options{
		Name:        "Local Sender",
		Description: "Устройства рядом. Сообщения напрямую.",
		Services:    []application.Service{application.NewService(service)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(frontend.Assets())},
		OnShutdown:  service.Close,
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Local Sender", Name: "main", Width: 1240, Height: 820, MinWidth: 860, MinHeight: 620, URL: "/",
		BackgroundColour: application.NewRGB(5, 10, 7),
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
