package entities

import "time"

// FeatureEvaluation es la evaluación de UNA ventana (6 h o 24 h) de un punto
// que termina en EvaluatedAt. Una por (punto, ventana, hora). Inmutable una
// vez creada: un registro tardío nunca la modifica.
//
// Los punteros nil son valores no calculados (NULL en la BD).
type FeatureEvaluation struct {
	MeasuringPointID int64
	WindowHours      int       // Window6h | Window24h
	EvaluatedAt      time.Time // t: fin exclusivo de la ventana, hora en punto UTC
	WindowStart      time.Time // t − WindowHours

	Status       EvaluationStatus
	NoCalcReason NoCalcReason // ReasonNone si Status = CALCULATED

	NExpected int     // 96 (24 h) | 24 (6 h)
	NReceived int     // samples tras normalizar
	NValid    int     // samples con B2 válido
	Coverage  float64 // % con tope 100

	D, PHours, Ex, C, DPl *float64
	CFrom24h              bool // true en 6 h: C y su estadística vienen de la ventana de 24 h

	ZD, ZP, ZEx, ZC *float64
	SD, SP, SEx, SC *float64
	I               *float64

	LowConfidence bool
	Silent        bool
	Flags         []string

	BaselineVersionID *int64
	BaselineVersion   *int
	// BaselineValidFrom no se persiste: solo lo informa el CSV del replay.
	BaselineValidFrom *time.Time

	AlgorithmVersion string
	ConfigVersion    string
	Trigger          Trigger
	SourceBatchSeq   *int64
	InputMaxTs       *time.Time // último timestamp considerado en la ventana
	CreatedAt        time.Time
}
