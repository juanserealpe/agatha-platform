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
func (e *SeriesExtractor) Extract(response *dashboard.RenderResponse) ([]dashboard.Series, error) {
	var series []dashboard.Series

	e.logger.Info(
		"INICIO extracción de series",
		"components",
		len(response.Components),
	)

	for componentIndex, component := range response.Components {
		e.logger.Info(
			"Procesando componente",
			"index",
			componentIndex,
			"component_id",
			component.ID,
		)

		if component.Error != nil {
			e.logger.Warn(
				"componente contiene error",
				"component_id",
				component.ID,
				"error",
				*component.Error,
			)
			continue
		}

		if len(component.Data) == 0 {
			e.logger.Warn(
				"componente tiene data vacía",
				"component_id",
				component.ID,
			)
			continue
		}

		var chartData dashboard.ChartData

		if err := json.Unmarshal(component.Data, &chartData); err != nil {
			e.logger.Error(
				"falló decode de component.Data",
				"component_id",
				component.ID,
				"error",
				err,
			)

			return nil, fmt.Errorf(
				"decode component %q data: %w",
				component.ID,
				err,
			)
		}

		e.logger.Info(
			"Series encontradas en componente",
			"component_id",
			component.ID,
			"series_count",
			len(chartData.Series),
		)

		for _, serie := range chartData.Series {

			// DataOS utiliza "MP-11556" como nombre tanto para
			// temperatura como para humedad. Conservamos el ID
			// del componente para poder diferenciarlas posteriormente.
			serie.ComponentID = component.ID

			e.logger.Info(
				"Serie extraída",
				"component_id",
				component.ID,
				"name",
				serie.Name,
				"unit",
				serie.Unit,
				"points",
				len(serie.DataPoints),
			)

			series = append(series, serie)
		}
	}

	e.logger.Info(
		"FIN extracción de series",
		"series_count",
		len(series),
	)

	return series, nil
}
