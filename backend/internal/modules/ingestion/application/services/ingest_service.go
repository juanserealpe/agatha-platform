package services

import (
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestService struct {
	repo   repositories.MeasurementRepository
	logger *logger.Logger
}

func NewIngestService(repo repositories.MeasurementRepository, logger *logger.Logger) *IngestService {
	return &IngestService{
		repo:   repo,
		logger: logger,
	}
}

func (s *IngestService) ProcessIngest(req *dtos.IngestRequest) (*dtos.IngestResponse, error) {
	if req.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	measurement := &entities.RawMeasurement{
		DeviceID:   req.DeviceID,
		Timestamp:  req.Timestamp,
		Payload:    req.Payload,
		ReceivedAt: time.Now(),
	}

	if err := s.repo.Save(measurement); err != nil {
		s.logger.Error("Failed to save measurement", "error", err)
		return nil, err
	}

	s.logger.Info("Data ingested successfully",
		"device_id", req.DeviceID,
		"timestamp", req.Timestamp)

	return &dtos.IngestResponse{
		Status:    "accepted",
		Message:   "Data received successfully",
		Timestamp: time.Now(),
	}, nil
}
