package services

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// BackfillCommand es la entrada de UC-B4.
type BackfillCommand struct {
	PointID int64
	From    time.Time // cero ⇒ primer registro de B2 del dispositivo
	To      time.Time // fin exclusivo del reloj virtual (normalmente ahora)
	DryRun  bool      // simula sin persistir
}

// BackfillResult es la salida de UC-B4. Versions trae las versiones simuladas
// (con ID solo si Created = true).
type BackfillResult struct {
	PointID       int64
	From, To      time.Time
	Created       bool
	Versions      []entities.BaselineVersion
	SkippedReason SkippedReason
}

// BackfillService implementa UC-B4: cargar el histórico de versiones que
// habrían existido desde que el dispositivo tiene datos. Simula con
// ReplayService (reloj virtual + RecalculationPolicy real, solo datos
// anteriores a cada recálculo) y persiste el resultado en una sola TX.
//
// Solo aplica a un punto SIN versiones: el versionado no es retroactivo.
type BackfillService struct {
	history   repositories.HistoryReader
	baselines repositories.BaselineRepository
	replay    *ReplayService
	log       *logger.Logger
}

func NewBackfillService(
	history repositories.HistoryReader,
	baselines repositories.BaselineRepository,
	replay *ReplayService,
	log *logger.Logger,
) *BackfillService {
	return &BackfillService{history: history, baselines: baselines, replay: replay, log: log}
}

// Backfill (UC-B4). Cada versión queda con created_at = valid_from (como si
// se hubiera creado en su momento), de modo que RecalculationPolicy continúa
// el calendario desde la última sin saltos. Los errores devueltos son
// *AppError de baseline/domain, salvo la cancelación del contexto.
func (s *BackfillService) Backfill(ctx context.Context, cmd BackfillCommand) (BackfillResult, error) {
	result := BackfillResult{PointID: cmd.PointID, To: cmd.To.UTC(), Versions: []entities.BaselineVersion{}}

	if cmd.PointID <= 0 {
		s.log.Error("[BackfillService] Punto de medición inválido", "measuring_point_id", cmd.PointID)
		return result, domain.ErrInvalidMeasuringPointID
	}

	if !cmd.DryRun {
		latest, err := s.baselines.GetLatest(ctx, cmd.PointID)
		if err != nil {
			s.log.Error("[BackfillService] Error consultando la última versión",
				"measuring_point_id", cmd.PointID, "error", err)
			return result, domain.ErrConsultingBaseline
		}
		if latest != nil {
			s.log.Warn("[BackfillService] El punto ya tiene versiones; no se carga el histórico",
				"measuring_point_id", cmd.PointID, "latest_version", latest.Version)
			return result, domain.ErrBaselineHistoryExists
		}
	}

	from := cmd.From.UTC()
	if cmd.From.IsZero() {
		firstSeen, err := s.history.GetFirstSeenAt(ctx, cmd.PointID)
		if err != nil {
			s.log.Error("[BackfillService] Error consultando el primer registro",
				"measuring_point_id", cmd.PointID, "error", err)
			return result, domain.ErrConsultingHistory
		}
		if firstSeen == nil {
			result.SkippedReason = SkippedInsufficientHistory
			return result, nil
		}
		from = firstSeen.UTC()
	}
	result.From = from

	versions, _, err := s.replay.SimulateVersions(ctx, cmd.PointID, from, result.To)
	if err != nil {
		return result, err
	}
	result.Versions = versions

	if len(versions) == 0 {
		result.SkippedReason = SkippedInsufficientHistory
		return result, nil
	}

	if cmd.DryRun {
		result.SkippedReason = SkippedDryRun
		return result, nil
	}

	saved, err := s.baselines.SaveHistory(ctx, cmd.PointID, versions)
	if err != nil {
		if errors.Is(err, domain.ErrBaselineHistoryExists) {
			s.log.Warn("[BackfillService] Otro proceso creó versiones; no se carga el histórico",
				"measuring_point_id", cmd.PointID)
			return result, domain.ErrBaselineHistoryExists
		}

		s.log.Error("[BackfillService] Error guardando el histórico",
			"measuring_point_id", cmd.PointID, "versions", len(versions), "error", err)
		return result, domain.ErrPersistingBaseline
	}

	result.Created = true
	result.Versions = saved

	s.log.Info("[BackfillService] Histórico de línea base cargado",
		"measuring_point_id", cmd.PointID,
		"versions", len(saved),
		"from", saved[0].ValidFrom,
		"to", saved[len(saved)-1].ValidFrom,
	)

	return result, nil
}
