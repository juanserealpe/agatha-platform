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
	// InsertedRecords es la cantidad de mediciones insertadas (solo logs).
	InsertedRecords int
	// Watermark es el last_ingested_at del lote. Cero ⇒ el evento no lo
	// informa (contrato actual de Ingestion): Processing lo deriva de la BD.
	Watermark time.Time
	// BatchSeq es el cursor del lote (solo trazabilidad). nil si no se informa.
	BatchSeq *int64
	// From es el menor timestamp del lote (detección de datos tardíos). nil si no se informa.
	From *time.Time
}
