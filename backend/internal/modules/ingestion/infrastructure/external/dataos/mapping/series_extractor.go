package mapping

import (
	"encoding/json"
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// SeriesExtractor se encarga de extraer las series de datos contenidas
// en la respuesta obtenida desde DataOS.
//
// Utiliza el logger para registrar el proceso de extracción, incluyendo
// los componentes procesados, las series encontradas y posibles errores
// durante la decodificación de los datos.
type SeriesExtractor struct {
	logger *logger.Logger
}

// NewSeriesExtractor crea una nueva instancia de SeriesExtractor.
//
// Recibe:
//   - logger: instancia del logger utilizada para registrar información,
//     advertencias y errores durante la extracción.
//
// Devuelve:
//   - *SeriesExtractor: nueva instancia configurada con el logger recibido.
func NewSeriesExtractor(
	logger *logger.Logger,
) *SeriesExtractor {
	return &SeriesExtractor{
		logger: logger,
	}
}

// Extract procesa los componentes de una respuesta de DataOS y extrae
// las series de datos contenidas en ellos.
//
// El método recorre los componentes de la respuesta, descarta aquellos
// que contienen errores o no tienen datos, decodifica el JSON de cada
// componente y obtiene las series definidas en su estructura.
//
// Recibe:
//   - response: respuesta obtenida desde DataOS que contiene los
//     componentes y sus datos.
//
// Devuelve:
//   - []dashboard.Series: lista de todas las series encontradas en los
//     componentes procesados.
//   - error: nil si la extracción fue exitosa; en caso contrario,
//     devuelve el error producido durante la decodificación de los datos.
//
// Los componentes que contienen errores o datos vacíos son omitidos y
// no generan un error en la operación.
func (e *SeriesExtractor) Extract(
	response *dashboard.RenderResponse,
) ([]dashboard.Series, error) {

	var series []dashboard.Series

	e.logger.Info("INICIO extracción de series", "components", len(response.Components))

	for componentIndex, component := range response.Components {
		e.logger.Info("Procesando componente", "index", componentIndex, "component_id", component.ID)
		// 1. Componente con error
		if component.Error != nil {

			e.logger.Warn(
				"componente contiene error",
				"component_id", component.ID,
				"error", *component.Error,
			)

			continue
		}

		// 2. Componente sin datos
		if len(component.Data) == 0 {

			e.logger.Warn(
				"componente tiene data vacía",
				"component_id", component.ID,
			)

			continue
		}

		// 4. Decodificar JSON
		var chartData dashboard.ChartData

		if err := json.Unmarshal(
			component.Data,
			&chartData,
		); err != nil {

			e.logger.Error(
				"falló decode de component.Data",
				"component_id", component.ID,
				"error", err,
				"raw_data", string(component.Data),
			)

			return nil, fmt.Errorf(
				"decode component %q data: %w",
				component.ID,
				err,
			)
		}

		// 6. Procesar series
		for seriesIndex, serie := range chartData.Series {

			e.logger.Info(
				"serie encontrada",
				"series_index", seriesIndex,
				"name", serie.Name,
				"unit", serie.Unit,
				"data_points", len(serie.DataPoints),
			)

			series = append(
				series,
				serie,
			)
		}
	}

	// 7. Resultado final
	e.logger.Info("FIN extracción de series", "series_count", len(series))
	return series, nil
}
