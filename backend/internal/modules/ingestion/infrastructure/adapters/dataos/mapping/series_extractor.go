package mapping

import (
	"encoding/json"
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type SeriesExtractor struct {
	logger *logger.Logger
}

func NewSeriesExtractor(
	logger *logger.Logger,
) *SeriesExtractor {
	return &SeriesExtractor{
		logger: logger,
	}
}

func (e *SeriesExtractor) Extract(
	response *dashboard.RenderResponse,
) ([]dashboard.Series, error) {

	var series []dashboard.Series

	e.logger.Info(
		"starting DataOS series extraction",
		"components", len(response.Components),
	)

	for _, component := range response.Components {

		e.logger.Info(
			"processing DataOS component",
			"component_id", component.ID,
			"type", component.Type,
			"title", component.Title,
			"data_size", len(component.Data),
		)

		if component.Error != nil {

			e.logger.Warn(
				"DataOS component contains an error",
				"component_id", component.ID,
				"error", *component.Error,
			)

			continue
		}

		if len(component.Data) == 0 {

			e.logger.Warn(
				"DataOS component has empty data",
				"component_id", component.ID,
			)

			continue
		}

		var chartData dashboard.ChartData

		if err := json.Unmarshal(
			component.Data,
			&chartData,
		); err != nil {

			e.logger.Error(
				"failed to decode DataOS component data",
				"component_id", component.ID,
				"error", err,
			)

			return nil, fmt.Errorf(
				"decode component %q data: %w",
				component.ID,
				err,
			)
		}

		e.logger.Info(
			"DataOS component decoded",
			"component_id", component.ID,
			"series", len(chartData.Series),
		)

		for _, serie := range chartData.Series {

			e.logger.Info(
				"DataOS series discovered",
				"component_id", component.ID,
				"metric", serie.Name,
				"unit", serie.Unit,
				"data_points", len(serie.DataPoints),
			)

			// Temporalmente NO descartamos series con 0 puntos.
			// Queremos comprobar exactamente qué está devolviendo DataOS.
			series = append(series, serie)
		}
	}

	e.logger.Info(
		"DataOS series extraction completed",
		"series_count", len(series),
	)

	return series, nil
}
