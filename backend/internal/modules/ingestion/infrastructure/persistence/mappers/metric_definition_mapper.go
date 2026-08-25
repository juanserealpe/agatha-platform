package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

func ToMetricDefinitionModel(
	entity *entities.MetricDefinition,
) *models.MetricDefinitionModel {
	if entity == nil {
		return nil
	}

	return &models.MetricDefinitionModel{
		Key:       entity.Key,
		Name:      entity.Name,
		Unit:      entity.Unit,
		Category:  string(entity.Category),
		Active:    entity.Active,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
}

func ToMetricDefinitionEntity(
	model *models.MetricDefinitionModel,
) *entities.MetricDefinition {

	if model == nil {
		return nil
	}

	return &entities.MetricDefinition{
		ID:        model.ID,
		Key:       model.Key,
		Name:      model.Name,
		Unit:      model.Unit,
		Category:  enums.MetricCategory(model.Category),
		Active:    model.Active,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
