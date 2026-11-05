package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	ingestionScheduler, err := app.NewScheduler()
	if err != nil {
		app.Logger.Error("no fue posible crear el scheduler de ingesta", "error", err)
		os.Exit(1)
	}

	// SIGINT/SIGTERM detienen el scheduler para que el bus drene en el defer.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Baseline arranca antes que Ingestion: su primera ejecución crea las
	// versiones faltantes. Se espera a que termine antes de drenar el bus.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.Baseline.NewScheduler().Start(ctx)
	}()

	app.Logger.Info("aplicación de ingesta iniciada")

	ingestionScheduler.Start(ctx)
	wg.Wait()
}
