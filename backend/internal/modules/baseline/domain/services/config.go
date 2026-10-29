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
	// KnownEventMargin se excluye antes del inicio y después del fin de cada
	// evento conocido (alerta), para que la línea base no vea la transición.
	KnownEventMargin time.Duration // BASELINE_KNOWN_EVENT_MARGIN (2 h)
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
		KnownEventMargin:       2 * time.Hour,
	}
}

// KnownEventsRange amplía [from, to) con el margen: un evento que empieza
// hasta KnownEventMargin después de to (o termina hasta ese margen antes de
// from) también recorta el histórico.
func (c Config) KnownEventsRange(from, to time.Time) (time.Time, time.Time) {
	return from.Add(-c.KnownEventMargin), to.Add(c.KnownEventMargin)
}

// HistoryWindow es la longitud del histórico antes de asOf.
func (c Config) HistoryWindow() time.Duration {
	return time.Duration(c.HistoryDays) * 24 * time.Hour
}
