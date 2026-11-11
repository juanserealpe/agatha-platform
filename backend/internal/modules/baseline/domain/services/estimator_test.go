package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// Los escenarios usan 20 días de muestras cada 15 min con B2 ≈ 50 (45–55).

func estimator() RobustBaselineEstimator { return NewRobustBaselineEstimator(DefaultConfig()) }

func twentyDays() (time.Time, time.Time) { return at(0), at(24 * 20) }

func TestEstimate_StableHistoryConvergesInOnePass(t *testing.T) {
	from, to := twentyDays()

	got := estimator().EstimateExcluding(normalSamples(from, to), from, to, nil)

	if got.Status != EstimationEstimated || got.Confidence != entities.ConfidenceNormal || len(got.Flags) != 0 {
		t.Fatalf("status=%s confianza=%s flags=%v", got.Status, got.Confidence, got.Flags)
	}
	if got.Summary.Passes != 1 || !got.Summary.Converged || got.Summary.ExcludedRatio != 0 || len(got.Exclusion.Periods) != 0 {
		t.Fatalf("resumen = %+v, periodos = %v", got.Summary, got.Exclusion.Periods)
	}
	if got.Signal.MedianB2 != 50 || got.Signal.ThetaD != 54 {
		t.Fatalf("mediana B2 = %v, θD = %v; se esperaba 50 y 54", got.Signal.MedianB2, got.Signal.ThetaD)
	}
	if len(got.Components) != 7 || !HasCompleteStats(got.Components, 6) || !HasCompleteStats(got.Components, 24) {
		t.Fatalf("se esperaban las 7 combinaciones ventana×componente: %+v", got.Components)
	}
	if len(got.Passes) != 1 || got.Passes[0].Windows6h != 159 || got.Passes[0].Windows24h != 77 {
		t.Fatalf("reporte de pasadas inesperado: %+v", got.Passes)
	}
}

func TestEstimate_AnomalyIsExcludedAndTheBaselineRecovers(t *testing.T) {
	from, to := twentyDays()
	samples := withAnomaly(normalSamples(from, to), at(200), at(248)) // 2 días con B2 ≈ 500

	got := estimator().EstimateExcluding(samples, from, to, nil)

	if got.Status != EstimationEstimated || !got.Summary.Converged || got.Summary.Passes != 2 {
		t.Fatalf("se esperaba convergencia en 2 pasadas: %+v", got.Summary)
	}

	// Pasada 1: la anomalía contamina θD; pasada 2: ya excluida, θD vuelve a la señal normal.
	if got.Passes[0].ThetaD < 90 {
		t.Fatalf("la primera pasada debía estar contaminada por la anomalía: θD = %v", got.Passes[0].ThetaD)
	}
	if got.Signal.ThetaD != 54 || got.Signal.MedianB2 != 50 {
		t.Fatalf("la línea base final debía recuperar la señal normal: mediana %v, θD %v", got.Signal.MedianB2, got.Signal.ThetaD)
	}

	covered := entities.ExclusionSet{Periods: got.Exclusion.Periods}
	for h := 200; h < 248; h++ {
		if !covered.Excludes(at(h)) {
			t.Fatalf("la hora %d de la anomalía no quedó excluida: %v", h, got.Exclusion.Periods)
		}
	}
	if got.Summary.ExcludedRatio <= 0 || got.Summary.ExcludedRatio >= DefaultConfig().ExclusionCapRatio {
		t.Fatalf("ExcludedRatio = %v fuera de (0, 0,40)", got.Summary.ExcludedRatio)
	}

	// Al excluir 2 días quedan menos de 60 ventanas de 24 h: confianza baja.
	if got.Confidence != entities.ConfidenceLow {
		t.Fatalf("confianza = %s, se esperaba LOW", got.Confidence)
	}
}

func TestEstimate_NotConvergingWithinMaxPassesAsksForReview(t *testing.T) {
	from, to := twentyDays()
	samples := withAnomaly(normalSamples(from, to), at(200), at(248))
	e := estimator()
	e.maxPasses = 1

	got := e.EstimateExcluding(samples, from, to, nil)

	if got.Summary.Converged || got.Summary.Passes != 1 || !entities.HasFlag(got.Flags, entities.FlagBaselineReview) {
		t.Fatalf("sin pasadas suficientes se esperaba BASELINE_REVIEW sin convergencia: %+v %v", got.Summary, got.Flags)
	}
	if len(got.Exclusion.Periods) != 0 {
		t.Fatalf("debe devolverse la base de la última pasada aceptada (sin exclusión): %v", got.Exclusion.Periods)
	}
}

