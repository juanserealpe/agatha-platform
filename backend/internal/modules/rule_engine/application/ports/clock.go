// Package ports define lo que el Rule Engine necesita del exterior y no es una
// tabla propia: el reloj, la publicación de eventos y la lectura de
// feature_evaluations (de Processing) para el backfill. Los adaptadores se
// cablean en internal/bootstrap.
package ports

import "time"

// Clock es el reloj del Rule Engine (time.Now en producción).
type Clock interface {
	Now() time.Time
}

// ClockFunc adapta una función a Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }
