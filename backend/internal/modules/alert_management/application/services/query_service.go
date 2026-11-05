package services

import (
	"context"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Tamaños de las consultas (HU-9.1, CA-2.1.4, RN-15).
const (
	PageSize            = 10
	MaxExportRows       = 10_000
	DefaultPendingLimit = 5
	maxPendingLimit     = 50
	// DefaultDeviceHistoryDays es lo que pide la app (HU-8.3).
	DefaultDeviceHistoryDays = 30
)

// FilterParams son los filtros tal como llegan de la API (los mismos para la
// búsqueda y la exportación, así el archivo siempre coincide con la tabla).
type FilterParams struct {
	PointID int64
	// From / To son fechas dd/mm/aaaa o aaaa-mm-dd sobre la fecha de inicio,
	// de 00:00 a 23:59 hora Colombia. Con una sola, el rango queda abierto.
	From, To  string
	MaxLevels []string
	Statuses  []string
	Events    []string
}

// SearchPage es una página del historial.
type SearchPage struct {
	Total    int64
	Page     int
	PageSize int
	Items    []ports.AlertSummary
}

// ActiveDevice son las alertas activas de un dispositivo: el color del punto
// es el de mayor criticidad (RN-03) y titila si alguna es rojo sin clasificar.
type ActiveDevice struct {
	MeasuringPointID int64
	PK               string
	Level            entities.Level
	Blinking         bool
	Alerts           []ports.AlertSummary
}

// QueryService son las consultas de la API y la exportación.
type QueryService struct {
	queries ports.AlertQueries
	writer  ports.SpreadsheetWriter
	clock   ports.Clock
	log     *logger.Logger
}

func NewQueryService(queries ports.AlertQueries, writer ports.SpreadsheetWriter, clock ports.Clock, log *logger.Logger) *QueryService {
	return &QueryService{queries: queries, writer: writer, clock: clock, log: log}
}

// ParseFilter valida los filtros. Fin anterior a inicio ⇒ no busca (CA-9.1.7).
func ParseFilter(p FilterParams) (ports.SearchFilter, error) {
	f := ports.SearchFilter{PointID: p.PointID}
	if p.PointID < 0 {
		return f, domain.ErrInvalidFilter
	}
	var err error
	if f.From, err = parseDay(p.From); err != nil {
		return f, domain.ErrInvalidFilter
	}
	if f.To, err = parseDay(p.To); err != nil {
		return f, domain.ErrInvalidFilter
	}
	if !f.To.IsZero() {
		f.To = f.To.AddDate(0, 0, 1) // hasta las 23:59 del día fin (exclusivo al día siguiente)
	}
	if !f.From.IsZero() && !f.To.IsZero() && !f.From.Before(f.To) {
		return f, domain.ErrInvalidFilter
	}
	for _, v := range splitValues(p.MaxLevels) {
		l := entities.Level(v)
		if !l.AlertLevel() {
			return f, domain.ErrInvalidFilter
		}
		f.MaxLevels = append(f.MaxLevels, l)
	}
	for _, v := range splitValues(p.Statuses) {
		s := entities.ManagementStatus(v)
		if !s.Valid() {
			return f, domain.ErrInvalidFilter
		}
		f.Statuses = append(f.Statuses, s)
	}
	for _, v := range splitValues(p.Events) {
		e := entities.EventType(v)
		if !e.Valid() {
			return f, domain.ErrInvalidFilter
		}
		f.Events = append(f.Events, e)
	}
	return f, nil
}

// parseDay convierte una fecha de Colombia a su 00:00 en UTC (vacío ⇒ cero).
func parseDay(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"02/01/2006", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, messages.Colombia()); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errors.New("fecha inválida")
}

