// Package services contiene la lógica de negocio de Processing, sin IO:
// qué horas evaluar (SlotPlanner), qué samples entran en cada ventana
// (SliceWindow) y cómo se evalúa una hora (WindowEvaluator).
package services

import "time"

// SlotPlanner decide qué horas (en punto, UTC) quedan pendientes de evaluar.
type SlotPlanner struct {
	initialLookbackHours int
}

// NewSlotPlanner crea el planificador. initialLookbackHours es la historia que
// se evalúa la primera vez que se ve un punto (para que el Rule Engine tenga
// filas desde el primer día).
func NewSlotPlanner(initialLookbackHours int) SlotPlanner {
	if initialLookbackHours < 1 {
		initialLookbackHours = 1
	}
	return SlotPlanner{initialLookbackHours: initialLookbackHours}
}

// Plan devuelve, en orden, las horas en (last, readyUntil] (o, sin last, las
// initialLookbackHours que terminan en readyUntil), hasta maxSlots
// (maxSlots ≤ 0 ⇒ sin tope). readyUntil se trunca a la hora: el llamador ya
// garantiza que no supera floorHour(now).
func (p SlotPlanner) Plan(last *time.Time, readyUntil time.Time, maxSlots int) []time.Time {
	end := readyUntil.UTC().Truncate(time.Hour)

	start := end.Add(-time.Duration(p.initialLookbackHours-1) * time.Hour)
	if last != nil {
		start = last.UTC().Truncate(time.Hour).Add(time.Hour)
	}

	if start.After(end) {
		return []time.Time{}
	}

	n := int(end.Sub(start)/time.Hour) + 1
	if maxSlots > 0 && n > maxSlots {
		n = maxSlots
	}

	slots := make([]time.Time, n)
	for i := range slots {
		slots[i] = start.Add(time.Duration(i) * time.Hour)
	}

	return slots
}
