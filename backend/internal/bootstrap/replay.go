package bootstrap

import (
	"context"
	"io"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/ports"
	processingServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/repositories"
	processingRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/replay"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Modos de la línea base en la reproducción histórica.
const (
	// ReplayModeAsOf usa, en cada hora, la versión que habría estado vigente.
	ReplayModeAsOf = "asof"
	// ReplayModeFixed usa una sola versión calculada en FixedAsOf (backtest del analista).
	ReplayModeFixed = "fixed"
)

// ReplayOptions configura cmd/replay. Se evalúan las horas t ∈ [From, To]
// (en punto, UTC) con samples de [From − 24 h, To).
type ReplayOptions struct {
	PointID   int64
	From, To  time.Time
	Mode      string    // ReplayModeAsOf | ReplayModeFixed
	FixedAsOf time.Time // solo fixed; cero ⇒ To
	Out       io.Writer // destino del CSV
}

// ReplaySummary resume la reproducción.
type ReplaySummary struct {
	Hours            int
	Rows             int
	Calculated       int
	BaselineVersions int
}

// RunReplay evalúa el pasado con el MISMO EvaluationService de producción,
// cambiando solo los adaptadores: lector real (conexión de solo lectura),
// línea base simulada o fija en memoria, sink CSV y reloj virtual en To.
// No migra, no registra suscripciones ni arranca schedulers.
func RunReplay(ctx context.Context, opts ReplayOptions) (ReplaySummary, error) {
	if err := opts.validate(); err != nil {
		return ReplaySummary{}, err
	}

	platform, err := NewReadOnlyPlatform()
	if err != nil {
		return ReplaySummary{}, err
	}

	log := logger.New("Replay", "Info")
	cfg, metricIDs := processingConfigFromEnv(log)

	return runReplay(
		ctx,
		processingRepositories.NewMeasurementReader(platform.DB, metricIDs),
		newBaselineReplayer(platform),
		cfg,
		opts,
		log,
	)
}

func runReplay(
	ctx context.Context,
	reader repositories.MeasurementReader,
	replayer baseline.Replayer,
	cfg processingServices.Config,
	opts ReplayOptions,
	log *logger.Logger,
) (ReplaySummary, error) {
	if err := opts.validate(); err != nil {
		return ReplaySummary{}, err
	}

	provider, err := newReplayBaselineProvider(ctx, replayer, opts)
	if err != nil {
		return ReplaySummary{}, err
	}

	to := opts.To.UTC()
	sink := replay.NewCSVEvaluationSink(opts.Mode, ceilHour(opts.From.UTC()))

	service := processingServices.NewEvaluationService(
		reader,
		sink,
		provider,
		ports.ClockFunc(func() time.Time { return to }),
		cfg,
		log,
	)

	result, err := service.EvaluatePending(ctx, processingServices.EvaluatePendingCommand{
		PointID:    opts.PointID,
		ReadyUntil: to,
		Trigger:    entities.TriggerReplay,
		MaxSlots:   0, // todo el rango, una sola lectura de samples
	})
	if err != nil {
		return ReplaySummary{}, err
	}

	if err := sink.WriteCSV(opts.Out); err != nil {
		return ReplaySummary{}, err
	}

	summary := ReplaySummary{Hours: result.Planned, BaselineVersions: provider.versionCount()}
	for _, row := range sink.Rows() {
		summary.Rows++
		if row.Status == entities.StatusCalculated {
			summary.Calculated++
		}
	}

	return summary, nil
}

func (o ReplayOptions) validate() error {
	switch {
	case o.PointID <= 0:
		return &appErrors.AppError{Code: appErrors.CodeInvalidMeasuringPointID, Message: "-point es obligatorio y debe ser > 0"}
	case !o.From.Before(o.To):
		return &appErrors.AppError{Code: appErrors.CodeInvalidInput, Message: "-from debe ser anterior a -to"}
	case o.Mode != ReplayModeAsOf && o.Mode != ReplayModeFixed:
		return &appErrors.AppError{Code: appErrors.CodeInvalidInput, Message: "-mode debe ser asof o fixed"}
	case o.Out == nil:
		return &appErrors.AppError{Code: appErrors.CodeInvalidInput, Message: "falta el destino del CSV"}
	}
	return nil
}

// replayBaselineProvider implementa el port BaselineProvider de Processing en
// memoria. Vive en bootstrap porque es el único lugar que conoce a la vez
// baseline.Replayer y el port de Processing.
type replayBaselineProvider struct {
	versions  []entities.BaselineSnapshot // modo asof, ascendente por ValidFrom
	fixed     *entities.BaselineSnapshot  // modo fixed
	firstSeen *time.Time
}

func newReplayBaselineProvider(ctx context.Context, replayer baseline.Replayer, opts ReplayOptions,
) (*replayBaselineProvider, error) {
	if opts.Mode == ReplayModeFixed {
		asOf := opts.FixedAsOf
		if asOf.IsZero() {
			asOf = opts.To
		}

		effective, err := replayer.ComputeAt(ctx, opts.PointID, asOf.UTC())
		if err != nil {
			return nil, err
		}

		fixed := toBaselineSnapshot(effective)
		return &replayBaselineProvider{fixed: &fixed, firstSeen: fixed.FirstSeenAt}, nil
	}

	simulated, err := replayer.SimulateVersions(ctx, opts.PointID, opts.From.UTC(), opts.To.UTC())
	if err != nil {
		return nil, err
	}

	p := &replayBaselineProvider{versions: make([]entities.BaselineSnapshot, 0, len(simulated))}
	for _, eb := range simulated {
		p.versions = append(p.versions, toBaselineSnapshot(eb))
	}
	if len(p.versions) > 0 {
		p.firstSeen = p.versions[0].FirstSeenAt
	}

	return p, nil
}

// GetEffective: modo fixed ⇒ siempre la misma versión; modo asof ⇒ la última
// con ValidFrom ≤ at (sin versión ⇒ Available = false).
func (p *replayBaselineProvider) GetEffective(_ context.Context, _ int64, at time.Time,
) (entities.BaselineSnapshot, error) {
	if p.fixed != nil {
		return *p.fixed, nil
	}

	i := sort.Search(len(p.versions), func(i int) bool { return p.versions[i].ValidFrom.After(at) })
	if i == 0 {
		return entities.BaselineSnapshot{FirstSeenAt: p.firstSeen}, nil
	}

	return p.versions[i-1], nil
}

func (p *replayBaselineProvider) versionCount() int {
	if p.fixed != nil {
		if p.fixed.Available {
			return 1
		}
		return 0
	}
	return len(p.versions)
}

func ceilHour(t time.Time) time.Time {
	floor := t.Truncate(time.Hour)
	if floor.Equal(t) {
		return t
	}
	return floor.Add(time.Hour)
}
