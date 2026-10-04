package baseline

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
)

// Replayer es la segunda superficie pública de Baseline, solo para la
// herramienta de reproducción histórica (cmd/replay). No toca la Facade
// congelada y NUNCA escribe en la BD.
type Replayer interface {
	// SimulateVersions devuelve las versiones que habrían existido en
	// [from, to): reloj virtual horario, RecalculationPolicy real y estimación
	// en dry-run con solo los datos anteriores a cada recálculo. Orden
	// ascendente por ValidFrom; VersionID = 0 y Version secuencial.
	SimulateVersions(ctx context.Context, pointID int64, from, to time.Time) ([]EffectiveBaseline, error)

	// ComputeAt calcula una sola versión en asOf (dry-run), para el modo
	// "línea base fija". Histórico insuficiente ⇒ Status = NONE.
	ComputeAt(ctx context.Context, pointID int64, asOf time.Time) (EffectiveBaseline, error)
}

type replayer struct {
	replay *services.ReplayService
}

// NewReplayer crea el Replayer sobre el ReplayService.
func NewReplayer(replay *services.ReplayService) Replayer {
	return &replayer{replay: replay}
}

func (r *replayer) SimulateVersions(ctx context.Context, pointID int64, from, to time.Time,
) ([]EffectiveBaseline, error) {
	versions, firstSeen, err := r.replay.SimulateVersions(ctx, pointID, from, to)
	if err != nil {
		return nil, err
	}

	out := make([]EffectiveBaseline, 0, len(versions))
	for i := range versions {
		out = append(out, toEffectiveBaseline(pointID, services.EffectiveResult{
			Version:     &versions[i],
			FirstSeenAt: firstSeen,
		}))
	}

	return out, nil
}

func (r *replayer) ComputeAt(ctx context.Context, pointID int64, asOf time.Time) (EffectiveBaseline, error) {
	result, err := r.replay.ComputeAt(ctx, pointID, asOf)
	if err != nil {
		return EffectiveBaseline{}, err
	}

	return toEffectiveBaseline(pointID, result), nil
}
