package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

func IngestionGapToModel(gap *entities.IngestionGap) *models.IngestionGapModel {
	return &models.IngestionGapModel{
		ID:               gap.ID,
		MeasuringPointID: gap.MeasuringPointID,
		StartedAt:        gap.StartedAt,
		EndedAt:          gap.EndedAt,
		ExpectedRecords:  gap.ExpectedRecords,
		ReceivedRecords:  gap.ReceivedRecords,
		Status:           string(gap.Status),
		CreatedAt:        gap.CreatedAt,
		ResolvedAt:       gap.ResolvedAt,
	}
}

func IngestionGapModelToEntity(model *models.IngestionGapModel) *entities.IngestionGap {
	return &entities.IngestionGap{
		ID:               model.ID,
		MeasuringPointID: model.MeasuringPointID,
		StartedAt:        model.StartedAt,
		EndedAt:          model.EndedAt,
		ExpectedRecords:  model.ExpectedRecords,
		ReceivedRecords:  model.ReceivedRecords,
		Status:           enums.GapStatus(model.Status),
		CreatedAt:        model.CreatedAt,
		ResolvedAt:       model.ResolvedAt,
	}
}
