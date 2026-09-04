package main

import (
	"context"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
)

func main() {
	app, err := bootstrap.NewIngestionApplication()
	if err != nil {
		os.Exit(1)
	}

	location, err := time.LoadLocation("America/Bogota")
	if err != nil {
		app.Logger.Error("no fue posible cargar la zona horaria de Colombia", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, location)
	end := time.Date(2026, 9, 3, 11, 10, 0, 0, location)
	if err := app.IngestService.IngestDataOs(ctx, 11556, start, end); err != nil {
		app.Logger.Error("error durante la ingesta programada", "error", err)
		return
	}

	app.Logger.Info("aplicación de ingesta iniciada")
}
