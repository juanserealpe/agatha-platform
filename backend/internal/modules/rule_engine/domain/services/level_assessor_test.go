package services

import (
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

const (
	normal   = entities.LevelNormal
	amarillo = entities.LevelAmarillo
	naranja  = entities.LevelNaranja
	rojo     = entities.LevelRojo

	threshold   = entities.RuleThreshold
	persistence = entities.RulePersistence
	hold        = entities.RuleHold
	none        = entities.RuleNone

	evNone    = entities.EpisodeEventNone
	evOpened  = entities.EpisodeEventOpened
	evCleared = entities.EpisodeEventCleared
)

// openedAt2 es el evento de una hora de un bucle que arranca en hours_above 2:
// la 2.ª hora abre el episodio, el resto no emite.
func openedAt2(h int) entities.EpisodeEvent {
	if h == 2 {
		return evOpened
	}
	return evNone
}

func intPtr(v int) *int { return &v }

// step es una hora calculada y lo que se espera de ella.
type step struct {
	name      string
	i         float64
	level     entities.Level
	rule      entities.Rule
	event     entities.EpisodeEvent
	inEpisode bool
	// hoursAbove < 0 ⇒ no se verifica.
	hoursAbove int
}

// runSequence evalúa las horas en orden como lo hace el caso de uso: cada hora
// ve solo las K filas anteriores (HistoryDepth), de la más reciente a la más antigua.
func runSequence(t *testing.T, p entities.ThresholdParams, steps []step) []Assessment {
	t.Helper()

	var history []entities.RuleEvaluation
	out := make([]Assessment, 0, len(steps))

	for n, s := range steps {
		window := history
		if k := p.HistoryDepth(); len(window) > k {
			window = window[:k]
		}

		got := Assess(s.i, p, window)
		out = append(out, got)

		label := s.name
		if label == "" {
			label = "hora"
		}
		if got.Level != s.level || got.Rule != s.rule || got.EpisodeEvent != s.event || got.InEpisode != s.inEpisode {
			t.Errorf("#%d %s (I=%.2f): level=%s rule=%s event=%s in_episode=%v; se esperaba %s/%s/%s/%v",
				n+1, label, s.i, got.Level, got.Rule, got.EpisodeEvent, got.InEpisode, s.level, s.rule, s.event, s.inEpisode)
		}
		if s.hoursAbove >= 0 && got.HoursAbove != s.hoursAbove {
			t.Errorf("#%d %s (I=%.2f): hours_above=%d, se esperaba %d", n+1, label, s.i, got.HoursAbove, s.hoursAbove)
		}

		row := entities.RuleEvaluation{
			IndicatorValue: s.i,
			RawLevel:       got.RawLevel,
			ComputedLevel:  got.ComputedLevel,
			Level:          got.Level,
			Rule:           got.Rule,
			InEpisode:      got.InEpisode,
			HoursAbove:     got.HoursAbove,
			HoursInLevel:   got.HoursInLevel,
			EpisodeEvent:   got.EpisodeEvent,
			Reasons:        got.Reasons,
		}
		history = append([]entities.RuleEvaluation{row}, history...)
	}

	return out
}

func TestRawLevel(t *testing.T) {
	p := entities.DefaultThresholdParams()

	tests := []struct {
		i    float64
		want entities.Level
	}{
		{0, normal},
		{0.1799, normal},
		{0.18, amarillo}, // justo en el umbral
		{0.2499, amarillo},
		{0.25, naranja},
		{0.3499, naranja},
		{0.35, rojo},
		{1, rojo},
	}

	for _, tt := range tests {
		if got := RawLevel(tt.i, p); got != tt.want {
			t.Errorf("RawLevel(%.4f) = %s, se esperaba %s", tt.i, got, tt.want)
		}
	}
}

// Escenario completo de la especificación con los valores por defecto.
func TestAssess_FullScenario(t *testing.T) {
	steps := []step{
		{name: "base", i: 0.12, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{name: "base", i: 0.14, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{name: "racha de entrada pendiente", i: 0.19, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{name: "abre", i: 0.20, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
		{i: 0.21, level: amarillo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 3},
		{i: 0.27, level: naranja, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 4},
		{name: "baja dentro del episodio", i: 0.22, level: amarillo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 5},
	}

	// 12 h seguidas entre 0,18 y 0,24. La racha >= AMARILLO viene de la 0,19
	// (5 horas), así que la persistencia (12 h) llega en la 7.ª hora del bloque
	// y se mantiene hasta la 12.ª. Entre la 6.ª y la 7.ª hay un hueco
	// NO_CALCULABLE: no hay fila y la racha sigue (pausa, no reinicia).
	block := []float64{0.19, 0.23, 0.18, 0.24, 0.20, 0.21 /* hueco */, 0.22, 0.19, 0.23, 0.20, 0.24, 0.18}
	for k, i := range block {
		s := step{i: i, level: amarillo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 6 + k}
		if 5+k+1 >= 12 {
			s.name, s.level, s.rule = "persistencia", naranja, persistence
		}
		steps = append(steps, s)
	}

	steps = append(steps,
		step{name: "histéresis", i: 0.17, level: naranja, rule: hold, event: evNone, inEpisode: true, hoursAbove: 18},
		step{name: "salida 1/2", i: 0.15, level: naranja, rule: hold, event: evNone, inEpisode: true, hoursAbove: 19},
		step{name: "cierra", i: 0.14, level: normal, rule: none, event: evCleared, inEpisode: false, hoursAbove: 20},
		step{name: "nueva racha pendiente", i: 0.20, level: normal, rule: none, event: evNone, hoursAbove: 0},
		step{name: "episodio nuevo", i: 0.21, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
	)

	got := runSequence(t, entities.DefaultThresholdParams(), steps)

	// La hora 12 del bloque: NARANJA por persistencia.
	if h12 := got[7+11]; h12.Level != naranja || h12.Rule != persistence || h12.EpisodeEvent != evNone {
		t.Errorf("hora 12 del bloque = %+v", h12)
	}
	// Abrir y cerrar quedan marcados; el cierre informa el último nivel.
	if got[3].EpisodeEvent != evOpened || got[4].EpisodeEvent != evNone {
		t.Errorf("episode_event: apertura=%s siguiente=%s", got[3].EpisodeEvent, got[4].EpisodeEvent)
	}
	cleared := got[len(got)-3]
	if cleared.PreviousLevel != naranja {
		t.Errorf("el cierre debe recordar el último nivel: previous=%s", cleared.PreviousLevel)
	}
	if got[len(got)-1].EpisodeEvent != evOpened {
		t.Error("el episodio nuevo debe marcarse como abierto")
	}
	// hours_in_level: consecutivas con el mismo level.
	if got[0].HoursInLevel != 1 || got[1].HoursInLevel != 2 || got[3].HoursInLevel != 1 || got[4].HoursInLevel != 2 {
		t.Errorf("hours_in_level = %d,%d,_,%d,%d", got[0].HoursInLevel, got[1].HoursInLevel, got[3].HoursInLevel, got[4].HoursInLevel)
	}
}

// Leída literalmente: 12 h por encima de AMARILLO desde una racha nueva ⇒ la
// 12.ª es la primera NARANJA por persistencia.
func TestAssess_PersistenceAmarilloToNaranjaAtHour12(t *testing.T) {
	steps := []step{{i: 0.20, level: normal, rule: none, event: evNone, hoursAbove: 0}}
	for h := 2; h <= 11; h++ {
		steps = append(steps, step{i: 0.20, level: amarillo, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h})
	}
	steps = append(steps, step{i: 0.20, level: naranja, rule: persistence, event: evNone, inEpisode: true, hoursAbove: 12})

	runSequence(t, entities.DefaultThresholdParams(), steps)
}

func TestAssess_DirectJumpToRojo(t *testing.T) {
	t.Run("desde NORMAL respeta la racha de entrada", func(t *testing.T) {
		runSequence(t, entities.DefaultThresholdParams(), []step{
			{i: 0.10, level: normal, rule: none, event: evNone, hoursAbove: 0},
			{i: 0.40, level: normal, rule: none, event: evNone, hoursAbove: 0},
			{i: 0.41, level: rojo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
		})
	})

	t.Run("dentro de un episodio sube de inmediato", func(t *testing.T) {
		runSequence(t, entities.DefaultThresholdParams(), []step{
			{i: 0.19, level: normal, rule: none, event: evNone, hoursAbove: 0},
			{i: 0.19, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
			{i: 0.50, level: rojo, rule: threshold, event: evNone, inEpisode: true, hoursAbove: 3},
		})
	})
}

func TestAssess_PersistenceNaranjaToRojo(t *testing.T) {
	steps := []step{{i: 0.30, level: normal, rule: none, event: evNone, hoursAbove: 0}}
	for h := 2; h <= 5; h++ {
		steps = append(steps, step{i: 0.30, level: naranja, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h})
	}
	// 6.ª hora con raw >= NARANJA ⇒ ROJO por persistencia.
	steps = append(steps, step{i: 0.30, level: rojo, rule: persistence, event: evNone, inEpisode: true, hoursAbove: 6})

	runSequence(t, entities.DefaultThresholdParams(), steps)
}

func TestAssess_PersistenceNeverRaisesTwoLevels(t *testing.T) {
	p := entities.DefaultThresholdParams()
	p.EscalateAmarilloAfterHours = intPtr(2)
	p.EscalateNaranjaAfterHours = intPtr(1)

	// raw AMARILLO con racha suficiente: sube a NARANJA, nunca a ROJO (la
	// racha NARANJA se cuenta sobre el raw, que sigue en AMARILLO).
	steps := []step{{i: 0.19, level: normal, rule: none, event: evNone, hoursAbove: 0}}
	for h := 2; h <= 30; h++ {
		steps = append(steps, step{i: 0.19, level: naranja, rule: persistence, event: openedAt2(h), inEpisode: true, hoursAbove: h})
	}

	runSequence(t, p, steps)
}

func TestAssess_NullEscalationNeverPersists(t *testing.T) {
	p := entities.DefaultThresholdParams()
	p.EscalateAmarilloAfterHours = nil
	p.EscalateNaranjaAfterHours = nil

	steps := []step{{i: 0.30, level: normal, rule: none, event: evNone, hoursAbove: 0}}
	for h := 2; h <= 48; h++ {
		steps = append(steps, step{i: 0.30, level: naranja, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h})
	}

	runSequence(t, p, steps)
}

// La histéresis no cierra mientras I quede en la banda [amarillo − margen, amarillo).
func TestAssess_HysteresisBandHoldsForever(t *testing.T) {
	steps := []step{
		{i: 0.20, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{i: 0.20, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
	}
	for h := 3; h <= 10; h++ {
		steps = append(steps, step{i: 0.165, level: amarillo, rule: hold, event: evNone, inEpisode: true, hoursAbove: h})
	}

	runSequence(t, entities.DefaultThresholdParams(), steps)
}

// Una hora de vuelta en la banda reinicia la racha de salida.
func TestAssess_ExitRunMustBeConsecutive(t *testing.T) {
	runSequence(t, entities.DefaultThresholdParams(), []step{
		{i: 0.20, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{i: 0.20, level: amarillo, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
		{i: 0.10, level: amarillo, rule: hold, event: evNone, inEpisode: true, hoursAbove: 3},
		{i: 0.17, level: amarillo, rule: hold, event: evNone, inEpisode: true, hoursAbove: 4},
		{i: 0.10, level: amarillo, rule: hold, event: evNone, inEpisode: true, hoursAbove: 5},
		{i: 0.10, level: normal, rule: none, event: evCleared, inEpisode: false, hoursAbove: 6},
	})
}

// Un cambio de umbral no reinterpreta el pasado: una hora que fue AMARILLO con
// su versión no cuenta para la racha de salida aunque su I quede bajo la banda
// de la versión nueva.
func TestAssess_ExitRunUsesStoredRawLevel(t *testing.T) {
	strict := entities.DefaultThresholdParams()
	strict.Amarillo, strict.Naranja, strict.Rojo = 0.30, 0.40, 0.50

	history := []entities.RuleEvaluation{ // más reciente primero
		{IndicatorValue: 0.20, RawLevel: amarillo, Level: amarillo, InEpisode: true, HoursAbove: 2, HoursInLevel: 1},
		{IndicatorValue: 0.20, RawLevel: amarillo, Level: normal},
	}

	got := Assess(0.20, strict, history)
	if got.Level != amarillo || got.Rule != hold || got.EpisodeEvent != evNone || !got.InEpisode {
		t.Fatalf("got = %+v; se esperaba HOLD en AMARILLO (la racha de salida es 1 < 2)", got)
	}
}

// computed_level es max(raw, persistencia), SIN la histéresis ni la racha de
// entrada: es lo que Alert Management usa para subir y estabilizar.
func TestAssess_ComputedLevelIgnoresHoldAndPendingEntry(t *testing.T) {
	got := runSequence(t, entities.DefaultThresholdParams(), []step{
		{name: "racha de entrada pendiente", i: 0.20, level: normal, rule: none, event: evNone, hoursAbove: 0},
		{name: "abre", i: 0.27, level: naranja, rule: threshold, event: evOpened, inEpisode: true, hoursAbove: 2},
		{name: "histéresis", i: 0.17, level: naranja, rule: hold, event: evNone, inEpisode: true, hoursAbove: 3},
		{name: "salida 1/2", i: 0.10, level: naranja, rule: hold, event: evNone, inEpisode: true, hoursAbove: 4},
		{name: "cierra", i: 0.10, level: normal, rule: none, event: evCleared, hoursAbove: 5},
	})

	want := []entities.Level{amarillo, naranja, normal, normal, normal}
	for n, a := range got {
		if a.ComputedLevel != want[n] {
			t.Errorf("hora %d: computed_level = %s, se esperaba %s", n, a.ComputedLevel, want[n])
		}
	}
}

func TestAssess_Reasons(t *testing.T) {
	p := entities.DefaultThresholdParams()

	t.Run("NORMAL no tiene motivos", func(t *testing.T) {
		if got := Assess(0.10, p, nil); len(got.Reasons) != 0 {
			t.Fatalf("reasons = %+v", got.Reasons)
		}
	})

	t.Run("umbral: nivel raw con I y el corte de ese nivel", func(t *testing.T) {
		got := Assess(0.27, p, nil)
		want := []entities.Reason{{Code: entities.ReasonLeakThreshold, Level: naranja, I: 0.27, Threshold: 0.25}}
		if !equalReasons(got.Reasons, want) {
			t.Fatalf("reasons = %+v, se esperaba %+v", got.Reasons, want)
		}
	})

	t.Run("persistencia AMARILLO → NARANJA a las 12 h", func(t *testing.T) {
		var steps []step
		steps = append(steps, step{i: 0.22, level: normal, rule: none, event: evNone, hoursAbove: 0})
		for h := 2; h <= 11; h++ {
			steps = append(steps, step{i: 0.22, level: amarillo, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h})
		}
		steps = append(steps, step{i: 0.22, level: naranja, rule: persistence, event: evNone, inEpisode: true, hoursAbove: 12})

		got := runSequence(t, p, steps)
		last := got[len(got)-1]
		want := []entities.Reason{
			{Code: entities.ReasonLeakThreshold, Level: amarillo, I: 0.22, Threshold: 0.18},
			{Code: entities.ReasonLeakPersistence, Level: naranja, Hours: 12, From: amarillo},
		}
		if last.ComputedLevel != naranja || !equalReasons(last.Reasons, want) {
			t.Fatalf("computed = %s, reasons = %+v; se esperaba NARANJA y %+v", last.ComputedLevel, last.Reasons, want)
		}
	})

	t.Run("persistencia NARANJA → ROJO a las 6 h", func(t *testing.T) {
		steps := []step{{i: 0.30, level: normal, rule: none, event: evNone, hoursAbove: 0}}
		for h := 2; h <= 5; h++ {
			steps = append(steps, step{i: 0.30, level: naranja, rule: threshold, event: openedAt2(h), inEpisode: true, hoursAbove: h})
		}
		steps = append(steps, step{i: 0.30, level: rojo, rule: persistence, event: evNone, inEpisode: true, hoursAbove: 6})

		got := runSequence(t, p, steps)
		want := []entities.Reason{
			{Code: entities.ReasonLeakThreshold, Level: naranja, I: 0.30, Threshold: 0.25},
			{Code: entities.ReasonLeakPersistence, Level: rojo, Hours: 6, From: naranja},
		}
		if last := got[len(got)-1]; !equalReasons(last.Reasons, want) {
			t.Fatalf("reasons = %+v, se esperaba %+v", last.Reasons, want)
		}
	})
}

func equalReasons(a, b []entities.Reason) bool {
	if len(a) != len(b) {
		return false
	}
	for n := range a {
		if a[n] != b[n] {
			return false
		}
	}
	return true
}
