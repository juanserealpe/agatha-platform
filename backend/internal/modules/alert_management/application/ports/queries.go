package ports

import (
	"context"
	"io"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// SearchFilter son los filtros del historial (HU-9.1): Y entre filtros, O dentro de uno.
type SearchFilter struct {
	PointID int64 // 0 = todos
	// From / To acotan started_at a [From, To) (cero = abierto).
	From, To  time.Time
	MaxLevels []entities.Level
	Statuses  []entities.ManagementStatus
	Events    []entities.EventType
}

// AlertSummary es una fila del historial.
type AlertSummary struct {
	Alert entities.Alert
	PK    string
}

// SegmentWithMessages es un color con sus mensajes, en orden.
type SegmentWithMessages struct {
	Segment  entities.Segment
	Messages []entities.Message
}

// AlertDetail es la alerta con sus colores, mensajes y observaciones.
type AlertDetail struct {
	AlertSummary
	Segments     []SegmentWithMessages
	Observations []entities.Observation
}

// AlertQueries es el lado de lectura (consultas de la API).
type AlertQueries interface {
	// Search devuelve una página del historial (inicio descendente) y el total.
	Search(ctx context.Context, f SearchFilter, offset, limit int) ([]AlertSummary, int64, error)
	// SearchDetails devuelve, con colores, mensajes y observaciones, hasta
	// limit alertas del filtro en el mismo orden que Search (exportación).
	SearchDetails(ctx context.Context, f SearchFilter, limit int) ([]AlertDetail, error)
	Count(ctx context.Context, f SearchFilter) (int64, error)
	// Pending devuelve las Sin clasificar más recientes y su total.
	Pending(ctx context.Context, limit int) ([]AlertSummary, int64, error)
	// Active devuelve las alertas activas.
	Active(ctx context.Context) ([]AlertSummary, error)
	// Detail devuelve la alerta completa (ErrNotFound si no existe).
	Detail(ctx context.Context, id int64) (*AlertDetail, error)
	// DeviceHistory devuelve las alertas del punto que se solapan con [from, to), recientes primero.
	DeviceHistory(ctx context.Context, pointID int64, from, to time.Time) ([]AlertDetail, error)
}

// Sheet es una hoja de cálculo: la primera fila son los encabezados.
type Sheet struct {
	Name string
	Rows [][]string
}

// SpreadsheetWriter escribe un libro .xlsx.
type SpreadsheetWriter interface {
	Write(w io.Writer, sheets []Sheet) error
}
