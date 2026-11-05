package dataos

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/mapping"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type Client struct {
	authClient      *auth.Client
	dashboardClient *dashboard.Client
	seriesExtractor *mapping.SeriesExtractor
	mapSeries       *mapping.Mapper
	logger          *logger.Logger
}

func NewClient(
	authClient *auth.Client,
	dashboardClient *dashboard.Client,
	seriesExtractor *mapping.SeriesExtractor,
	mapSeries *mapping.Mapper,
	logger *logger.Logger,
) *Client {
	return &Client{
		authClient:      authClient,
		dashboardClient: dashboardClient,
		seriesExtractor: seriesExtractor,
		mapSeries:       mapSeries,
		logger:          logger,
	}
}

// GetMeasurements implementa ports.DataProvider: inicia sesión, pide el
// dashboard del punto de medición en [start, end] y mapea sus series.
func (c *Client) GetMeasurements(ctx context.Context, device *entities.Device, start time.Time, end time.Time,
) ([]entities.RawMeasurement, error) {
	if err := c.authClient.Login(ctx); err != nil {
		c.logger.Error("no se pudo iniciar sesión en DataOS", "error", err)
		return nil, fmt.Errorf("iniciar sesión en DataOS: %w", err)
	}

	response, err := c.fetchDataOSData(ctx, device.MeasuringPointID, start, end)
	if err != nil {
		c.logger.Error("no se pudieron consultar los datos de DataOS", "mp_id", device.MeasuringPointID, "start", start, "end", end, "error", err)
		return nil, err
	}

	series, err := c.seriesExtractor.Extract(response)
	if err != nil {
		c.logger.Error("no se pudieron extraer las series de DataOS", "mp_id", device.MeasuringPointID, "error", err)
		return nil, err
	}

	measurements, err := c.mapSeriesMethod(device, series)
	if err != nil {
		c.logger.Error("no se pudieron mapear las series de DataOS", "mp_id", device.MeasuringPointID, "error", err)
		return nil, err
	}

	c.logger.Info("datos obtenidos de DataOS", "mp_id", device.MeasuringPointID, "series", len(series), "mediciones", len(measurements))
	return measurements, nil
}

func (c *Client) fetchDataOSData(ctx context.Context, measuringPointID int64, start time.Time, end time.Time,
) (*dashboard.RenderResponse, error) {
	request := dashboard.RenderRequest{
		Period: dashboard.Period{
			Type:  "absolute",
			Start: start.Format("2006-01-02T15:04:05"),
			End:   end.Format("2006-01-02T15:04:05"),
		},
		Timezone:         "America/Bogota",
		MeasuringPointID: measuringPointID,
	}

	response, err := c.dashboardClient.Render(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("consultar mediciones de DataOS: %w", err)
	}
	return response, nil
}

func (c *Client) mapSeriesMethod(device *entities.Device, series []dashboard.Series,
) ([]entities.RawMeasurement, error) {
	var measurements []entities.RawMeasurement
	var unmapped []string

	for _, serie := range series {
		mapped, err := c.mapSeries.MapSeries(device, serie)
		if err != nil {
			return nil, fmt.Errorf("mapear la serie %q del punto %d: %w", serie.Name, device.MeasuringPointID, err)
		}
		if mapped == nil {
			unmapped = append(unmapped, serie.Name+" ("+serie.ComponentID+")")
			continue
		}
		measurements = append(measurements, mapped...)
	}

	if len(unmapped) > 0 {
		c.logger.Warn("series sin mapeo; no se guardan", "mp_id", device.MeasuringPointID, "series", unmapped)
	}
	return measurements, nil
}
