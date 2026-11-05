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

	// RunAmarillo y RunNaranja son las horas sobre cada corte con las que se
	// evaluó la persistencia: racha seguida (STRICT) o conteo en la ventana (WINDOW).
	RunAmarillo int
	RunNaranja  int
	// BelowRun son las horas calculadas seguidas bajo la salida de AMARILLO,
	// incluida esta (0 si esta no está bajo la salida).
	BelowRun int
	// DPl es el % de registros con planitud alta de la hora (nil si Processing no lo calculó).
	DPl *float64

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
//   - LEAK_PERSISTENCE: Level = nivel ganado, Hours (racha o conteo), From
//     (nivel de la racha) y WindowHours (solo WINDOW).
//   - NARANJA_NOT_CONFIRMED: Level = AMARILLO (el asignado), From (nivel que
//     no se confirmó), DPl (nil si no se calculó) y DPlMin.
type Reason struct {
	Code        ReasonCode
	Level       Level
	I           float64
	Threshold   float64
	Hours       int
	From        Level
	WindowHours int
	DPl         *float64
	DPlMin      float64
}

// ThresholdSegment es un tramo de la línea de tiempo de umbrales recortado a
// un rango de consulta: [From, To) con la versión que aplica en él.
type ThresholdSegment struct {
	From    time.Time
	To      time.Time
	Version ThresholdVersion
}
