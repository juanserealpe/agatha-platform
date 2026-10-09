// Command callback ejecuta el monolito con el callback de Dataflow Labs como
// ingesta PRINCIPAL (default de la imagen Docker): POST /ingestion/callback +
// Baseline (scheduler de recálculo) + Processing (subscriber + reconciliador
// horario) + Dashboard (API HTTP), todo en :8080.
//
// NO arranca el scheduler de DataOS ni requiere sus credenciales: esa consulta
// queda como contingencia en cmd/platform (APP=platform) y cmd/ingestion.
// Correr ambas ingestas a la vez duplicaría filas en raw_measurements.
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
	log := logger.New("Callback", "Info")

	// 1. Infraestructura compartida: .env, BD y bus de eventos.
	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	// 2. Baseline (solo lee la BD).
	baselineModule, err := bootstrap.NewBaselineModule(platform)
	if err != nil {
		os.Exit(1)
	}

	// 3. Processing: su reconciliador completa las horas sin fila.
	processingModule, err := bootstrap.NewProcessingModule(platform, baselineModule)
	if err != nil {
		os.Exit(1)
	}

	// 4. Ingestion por callback (migra el esquema de Ingestion).
	callbackModule, err := bootstrap.NewCallbackModule(platform)
	if err != nil {
		log.Error("no fue posible crear el callback de ingesta", "error", err)
		os.Exit(1)
	}

	// 5. Rutas HTTP: callback + Dashboard en el mismo servidor.
	mux := http.NewServeMux()
	callbackModule.RegisterRoutes(mux)
	bootstrap.NewDashboardModule(platform).RegisterRoutes(mux)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
	}

	// SIGINT/SIGTERM detienen schedulers y servidor; luego se drena el bus.
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
		log.Info("servidor HTTP iniciado (callback + dashboard)", "addr", httpAddr)
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

	wg.Wait()

	if err := platform.Close(shutdownCtx); err != nil {
		log.Error("el bus de eventos no terminó de drenar antes del cierre", "error", err)
	}

	log.Info("plataforma detenida")
}
