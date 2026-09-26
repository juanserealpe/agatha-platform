package main

import (
	"context"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/scheduler"
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

	ingestionScheduler := scheduler.NewIngestionScheduler(
		app.IngestService,
		app.Logger,
		11556,
		15*time.Minute,
		location,
		50*time.Second,
	)

	ctx := context.Background()

	app.Logger.Info("aplicación de ingesta iniciada")

	ingestionScheduler.Start(ctx)
}
