package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"
)

// evaluationColumns son las columnas numéricas de feature_evaluations que se
// pueden graficar. El nombre de la columna se interpola en el SQL, así que
// solo se acepta si está en esta lista (nunca viene de la petición HTTP, pero
// la lista evita que un error en el catálogo termine en SQL arbitrario).
var evaluationColumns = map[string]bool{
	"i": true, "coverage": true,
	"d": true, "p_hours": true, "ex": true, "c": true, "d_pl": true,
	"z_d": true, "z_p": true, "z_ex": true, "z_c": true,
	"s_d": true, "s_p": true, "s_ex": true, "s_c": true,
}

// EvaluationSeriesReader lee las variables de entities.SourceEvaluation desde
// feature_evaluations (tabla de Processing, solo lectura).
type EvaluationSeriesReader struct {
	db *gorm.DB
}

func NewEvaluationSeriesReader(db *gorm.DB) *EvaluationSeriesReader {
	return &EvaluationSeriesReader{db: db}
}

// Read hace una consulta por variable. El instante de cada punto es
// evaluated_at (fin de la ventana). Las horas sin valor (NO_CALCULABLE) no
// devuelven punto: quedan como hueco en la serie.
func (r *EvaluationSeriesReader) Read(
	ctx context.Context,
	deviceID int64,
	definitions []entities.SeriesDefinition,
	startDate time.Time,
	endDate time.Time,
) (map[string][]entities.SeriesPoint, error) {
	result := make(map[string][]entities.SeriesPoint, len(definitions))

	for _, definition := range definitions {
		if !evaluationColumns[definition.Column] {
			return nil, fmt.Errorf("columna de feature_evaluations no permitida: %q", definition.Column)
		}

		var rows []models.SeriesPointModel

		err := r.db.WithContext(ctx).
			Table("feature_evaluations").
			Select("evaluated_at AS timestamp, "+definition.Column+" AS value").
			Where("measuring_point_id = ? AND window_hours = ?", deviceID, definition.WindowHours).
			Where("evaluated_at >= ? AND evaluated_at < ?", startDate, endDate).
			Where(definition.Column + " IS NOT NULL").
			Order("evaluated_at ASC").
			Scan(&rows).Error
		if err != nil {
			return nil, err
		}

		points := make([]entities.SeriesPoint, 0, len(rows))
		for _, row := range rows {
			points = append(points, entities.SeriesPoint{Timestamp: row.Timestamp.UTC(), Value: row.Value})
		}
		result[definition.Key] = points
	}

	return result, nil
}
