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

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Info("servidor HTTP iniciado", "addr", httpAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Error("el servidor HTTP terminó con error", "error", err)
		os.Exit(1)
	}
}
