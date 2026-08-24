package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/mapping"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestService struct {
	dataosClient    *dataos.Client
	mapper          *mapping.Mapper
	seriesExtractor *mapping.SeriesExtractor

	deviceRepository           repositories.DeviceRepository
	metricDefinitionRepository repositories.MetricDefinitionRepository
	measurementRepository      repositories.MeasurementRepository

	logger *logger.Logger
}

func NewIngestService(
	dataosClient *dataos.Client,
	mapper *mapping.Mapper,
	seriesExtractor *mapping.SeriesExtractor,
	deviceRepository repositories.DeviceRepository,
	metricDefinitionRepository repositories.MetricDefinitionRepository,
	measurementRepository repositories.MeasurementRepository,
	logger *logger.Logger,
) *IngestService {
	return &IngestService{
		dataosClient:               dataosClient,
		mapper:                     mapper,
		seriesExtractor:            seriesExtractor,
		deviceRepository:           deviceRepository,
		metricDefinitionRepository: metricDefinitionRepository,
		measurementRepository:      measurementRepository,
		logger:                     logger,
	}
}

func (s *IngestService) Ingest(
	ctx context.Context,
	device *entities.Device,
	start time.Time,
	end time.Time,
) error {
	return nil
}
