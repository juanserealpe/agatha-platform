package entities

import "time"

// SaveStateResult informa el resultado de persistir un estado de ingesta.
type SaveStateResult struct {
	// BatchSeq es el cursor monótono del lote: el id de la fila insertada en ingestion_logs.
	BatchSeq int64
	// LastIngestedAt es el last_ingested_at persistido (cero si el estado no lo tenía).
	LastIngestedAt time.Time
}
