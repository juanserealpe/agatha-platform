// Package entities define el modelo de Alert Management: alertas con sus
// colores (tramos) y mensajes, la gestión del operador (clasificación,
// etiqueta, observaciones), la política versionada y la vista propia del
// nivel horario que publica el Rule Engine.
package entities

// EventType es el tipo de evento de la alerta. Hoy solo FUGA; MOVIMIENTO
// queda listo para cuando exista su indicador.
type EventType string

const (
	EventTypeLeak     EventType = "FUGA"
	EventTypeMovement EventType = "MOVIMIENTO"
)

// Valid indica si el tipo de evento existe.
func (e EventType) Valid() bool {
	return e == EventTypeLeak || e == EventTypeMovement
}

// Level es el nivel de una hora o de una alerta (NORMAL < AMARILLO < NARANJA < ROJO).
// Una alerta nunca está en NORMAL.
type Level string

const (
	LevelNormal   Level = "NORMAL"
	LevelAmarillo Level = "AMARILLO"
	LevelNaranja  Level = "NARANJA"
	LevelRojo     Level = "ROJO"
)

// Rank ordena los niveles; un nivel desconocido es 0 (como NORMAL).
func (l Level) Rank() int {
	switch l {
	case LevelAmarillo:
		return 1
	case LevelNaranja:
		return 2
	case LevelRojo:
		return 3
	default:
		return 0
	}
}

// AlertLevel indica si el nivel puede ser el de una alerta.
func (l Level) AlertLevel() bool {
	return l.Rank() > 0
}

// Below es el nivel inmediatamente inferior (AMARILLO no tiene: devuelve AMARILLO).
func (l Level) Below() Level {
	switch l {
	case LevelRojo:
		return LevelNaranja
	default:
		return LevelAmarillo
	}
}

// Above es el nivel inmediatamente superior ("" para ROJO).
func (l Level) Above() Level {
	switch l {
	case LevelAmarillo:
		return LevelNaranja
	case LevelNaranja:
		return LevelRojo
	default:
		return ""
	}
}

// MaxLevel devuelve el mayor de dos niveles.
func MaxLevel(a, b Level) Level {
	if b.Rank() > a.Rank() {
		return b
	}
	return a
}

// AlertState es el estado del ciclo de la alerta. Puede volver de FINALIZADA a
// ACTIVA por reactivación.
type AlertState string

const (
	AlertActive   AlertState = "ACTIVA"
	AlertFinished AlertState = "FINALIZADA"
)

// ManagementStatus es el estado de gestión. Solo avanza.
type ManagementStatus string

const (
	StatusUnclassified       ManagementStatus = "SIN_CLASIFICAR"
	StatusClassifiedNoLabel  ManagementStatus = "CLASIFICADA_SIN_ETIQUETA"
	StatusClassifiedAndLabel ManagementStatus = "CLASIFICADA_CON_ETIQUETA"
)

// Valid indica si el estado de gestión existe.
func (s ManagementStatus) Valid() bool {
	return s == StatusUnclassified || s == StatusClassifiedNoLabel || s == StatusClassifiedAndLabel
}

// Classification es la conclusión del operador (una sola vez, inmutable).
type Classification string

const (
	ClassificationConfirmed  Classification = "CONFIRMADA"
	ClassificationFalseAlarm Classification = "FALSA_ALERTA"
)

// Valid indica si la clasificación existe.
func (c Classification) Valid() bool {
	return c == ClassificationConfirmed || c == ClassificationFalseAlarm
}

// Label es la causa elegida de la lista de la clasificación (RN-07).
type Label string

const (
	LabelLeakConfirmed      Label = "FUGA_CONFIRMADA_CAMPO"
	LabelGroundMovement     Label = "MOVIMIENTO_TIERRA"
	LabelThirdPartyMachines Label = "MAQUINARIA_AJENA"
	LabelExtremeWeather     Label = "CLIMA_EXTREMO"
	LabelHeavyTraffic       Label = "TRANSITO_PESADO"
	LabelScheduledWork      Label = "MANTENIMIENTO_PROGRAMADO"
	// LabelOther exige un texto libre de 1 a 100 caracteres.
	LabelOther Label = "OTRO"
)

// labelsByClassification son las listas validadas por el cliente (D-06).
var labelsByClassification = map[Classification][]Label{
	ClassificationConfirmed:  {LabelLeakConfirmed, LabelGroundMovement, LabelThirdPartyMachines, LabelOther},
	ClassificationFalseAlarm: {LabelExtremeWeather, LabelHeavyTraffic, LabelScheduledWork, LabelOther},
}

