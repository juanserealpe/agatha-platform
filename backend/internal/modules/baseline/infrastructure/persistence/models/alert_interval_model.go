package models

import "time"

// AlertIntervalRowModel es la vista propia (solo lectura) de alerts que usa
// Baseline para excluir eventos reales. La tabla la crea y escribe Alert
// Management: NO se migra acá.
type AlertIntervalRowModel struct {
	Code      string    `gorm:"column:code"`
	StartedAt time.Time `gorm:"column:started_at"`
	EndedAt   time.Time `gorm:"column:ended_at"`
	// Tag es la etiqueta o, sin etiqueta, la clasificación.
	Tag string `gorm:"column:tag"`
}
