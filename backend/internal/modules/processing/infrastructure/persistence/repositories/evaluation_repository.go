package repositories

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

// saveBatchSize acota los parámetros por INSERT (≈ 35 columnas por fila).
const saveBatchSize = 200

// EvaluationRepository persiste feature_evaluations.
type EvaluationRepository struct {
	db *gorm.DB
}

func NewEvaluationRepository(db *gorm.DB) *EvaluationRepository {
	return &EvaluationRepository{db: db}
}

// GetLastEvaluatedAt devuelve MAX(evaluated_at) de la ventana de 24 h (usa el índice único).
func (r *EvaluationRepository) GetLastEvaluatedAt(ctx context.Context, pointID int64) (*time.Time, error) {
	var last sql.NullTime

	err := r.db.WithContext(ctx).
		Model(&models.FeatureEvaluationModel{}).
		Select("MAX(evaluated_at)").
		Where("measuring_point_id = ? AND window_hours = ?", pointID, entities.Window24h).
		Row().
		Scan(&last)
	if err != nil {
		return nil, err
	}

	if !last.Valid {
		return nil, nil
	}

	utc := last.Time.UTC()
	return &utc, nil
}

// SaveAll inserta todas las filas en una TX con ON CONFLICT DO NOTHING sobre
// (measuring_point_id, window_hours, evaluated_at): o todas las horas o ninguna.
func (r *EvaluationRepository) SaveAll(ctx context.Context, evals []entities.FeatureEvaluation) (int, error) {
	if len(evals) == 0 {
		return 0, nil
	}

	rows := make([]models.FeatureEvaluationModel, len(evals))
	for i, e := range evals {
		rows[i] = mappers.ToFeatureEvaluationModel(e)
	}

	var inserted int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "measuring_point_id"},
					{Name: "window_hours"},
					{Name: "evaluated_at"},
				},
				DoNothing: true,
			}).
			CreateInBatches(&rows, saveBatchSize)
		if result.Error != nil {
			return result.Error
		}

		inserted = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}

	return int(inserted), nil
}

// ReplaceRange, en una TX: DELETE de las filas del punto con evaluated_at en
// [from, to] (from nil ⇒ sin límite inferior) → INSERT de evals. Todo o nada.
func (r *EvaluationRepository) ReplaceRange(ctx context.Context, pointID int64, from *time.Time, to time.Time,
	evals []entities.FeatureEvaluation) (int, int, error) {
	rows := make([]models.FeatureEvaluationModel, len(evals))
	for i, e := range evals {
		rows[i] = mappers.ToFeatureEvaluationModel(e)
	}

	var deleted, inserted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("measuring_point_id = ? AND evaluated_at <= ?", pointID, to.UTC())
		if from != nil {
			q = q.Where("evaluated_at >= ?", from.UTC())
		}
		result := q.Delete(&models.FeatureEvaluationModel{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected

		if len(rows) == 0 {
			return nil
		}
		result = tx.CreateInBatches(&rows, saveBatchSize)
		if result.Error != nil {
			return result.Error
		}
		inserted = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return int(deleted), int(inserted), nil
}
