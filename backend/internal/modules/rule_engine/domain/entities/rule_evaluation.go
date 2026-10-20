package entities

import "time"

// RuleEvaluation es una fila de rule_evaluations: la interpretación de UNA
// hora calculada de la ventana de 24 h. Inmutable. Las horas NO_CALCULABLE no
// tienen fila: las rachas se cuentan sobre filas (horas calculadas).
type RuleEvaluation struct {
	ID               int64
	MeasuringPointID int64
	EventType        EventType
	EvaluatedAt      time.Time // fin exclusivo de la ventana (hora en punto UTC)
	IndicatorValue   float64

	// RawLevel es el nivel que marca el umbral, sin persistencia ni histéresis.
	// Las rachas de las horas siguientes se cuentan sobre este valor guardado.
	RawLevel Level
	// ComputedLevel es max(raw, persistencia): el nivel que sostiene la
	// evidencia en ESTA hora, sin histéresis ni racha de entrada.
	ComputedLevel Level
	// Level es el nivel del episodio (persistencia, histéresis, racha de entrada).
	Level     Level
	Rule      Rule
	InEpisode bool
	// HoursAbove son las horas calculadas del episodio, contadas desde la primera
	// hora de su racha de entrada e incluida la actual (0 fuera de episodio).
	HoursAbove int
	// HoursInLevel son las horas calculadas consecutivas con el mismo Level.
	HoursInLevel int

	ThresholdVersionID int64
	LowConfidence      bool
	EpisodeEvent       EpisodeEvent
	// Reasons explican ComputedLevel (vacío si es NORMAL).
	Reasons   []Reason
	CreatedAt time.Time
}

// Reason es un motivo del ComputedLevel de una hora. Los campos que no aplican
// al código quedan en cero:
//   - LEAK_THRESHOLD: Level = raw, I y Threshold (corte de ese nivel).
//   - LEAK_PERSISTENCE: Level = nivel ganado, Hours (racha) y From (nivel de la racha).
type Reason struct {
	Code      ReasonCode
	Level     Level
	I         float64
	Threshold float64
	Hours     int
	From      Level
}

// ThresholdSegment es un tramo de la línea de tiempo de umbrales recortado a
// un rango de consulta: [From, To) con la versión que aplica en él.
type ThresholdSegment struct {
	From    time.Time
	To      time.Time
	Version ThresholdVersion
}
