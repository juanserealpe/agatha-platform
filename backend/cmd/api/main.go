package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	ingestion "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger := logger.New("info")

	ingestionModule := ingestion.NewModule(logger)

	router := gin.Default()

	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Next()
	})

	v1 := router.Group("/api/v1")
	{
		ingestionModule.RegisterRoutes(v1)
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"modules": []string{"ingestion"},
		})
	})

	log.Printf("Server running on port %s", port)
	log.Printf("Endpoint: POST http://localhost:%s/api/v1/ingest", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
