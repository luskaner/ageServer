package main

import (
	"embed"

	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// main initializes the configuration application and runs its window.
func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "config",
		Description: "ageServer configuration editor",
		Services: []application.Service{
			application.NewService(&ConfigService{}),
			application.NewService(&SystemService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	// 'BackgroundType' asks for a translucent window so the platform backdrop
	// stays visible behind the webview: Mica on Windows 11 (see
	// backdrop_windows.go for the older Windows fallback), the vibrancy
	// material on macOS and a compositor-transparent window on Linux.
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "ageServer Server Configurator",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:  1000,
		Height: 618,
		// Floor for the layout: the interface still holds together at about
		// 580 x 430 (measured), where the game grid already falls back to a
		// single column; these leave a little margin above that.
		MinWidth:         640,
		MinHeight:        480,
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			BackdropType: application.Mica,
		},
		EnableFileDrop: true,
		URL:            "/",
	})
	// Some backdrops can only be applied once the native window exists.
	setupWindowBackdrop(window)

	// The desktop reports theme changes (light/dark scheme, accent colour) while
	// the application runs; pass them on so the interface can follow along. A
	// change of the accent colour is not part of that report on every platform,
	// so it is watched as well.
	app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(*application.ApplicationEvent) {
		app.Event.Emit("system-theme-changed")
	})
	watchAccentColour(func(string) {
		app.Event.Emit("system-theme-changed")
	})
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		application.Get().Event.Emit("files-dropped", map[string]any{
			"files":   event.Context().DroppedFiles(),
			"details": event.Context().DropTargetDetails(),
		})
	})

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
