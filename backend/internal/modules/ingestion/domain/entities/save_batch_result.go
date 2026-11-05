package entities

// SaveBatchResult informa el resultado de persistir un lote de mediciones.
type SaveBatchResult struct {
	// Inserted es la cantidad de filas realmente insertadas.
	Inserted int
}
