package entities

import (
	"fmt"
	"time"
)

// AlertCode es el ID visible de la alerta: ALR- + id con 6 dígitos (la
// columna alerts.code es GENERATED con la misma regla).
func AlertCode(id int64) string {
	return fmt.Sprintf("ALR-%06d", id)
}

// Management es la gestión del operador. Cada paso queda fijo al guardarse
// (la BD lo garantiza con un trigger).
type Management struct {
	Status ManagementStatus

	Classification           Classification // vacío = sin clasificar
	ClassifiedAt             *time.Time
	ClassifiedOrigin         Origin
	ClassifiedDevice         string
	ClassifiedRecordedAt     *time.Time
	ClassifiedClientRecordID string

	Label                 Label // vacío = sin etiqueta
	LabelOtherText        string
	LabeledAt             *time.Time
	LabeledOrigin         Origin
	LabeledDevice         string
	LabeledRecordedAt     *time.Time
	LabeledClientRecordID string
}

// Classified indica si la alerta ya fue clasificada.
func (m Management) Classified() bool {
	return m.Classification != ""
}

// HourValue es el I de una hora calculada (memoria de la tendencia).
type HourValue struct {
	At time.Time `json:"at"`
	I  float64   `json:"i"`
}

// Zonas de una hora respecto de la banda de cierre (reglas 4 y 5).
const (
	ZoneAbove = "ABOVE" // raw ≥ AMARILLO
	ZoneBand  = "BAND"  // raw NORMAL pero I ≥ salida (histéresis)
	ZoneBelow = "BELOW" // I bajo la salida (cuenta para cerrar)
)

// Tendencias (regla 8).
const (
	TrendUp     = "SUBIENDO"
	TrendDown   = "BAJANDO"
	TrendStable = "ESTABLE"
)

// Runtime es la memoria de las reglas de mensaje (alerts.runtime, JSONB): no
// hay estado en memoria del proceso.
type Runtime struct {
	PrevZone         string      `json:"prev_zone,omitempty"`
	Flips            []time.Time `json:"flips,omitempty"`
	LastFlipAt       *time.Time  `json:"last_flip_at,omitempty"`
	Oscillating      bool        `json:"oscillating,omitempty"`
	CountdownSentFor Level       `json:"countdown_sent_for,omitempty"`
	RojoHeldSent     bool        `json:"rojo_held_sent,omitempty"`
	Recent           []HourValue `json:"recent,omitempty"`
	LastTrend        string      `json:"last_trend,omitempty"`
	LastMessageAt    *time.Time  `json:"last_message_at,omitempty"`
	InNoCalc         bool        `json:"in_no_calc,omitempty"`
	// PausedHours son las horas entre cada fin y su reactivación: no suman a
	// la duración (se conservan entre reactivaciones).
	PausedHours int `json:"paused_hours,omitempty"`
}

// Alert es una alerta de un dispositivo y un evento.
type Alert struct {
	ID               int64
	Code             string
	MeasuringPointID int64
	EventType        EventType
	State            AlertState

	// StartedAt es la primera hora sobre el umbral de la racha que abrió el
	// episodio; OpenedAt, la hora en que se creó; EndedAt, el CLEARED (nil = en curso).
	StartedAt         time.Time
	OpenedAt          time.Time
	EndedAt           *time.Time
	ReactivationCount int
	LastReactivatedAt *time.Time

	CurrentLevel Level
	MaxLevel     Level

	OpeningIndicatorValue float64
	LastIndicatorValue    float64
	// MaxIndicatorValue es el índice más alto de las horas calculadas de la
	// alerta (sus reactivaciones incluidas): lo citan los mensajes.
	MaxIndicatorValue float64
	LastEvaluatedAt   time.Time

	// HoursCalculated: horas calculadas de la alerta (sin las pausas entre fin
	// y reactivación); HoursNoData: horas sin cálculo con la alerta activa;
	// BelowLevelHours: horas calculadas seguidas por debajo del nivel actual.
	HoursCalculated int
	HoursNoData     int
	BelowLevelHours int

	LowConfidenceAtOpen bool
	Runtime             Runtime

	PolicyVersionID          int64
	ThresholdVersionIDAtOpen int64
	OpenedByRuleEvaluationID int64

	Management Management

	Source    Source
	CreatedAt time.Time
}

// DurationHours es el tiempo transcurrido de la alerta, en horas, desde
// StartedAt hasta until (su fin, o la última hora procesada si sigue activa),
// sin las pausas entre fin y reactivación. Es lo que suman sus colores. Las
// horas sin datos cuentan: la alerta siguió activa. No es HoursCalculated
// (horas evaluadas, que cuenta también la hora de inicio).
func (a Alert) DurationHours(until time.Time) int {
	return max(int(until.Sub(a.StartedAt).Hours())-a.Runtime.PausedHours, 0)
}

// Blinking: rojo sin clasificar (CA-1.2.3). Clasificar solo quita el titileo.
func (a Alert) Blinking() bool {
	return a.State == AlertActive && a.CurrentLevel == LevelRojo && !a.Management.Classified()
}

// InIntervention: la etiqueta registra una intervención programada (las
// subidas hasta intervention_silent_max_level no notifican).
func (a Alert) InIntervention() bool {
	return a.Management.Label == LabelScheduledWork
}

// Segment es un color (tramo) de la alerta: intervalo continuo con un mismo nivel.
type Segment struct {
	ID        int64
	AlertID   int64
	Seq       int
	OpenedBy  SegmentCause
	Level     Level
	StartedAt time.Time
	EndedAt   *time.Time // nil = color vigente
}

// Message es una entrada de la línea de tiempo dentro de un color.
type Message struct {
	ID          int64
	AlertID     int64
	SegmentID   int64
	EvaluatedAt time.Time
	Kind        MessageKind
	Code        string
	Level       Level
	// IndicatorValue es el I de la hora (nil en gestión y horas sin cálculo).
	IndicatorValue  *float64
	Title           string
	Body            string
	Params          map[string]string
	CatalogVersion  int
	PolicyVersionID int64
	CreatedAt       time.Time
}

// Observation es un texto del operador (solo se agrega) o el registro de un
// conflicto de sincronización.
type Observation struct {
	ID               int64
	AlertID          int64
	Text             string
	Kind             ObservationKind
	Step             ObservationStep
	Origin           Origin
	DeviceIdentifier string
	ClientRecordID   string
	RecordedAt       time.Time
	ReceivedAt       time.Time
}

// ProcessingState es el estado del procesamiento horario por (punto, evento).
type ProcessingState struct {
	MeasuringPointID int64
	EventType        EventType
	LastEvaluatedAt  *time.Time
	// PendingRunStartedAt es la primera hora sobre el umbral de la racha en
	// curso sin alerta activa (será la fecha inicio si el episodio abre).
	PendingRunStartedAt *time.Time
	// LastAlertID es la última alerta finalizada (candidata a reactivación).
	LastAlertID *int64
	LastStatus  string
}

// Device son los datos del dispositivo que usan los mensajes.
type Device struct {
	MeasuringPointID int64
	// PK es el nombre con el que se muestra el punto (hoy, devices.name).
	PK string
}

// DeviceOrDefault completa un dispositivo sin datos con MP-<id>.
func DeviceOrDefault(pointID int64, d *Device) Device {
	if d != nil && d.PK != "" {
		return *d
	}
	return Device{MeasuringPointID: pointID, PK: fmt.Sprintf("MP-%d", pointID)}
}
