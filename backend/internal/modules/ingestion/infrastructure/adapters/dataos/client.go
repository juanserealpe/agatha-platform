package dataos

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/mapping"
)

type Client struct {
	authClient      *auth.Client
	dashboardClient *dashboard.Client
	seriesExtractor *mapping.SeriesExtractor
	mapSeries       *mapping.Mapper
}

func NewClient(
	authClient *auth.Client,
	dashboardClient *dashboard.Client,
	seriesExtractor *mapping.SeriesExtractor,
	mapSeries *mapping.Mapper,
) *Client {
	return &Client{
		authClient:      authClient,
		dashboardClient: dashboardClient,
		seriesExtractor: seriesExtractor,
		mapSeries:       mapSeries,
	}
}

func (c *Client) Login(ctx context.Context) error {
	return c.authClient.Login(ctx)
}

func (c *Client) GetMeasurements(
	ctx context.Context,
	device *entities.Device,
	start time.Time,
	end time.Time,
) ([]entities.RawMeasurement, error) {

	if err := c.authClient.Login(ctx); err != nil {
		return nil, fmt.Errorf(
			"login to DataOS: %w",
			err,
		)
	}

	response, err := c.fetchDataOSData(
		ctx,
		device,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}

	series, err := c.seriesExtractor.Extract(response)
	if err != nil {
		return nil, err
	}

	measurements, err := c.mapSeriesMethod(
		ctx,
		device,
		series,
	)
	if err != nil {
		return nil, err
	}

	return measurements, nil
}

func (c *Client) fetchDataOSData(
	ctx context.Context,
	device *entities.Device,
	start time.Time,
	end time.Time,
) (*dashboard.RenderResponse, error) {

	request := dashboard.RenderRequest{
		Period: dashboard.Period{
			Type:  "absolute",
			Start: start.Format("2006-01-02T15:04:05"),
			End:   end.Format("2006-01-02T15:04:05"),
		},
		Timezone:         "America/Bogota",
		MeasuringPointID: device.MeasuringPointID,
	}

	response, err := c.dashboardClient.Render(ctx, request)

	if err != nil {
		return nil, fmt.Errorf(
			"fetch DataOS measurements: %w",
			err,
		)
	}

	return response, nil
}
func (c *Client) mapSeriesMethod(ctx context.Context, device *entities.Device, series []dashboard.Series) ([]entities.RawMeasurement, error) {

	var measurements []entities.RawMeasurement

	for _, serie := range series {
		metricKey, ok := mapping.MetricKeyFromDataOS(serie.Name)
		if !ok {
			continue
		}

		mapped, err := c.mapSeries.MapSeries(
			device,
			serie,
			metricKey,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"map series %q for device %q: %w",
				serie.Name,
				device.MeasuringPointID,
				err,
			)
		}

		measurements = append(measurements, mapped...)
	}

	return measurements, nil
}
