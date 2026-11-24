package http

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

// Las fechas se devuelven en hora de Colombia con offset (RN-11): 2026-08-24T11:00:00-05:00.
func local(t time.Time) string {
	return t.In(messages.Colombia()).Format(time.RFC3339)
}

func localPtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := local(*t)
	return &s
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// DeviceRef es el dispositivo de una alerta.
type DeviceRef struct {
	ID int64  `json:"id"`
	PK string `json:"pk"`
}

// AlertItem es una fila del historial, del panel o del mapa.
type AlertItem struct {
	ID         int64     `json:"id"`
	Code       string    `json:"code"`
	Device     DeviceRef `json:"device"`
	Event      string    `json:"event"`
	State      string    `json:"state"`
	StartedAt  string    `json:"started_at"`
	EndedAt    *string   `json:"ended_at"`
	InProgress bool      `json:"in_progress"`
	// DurationHours es el tiempo transcurrido (hasta el fin, o hasta la última
	// hora procesada si sigue activa), sin las pausas entre fin y reactivación.
	// Es el mismo número que el mensaje de fin ("Duró N h").
	DurationHours    int               `json:"duration_hours"`
	CurrentLevel     *string           `json:"current_level"`
	MaxLevel         string            `json:"max_level"`
	Blinking         bool              `json:"blinking"`
	Classification   *string           `json:"classification"`
	ManagementStatus string            `json:"management_status"`
	Label            *string           `json:"label"`
	LabelOtherText   *string           `json:"label_other_text"`
	AvailableActions []entities.Action `json:"available_actions"`
}

func toItem(s ports.AlertSummary) AlertItem {
	a := s.Alert
	item := AlertItem{
		ID: a.ID, Code: a.Code, Device: DeviceRef{ID: a.MeasuringPointID, PK: s.PK}, Event: string(a.EventType),
		State: string(a.State), StartedAt: local(a.StartedAt), InProgress: a.State == entities.AlertActive,
		MaxLevel: string(a.MaxLevel), Blinking: a.Blinking(),
		Classification:   strOrNil(string(a.Management.Classification)),
		ManagementStatus: string(a.Management.Status),
		Label:            strOrNil(string(a.Management.Label)), LabelOtherText: strOrNil(a.Management.LabelOtherText),
		AvailableActions: entities.ActionsFor(a.Management.Status),
	}
	until := a.LastEvaluatedAt
	if a.State == entities.AlertActive {
		level := string(a.CurrentLevel)
		item.CurrentLevel = &level
	} else {
		item.EndedAt = localPtr(a.EndedAt)
		if a.EndedAt != nil {
			until = *a.EndedAt
		}
	}
	item.DurationHours = a.DurationHours(until)
	return item
}

func toItems(list []ports.AlertSummary) []AlertItem {
	out := make([]AlertItem, len(list))
	for i, s := range list {
		out[i] = toItem(s)
	}
	return out
}

// SearchResponse es GET /api/alerts.
type SearchResponse struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    []AlertItem `json:"items"`
}

// PendingResponse es GET /api/alerts/pending.
type PendingResponse struct {
	Total int64       `json:"total"`
	Items []AlertItem `json:"items"`
}

// ActiveDevice es un punto del mapa con sus alertas activas.
type ActiveDevice struct {
	MeasuringPointID int64       `json:"measuring_point_id"`
	PK               string      `json:"pk"`
	Level            string      `json:"level"`
	Blinking         bool        `json:"blinking"`
	Alerts           []AlertItem `json:"alerts"`
}

// ActiveResponse es GET /api/alerts/active.
type ActiveResponse struct {
	Devices []ActiveDevice `json:"devices"`
}

func toActive(list []services.ActiveDevice) ActiveResponse {
	out := ActiveResponse{Devices: make([]ActiveDevice, len(list))}
	for i, d := range list {
		out.Devices[i] = ActiveDevice{MeasuringPointID: d.MeasuringPointID, PK: d.PK, Level: string(d.Level), Blinking: d.Blinking, Alerts: toItems(d.Alerts)}
	}
	return out
}

// MessageItem es un mensaje de la línea de tiempo.
type MessageItem struct {
	ID             int64    `json:"id"`
	At             string   `json:"at"`
	Kind           string   `json:"kind"`
	Code           string   `json:"code"`
	Level          string   `json:"level"`
	IndicatorValue *float64 `json:"indicator_value"`
	Title          string   `json:"title"`
	Body           string   `json:"body"`
}

func toMessage(m entities.Message) MessageItem {
	return MessageItem{ID: m.ID, At: local(m.EvaluatedAt), Kind: string(m.Kind), Code: m.Code, Level: string(m.Level),
		IndicatorValue: m.IndicatorValue, Title: m.Title, Body: m.Body}
}

