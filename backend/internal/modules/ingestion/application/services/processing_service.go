package services

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)
import "github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"

type ProcessingService struct {
	logger *logger.Logger
}

func NewProcessingService(logger *logger.Logger) *ProcessingService {
	return &ProcessingService{
		logger: logger,
	}
}

func (s *ProcessingService) Process(
	ctx context.Context,
	measurements []entities.RawMeasurement,
) ([]entities.RawMeasurement, error) {

	processedMeasurements := make(
		[]entities.RawMeasurement,
		0,
		len(measurements),
	)

	for _, measurement := range measurements {

		processed := measurement

		// Aquí irá la lógica de procesamiento.
		//
		// Ejemplos futuros:
		// - Normalizar valores
		// - Validar rangos
		// - Eliminar valores inválidos
		// - Redondear valores
		// - Convertir unidades
		// - Detectar datos duplicados

		processedMeasurements = append(
			processedMeasurements,
			processed,
		)
	}

	return processedMeasurements, nil
}
