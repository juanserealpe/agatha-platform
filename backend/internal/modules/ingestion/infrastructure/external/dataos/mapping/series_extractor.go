package mapping

import (
	"encoding/json"
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// SeriesExtractor se encarga de extraer las series de datos contenidas
// en la respuesta obtenida desde DataOS.
type SeriesExtractor struct {
	logger *logger.Logger
}

// NewSeriesExtractor crea una nueva instancia de SeriesExtractor.
func NewSeriesExtractor(logger *logger.Logger) *SeriesExtractor {
	return &SeriesExtractor{logger: logger}
}

// Extract procesa los componentes de una respuesta de DataOS y extrae
// las series de datos contenidas en ellos.
func (e *SeriesExtractor) Extract(response *dashboard.RenderResponse) ([]dashboard.Series, error) {
	var series []dashboard.Series
	e.logger.Info("INICIO extracción de series", "components", len(response.Components))

	for componentIndex, component := range response.Components {
		e.logger.Info("Procesando componente", "index", componentIndex, "component_id", component.ID)

		if component.Error != nil {
			e.logger.Warn("componente contiene error", "component_id", component.ID, "error", *component.Error)
			continue
		}

		if len(component.Data) == 0 {
			e.logger.Warn("componente tiene data vacía", "component_id", component.ID)
			continue
		}

		var chartData dashboard.ChartData
		if err := json.Unmarshal(component.Data, &chartData); err != nil {
			e.logger.Error("falló decode de component.Data", "component_id", component.ID, "error", err, "raw_data", string(component.Data))
			return nil, fmt.Errorf("decode component %q data: %w", component.ID, err)
		}

		for _, serie := range chartData.Series {
			series = append(series, serie)
		}
	}

	e.logger.Info("FIN extracción de series", "series_count", len(series))
	return series, nil
}
