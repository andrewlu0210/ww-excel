package main

import (
	"embed"
	"log"
	"os"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ww-excel/internal/model"
	"ww-excel/internal/service"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Registered events get strongly typed TS bindings.
	application.RegisterEvent[service.BenchEvent](service.BenchEventName)
}

func main() {
	// v0.1 uses generated data; WW_FAKE_ROWS / WW_FAKE_COLS override the size.
	wb := model.NewWorkbook()
	model.FillFake(wb.Sheets[0], envInt("WW_FAKE_ROWS", 20_000), envInt("WW_FAKE_COLS", 5), 1)

	sheets := service.NewSheetService(wb)

	app := application.New(application.Options{
		Name:        "WW Excel",
		Description: "Spreadsheet app for macOS",
		Services: []application.Service{
			application.NewService(sheets),
			application.NewService(service.NewBenchService(sheets)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "WW Excel",
		Width:            1280,
		Height:           800,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}
