package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

func ToIngestionStateModel(entity *entities.IngestionState) *models.IngestionStateModel {
	if entity == nil {
		return nil
	}

	return &models.IngestionStateModel{
		ID:               entity.ID,
		MeasuringPointID: entity.MeasuringPointID,
		Source:           entity.Source,
		InitialDate:      entity.InitialDate,
		LastIngestedAt:   entity.LastIngestedAt,
		ReceivedRecords:  entity.ReceivedRecords,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func ToIngestionStateEntity(model *models.IngestionStateModel) *entities.IngestionState {
	if model == nil {
		return nil
	}

	return &entities.IngestionState{
		ID:               model.ID,
		MeasuringPointID: model.MeasuringPointID,
		Source:           model.Source,
		InitialDate:      model.InitialDate,
		LastIngestedAt:   model.LastIngestedAt,
		ReceivedRecords:  model.ReceivedRecords,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}