func TestEstimate_ExclusionCapStopsAndFlagsForReview(t *testing.T) {
	from, to := twentyDays()
	samples := normalSamples(from, to)
	for k := 0; k < 6; k++ { // 6 anomalías de 12 h separadas: ≈ 15 % de muestras elevadas
		samples = withAnomaly(samples, at(24*(2*k+1)), at(24*(2*k+1)+12))
	}

	got := estimator().EstimateExcluding(samples, from, to, nil)

	if !got.Summary.CapReached || !got.Exclusion.CapReached {
		t.Fatalf("se esperaba alcanzar el tope del 40 %%: %+v", got.Summary)
	}
	for _, f := range []entities.Flag{entities.FlagExclusionCapReached, entities.FlagBaselineReview} {
		if !entities.HasFlag(got.Flags, f) {
			t.Fatalf("falta el flag %s: %v", f, got.Flags)
		}
	}
	if got.Summary.ExcludedRatio > DefaultConfig().ExclusionCapRatio {
		t.Fatalf("ExcludedRatio %v supera el tope", got.Summary.ExcludedRatio)
	}
}

func TestEstimate_KnownEventsAreExcludedFromTheFirstPass(t *testing.T) {
	from, to := twentyDays()
	known := []entities.KnownEvent{{From: at(100), To: at(110)}}

	got := estimator().EstimateExcluding(normalSamples(from, to), from, to, known)

	if !entities.HasFlag(got.Flags, entities.FlagKnownEventsExcluded) {
		t.Fatalf("falta KNOWN_EVENTS_EXCLUDED: %v", got.Flags)
	}
	// [100, 110) con ±2 h de margen.
	if len(got.Exclusion.Periods) != 1 || !got.Exclusion.Periods[0].From.Equal(at(98)) || !got.Exclusion.Periods[0].To.Equal(at(112)) {
		t.Fatalf("periodos = %v, se esperaba [%v, %v)", got.Exclusion.Periods, at(98), at(112))
	}
	if got.Passes[0].ExcludedRatio == 0 {
		t.Fatal("el evento conocido debía excluirse ya en la primera pasada")
	}
}

func TestEstimate_KnownEventOutsideTheRangeAddsNoFlag(t *testing.T) {
	from, to := twentyDays()
	known := []entities.KnownEvent{{From: at(-100), To: at(-50)}}

	got := estimator().EstimateExcluding(normalSamples(from, to), from, to, known)

	if entities.HasFlag(got.Flags, entities.FlagKnownEventsExcluded) || len(got.Exclusion.Periods) != 0 {
		t.Fatalf("un evento fuera del rango no excluye nada: flags %v periodos %v", got.Flags, got.Exclusion.Periods)
	}
}

func TestEstimate_InsufficientHistory(t *testing.T) {
	from, to := twentyDays()

	got := estimator().EstimateExcluding(normalSamples(from, at(10)), from, to, nil) // 40 registros < 96

	if got.Status != EstimationInsufficientHistory || got.Flags == nil || len(got.Flags) != 0 {
		t.Fatalf("status=%s flags=%v", got.Status, got.Flags)
	}
	if len(got.Passes) != 1 || got.Passes[0].SignalOK {
		t.Fatalf("se esperaba una pasada con señal insuficiente: %+v", got.Passes)
	}
}

func TestEstimate_ShortHistoryHasNoExclusionHistory(t *testing.T) {
	from, to := at(0), at(72) // 3 días: 9 ventanas de 24 h < 20

	got := estimator().EstimateExcluding(normalSamples(from, to), from, to, nil)

	if got.Status != EstimationEstimated || got.Confidence != entities.ConfidenceLow ||
		!entities.HasFlag(got.Flags, entities.FlagNoExclusionHistory) || got.Summary.Converged {
		t.Fatalf("status=%s confianza=%s flags=%v resumen=%+v", got.Status, got.Confidence, got.Flags, got.Summary)
	}
	if HasCompleteStats(got.Components, 24) {
		t.Fatal("con 9 ventanas no debe haber stats de 24 h")
	}
}

func TestEstimate_ConstantSignalIsDegenerate(t *testing.T) {
	from, to := twentyDays()
	samples := normalSamples(from, to)
	for i := range samples {
		samples[i].B2 = 50
	}

	got := estimator().EstimateExcluding(samples, from, to, nil)

	for _, f := range []entities.Flag{entities.FlagSignalDegenerate, entities.FlagSignalLowVariability, entities.FlagNoExclusionHistory} {
		if !entities.HasFlag(got.Flags, f) {
			t.Fatalf("falta el flag %s: %v", f, got.Flags)
		}
	}
	if got.Signal.MinElevatedB2 != nil || got.Summary.Converged {
		t.Fatalf("una señal constante no tiene B2 elevado ni converge: %+v", got)
	}
}

