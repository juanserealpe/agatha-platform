// Command platform ejecuta el monolito en un solo proceso con los módulos que
// elige AGATHA_MODULES (lista separada por comas):
//
//	baseline    scheduler de recálculo de líneas base
//	processing  evaluación horaria del índice I (requiere baseline)
//	rule_engine interpreta processing.indicator_evaluated con umbrales versionados (requiere processing)
//	dataos      ingesta de contingencia: consulta programada a DataOS
//	callback    ingesta principal: POST /ingestion/callback (excluyente con dataos)
//	dashboard   API HTTP de solo lectura: GET /api/dashboard/*
//
// Vacía equivale a bootstrap.DefaultModules. Todos comparten la misma Platform
// (BD + bus en memoria): por eso viven en el mismo proceso.
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

	// 1. Infraestructura compartida: .env, BD y bus de eventos. Va primero porque
	// carga el .env del que también puede salir AGATHA_MODULES.
	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	modules, err := bootstrap.ParseModules(os.Getenv(bootstrap.ModulesEnv))
	if err != nil {
		log.Error("configuración de módulos inválida", "variable", bootstrap.ModulesEnv, "error", err)
		os.Exit(2)
	}
	log.Info("módulos habilitados", "modules", modules.String())

	// 2. Módulos habilitados, en orden de arranque.
	app, err := bootstrap.NewApplication(platform, modules)
	if err != nil {
		log.Error("no fue posible iniciar los módulos", "error", err)
		os.Exit(1)
	}

	// SIGINT/SIGTERM detienen schedulers y servidor; luego se drena el bus.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	for _, runner := range app.Runners() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info(runner.Name + " iniciado")
			runner.Start(ctx)
		}()
	}

	// 3. Servidor HTTP solo si algún módulo expone rutas.
	var server *http.Server
	if modules.HasHTTP() {
		mux := http.NewServeMux()
		app.RegisterRoutes(mux)

		handler, err := bootstrap.WithCORS(mux)
		if err != nil {
			log.Error("configuración CORS inválida", "variable", bootstrap.CORSAllowedOriginsEnv, "error", err)
			os.Exit(2)
		}

		server = &http.Server{
			Addr:              httpAddr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       15 * time.Second,
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info("servidor HTTP iniciado", "addr", httpAddr)
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("el servidor HTTP terminó con error", "error", err)
				stop()
			}
		}()
	}

	<-ctx.Done()
	log.Info("apagando la plataforma")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if server != nil {
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("el servidor HTTP no terminó de cerrar a tiempo", "error", err)
		}
	}

	// Espera a que los schedulers terminen su ejecución en curso antes de drenar el bus.
	wg.Wait()

	if err := platform.Close(shutdownCtx); err != nil {
		log.Error("el bus de eventos no terminó de drenar antes del cierre", "error", err)
	}

	log.Info("plataforma detenida")
}
