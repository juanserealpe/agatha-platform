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

// La confirmación por planitud se retiró (nov 2026): el nivel calculado nunca
// queda por debajo del raw. Un NARANJA por valor, aun con una planitud que
// antes lo habría frenado, es NARANJA; un ROJO por valor, ROJO.
func TestAssess_NoPlanitudRestriction(t *testing.T) {
	p := paramsV2()

	got := runSequence(t, p, []step{
		{i: 0.17, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{i: 0.20, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
		{name: "pico NARANJA", i: 0.2504, level: naranja, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 3},
		{name: "ROJO por valor", i: 0.36, level: rojo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 4},
	})

	for n, a := range got[1:] {
		if a.ComputedLevel != a.RawLevel {
			t.Fatalf("hora %d: raw=%s computed=%s; computed nunca queda bajo el raw", n+2, a.RawLevel, a.ComputedLevel)
		}
		for _, r := range a.Reasons {
			if r.Code != entities.ReasonLeakThreshold {
				t.Fatalf("hora %d: solo se esperaba LEAK_THRESHOLD: %+v", n+2, a.Reasons)
			}
		}
	}
}

// La persistencia escala sin frenos: 12 h seguidas en AMARILLO dan NARANJA.
func TestAssess_PersistenceEscalatesWithoutCap(t *testing.T) {
	p := paramsV2()

	steps := make([]step, 0, 12)
	for h := 1; h <= 12; h++ {
		s := step{i: 0.20, level: amarillo, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h}
		if h == 1 {
			s.level, s.rule, s.inEpisode, s.hoursAbove = normal, none, false, 0
		}
		steps = append(steps, s)
	}
	steps[11].level, steps[11].rule = naranja, persistence

	got := runSequence(t, p, steps)
	if r, ok := hasReason(got[11].Reasons, entities.ReasonLeakPersistence); !ok || r.Level != naranja || r.From != amarillo {
		t.Fatalf("se esperaba LEAK_PERSISTENCE a NARANJA desde AMARILLO: %+v", got[11].Reasons)
	}
}

// Caso 9: con 8 h de racha, bajar la persistencia de 12 a 6 escala en la hora siguiente.
func TestAssess_PersistenceChangeUsesAccumulatedRun(t *testing.T) {
	p12 := paramsV2()
	var history []entities.RuleEvaluation
	for h := 0; h < 8; h++ {
		a := Assess(0.20, p12, history)
		history = append([]entities.RuleEvaluation{{
			IndicatorValue: 0.20, RawLevel: a.RawLevel, Level: a.Level, InEpisode: a.InEpisode,
			HoursAbove: a.HoursAbove, HoursInLevel: a.HoursInLevel,
		}}, history...)
	}

	p6 := p12
	p6.EscalateAmarilloAfterHours = intPtr(6)
	got := Assess(0.20, p6, history)
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
			a := Assess(i, p, window)
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
	got := Assess(0.15, p, history)
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
	if got := Assess(0.12, p, history); got.BelowRun != 2 || got.EpisodeEvent != evCleared {
		t.Fatalf("below_run=%d event=%s; se esperaba 2 y CLEARED", got.BelowRun, got.EpisodeEvent)
	}
	if got := Assess(0.15, p, history); got.BelowRun != 0 {
		t.Fatalf("I sobre la salida: below_run=%d, se esperaba 0", got.BelowRun)
	}
}
