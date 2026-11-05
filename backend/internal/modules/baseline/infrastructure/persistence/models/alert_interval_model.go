package models

import "time"

// AlertIntervalRowModel es la vista propia (solo lectura) de alerts que usa
// Baseline para excluir eventos reales. La tabla la crea y escribe Alert
// Management: NO se migra acá.
type AlertIntervalRowModel struct {
	StartedAt time.Time `gorm:"column:started_at"`
	// EndedAt de una alerta ACTIVA es el fin del rango consultado.
	EndedAt time.Time `gorm:"column:ended_at"`
}