// LevelItem es un color con sus mensajes.
type LevelItem struct {
	Seq       int           `json:"seq"`
	OpenedBy  string        `json:"opened_by"`
	Level     string        `json:"level"`
	StartedAt string        `json:"started_at"`
	EndedAt   *string       `json:"ended_at"`
	Messages  []MessageItem `json:"messages"`
}

func toLevels(segments []ports.SegmentWithMessages) []LevelItem {
	out := make([]LevelItem, len(segments))
	for i, s := range segments {
		msgs := make([]MessageItem, len(s.Messages))
		for j, m := range s.Messages {
			msgs[j] = toMessage(m)
		}
		out[i] = LevelItem{Seq: s.Segment.Seq, OpenedBy: string(s.Segment.OpenedBy), Level: string(s.Segment.Level),
			StartedAt: local(s.Segment.StartedAt), EndedAt: localPtr(s.Segment.EndedAt), Messages: msgs}
	}
	return out
}

// ObservationItem es una observación.
type ObservationItem struct {
	ID               int64   `json:"id"`
	Text             string  `json:"text"`
	Kind             string  `json:"kind"`
	Step             string  `json:"step"`
	Origin           string  `json:"origin"`
	DeviceIdentifier *string `json:"device_identifier"`
	RecordedAt       string  `json:"recorded_at"`
	ReceivedAt       string  `json:"received_at"`
}

// ManagementStep es un paso de la gestión (clasificación o etiqueta).
type ManagementStep struct {
	Value            string  `json:"value"`
	OtherText        *string `json:"other_text,omitempty"`
	At               *string `json:"at"`
	Origin           string  `json:"origin"`
	DeviceIdentifier *string `json:"device_identifier"`
	RecordedAt       *string `json:"recorded_at"`
}

// AlertDetailResponse es GET /api/alerts/{id} (lo que abre "Ver detalle").
type AlertDetailResponse struct {
	AlertItem
	OpenedAt              string            `json:"opened_at"`
	ReactivationCount     int               `json:"reactivation_count"`
	LastReactivatedAt     *string           `json:"last_reactivated_at"`
	OpeningIndicatorValue float64           `json:"opening_indicator_value"`
	LastIndicatorValue    float64           `json:"last_indicator_value"`
	MaxIndicatorValue     float64           `json:"max_indicator_value"`
	LastEvaluatedAt       string            `json:"last_evaluated_at"`
	HoursCalculated       int               `json:"hours_calculated"`
	HoursNoData           int               `json:"hours_no_data"`
	LowConfidenceAtOpen   bool              `json:"low_confidence_at_open"`
	ThresholdVersionID    int64             `json:"threshold_version_id_at_open"`
	ClassificationDetail  *ManagementStep   `json:"classification_detail"`
	LabelDetail           *ManagementStep   `json:"label_detail"`
	Observations          []ObservationItem `json:"observations"`
	Levels                []LevelItem       `json:"levels"`
	// MotionHistory (motion-v2): historial del OPENED que abrió o reactivó la
	// alerta de MOVIMIENTO; MotionTableLevel, el nivel calculado máximo. null en FUGA.
	MotionHistory    map[string]any `json:"motion_history"`
	MotionTableLevel *string        `json:"motion_table_level"`
}

func toDetail(d ports.AlertDetail) AlertDetailResponse {
	a := d.Alert
	m := a.Management
	out := AlertDetailResponse{
		AlertItem: toItem(d.AlertSummary), OpenedAt: local(a.OpenedAt), ReactivationCount: a.ReactivationCount,
		LastReactivatedAt: localPtr(a.LastReactivatedAt), OpeningIndicatorValue: a.OpeningIndicatorValue,
		LastIndicatorValue: a.LastIndicatorValue, MaxIndicatorValue: a.MaxIndicatorValue, LastEvaluatedAt: local(a.LastEvaluatedAt),
		HoursCalculated: a.HoursCalculated, HoursNoData: a.HoursNoData, LowConfidenceAtOpen: a.LowConfidenceAtOpen,
		ThresholdVersionID: a.ThresholdVersionIDAtOpen, Observations: []ObservationItem{}, Levels: toLevels(d.Segments),
		MotionHistory: a.MotionHistory, MotionTableLevel: strOrNil(string(a.MotionTableLevel)),
	}
	if m.Classified() {
		out.ClassificationDetail = &ManagementStep{Value: string(m.Classification), At: localPtr(m.ClassifiedAt),
			Origin: string(m.ClassifiedOrigin), DeviceIdentifier: strOrNil(m.ClassifiedDevice), RecordedAt: localPtr(m.ClassifiedRecordedAt)}
	}
	if m.Label != "" {
		out.LabelDetail = &ManagementStep{Value: string(m.Label), OtherText: strOrNil(m.LabelOtherText), At: localPtr(m.LabeledAt),
			Origin: string(m.LabeledOrigin), DeviceIdentifier: strOrNil(m.LabeledDevice), RecordedAt: localPtr(m.LabeledRecordedAt)}
	}
	for _, o := range d.Observations {
		out.Observations = append(out.Observations, ObservationItem{ID: o.ID, Text: o.Text, Kind: string(o.Kind), Step: string(o.Step),
			Origin: string(o.Origin), DeviceIdentifier: strOrNil(o.DeviceIdentifier), RecordedAt: local(o.RecordedAt), ReceivedAt: local(o.ReceivedAt)})
	}
	return out
}

