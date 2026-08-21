package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type HTTPHandler struct {
	service *services.IngestService
	logger  *logger.Logger
}

func NewHTTPHandler(service *services.IngestService, logger *logger.Logger) *HTTPHandler {
	return &HTTPHandler{
		service: service,
		logger:  logger,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/ingest", h.IngestData)
	router.GET("/ingest/health", h.HealthCheck)
}

func (h *HTTPHandler) IngestData(c *gin.Context) {
	var req dtos.IngestRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid request format", "error", err)
		c.JSON(http.StatusBadRequest, dtos.IngestResponse{
			Status:  "rejected",
			Message: "Invalid request format: " + err.Error(),
		})
		return
	}

	response, err := h.service.ProcessIngest(&req)
	if err != nil {
		h.logger.Error("Failed to process ingest", "error", err)
		c.JSON(http.StatusInternalServerError, dtos.IngestResponse{
			Status:  "error",
			Message: "Failed to process data: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *HTTPHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"module": "ingestion",
		"status": "healthy",
	})
}
