package services

import "time"

// Config son los parámetros del cálculo de línea base (sección 4 del spec).
type Config struct {
	HistoryDays            int           // BASELINE_HISTORY_DAYS (90)
	MinSignalRecords       int           // BASELINE_MIN_SIGNAL_RECORDS (96)
	MinComponentWindows    int           // BASELINE_MIN_COMPONENT_WINDOWS (20)
	NormalComponentWindows int           // fijo: 60
	ExclusionICutoff       float64       // BASELINE_EXCLUSION_I_CUTOFF (0,18)
	ExclusionCapRatio      float64       // fijo: 0,40
	MaxPasses              int           // BASELINE_MAX_PASSES (5)
	RecalcInterval         time.Duration // BASELINE_RECALC_INTERVAL (168 h)
	LowRecalcInterval      time.Duration // BASELINE_LOW_RECALC_INTERVAL (24 h)
}

// DefaultConfig devuelve los valores cerrados con el analista.
func DefaultConfig() Config {
	return Config{
		HistoryDays:            90,
		MinSignalRecords:       96,
		MinComponentWindows:    20,
		NormalComponentWindows: 60,
		ExclusionICutoff:       0.18,
		ExclusionCapRatio:      0.40,
		MaxPasses:              5,
		RecalcInterval:         168 * time.Hour,
		LowRecalcInterval:      24 * time.Hour,
	}
}

// HistoryWindow es la longitud del histórico antes de asOf.
func (c Config) HistoryWindow() time.Duration {
	return time.Duration(c.HistoryDays) * 24 * time.Hour
}