func TestNewRobustBaselineEstimator_AtLeastOnePass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxPasses = 0

	if e := NewRobustBaselineEstimator(cfg); e.maxPasses != 1 {
		t.Fatalf("maxPasses = %d, se esperaba 1", e.maxPasses)
	}
}

func TestHourlyAnomalyScanner_MarksOnlyAnomalousHours(t *testing.T) {
	from, to := twentyDays()
	base := estimator().EstimateExcluding(normalSamples(from, to), from, to, nil)
	sig := base.Signal.ToFeatureSignal()
	stats24 := entities.FeatureStats(base.Components, 24)
	scanner := NewHourlyAnomalyScanner(DefaultConfig())

	if marks := scanner.Scan(normalSamples(from, to), sig, stats24, from, to); len(marks) != 0 {
		t.Fatalf("una historia estable no debe tener horas marcadas: %d", len(marks))
	}

	marks := scanner.Scan(withAnomaly(normalSamples(from, to), at(200), at(248)), sig, stats24, from, to)
	if len(marks) == 0 {
		t.Fatal("la anomalía debía marcar horas")
	}
	for _, m := range marks {
		if m.I < DefaultConfig().ExclusionICutoff {
			t.Fatalf("marca por debajo del corte: %+v", m)
		}
		// Una marca en T cubre [T−24 h, T): debe tocar la anomalía.
		if !m.T.After(at(200)) || m.T.After(at(248+24)) {
			t.Fatalf("marca fuera del alcance de la anomalía: %v", m.T)
		}
	}
}

func TestHourlyAnomalyScanner_SkipsWindowsBelowCoverage(t *testing.T) {
	from, to := twentyDays()
	base := estimator().EstimateExcluding(normalSamples(from, to), from, to, nil)
	scanner := NewHourlyAnomalyScanner(DefaultConfig())

	// Solo 1 de cada 2 muestras (50 % < 70 %) y con la anomalía: ninguna ventana es calculable.
	sparse := []features.Sample{}
	for i, s := range withAnomaly(normalSamples(from, to), at(200), at(248)) {
		if i%2 == 0 {
			sparse = append(sparse, s)
		}
	}

	if marks := scanner.Scan(sparse, base.Signal.ToFeatureSignal(), entities.FeatureStats(base.Components, 24), from, to); len(marks) != 0 {
		t.Fatalf("ventanas con cobertura < 70 %% no se marcan: %d marcas", len(marks))
	}
}

func TestComponentBaselineCalculator(t *testing.T) {
	from, to := twentyDays()
	samples := normalSamples(from, to)
	sig := estimator().EstimateExcluding(samples, from, to, nil).Signal.ToFeatureSignal()
	calc := NewComponentBaselineCalculator(DefaultConfig())

	all := calc.Calculate(samples, sig, entities.ExclusionSet{}, from, to)
	if all.Windows6h != 159 || all.Windows24h != 77 || len(all.Stats) != 7 {
		t.Fatalf("ventanas 6h=%d 24h=%d stats=%d", all.Windows6h, all.Windows24h, len(all.Stats))
	}
	for _, st := range all.Stats {
		want := all.Windows6h
		if st.WindowHours == 24 {
			want = all.Windows24h
		}
		if st.NWindows != want {
			t.Fatalf("%dh/%s usa %d ventanas, se esperaban %d", st.WindowHours, st.Component, st.NWindows, want)
		}
	}

	excluded := entities.ExclusionSet{Periods: []entities.ExcludedPeriod{{From: at(100), To: at(148)}}}
	fewer := calc.Calculate(samples, sig, excluded, from, to)
	if fewer.Windows6h >= all.Windows6h || fewer.Windows24h >= all.Windows24h {
		t.Fatalf("las ventanas que tocan un periodo excluido no se usan: 6h %d→%d, 24h %d→%d",
			all.Windows6h, fewer.Windows6h, all.Windows24h, fewer.Windows24h)
	}

	strict := ComponentBaselineCalculator{minWindows: 100}.Calculate(samples, sig, entities.ExclusionSet{}, from, to)
	if !HasCompleteStats(strict.Stats, 6) || HasCompleteStats(strict.Stats, 24) {
		t.Fatalf("con mínimo 100 solo alcanzan las de 6 h (159), no las de 24 h (77): %+v", strict.Stats)
	}
}
