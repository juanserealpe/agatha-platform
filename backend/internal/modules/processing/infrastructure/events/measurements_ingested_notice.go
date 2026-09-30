// Package events contiene los adaptadores de eventos entrantes de Processing.
//
// Processing NO importa modules/ingestion: el composition root traduce el
// evento de Ingestion a los tipos propios definidos acá.
package events

import "time"

// MeasurementsIngestedNotice es la vista de Processing de un lote ingerido.
// Es una pista: la fuente de verdad son las mediciones en la BD.
type MeasurementsIngestedNotice struct {
	// MeasuringPointID es el punto de medición del lote.
	MeasuringPointID int64
	// Source es el origen del lote (p. ej. "dataos").
	Source string
	// From y To delimitan (UTC) los timestamps insertados.
	From, To time.Time
	// Watermark es el last_ingested_at persistido por Ingestion (UTC).
	Watermark time.Time
	// BatchSeq es el cursor monótono del lote.
	BatchSeq int64
	// InsertedRecords es la cantidad de mediciones insertadas.
	InsertedRecords int
}