// splitValues acepta multiselección repetida (?x=A&x=B) o separada por comas (?x=A,B).
func splitValues(values []string) []string {
	var out []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part = strings.ToUpper(strings.TrimSpace(part)); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// Search devuelve una página (10 filas, inicio descendente) y el total.
func (s *QueryService) Search(ctx context.Context, p FilterParams, page int) (SearchPage, error) {
	f, err := ParseFilter(p)
	if err != nil {
		return SearchPage{}, err
	}
	if page < 1 {
		page = 1
	}
	items, total, err := s.queries.Search(ctx, f, (page-1)*PageSize, PageSize)
	if err != nil {
		return SearchPage{}, s.consultingError("Search", err)
	}
	return SearchPage{Total: total, Page: page, PageSize: PageSize, Items: items}, nil
}

// Pending devuelve las Sin clasificar más recientes y el total (panel).
func (s *QueryService) Pending(ctx context.Context, limit int) ([]ports.AlertSummary, int64, error) {
	if limit <= 0 {
		limit = DefaultPendingLimit
	}
	limit = min(limit, maxPendingLimit)
	items, total, err := s.queries.Pending(ctx, limit)
	if err != nil {
		return nil, 0, s.consultingError("Pending", err)
	}
	return items, total, nil
}

// Active agrupa las alertas activas por dispositivo (mapa web y móvil).
func (s *QueryService) Active(ctx context.Context) ([]ActiveDevice, error) {
	items, err := s.queries.Active(ctx)
	if err != nil {
		return nil, s.consultingError("Active", err)
	}
	byPoint := map[int64]*ActiveDevice{}
	for _, it := range items {
		d, ok := byPoint[it.Alert.MeasuringPointID]
		if !ok {
			d = &ActiveDevice{MeasuringPointID: it.Alert.MeasuringPointID, PK: it.PK}
			byPoint[it.Alert.MeasuringPointID] = d
		}
		d.Alerts = append(d.Alerts, it)
		d.Level = entities.MaxLevel(d.Level, it.Alert.CurrentLevel)
		d.Blinking = d.Blinking || it.Alert.Blinking()
	}
	out := make([]ActiveDevice, 0, len(byPoint))
	for _, d := range byPoint {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MeasuringPointID < out[j].MeasuringPointID })
	return out, nil
}

// Detail devuelve la alerta completa (en cualquier estado).
func (s *QueryService) Detail(ctx context.Context, id int64) (*ports.AlertDetail, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidAlertID
	}
	d, err := s.queries.Detail(ctx, id)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, domain.ErrAlertNotFound
	}
	if err != nil {
		return nil, s.consultingError("Detail", err)
	}
	return d, nil
}

// DeviceHistory devuelve las alertas del dispositivo en [from, to) (cero ⇒ los últimos 30 días).
func (s *QueryService) DeviceHistory(ctx context.Context, pointID int64, from, to time.Time) ([]ports.AlertDetail, error) {
	if pointID <= 0 {
		return nil, domain.ErrInvalidMeasuringPointID
	}
	now := s.clock.Now().UTC()
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -DefaultDeviceHistoryDays)
	}
	if !from.Before(to) {
		return nil, domain.ErrInvalidTimeRange
	}
	items, err := s.queries.DeviceHistory(ctx, pointID, from, to)
	if err != nil {
		return nil, s.consultingError("DeviceHistory", err)
	}
	return items, nil
}

// Export escribe el Excel de la búsqueda: todas las páginas, mismo orden;
// hojas "Alertas" y "Parámetros". Más de 10.000 alertas ⇒ 400 sin archivo (CA-9.2.8).
// Devuelve el error ANTES de escribir un solo byte.
func (s *QueryService) Export(ctx context.Context, p FilterParams, w io.Writer) error {
	f, err := ParseFilter(p)
	if err != nil {
		return err
	}
	total, err := s.queries.Count(ctx, f)
	if err != nil {
		return s.consultingError("Export", err)
	}
	if total > MaxExportRows {
		return domain.ErrExportTooLarge
	}
	rows, err := s.queries.SearchDetails(ctx, f, MaxExportRows)
	if err != nil {
		return s.consultingError("Export", err)
	}
	return s.writer.Write(w, ExportSheets(rows, p, s.clock.Now()))
}

