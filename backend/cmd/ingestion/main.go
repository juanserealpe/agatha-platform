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

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.Close(shutdownCtx); err != nil {
			app.Logger.Error("el bus de eventos no terminó de drenar antes del cierre", "error", err)
		}
	}()

	location, err := time.LoadLocation("America/Bogota")
	if err != nil {
		app.Logger.Error("no fue posible cargar la zona horaria de Colombia", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	start := time.Date(2026, 9, 28, 20, 18, 0, 0, location)
	end := time.Date(2026, 9, 28, 23, 59, 59, 0, location)
	if err := app.IngestService.IngestDataOs(ctx, 11556, start, end); err != nil {
		app.Logger.Error("error durante la ingesta programada", "error", err)
		return
	}

	app.Logger.Info("aplicación de ingesta iniciada")
}
