package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Создаем экземпляр нашего приложения
	app := NewApp()

	// Создаем приложение с опциями
	err := wails.Run(&options.App{
		Title:            "JustVpn",
		Width:            520,  // Начальная ширина для окна активации
		Height:           420,  // Начальная высота для окна активации
		DisableResize:    true, // Запрещаем изменение размера окна
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 45, G: 45, B: 45, A: 1}, // Цвет фона как в референсе
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown, // Гарантирует отключение VPN при закрытии
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}