package entities

// seriesCatalog es la lista de variables graficables. PARA AGREGAR UNA
// VARIABLE basta con sumar una línea acá: el endpoint, el servicio y los
// lectores no cambian.
//
//   - De la telemetría cruda: Source = SourceMeasurement y el MetricID del
//     catálogo de métricas (ARQUITECTURA_LITE.md, sección 2.1).
//   - De Processing: Source = SourceEvaluation, la columna de
//     feature_evaluations y la ventana (6 | 24). La columna debe estar en la
//     lista permitida del lector (evaluation_series_reader.go).
//
// El orden de esta lista es el orden de la respuesta.
var seriesCatalog = []SeriesDefinition{
	{Key: "b1", Name: "B1 fuga 500-1200Hz", Unit: "-", Source: SourceMeasurement, MetricID: 12},
	{Key: "b2", Name: "B2 fuga 1200-2000Hz", Unit: "-", Source: SourceMeasurement, MetricID: 13},
	{Key: "densidad", Name: "Densidad (D, 24 h)", Unit: "%", Source: SourceEvaluation, Column: "d", WindowHours: 24},
	{Key: "indicador", Name: "Indicador I (24 h)", Unit: "-", Source: SourceEvaluation, Column: "i", WindowHours: 24},
}

// SeriesCatalog devuelve una copia del catálogo, en su orden.
func SeriesCatalog() []SeriesDefinition {
	return append([]SeriesDefinition{}, seriesCatalog...)
}

// ResolveSeries devuelve, en el orden del catálogo, las definiciones de las
// claves pedidas (sin repetir). Sin claves ⇒ todo el catálogo. Si alguna clave
// no existe devuelve ok = false y la clave desconocida.
func ResolveSeries(keys []string) (definitions []SeriesDefinition, unknown string, ok bool) {
	if len(keys) == 0 {
		return SeriesCatalog(), "", true
	}

	requested := make(map[string]bool, len(keys))
	for _, key := range keys {
		requested[key] = true
	}

	for _, definition := range seriesCatalog {
		if requested[definition.Key] {
			definitions = append(definitions, definition)
			delete(requested, definition.Key)
		}
	}

	for key := range requested {
		return nil, key, false
	}

	return definitions, "", true
}
