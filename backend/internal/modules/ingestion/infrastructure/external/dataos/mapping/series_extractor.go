package mapping

import (
	"encoding/json"
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// SeriesExtractor extrae las series de datos de la respuesta de DataOS.
type SeriesExtractor struct {
	logger *logger.Logger
}

func NewSeriesExtractor(logger *logger.Logger) *SeriesExtractor {
	return &SeriesExtractor{logger: logger}
}

// Extract recorre los componentes del dashboard. Los que traen error o vienen
// vacíos se omiten; un componente que no se puede decodificar aborta.
func (e *SeriesExtractor) Extract(response *dashboard.RenderResponse) ([]dashboard.Series, error) {
	var series []dashboard.Series

	for _, component := range response.Components {
		if component.Error != nil {
			e.logger.Warn("componente de DataOS con error; se omite", "component_id", component.ID, "error", *component.Error)
			continue
		}
		if len(component.Data) == 0 {
			continue
		}

		var chartData dashboard.ChartData
		if err := json.Unmarshal(component.Data, &chartData); err != nil {
			return nil, fmt.Errorf("decodificar el componente %q: %w", component.ID, err)
		}

		for _, serie := range chartData.Series {
			// DataOS nombra igual ("MP-11556") temperatura y humedad: el id del
			// componente permite diferenciarlas al mapear.
			serie.ComponentID = component.ID
			series = append(series, serie)
		}
	}

	return series, nil
}