// DeviceHistoryAlert es una alerta del historial del dispositivo.
type DeviceHistoryAlert struct {
	ID                int64       `json:"id"`
	Code              string      `json:"code"`
	Event             string      `json:"event"`
	State             string      `json:"state"`
	StartedAt         string      `json:"started_at"`
	EndedAt           *string     `json:"ended_at"`
	MaxLevel          string      `json:"max_level"`
	ReactivationCount int         `json:"reactivation_count"`
	Classification    *string     `json:"classification"`
	Label             *string     `json:"label"`
	Levels            []LevelItem `json:"levels"`
}

// DeviceHistoryResponse es GET /api/alerts/devices/{id}/alerts.
type DeviceHistoryResponse struct {
	MeasuringPointID int64                `json:"measuring_point_id"`
	PK               string               `json:"pk"`
	Alerts           []DeviceHistoryAlert `json:"alerts"`
}

func toDeviceHistory(pointID int64, list []ports.AlertDetail) DeviceHistoryResponse {
	out := DeviceHistoryResponse{MeasuringPointID: pointID, PK: entities.DeviceOrDefault(pointID, nil).PK, Alerts: []DeviceHistoryAlert{}}
	for _, d := range list {
		a := d.Alert
		if d.PK != "" {
			out.PK = d.PK
		}
		out.Alerts = append(out.Alerts, DeviceHistoryAlert{
			ID: a.ID, Code: a.Code, Event: string(a.EventType), State: string(a.State), StartedAt: local(a.StartedAt),
			EndedAt: localPtr(a.EndedAt), MaxLevel: string(a.MaxLevel), ReactivationCount: a.ReactivationCount,
			Classification: strOrNil(string(a.Management.Classification)), Label: strOrNil(string(a.Management.Label)),
			Levels: toLevels(d.Segments),
		})
	}
	return out
}

// OriginRequest son los campos de origen de toda escritura (RN-10).
type OriginRequest struct {
	Origin           string     `json:"origin"`
	DeviceIdentifier string     `json:"device_identifier"`
	ClientRecordID   string     `json:"client_record_id"`
	RecordedAt       *time.Time `json:"recorded_at"`
}

func (o OriginRequest) toOrigin() services.ActionOrigin {
	return services.ActionOrigin{Origin: entities.Origin(o.Origin), DeviceIdentifier: o.DeviceIdentifier,
		ClientRecordID: o.ClientRecordID, RecordedAt: o.RecordedAt}
}

// ClassificationRequest es POST /api/alerts/{id}/classification.
type ClassificationRequest struct {
	Classification string `json:"classification"`
	Observation    string `json:"observation"`
	OriginRequest
}

// LabelRequest es POST /api/alerts/{id}/label.
type LabelRequest struct {
	Label       string `json:"label"`
	OtherText   string `json:"other_text"`
	Observation string `json:"observation"`
	OriginRequest
}

// ObservationRequest es POST /api/alerts/{id}/observations.
type ObservationRequest struct {
	Text string `json:"text"`
	OriginRequest
}

// ManagementResponse es la respuesta de clasificar, etiquetar u observar.
type ManagementResponse struct {
	Code     string              `json:"code,omitempty"`
	Outcome  string              `json:"outcome"`
	Alert    AlertItem           `json:"alert"`
	Existing *ExistingManagement `json:"existing,omitempty"`
}

// ExistingManagement es la gestión que prevaleció en un conflicto (RN-18).
type ExistingManagement struct {
	Classification   *string `json:"classification,omitempty"`
	ClassifiedAt     *string `json:"classified_at,omitempty"`
	Label            *string `json:"label,omitempty"`
	LabelOtherText   *string `json:"label_other_text,omitempty"`
	LabeledAt        *string `json:"labeled_at,omitempty"`
	Origin           string  `json:"origin"`
	DeviceIdentifier *string `json:"device_identifier,omitempty"`
}

