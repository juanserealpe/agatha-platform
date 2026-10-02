package entities

// MetricDefinition es la vista de Processing de una variable del catálogo
// (tabla metric_definitions). Solo lleva los campos que Processing usa.
type MetricDefinition struct {
	ID   uint64
	Key  string
	Name string
	Unit string
}
