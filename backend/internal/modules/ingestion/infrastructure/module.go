package ingestion

import (
	"github.com/gin-gonic/gin"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/handlers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type Module struct {
	service *services.IngestService
	handler *handlers.HTTPHandler
	repo    repositories.MeasurementRepository
}

func NewModule(logger *logger.Logger) *Module {
	repo := persistence.NewMemoryRepository()
	service := services.NewIngestService(repo, logger)
	handler := handlers.NewHTTPHandler(service, logger)

	return &Module{
		service: service,
		handler: handler,
		repo:    repo,
	}
}

func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
}

func (m *Module) GetService() *services.IngestService {
	return m.service
}
