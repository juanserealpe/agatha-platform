// Package knownevents implementa repositories.KnownEventsProvider con una
// lista estática en código (eventos del estudio del analista, anteriores a las
// alertas) y con CompositeProvider, que la une a las alertas.
package knownevents

import (
	"context"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// eventsTimezone es la zona en la que están escritas las fechas de la lista:
// las del estudio del analista son hora local de Colombia.
const eventsTimezone = "America/Bogota"

const eventLayout = "2006-01-02 15:04"

type staticEvent struct {
	label    string
	from, to string // eventLayout, en eventsTimezone
}

// staticEvents son los eventos conocidos por punto de medición. Fuente: tabla
// PERIODOS del notebook del analista (AGATHA_MVP_baseline_procesamiento),
// SOLO los eventos: las referencias R1–R7 y los datos nuevos son
// comportamiento normal y deben entrar a la línea base.
var staticEvents = map[int64][]staticEvent{
	11556: {
		{"22–28 jul (ambiguo)", "2026-07-23 06:00", "2026-07-28 06:00"},
		{"E1 extendido (24 jul–2 ago)", "2026-07-24 00:00", "2026-08-03 00:00"},
		{"E1 (28 jul–1 ago)", "2026-07-28 00:00", "2026-08-02 00:00"},
		{"E2 (20–22 ago)", "2026-08-20 00:00", "2026-08-23 00:00"},
		{"E3 pre-explosión", "2026-08-23 00:00", "2026-08-25 06:00"},
		{"E4 intervención", "2026-08-25 06:00", "2026-08-27 12:00"},
		{"E5 post-reparación", "2026-08-27 12:00", "2026-08-30 00:00"},
	},
}

// StaticProvider entrega los eventos de staticEvents.
type StaticProvider struct {
	events map[int64][]entities.KnownEvent
}

// NewStaticProvider convierte la lista a UTC. Una fecha mal escrita es un
// error de programación: entra en pánico al arrancar, no al calcular.
func NewStaticProvider() *StaticProvider {
	location, err := time.LoadLocation(eventsTimezone)
	if err != nil {
		// Sin base de zonas horarias: Colombia no tiene horario de verano.
		location = time.FixedZone(eventsTimezone, -5*60*60)
	}

	events := make(map[int64][]entities.KnownEvent, len(staticEvents))
	for pointID, list := range staticEvents {
		parsed := make([]entities.KnownEvent, 0, len(list))
		for _, e := range list {
			from, to := mustParse(e.from, location), mustParse(e.to, location)
			if !from.Before(to) {
				panic("knownevents: evento con rango vacío: " + e.label)
			}
			parsed = append(parsed, entities.KnownEvent{From: from, To: to, Label: e.label})
		}
		sort.SliceStable(parsed, func(a, b int) bool { return parsed[a].From.Before(parsed[b].From) })
		events[pointID] = parsed
	}

	return &StaticProvider{events: events}
}

func mustParse(value string, location *time.Location) time.Time {
	t, err := time.ParseInLocation(eventLayout, value, location)
	if err != nil {
		panic("knownevents: fecha inválida: " + value)
	}
	return t.UTC()
}

// GetKnownEvents devuelve los eventos del punto que se solapan con [from, to).
func (p *StaticProvider) GetKnownEvents(_ context.Context, pointID int64, from, to time.Time,
) ([]entities.KnownEvent, error) {
	out := []entities.KnownEvent{}
	for _, e := range p.events[pointID] {
		if e.From.Before(to) && from.Before(e.To) {
			out = append(out, e)
		}
	}
	return out, nil
}