// LabelsFor devuelve la lista de etiquetas de la clasificación.
func LabelsFor(c Classification) []Label {
	return append([]Label(nil), labelsByClassification[c]...)
}

// AllowedFor indica si la etiqueta pertenece a la lista de la clasificación.
func (l Label) AllowedFor(c Classification) bool {
	for _, allowed := range labelsByClassification[c] {
		if allowed == l {
			return true
		}
	}
	return false
}

// labelNames son los textos de campo de cada etiqueta.
var labelNames = map[Label]string{
	LabelLeakConfirmed:      "Fuga confirmada en campo",
	LabelGroundMovement:     "Movimiento de tierra",
	LabelThirdPartyMachines: "Intervención de maquinaria ajena",
	LabelExtremeWeather:     "Condiciones climáticas extremas",
	LabelHeavyTraffic:       "Tránsito de vehículo pesado",
	LabelScheduledWork:      "Mantenimiento o intervención programada",
	LabelOther:              "Otro",
}

// DisplayName es el texto de la etiqueta para el operador.
func (l Label) DisplayName() string {
	return labelNames[l]
}

// Origin es de dónde llegó una acción del operador (reemplaza al usuario
// mientras no haya inicio de sesión, RN-10).
type Origin string

const (
	OriginWeb    Origin = "WEB"
	OriginMobile Origin = "APP_MOVIL"
)

// Valid indica si el origen existe.
func (o Origin) Valid() bool {
	return o == OriginWeb || o == OriginMobile
}

// DisplayName es el texto del origen para los mensajes.
func (o Origin) DisplayName() string {
	if o == OriginMobile {
		return "la app móvil"
	}
	return "la web"
}

// Source es cómo llegó la hora que creó la alerta.
type Source string

const (
	SourceLive     Source = "LIVE"
	SourceCatchUp  Source = "CATCH_UP"
	SourceBackfill Source = "BACKFILL"
)

// SegmentCause es por qué empezó un color de la alerta.
type SegmentCause string

const (
	SegmentOpen         SegmentCause = "OPEN"
	SegmentUp           SegmentCause = "UP"
	SegmentDown         SegmentCause = "DOWN"
	SegmentReactivation SegmentCause = "REACTIVATION"
)

// MessageKind es el tipo de mensaje de la línea de tiempo.
type MessageKind string

const (
	KindNotification MessageKind = "NOTIFICACION"
	KindUpdate       MessageKind = "ACTUALIZACION"
	KindSummary      MessageKind = "RESUMEN"
	KindEnd          MessageKind = "FIN"
	KindManagement   MessageKind = "GESTION"
)

// ObservationKind es el tipo de observación.
type ObservationKind string

const (
	ObservationFree                   ObservationKind = "OBSERVACION"
	ObservationClassificationConflict ObservationKind = "CONFLICTO_CLASIFICACION"
	ObservationLabelConflict          ObservationKind = "CONFLICTO_ETIQUETA"
)

// ObservationStep es en qué paso de la gestión se escribió la observación.
type ObservationStep string

const (
	StepClassification ObservationStep = "CLASIFICACION"
	StepLabel          ObservationStep = "ETIQUETA"
	StepFree           ObservationStep = "LIBRE"
)

// Action es lo que el operador puede hacer con una alerta (available_actions).
type Action string

const (
	ActionConfirm    Action = "CONFIRM"
	ActionFalseAlarm Action = "FALSE_ALERT"
	ActionAddLabel   Action = "ADD_LABEL"
	ActionView       Action = "VIEW"
)

// ActionsFor devuelve las acciones disponibles según el estado de gestión.
func ActionsFor(s ManagementStatus) []Action {
	switch s {
	case StatusUnclassified:
		return []Action{ActionConfirm, ActionFalseAlarm, ActionView}
	case StatusClassifiedNoLabel:
		return []Action{ActionAddLabel, ActionView}
	default:
		return []Action{ActionView}
	}
}

// Valores de estado de las horas (los mismos del Rule Engine).
const (
	StatusCalculated   = "CALCULATED"
	StatusNoCalculable = "NO_CALCULABLE"
)

// EpisodeEvent: qué pasó con el episodio del Rule Engine en la hora.
type EpisodeEvent string

const (
	EpisodeOpened  EpisodeEvent = "OPENED"
	EpisodeCleared EpisodeEvent = "CLEARED"
	EpisodeNone    EpisodeEvent = "NONE"
)
