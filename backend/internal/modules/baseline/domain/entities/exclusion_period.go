package entities

import "time"

// ExclusionSource indica de dónde viene un período de exclusión.
type ExclusionSource string

const (
	// ExclusionSourceAlert: copiado de una alerta de Alert Management (en vivo
	// por alert_changed o con el seed). Se identifica por SourceAlertCode.
	ExclusionSourceAlert ExclusionSource = "ALERT"
	// ExclusionSourceManual: cargado por el analista (mantenimiento, prueba…).
	ExclusionSourceManual ExclusionSource = "MANUAL"
)

// ExclusionPeriod es un intervalo [StartedAt, EndedAt) en el que el tubo no
// estuvo en su condición normal de referencia y que la línea base excluye
// siempre (con el margen Config.KnownEventMargin). EndedAt = nil ⇒ en curso.
//
// Es un dato de ENTRADA de Baseline, no un derivado: ningún -rebuild lo
// borra, así el histórico se reconstruye siempre con las mismas exclusiones.
type ExclusionPeriod struct {
	ID               int64
	MeasuringPointID int64
	StartedAt        time.Time
	EndedAt          *time.Time
	Source           ExclusionSource
	// SourceAlertCode es el código de la alerta (ALR-000023), sin FK: un
	// rebuild de Alert Management cambia los ids pero el período se queda.
	SourceAlertCode string
	Note            string
}

// KnownEvent devuelve el período como evento conocido; uno en curso se
// extiende hasta until.
func (p ExclusionPeriod) KnownEvent(until time.Time) KnownEvent {
	end := until
	if p.EndedAt != nil {
		end = *p.EndedAt
	}
	return KnownEvent{From: p.StartedAt.UTC(), To: end.UTC()}
}
