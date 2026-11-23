package entities

import "time"

// TimeRange es un intervalo (Start, End]: Start es el último dato conocido (o
// el borde de la ventana) y End el siguiente. IngestDataOs lo pide con Start
// exclusivo.
type TimeRange struct {
	Start time.Time
	End   time.Time
}
