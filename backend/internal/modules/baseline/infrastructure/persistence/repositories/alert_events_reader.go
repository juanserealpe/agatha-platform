package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

// AlertEventsReader implementa KnownEventsProvider leyendo alerts (tabla de
// Alert Management), solo lectura. Excluye TODAS las alertas del punto,
// cualquiera sea su clasificación (también Falsa alerta y Sin clasificar):
// mientras hubo una alerta, el tubo no estuvo en su condición normal de
// referencia. Una alerta ACTIVA se excluye hasta el fin del rango consultado.
type AlertEventsReader struct {
	db *gorm.DB
}

func NewAlertEventsReader(db *gorm.DB) *AlertEventsReader {
	return &AlertEventsReader{db: db}
}

// GetKnownEvents devuelve las alertas del punto que se solapan con [from, to),
// ordenadas por inicio (las activas, recortadas a to).
func (r *AlertEventsReader) GetKnownEvents(ctx context.Context, pointID int64, from, to time.Time) ([]entities.KnownEvent, error) {
	var rows []models.AlertIntervalRowModel
	err := r.db.WithContext(ctx).Raw(`
SELECT code, started_at, COALESCE(ended_at, @to) AS ended_at,
       COALESCE(label, classification, 'SIN_CLASIFICAR') AS tag
  FROM alerts
 WHERE measuring_point_id = @point
   AND started_at < @to AND COALESCE(ended_at, @to) > @from
 ORDER BY started_at`, map[string]any{"point": pointID, "from": from.UTC(), "to": to.UTC()}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]entities.KnownEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, entities.KnownEvent{From: row.StartedAt.UTC(), To: row.EndedAt.UTC(), Label: row.Code + " (" + row.Tag + ")"})
	}
	return out, nil
}
