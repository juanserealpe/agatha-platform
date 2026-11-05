package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

// AlertQueries implementa ports.AlertQueries (lado de lectura de la API).
type AlertQueries struct {
	db      *gorm.DB
	devices *DeviceDirectory
}

func NewAlertQueries(db *gorm.DB) *AlertQueries {
	return &AlertQueries{db: db, devices: NewDeviceDirectory(db)}
}

func (q *AlertQueries) filtered(ctx context.Context, f ports.SearchFilter) *gorm.DB {
	db := q.db.WithContext(ctx).Model(&models.AlertModel{})
	if f.PointID > 0 {
		db = db.Where("measuring_point_id = ?", f.PointID)
	}
	if !f.From.IsZero() {
		db = db.Where("started_at >= ?", f.From.UTC())
	}
	if !f.To.IsZero() {
		db = db.Where("started_at < ?", f.To.UTC())
	}
	if len(f.MaxLevels) > 0 {
		db = db.Where("max_level IN ?", toStrings(f.MaxLevels))
	}
	if len(f.Statuses) > 0 {
		db = db.Where("management_status IN ?", toStrings(f.Statuses))
	}
	if len(f.Events) > 0 {
		db = db.Where("event_type IN ?", toStrings(f.Events))
	}
	return db
}

func toStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

func (q *AlertQueries) Count(ctx context.Context, f ports.SearchFilter) (int64, error) {
	var total int64
	err := q.filtered(ctx, f).Count(&total).Error
	return total, err
}

func (q *AlertQueries) Search(ctx context.Context, f ports.SearchFilter, offset, limit int) ([]ports.AlertSummary, int64, error) {
	total, err := q.Count(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	var rows []models.AlertModel
	if err := q.filtered(ctx, f).Order("started_at DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items, err := q.summaries(ctx, rows)
	return items, total, err
}

func (q *AlertQueries) SearchDetails(ctx context.Context, f ports.SearchFilter, limit int) ([]ports.AlertDetail, error) {
	var rows []models.AlertModel
	if err := q.filtered(ctx, f).Order("started_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return q.details(ctx, rows)
}

func (q *AlertQueries) Pending(ctx context.Context, limit int) ([]ports.AlertSummary, int64, error) {
	return q.Search(ctx, ports.SearchFilter{Statuses: []entities.ManagementStatus{entities.StatusUnclassified}}, 0, limit)
}

func (q *AlertQueries) Active(ctx context.Context) ([]ports.AlertSummary, error) {
	var rows []models.AlertModel
	if err := q.db.WithContext(ctx).Where("state = ?", string(entities.AlertActive)).
		Order("measuring_point_id, started_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return q.summaries(ctx, rows)
}

func (q *AlertQueries) Detail(ctx context.Context, id int64) (*ports.AlertDetail, error) {
	var row models.AlertModel
	err := q.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	details, err := q.details(ctx, []models.AlertModel{row})
	if err != nil {
		return nil, err
	}
	return &details[0], nil
}

func (q *AlertQueries) DeviceHistory(ctx context.Context, pointID int64, from, to time.Time) ([]ports.AlertDetail, error) {
	var rows []models.AlertModel
	err := q.db.WithContext(ctx).
		Where("measuring_point_id = ? AND started_at < ? AND (ended_at IS NULL OR ended_at >= ?)", pointID, to.UTC(), from.UTC()).
		Order("started_at DESC, id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return q.details(ctx, rows)
}

func (q *AlertQueries) summaries(ctx context.Context, rows []models.AlertModel) ([]ports.AlertSummary, error) {
	points := make([]int64, 0, len(rows))
	for _, r := range rows {
		points = append(points, r.MeasuringPointID)
	}
	names, err := q.devices.Names(ctx, points)
	if err != nil {
		return nil, err
	}
	out := make([]ports.AlertSummary, len(rows))
	for i, r := range rows {
		out[i] = ports.AlertSummary{Alert: mappers.ToAlert(r), PK: pkOrDefault(names, r.MeasuringPointID)}
	}
	return out, nil
}

// details carga colores, mensajes y observaciones de las alertas (3 consultas).
func (q *AlertQueries) details(ctx context.Context, rows []models.AlertModel) ([]ports.AlertDetail, error) {
	summaries, err := q.summaries(ctx, rows)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	out := make([]ports.AlertDetail, len(summaries))
	if len(ids) == 0 {
		return out, nil
	}

	var segments []models.AlertLevelSegmentModel
	if err := q.db.WithContext(ctx).Where("alert_id IN ?", ids).Order("alert_id, seq").Find(&segments).Error; err != nil {
		return nil, err
	}
	var msgs []models.AlertMessageModel
	if err := q.db.WithContext(ctx).Where("alert_id IN ?", ids).Order("evaluated_at, id").Find(&msgs).Error; err != nil {
		return nil, err
	}
	var obs []models.AlertObservationModel
	if err := q.db.WithContext(ctx).Where("alert_id IN ?", ids).Order("received_at, id").Find(&obs).Error; err != nil {
		return nil, err
	}

	bySegment := map[int64][]entities.Message{}
	for _, m := range msgs {
		bySegment[m.SegmentID] = append(bySegment[m.SegmentID], mappers.ToMessage(m))
	}
	byAlertSegments := map[int64][]ports.SegmentWithMessages{}
	for _, s := range segments {
		byAlertSegments[s.AlertID] = append(byAlertSegments[s.AlertID], ports.SegmentWithMessages{
			Segment: mappers.ToSegment(s), Messages: bySegment[s.ID],
		})
	}
	byAlertObs := map[int64][]entities.Observation{}
	for _, o := range obs {
		byAlertObs[o.AlertID] = append(byAlertObs[o.AlertID], mappers.ToObservation(o))
	}
	for i, s := range summaries {
		out[i] = ports.AlertDetail{AlertSummary: s, Segments: byAlertSegments[s.Alert.ID], Observations: byAlertObs[s.Alert.ID]}
	}
	return out, nil
}

func pkOrDefault(names map[int64]string, pointID int64) string {
	if n, ok := names[pointID]; ok && n != "" {
		return n
	}
	return entities.DeviceOrDefault(pointID, nil).PK
}
