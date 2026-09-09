package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	dashboardHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/repositories"
)

func main() {

	// 1. Cargar .env
	if err := godotenv.Load(); err != nil {
		fmt.Println("No se encontró archivo .env")
	}

	// 2. Obtener conexión
	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		panic("DATABASE_URL no está configurada")
	}

	// 3. Crear DB
	db, err := database.NewPostgres(dsn)
	if err != nil {
		panic(err)
	}

	// 4. Repository
	deviceRepository := repositories.NewDeviceRepository(db)

	// 5. Service
	dashboardService := services.NewDashboardService(
		deviceRepository,
	)

	// 6. Handler
	deviceHandler := dashboardHTTP.NewDeviceHandler(
		dashboardService,
	)

	// 7. Router
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /api/dashboard/devices",
		deviceHandler.GetAllDevices,
	)

	// 8. Levantar servidor
	fmt.Println("Servidor iniciado en http://localhost:8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
