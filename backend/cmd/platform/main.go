// Command platform ejecuta el monolito completo en un solo proceso:
// Baseline (scheduler de recálculo), Processing (subscriber del bus +
// reconciliador horario), Ingestion (scheduler) y Dashboard (API HTTP).
//
// Los tres comparten la misma Platform (BD + bus en memoria). Por eso viven en
// el mismo proceso: el bus no cruza procesos ni contenedores.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const (
	httpAddr        = ":8080"
	shutdownTimeout = 10 * time.Second
)

func main() {
	log := logger.New("Platform", "Info")

	// 1. Infraestructura compartida: .env, BD y bus de eventos.
	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	// 2. Baseline (solo lee la BD). Migra su esquema; su scheduler arranca antes que el de Ingestion.
	baselineModule, err := bootstrap.NewBaselineModule(platform)
	if err != nil {
		os.Exit(1)
	}

	// 3. Processing (consumidor). Migra feature_evaluations y se suscribe ANTES de que Ingestion publique.
	processingModule, err := bootstrap.NewProcessingModule(platform, baselineModule)
	if err != nil {
		os.Exit(1)
	}

	// 4. Ingestion (publicador). Migra el esquema y valida credenciales de DataOS.
	ingestion, err := bootstrap.NewIngestionModule(platform)
	if err != nil {
		os.Exit(1)
	}

	ingestionScheduler, err := ingestion.NewScheduler()
	if err != nil {
		log.Error("no fue posible crear el scheduler de ingesta", "error", err)
		os.Exit(1)
	}

	// 5. Dashboard (API HTTP de solo lectura).
	mux := http.NewServeMux()
	bootstrap.NewDashboardModule(platform).RegisterRoutes(mux)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// SIGINT/SIGTERM detienen scheduler y servidor; luego se drena el bus.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("scheduler de línea base iniciado")
		baselineModule.NewScheduler().Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("reconciliador de processing iniciado")
		processingModule.NewScheduler().Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("scheduler de ingesta iniciado")
		ingestionScheduler.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("servidor HTTP iniciado", "addr", httpAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("el servidor HTTP terminó con error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("apagando la plataforma")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("el servidor HTTP no terminó de cerrar a tiempo", "error", err)
	}

	// Espera a que los schedulers (Baseline, Processing e Ingestion) terminen su ejecución en curso antes de drenar el bus.
	wg.Wait()

	if err := platform.Close(shutdownCtx); err != nil {
		log.Error("el bus de eventos no terminó de drenar antes del cierre", "error", err)
	}

	log.Info("plataforma detenida")
}