// ExportSheets arma las hojas del Excel.
func ExportSheets(rows []ports.AlertDetail, p FilterParams, generatedAt time.Time) []ports.Sheet {
	alerts := [][]string{{
		"ID de alerta", "Dispositivo", "PK", "Inicio", "Fin", "Nivel actual", "Nivel máximo", "Evento",
		"Clasificación", "Estado", "Etiqueta", "Observaciones",
		"Origen de la clasificación", "Fecha-hora de la clasificación",
		"Origen de la etiqueta", "Fecha-hora de la etiqueta", "Cambios de nivel",
	}}
	for _, d := range rows {
		a := d.Alert
		alerts = append(alerts, []string{
			a.Code, strconv.FormatInt(a.MeasuringPointID, 10), d.PK,
			FormatLocal(&a.StartedAt), EndText(a), CurrentLevelText(a), messages.LevelName(a.MaxLevel), EventName(a.EventType),
			ClassificationText(a.Management), StatusName(a.Management.Status), LabelText(a.Management),
			observationsText(d.Observations),
			originName(a.Management.ClassifiedOrigin, a.Management.ClassifiedDevice), FormatLocal(a.Management.ClassifiedAt),
			originName(a.Management.LabeledOrigin, a.Management.LabeledDevice), FormatLocal(a.Management.LabeledAt),
			levelChangesText(d.Segments),
		})
	}

	device := "Todos"
	if p.PointID > 0 {
		device = strconv.FormatInt(p.PointID, 10)
	}
	params := [][]string{
		{"Filtro", "Valor"},
		{"Dispositivo", device},
		{"Fecha inicio", orDash(p.From)},
		{"Fecha fin", orDash(p.To)},
		{"Nivel de criticidad", orDash(strings.Join(splitValues(p.MaxLevels), ", "))},
		{"Estado", orDash(strings.Join(splitValues(p.Statuses), ", "))},
		{"Evento", orDash(strings.Join(splitValues(p.Events), ", "))},
		{"Generado", FormatLocal(&generatedAt)},
		{"Número de alertas", strconv.Itoa(len(rows))},
	}
	return []ports.Sheet{{Name: "Alertas", Rows: alerts}, {Name: "Parámetros", Rows: params}}
}

// FormatLocal es una fecha-hora en hora de Colombia (vacío si nil).
func FormatLocal(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.In(messages.Colombia()).Format("02/01/2006 15:04")
}

// EndText es la fecha fin o "En curso".
func EndText(a entities.Alert) string {
	if a.State == entities.AlertActive || a.EndedAt == nil {
		return "En curso"
	}
	return FormatLocal(a.EndedAt)
}

// CurrentLevelText es el color actual; "—" si la alerta finalizó.
func CurrentLevelText(a entities.Alert) string {
	if a.State != entities.AlertActive {
		return "—"
	}
	return messages.LevelName(a.CurrentLevel)
}

// EventName es el evento en lenguaje de campo.
func EventName(e entities.EventType) string {
	if e == entities.EventTypeMovement {
		return "Movimiento"
	}
	return "Fuga"
}

// ClassificationText es la clasificación o vacío.
func ClassificationText(m entities.Management) string {
	if !m.Classified() {
		return ""
	}
	return messages.ClassificationName(m.Classification)
}

// StatusName es el estado de gestión en lenguaje de campo.
func StatusName(s entities.ManagementStatus) string {
	switch s {
	case entities.StatusClassifiedNoLabel:
		return "Clasificada sin etiqueta"
	case entities.StatusClassifiedAndLabel:
		return "Clasificada con etiqueta"
	default:
		return "Sin clasificar"
	}
}

// LabelText es la etiqueta, o el texto de "Otro ¿Cuál?"; vacío si no tiene.
func LabelText(m entities.Management) string {
	if m.Label == "" {
		return ""
	}
	if m.Label == entities.LabelOther {
		return m.LabelOtherText
	}
	return m.Label.DisplayName()
}

func observationsText(obs []entities.Observation) string {
	parts := make([]string, 0, len(obs))
	for _, o := range obs {
		parts = append(parts, FormatLocal(&o.ReceivedAt)+" ("+originName(o.Origin, o.DeviceIdentifier)+"): "+o.Text)
	}
	return strings.Join(parts, " | ")
}

func originName(o entities.Origin, device string) string {
	switch o {
	case entities.OriginWeb:
		return "Web"
	case entities.OriginMobile:
		if device != "" {
			return "App móvil (" + device + ")"
		}
		return "App móvil"
	default:
		return ""
	}
}

func levelChangesText(segments []ports.SegmentWithMessages) string {
	parts := make([]string, 0, len(segments))
	for _, s := range segments {
		parts = append(parts, messages.LevelName(s.Segment.Level)+" "+FormatLocal(&s.Segment.StartedAt))
	}
	return strings.Join(parts, " → ")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func (s *QueryService) consultingError(op string, err error) error {
	s.log.Error("[QueryService] Error consultando las alertas", "op", op, "error", err)
	return domain.ErrConsultingAlerts
}
