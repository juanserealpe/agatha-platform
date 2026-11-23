package services

import (
	"context"
	"errors"
	"time"

	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DataOSIngester trae de DataOS el rango (start, end] y lo persiste
// (IngestService.IngestDataOs).
type DataOSIngester interface {
	IngestDataOs(ctx context.Context, measuringPointID int64, start time.Time, end time.Time) error
}

// CatchUpConfig parametriza la búsqueda de huecos.
type CatchUpConfig struct {
	// Lookback es la ventana en la que se buscan huecos internos. Si el último
	// dato es más viejo, la búsqueda arranca en él (PC apagada varios días).
	Lookback time.Duration
	// MinAge deja sin tocar lo más reciente: en el VPS lo entrega el callback.
	MinAge time.Duration
	// MaxStep es la separación máxima normal entre registros (cada 15 min).
	MaxStep time.Duration
	// JoinWithin pide juntos los huecos más cercanos que esto (menos consultas).
	JoinWithin time.Duration
	// Partition es el rango máximo de una consulta a DataOS.
	Partition time.Duration
}

// CatchUpResult resume una corrida para un punto de medición.
type CatchUpResult struct {
	MeasuringPointID int64
	Gaps             int // huecos encontrados
	Requests         int // consultas a DataOS
	NoData           int // consultas sin datos (equipo sin transmitir)
	Failed           int // consultas que fallaron
}

// CatchUpService recupera las mediciones que faltan: busca huecos en la
// grilla de registros de cada punto activo y los pide a DataOS. Es idempotente:
// lo que ya estaba lo descarta el índice único y un lote sin filas nuevas se
// revierte (IngestDataOs).
type CatchUpService struct {
	devices  repositories.DeviceRepository
	coverage repositories.MeasurementCoverageRepository
	ingester DataOSIngester
	cfg      CatchUpConfig
	location *time.Location
	logger   *logger.Logger
}

func NewCatchUpService(
	devices repositories.DeviceRepository,
	coverage repositories.MeasurementCoverageRepository,
	ingester DataOSIngester,
	cfg CatchUpConfig,
	location *time.Location,
	logger *logger.Logger,
) *CatchUpService {
	return &CatchUpService{
		devices:  devices,
		coverage: coverage,
		ingester: ingester,
		cfg:      cfg,
		location: location,
		logger:   logger,
	}
}

// Run recorre los puntos activos. Un punto que falla no frena a los demás; solo
// devuelve error si no se pudo listar los puntos o si se canceló ctx.
func (s *CatchUpService) Run(ctx context.Context, now time.Time) ([]CatchUpResult, error) {
	devices, err := s.devices.FindActive(ctx)
	if err != nil {
		s.logger.Error("no se pudieron listar los puntos de medición", "error", err)
		return nil, ingestionErrors.ErrConsultingDevice
	}

	results := make([]CatchUpResult, 0, len(devices))
	for _, device := range devices {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		results = append(results, s.runDevice(ctx, device.MeasuringPointID, now))
	}
	return results, nil
}

func (s *CatchUpService) runDevice(ctx context.Context, mp int64, now time.Time) CatchUpResult {
	result := CatchUpResult{MeasuringPointID: mp}

	until := now.Add(-s.cfg.MinAge)
	from := now.Add(-s.cfg.Lookback)

	latest, err := s.coverage.LatestTimestamp(ctx, mp)
	if err != nil {
		s.logger.Error("no se pudo consultar el último dato", "mp_id", mp, "error", err)
		result.Failed++
		return result
	}
	if latest != nil && latest.Before(from) {
		from = *latest
	}
	if !from.Before(until) {
		return result
	}

	timestamps, err := s.coverage.ListTimestamps(ctx, mp, from, until)
	if err != nil {
		s.logger.Error("no se pudieron consultar las mediciones existentes", "mp_id", mp, "error", err)
		result.Failed++
		return result
	}

	gaps := findGaps(timestamps, from, until, s.cfg.MaxStep)
	result.Gaps = len(gaps)
	if len(gaps) == 0 {
		return result
	}

	for _, r := range splitRanges(mergeRanges(gaps, s.cfg.JoinWithin), s.cfg.Partition) {
		if ctx.Err() != nil {
			return result
		}
		result.Requests++

		// DataOS interpreta el rango en hora de Bogotá (dataos.Client).
		err := s.ingester.IngestDataOs(ctx, mp, r.Start.In(s.location), r.End.In(s.location))
		switch {
		case errors.Is(err, ingestionErrors.ErrRecivingIngestedData):
			result.NoData++
		case err != nil:
			result.Failed++
			s.logger.Error("no se pudo recuperar el hueco", "mp_id", mp, "desde", r.Start, "hasta", r.End, "error", err)
		}
	}

	s.logger.Info("catch-up del punto terminado", "mp_id", mp, "huecos", result.Gaps, "consultas", result.Requests,
		"sin_datos", result.NoData, "fallidas", result.Failed)
	return result
}

// findGaps devuelve los tramos (a, b] entre timestamps consecutivos —incluidos
// los bordes from y until— separados por más de maxStep.
func findGaps(timestamps []time.Time, from, until time.Time, maxStep time.Duration) []entities.TimeRange {
	var gaps []entities.TimeRange
	prev := from
	visit := func(ts time.Time) {
		if ts.Sub(prev) > maxStep {
			gaps = append(gaps, entities.TimeRange{Start: prev, End: ts})
		}
		if ts.After(prev) {
			prev = ts
		}
	}
	for _, ts := range timestamps {
		visit(ts)
	}
	visit(until)
	return gaps
}

// mergeRanges une los tramos (ordenados) separados por joinWithin o menos.
func mergeRanges(ranges []entities.TimeRange, joinWithin time.Duration) []entities.TimeRange {
	var merged []entities.TimeRange
	for _, r := range ranges {
		if n := len(merged); n > 0 && r.Start.Sub(merged[n-1].End) <= joinWithin {
			if r.End.After(merged[n-1].End) {
				merged[n-1].End = r.End
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// splitRanges parte cada tramo en consultas de hasta partition, contiguas.
func splitRanges(ranges []entities.TimeRange, partition time.Duration) []entities.TimeRange {
	var out []entities.TimeRange
	for _, r := range ranges {
		for start := r.Start; start.Before(r.End); {
			end := start.Add(partition)
			if end.After(r.End) {
				end = r.End
			}
			out = append(out, entities.TimeRange{Start: start, End: end})
			start = end
		}
	}
	return out
}
