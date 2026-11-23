package contract

import "time"

// Change es el cambio visible que originó un AlertChanged.
const (
	ChangeOpened        = "OPENED"
	ChangeLevelRaised   = "LEVEL_RAISED"
	ChangeLevelLowered  = "LEVEL_LOWERED"
	ChangeReactivated   = "REACTIVATED"
	ChangeFinished      = "FINISHED"
	ChangeNoDataStarted = "NO_DATA_STARTED"
	ChangeNoDataEnded   = "NO_DATA_ENDED"
	ChangeClassified    = "CLASSIFIED"
	ChangeLabeled       = "LABELED"
)

// LeakSignature es la firma de fuga de la última hora calculada de la alerta:
// DPl es el % de registros de las últimas 24 h con planitud alta (null si no
// se calculó), DPlMin el mínimo de la versión de umbrales y Confirmed indica
// que la hora no se limitó y DPl >= DPlMin.
type LeakSignature struct {
	DPl       *float64 `json:"d_pl"`
	DPlMin    float64  `json:"d_pl_min"`
	Confirmed bool     `json:"confirmed"`
}

// AlertChanged es la foto de una alerta después de un cambio visible (nivel,
// estado, horas sin datos o gestión). La consume el canal web para actualizar
// el mapa y el panel sin consultar la API.
//
// Para descartar fotos viejas: los campos de nivel y estado valen si
// EvaluatedAt es más nuevo que el conocido; los de gestión, si
// ManagementStatus avanzó (la gestión solo avanza).
type AlertChanged struct {
	AlertID          int64  `json:"alert_id"`
	AlertCode        string `json:"alert_code"`
	MeasuringPointID int64  `json:"measuring_point_id"`
	EventType        string `json:"event_type"`
	// Change es uno de los Change* de este paquete.
	Change       string `json:"change"`
	State        string `json:"state"`
	CurrentLevel string `json:"current_level"`
	MaxLevel     string `json:"max_level"`
	// Blinking: rojo sin clasificar.
	Blinking         bool   `json:"blinking"`
	ManagementStatus string `json:"management_status"`
	// Classification: CONFIRMADA o FALSA_ALERTA; vacío = sin clasificar.
	Classification string `json:"classification"`
	// LeakSignature es la de la última hora calculada; null si no hay d_pl_min
	// (la regla de planitud no está activa para el punto) o datos.
	LeakSignature *LeakSignature `json:"leak_signature"`
	StartedAt     time.Time      `json:"started_at"`
	// EndedAt es el fin de la alerta (UTC); null mientras está en curso (una
	// reactivación lo vuelve a null). Lo usa Baseline para excluir el intervalo.
	EndedAt *time.Time `json:"ended_at"`
	// EvaluatedAt es la hora (UTC) del procesamiento que produjo la foto; en
	// los cambios de gestión, la última hora procesada de la alerta.
	EvaluatedAt   time.Time `json:"evaluated_at"`
	OccurredAt    time.Time `json:"occurred_at"`
	SchemaVersion int       `json:"schema_version"`
}
