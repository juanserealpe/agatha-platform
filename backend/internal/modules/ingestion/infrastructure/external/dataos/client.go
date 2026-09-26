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

func (c *Client) Login(ctx context.Context) error {
	return c.authClient.Login(ctx)
}

func (c *Client) GetMeasurements(ctx context.Context, device *entities.Device, start time.Time, end time.Time,
) ([]entities.RawMeasurement, error) {
	c.logger.Info("[GetMeasurements] INGRESANDO AL MÉTODO")

	c.logger.Info("[GetMeasurements] Iniciando proceso de login a DataOS")
	if err := c.authClient.Login(ctx); err != nil {
		c.logger.Error("[GetMeasurements] Error al iniciar sesión en DataOS", "error", err)
		return nil, fmt.Errorf("login to DataOS: %w", err)
	}
	c.logger.Info("[GetMeasurements] Sesión iniciada correctamente en DataOS")
	c.logger.Info("[GetMeasurements] Iniciando proceso de obtención de datos de DataOS", "device", device.MeasuringPointID, "start", start, "end", end)
	response, err := c.fetchDataOSData(ctx, device.MeasuringPointID, start, end)
	if err != nil {
		c.logger.Error("[GetMeasurements] Error al obtener datos de DataOS", "error", err, "device", device.MeasuringPointID, "start", start, "end", end)
		return nil, err
	}
	c.logger.Info("[GetMeasurements] Datos obtenidos correctamente de DataOS", "device", device.MeasuringPointID, "start", start, "end", end)

	c.logger.Info("[GetMeasurements] Iniciando proceso de extracción de series de datos de DataOS", "device", device.MeasuringPointID, "start", start, "end", end)
	series, err := c.seriesExtractor.Extract(response)
	if err != nil {
		c.logger.Error("[GetMeasurements] Error al extraer series de datos de DataOS", "error", err, "device", device.MeasuringPointID, "start", start, "end", end)
		return nil, err
	}
	c.logger.Info("[GetMeasurements] Series de datos extraídas correctamente de DataOS", "device", device.MeasuringPointID, "start", start, "end", end)

	measurements, err := c.mapSeriesMethod(device, series)
	if err != nil {
		c.logger.Error("[GetMeasurements] Error al mapear series de datos", "error", err, "device", device.MeasuringPointID)
		return nil, err
	}
	c.logger.Info("[GetMeasurements] Series de datos mapeadas correctamente", "device", device.MeasuringPointID, "start", start, "end", end)

	return measurements, nil
}

func (c *Client) fetchDataOSData(
	ctx context.Context,
	measuringPointID int64,
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
		MeasuringPointID: measuringPointID,
	}

	response, err := c.dashboardClient.Render(ctx, request)
	if err != nil {
		c.logger.Error(
			"[FetchDataOsData] Error al obtener mediciones de DataOS",
			"error", err,
			"measuring_point_id", request.MeasuringPointID,
			"start", request.Period.Start,
			"end", request.Period.End,
			"timezone", request.Timezone,
		)

		return nil, fmt.Errorf("fetch DataOS measurements: %w", err)
	}

	c.logger.Info(
		"[FetchDataOsData] Respuesta de DataOS recibida",
		"measuring_point_id", request.MeasuringPointID,
		"start", request.Period.Start,
		"end", request.Period.End,
		"timezone", request.Timezone,
		"components", len(response.Components),
	)

	return response, nil
}

func (c *Client) mapSeriesMethod(device *entities.Device, series []dashboard.Series,
) ([]entities.RawMeasurement, error) {

	var measurements []entities.RawMeasurement
	c.logger.Info("[mapSeriesMethos] INGRESANDO AL MÉTODO")
	c.logger.Info("[mapSeriesMethod] Número de series a mapear", "cantidad", len(series), "device", device.MeasuringPointID)

	for _, serie := range series {

		c.logger.Info("[mapSeriesMethod] mapeando serie", "serie", serie.Name)
		mapped, err := c.mapSeries.MapSeries(device, serie)

		if err != nil {
			c.logger.Error("[mapSeriesMethod] error al mapear serie", "serie", serie.Name, "device", device.MeasuringPointID, "error", err)
			return nil, fmt.Errorf("map series %q for device %d: %w", serie.Name, device.MeasuringPointID, err)
		}

		if mapped == nil {
			c.logger.Warn("[mapSeriesMethod] serie no mapeada", "serie", serie.Name, "device", device.MeasuringPointID)
			continue
		}

		measurements = append(measurements, mapped...)
	}

	return measurements, nil
}
