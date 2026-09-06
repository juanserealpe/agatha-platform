package dataos

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/mapping"
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

func (c *Client) Login(ctx context.Context) error {
	return c.authClient.Login(ctx)
}

func (c *Client) GetMeasurements(ctx context.Context, device *entities.Device, start time.Time, end time.Time,
) ([]entities.RawMeasurement, error) {

	if err := c.authClient.Login(ctx); err != nil {
		return nil, fmt.Errorf("login to DataOS: %w", err)
	}

	response, err := c.fetchDataOSData(ctx, device.MeasuringPointID, start, end)
	if err != nil {
		return nil, err
	}

	series, err := c.seriesExtractor.Extract(response)
	if err != nil {
		return nil, err
	}

	measurements, err := c.mapSeriesMethod(device, series)
	if err != nil {
		return nil, err
	}

	return measurements, nil
}

func (c *Client) fetchDataOSData(ctx context.Context, measuringPointID int64, start time.Time, end time.Time) (*dashboard.RenderResponse, error) {
	request := dashboard.RenderRequest{
		Period: dashboard.Period{
			Type:  "absolute",
			Start: start.Format("2006-01-02T15:04:05"),
			End:   end.Format("2006-01-02T15:04:05"),
		},
		Timezone:         "UTC",
		MeasuringPointID: measuringPointID,
	}
	c.logger.Info("Request para DataOS", "Inicio", request.Period.Start, "Fin", request.Period.End, "Zona horaria", request.Timezone)
	response, err := c.dashboardClient.Render(ctx, request)

	if err != nil {
		return nil, fmt.Errorf("fetch DataOS measurements: %w", err)
	}

	return response, nil
}
func (c *Client) mapSeriesMethod(
	device *entities.Device,
	series []dashboard.Series,
) ([]entities.RawMeasurement, error) {

	var measurements []entities.RawMeasurement

	for _, serie := range series {

		mapped, err := c.mapSeries.MapSeries(device, serie)

		if err != nil {
			return nil, fmt.Errorf("map series %q for device %d: %w", serie.Name, device.MeasuringPointID, err)
		}

		if mapped == nil {
			fmt.Println("[Ingestion] serie ignorada: métrica no encontrada", "series_name", serie.Name, "measuring_point_id", device.MeasuringPointID)
			continue
		}

		measurements = append(measurements, mapped...)
	}

	return measurements, nil
}
