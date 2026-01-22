package main

import (
	"embed"
	"log"
	app "splash-trading-bot/src"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Создаем структуру приложения
	app := app.NewApp()

	// Запускаем окно Wails
	err := wails.Run(&options.App{
		Title:  "Terminus Splash Analytics",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 15, A: 255}, // Темный фон
		OnStartup:        app.Startup,                                // Здесь запустится твой client.StartPolling
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal("Error:", err)
	}
}
