package main

import (
	"net/http"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const httpAddr = ":8080"

func main() {
	log := logger.New("Api", "Info")

	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	mux := http.NewServeMux()
	bootstrap.NewDashboardModule(platform).RegisterRoutes(mux)

	handler, err := bootstrap.WithCORS(mux)
	if err != nil {
		log.Error("configuración CORS inválida", "variable", bootstrap.CORSAllowedOriginsEnv, "error", err)
		os.Exit(2)
	}

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Info("servidor HTTP iniciado", "addr", httpAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Error("el servidor HTTP terminó con error", "error", err)
		os.Exit(1)
	}
}
