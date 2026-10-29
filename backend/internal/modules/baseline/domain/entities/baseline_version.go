package entities

import "time"

// BaselineVersion es una versión inmutable de la línea base de un punto de
// medición, vigente en [ValidFrom, ValidTo). ValidTo = nil ⇒ vigente.
type BaselineVersion struct {
	ID               int64
	MeasuringPointID int64
	Version          int
	ValidFrom        time.Time
	ValidTo          *time.Time
	HistoryFrom      time.Time
	HistoryTo        time.Time
	Signal           SignalBaseline
	Components       []ComponentStat
	Confidence       Confidence
	ExcludedPeriods  []ExcludedPeriod
	Estimation       EstimationSummary
	Flags            []Flag
	Trigger          Trigger
	AlgorithmVersion string
	CreatedAt        time.Time
}
