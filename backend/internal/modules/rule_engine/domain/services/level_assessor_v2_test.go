package services

import (
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

func floatPtr(v float64) *float64 { return &v }

// paramsV2 es la versión 2 de la especificación: amarillo 0,16 (salida 0,14).
func paramsV2() entities.ThresholdParams {
	p := referenceParams()
	p.Amarillo = 0.16
	return p
}

func withPlanitud(p entities.ThresholdParams, min float64) entities.ThresholdParams {
	p.ConfirmNaranjaWithPlanitud = true
	p.DPlMin = floatPtr(min)
	return p
}

func withWindow(p entities.ThresholdParams, window, min int) entities.ThresholdParams {
	p.PersistenceMode = entities.PersistenceWindow
	p.PersistenceWindowHours = intPtr(window)
	p.PersistenceMinHours = intPtr(min)
	return p
}

func hasReason(reasons []entities.Reason, code entities.ReasonCode) (entities.Reason, bool) {
	for _, r := range reasons {
		if r.Code == code {
			return r, true
		}
	}
	return entities.Reason{}, false
}

// Caso 7: un R7 sintético (pico 0,2504 con d_pl 8 %) con planitud activa nunca llega a NARANJA.
func TestAssess_PlanitudBlocksUnconfirmedNaranja(t *testing.T) {
	p := withPlanitud(paramsV2(), 12)
	low := floatPtr(8)

	got := runSequence(t, p, []step{
		{i: 0.17, level: normal, rule: none, event: evNone, hoursAbove: 0, dPl: low},
		{i: 0.20, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2, dPl: low},
		{name: "pico sin planitud", i: 0.2504, level: amarillo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 3, dPl: low},
		{name: "sin d_pl", i: 0.26, level: amarillo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 4},
	})

	for _, n := range []int{2, 3} {
		a := got[n]
		if a.RawLevel != naranja || a.ComputedLevel != amarillo {
			t.Fatalf("hora %d: raw=%s computed=%s; se esperaba raw NARANJA y computed AMARILLO", n+1, a.RawLevel, a.ComputedLevel)
		}
		r, ok := hasReason(a.Reasons, entities.ReasonNaranjaNotConfirmed)
		if !ok || r.Level != amarillo || r.From != naranja || r.DPlMin != 12 {
			t.Fatalf("hora %d: motivo NARANJA_NOT_CONFIRMED ausente o incompleto: %+v", n+1, a.Reasons)
		}
	}
	if r, _ := hasReason(got[2].Reasons, entities.ReasonNaranjaNotConfirmed); r.DPl == nil || *r.DPl != 8 {
		t.Fatalf("el motivo debe llevar el d_pl de la hora: %+v", r)
	}
	if r, _ := hasReason(got[3].Reasons, entities.ReasonNaranjaNotConfirmed); r.DPl != nil {
		t.Fatalf("sin d_pl el motivo lleva d_pl nil: %+v", r)
	}
}

// Caso 8: con d_pl >= d_pl_min, NARANJA llega igual (por umbral y por persistencia).
func TestAssess_PlanitudConfirmsNaranja(t *testing.T) {
	p := withPlanitud(paramsV2(), 12)

	got := runSequence(t, p, []step{
		{i: 0.20, level: normal, rule: none, event: evNone, hoursAbove: 0, dPl: floatPtr(15)},
		{i: 0.20, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2, dPl: floatPtr(15)},
		{i: 0.26, level: naranja, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 3, dPl: floatPtr(12)},
	})
	if _, ok := hasReason(got[2].Reasons, entities.ReasonNaranjaNotConfirmed); ok {
		t.Fatalf("d_pl = d_pl_min confirma: %+v", got[2].Reasons)
	}
}

// La planitud también frena la persistencia (AMARILLO → NARANJA y NARANJA → ROJO),
// pero no un ROJO por umbral.
func TestAssess_PlanitudCapsPersistenceButNotRojoThreshold(t *testing.T) {
	p := withPlanitud(paramsV2(), 12)
	low := floatPtr(5)

	steps := make([]step, 0, 14)
	for h := 1; h <= 12; h++ {
		s := step{i: 0.20, level: amarillo, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h, dPl: low}
		if h == 1 {
			s.level, s.rule, s.inEpisode, s.hoursAbove = normal, none, false, 0
		}
		steps = append(steps, s)
	}
	steps = append(steps,
		step{name: "rojo por umbral", i: 0.36, level: rojo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 13, dPl: low},
	)

	got := runSequence(t, p, steps)
	if r, ok := hasReason(got[11].Reasons, entities.ReasonNaranjaNotConfirmed); !ok || r.From != naranja {
		t.Fatalf("la persistencia a NARANJA sin confirmar debe dar NARANJA_NOT_CONFIRMED: %+v", got[11].Reasons)
	}
	if _, ok := hasReason(got[11].Reasons, entities.ReasonLeakPersistence); ok {
		t.Fatalf("una persistencia no confirmada no lleva LEAK_PERSISTENCE: %+v", got[11].Reasons)
	}
}

// Caso 9: con 8 h de racha, bajar la persistencia de 12 a 6 escala en la hora siguiente.
func TestAssess_PersistenceChangeUsesAccumulatedRun(t *testing.T) {
	p12 := paramsV2()
	var history []entities.RuleEvaluation
	for h := 0; h < 8; h++ {
		a := Assess(0.20, nil, p12, history)
		history = append([]entities.RuleEvaluation{{
			IndicatorValue: 0.20, RawLevel: a.RawLevel, Level: a.Level, InEpisode: a.InEpisode,
			HoursAbove: a.HoursAbove, HoursInLevel: a.HoursInLevel,
		}}, history...)
	}

	p6 := p12
	p6.EscalateAmarilloAfterHours = intPtr(6)
	got := Assess(0.20, nil, p6, history)
	if got.Level != naranja || got.Rule != persistence || got.RunAmarillo != 9 {
		t.Fatalf("level=%s rule=%s run=%d; se esperaba NARANJA/PERSISTENCE con racha 9", got.Level, got.Rule, got.RunAmarillo)
	}
}

// Caso 10: I oscila 0,17/0,17/0,17/0,15 (amarillo 0,16, salida 0,14). En
// STRICT la racha nunca pasa de 3 y no escala; en WINDOW 18 de 24 sube a
// NARANJA en la hora 23, la que completa 18 horas sobre el corte en la ventana.
// (El patrón 0,17/0,17/0,15 del documento tiene 2 de cada 3 horas sobre el
// corte: como máximo 16 de 24, nunca llega a 18.)
func TestAssess_WindowPersistenceEscalatesOscillation(t *testing.T) {
	pattern := []float64{0.17, 0.17, 0.17, 0.15}
	series := make([]float64, 30)
	for n := range series {
		series[n] = pattern[n%len(pattern)]
	}

	firstNaranja := func(p entities.ThresholdParams) (int, Assessment) {
		var history []entities.RuleEvaluation
		for n, i := range series {
			window := history
			if k := p.HistoryDepth(); len(window) > k {
				window = window[:k]
			}
			a := Assess(i, nil, p, window)
			if a.Level == naranja {
				return n + 1, a
			}
			history = append([]entities.RuleEvaluation{{
				IndicatorValue: i, RawLevel: a.RawLevel, Level: a.Level, InEpisode: a.InEpisode,
				HoursAbove: a.HoursAbove, HoursInLevel: a.HoursInLevel,
			}}, history...)
		}
		return 0, Assessment{}
	}

	if hour, _ := firstNaranja(paramsV2()); hour != 0 {
		t.Fatalf("STRICT no debe escalar la oscilación; escaló en la hora %d", hour)
	}

	p := withWindow(paramsV2(), 24, 18)
	if p.HistoryDepth() != 25 {
		t.Fatalf("HistoryDepth con ventana 24 = %d, se esperaba 25", p.HistoryDepth())
	}
	hour, a := firstNaranja(p)
	if hour != 23 || a.Rule != persistence || a.RunAmarillo != 18 {
		t.Fatalf("WINDOW: primera NARANJA en la hora %d (rule %s, conteo %d); se esperaba la 23 por PERSISTENCE con 18", hour, a.Rule, a.RunAmarillo)
	}
	r, ok := hasReason(a.Reasons, entities.ReasonLeakPersistence)
	if !ok || r.Hours != 18 || r.WindowHours != 24 || r.From != amarillo {
		t.Fatalf("motivo de persistencia por ventana incompleto: %+v", a.Reasons)
	}
}

// En WINDOW una hora bajo el corte no reinicia el conteo, pero tampoco escala.
func TestAssess_WindowDoesNotEscalateBelowCut(t *testing.T) {
	p := withWindow(paramsV2(), 4, 2)
	history := []entities.RuleEvaluation{
		{IndicatorValue: 0.17, RawLevel: amarillo, Level: amarillo, InEpisode: true, HoursAbove: 3},
		{IndicatorValue: 0.17, RawLevel: amarillo, Level: amarillo, InEpisode: true, HoursAbove: 2},
	}
	got := Assess(0.15, nil, p, history)
	if got.RunAmarillo != 2 || got.ComputedLevel != normal || got.Level != amarillo || got.Rule != hold {
		t.Fatalf("run=%d computed=%s level=%s rule=%s; se esperaba 2/NORMAL/AMARILLO/HOLD", got.RunAmarillo, got.ComputedLevel, got.Level, got.Rule)
	}
}

func TestAssess_BelowRunIsReported(t *testing.T) {
	p := paramsV2()
	history := []entities.RuleEvaluation{
		{IndicatorValue: 0.10, RawLevel: normal, Level: amarillo, InEpisode: true, HoursAbove: 5},
		{IndicatorValue: 0.20, RawLevel: amarillo, Level: amarillo, InEpisode: true, HoursAbove: 4},
	}
	if got := Assess(0.12, nil, p, history); got.BelowRun != 2 || got.EpisodeEvent != evCleared {
		t.Fatalf("below_run=%d event=%s; se esperaba 2 y CLEARED", got.BelowRun, got.EpisodeEvent)
	}
	if got := Assess(0.15, nil, p, history); got.BelowRun != 0 {
		t.Fatalf("I sobre la salida: below_run=%d, se esperaba 0", got.BelowRun)
	}
}