// PolicyPatchRequest son los campos que cambian (ausente = se copia de la vigente).
type PolicyPatchRequest struct {
	DowngradeHours                        *int     `json:"downgrade_hours"`
	RojoRequiresClassificationToDowngrade *bool    `json:"rojo_requires_classification_to_downgrade"`
	MergeWindowHours                      *int     `json:"merge_window_hours"`
	ReactivateOnlyIfUnclassified          *bool    `json:"reactivate_only_if_unclassified"`
	InterventionSilentMaxLevel            *string  `json:"intervention_silent_max_level"`
	SummaryEveryHours                     *int     `json:"summary_every_hours"`
	CountdownHours                        *int     `json:"countdown_hours"`
	TrendLookbackHours                    *int     `json:"trend_lookback_hours"`
	TrendDelta                            *float64 `json:"trend_delta"`
	OscillationFlips                      *int     `json:"oscillation_flips"`
	OscillationWindowHours                *int     `json:"oscillation_window_hours"`
	LateNotificationMaxHours              *int     `json:"late_notification_max_hours"`
}

func (p PolicyPatchRequest) toPatch() entities.PolicyPatch {
	patch := entities.PolicyPatch{
		DowngradeHours: p.DowngradeHours, RojoRequiresClassificationToDowngrade: p.RojoRequiresClassificationToDowngrade,
		MergeWindowHours: p.MergeWindowHours, ReactivateOnlyIfUnclassified: p.ReactivateOnlyIfUnclassified,
		SummaryEveryHours: p.SummaryEveryHours, CountdownHours: p.CountdownHours, TrendLookbackHours: p.TrendLookbackHours,
		TrendDelta: p.TrendDelta, OscillationFlips: p.OscillationFlips, OscillationWindowHours: p.OscillationWindowHours,
		LateNotificationMaxHours: p.LateNotificationMaxHours,
	}
	if p.InterventionSilentMaxLevel != nil {
		l := entities.Level(*p.InterventionSilentMaxLevel)
		patch.InterventionSilentMaxLevel = &l
	}
	return patch
}

// CreatePolicyRequest es POST /api/alerts/policies. created_by viaja en el
// cuerpo mientras no haya autenticación (igual que el Rule Engine).
type CreatePolicyRequest struct {
	MeasuringPointID *int64             `json:"measuring_point_id"`
	EventType        string             `json:"event_type"`
	Patch            PolicyPatchRequest `json:"patch"`
	ValidFrom        *time.Time         `json:"valid_from"`
	Reason           string             `json:"reason"`
	CreatedBy        string             `json:"created_by"`
}

// PolicyResponse es una versión de la política.
type PolicyResponse struct {
	ID                                    int64   `json:"id"`
	Version                               int     `json:"version"`
	MeasuringPointID                      *int64  `json:"measuring_point_id"`
	EventType                             string  `json:"event_type"`
	DowngradeHours                        int     `json:"downgrade_hours"`
	RojoRequiresClassificationToDowngrade bool    `json:"rojo_requires_classification_to_downgrade"`
	MergeWindowHours                      int     `json:"merge_window_hours"`
	ReactivateOnlyIfUnclassified          bool    `json:"reactivate_only_if_unclassified"`
	InterventionSilentMaxLevel            string  `json:"intervention_silent_max_level"`
	SummaryEveryHours                     int     `json:"summary_every_hours"`
	CountdownHours                        int     `json:"countdown_hours"`
	TrendLookbackHours                    int     `json:"trend_lookback_hours"`
	TrendDelta                            float64 `json:"trend_delta"`
	OscillationFlips                      int     `json:"oscillation_flips"`
	OscillationWindowHours                int     `json:"oscillation_window_hours"`
	LateNotificationMaxHours              int     `json:"late_notification_max_hours"`
	ValidFrom                             *string `json:"valid_from"`
	ValidTo                               *string `json:"valid_to"`
	Reason                                string  `json:"reason"`
	CreatedBy                             string  `json:"created_by"`
}

func toPolicy(p entities.Policy) PolicyResponse {
	return PolicyResponse{
		ID: p.ID, Version: p.Version, MeasuringPointID: p.MeasuringPointID, EventType: string(p.EventType),
		DowngradeHours: p.DowngradeHours, RojoRequiresClassificationToDowngrade: p.RojoRequiresClassificationToDowngrade,
		MergeWindowHours: p.MergeWindowHours, ReactivateOnlyIfUnclassified: p.ReactivateOnlyIfUnclassified,
		InterventionSilentMaxLevel: string(p.InterventionSilentMaxLevel), SummaryEveryHours: p.SummaryEveryHours,
		CountdownHours: p.CountdownHours, TrendLookbackHours: p.TrendLookbackHours, TrendDelta: p.TrendDelta,
		OscillationFlips: p.OscillationFlips, OscillationWindowHours: p.OscillationWindowHours,
		LateNotificationMaxHours: p.LateNotificationMaxHours,
		ValidFrom:                localPtr(&p.ValidFrom), ValidTo: localPtr(p.ValidTo), Reason: p.Reason, CreatedBy: p.CreatedBy,
	}
}
