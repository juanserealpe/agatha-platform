package ports

import "time"

// Clock es el reloj de Processing. En producción es time.Now; en el replay es
// un reloj virtual fijo en el fin del rango.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapta una función a Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }
