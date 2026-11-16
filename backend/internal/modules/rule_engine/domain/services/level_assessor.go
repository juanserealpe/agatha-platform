// Package services contiene la lógica PURA del Rule Engine: sin BD, sin reloj
// y sin bus. Todo lo que necesita llega por parámetro.
package services

import "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"

// Assessment es la interpretación de una hora calculada.
type Assessment struct {
	RawLevel entities.Level
	// ComputedLevel es max(raw, persistencia), sin histéresis ni racha de
	// entrada (con la confirmación por planitud, NARANJA sin confirmar baja a AMARILLO).
	ComputedLevel entities.Level
	// Level es el nivel del episodio.
	Level entities.Level
	// PreviousLevel es el Level de la hora calculada anterior (NORMAL si no hay).
	PreviousLevel entities.Level
	Rule          entities.Rule
	InEpisode     bool
	HoursAbove    int
	HoursInLevel  int
	// RunAmarillo y RunNaranja: horas sobre cada corte con las que se evaluó la
	// persistencia (racha seguida en STRICT, conteo en la ventana en WINDOW).
	RunAmarillo int
	RunNaranja  int
	// BelowRun: horas seguidas bajo la salida de AMARILLO, incluida esta.
	BelowRun int
	// EpisodeEvent: OPENED en la hora que abre el episodio, CLEARED en la que lo cierra.
	EpisodeEvent entities.EpisodeEvent
	// Reasons explican ComputedLevel.
	Reasons []entities.Reason
}

// RawLevel es el nivel que marca el umbral: ROJO si I >= rojo; NARANJA si
// I >= naranja; AMARILLO si I >= amarillo; si no NORMAL.
func RawLevel(i float64, p entities.ThresholdParams) entities.Level {
	switch {
	case i >= p.Rojo:
		return entities.LevelRojo
	case i >= p.Naranja:
		return entities.LevelNaranja
	case i >= p.Amarillo:
		return entities.LevelAmarillo
	default:
		return entities.LevelNormal
	}
}

// Assess interpreta la hora actual (índice i y su d_pl, nil si no se
// calculó) con los parámetros de la versión vigente en ella y el historial de
// horas CALCULADAS anteriores, ordenado de la más reciente a la más antigua.
// Basta con p.HistoryDepth() filas. Las rachas usan el RawLevel GUARDADO de
// cada hora: un cambio de umbral no reinterpreta el pasado. Determinista: la
// misma entrada da siempre la misma salida.
func Assess(i float64, dPl *float64, p entities.ThresholdParams, history []entities.RuleEvaluation) Assessment {
	raw := RawLevel(i, p)

	a := Assessment{RawLevel: raw, PreviousLevel: entities.LevelNormal}

	var prev *entities.RuleEvaluation
	if len(history) > 0 {
		prev = &history[0]
		a.PreviousLevel = prev.Level
	}
	inEpisode := prev != nil && prev.InEpisode

	// La apertura siempre exige horas SEGUIDAS (min_hours); la persistencia
	// cuenta con el modo de la versión.
	entryRun := runAtLeast(raw, history, entities.LevelAmarillo)
	a.RunAmarillo, a.RunNaranja = entryRun, runAtLeast(raw, history, entities.LevelNaranja)
	if p.IsWindow() {
		a.RunAmarillo = countInWindow(raw, history, entities.LevelAmarillo, *p.PersistenceWindowHours)
		a.RunNaranja = countInWindow(raw, history, entities.LevelNaranja, *p.PersistenceWindowHours)
	}
	a.BelowRun = belowRun(i, history, p)

	calculated, rule, reasons := escalate(i, dPl, raw, a.RunAmarillo, a.RunNaranja, p)
	a.ComputedLevel, a.Reasons = calculated, reasons

	switch {
	// c) Fuera de episodio: abre con la racha de entrada cumplida.
	case !inEpisode && raw.AtLeast(entities.LevelAmarillo) && entryRun >= p.MinHours:
		a.Level, a.Rule, a.InEpisode, a.EpisodeEvent = calculated, rule, true, entities.EpisodeEventOpened
		// El episodio cuenta desde la primera hora de la racha de entrada.
		a.HoursAbove = entryRun

	case !inEpisode:
		a.Level, a.Rule, a.InEpisode, a.EpisodeEvent = entities.LevelNormal, entities.RuleNone, false, entities.EpisodeEventNone

	// d) Dentro de episodio y sobre el umbral: el nivel sube o baja.
	case raw.AtLeast(entities.LevelAmarillo):
		a.Level, a.Rule, a.InEpisode, a.EpisodeEvent = calculated, rule, true, entities.EpisodeEventNone
		a.HoursAbove = prev.HoursAbove + 1

	// d) raw NORMAL con la racha de salida cumplida: cierra (la hora de cierre
	// es la última del episodio).
	case a.BelowRun >= p.ExitHours:
		a.Level, a.Rule, a.InEpisode, a.EpisodeEvent = entities.LevelNormal, entities.RuleNone, false, entities.EpisodeEventCleared
		a.HoursAbove = prev.HoursAbove + 1

	// d) Histéresis: conserva el nivel anterior sin emitir.
	default:
		a.Level, a.Rule, a.InEpisode, a.EpisodeEvent = prev.Level, entities.RuleHold, true, entities.EpisodeEventNone
		a.HoursAbove = prev.HoursAbove + 1
	}

	a.HoursInLevel = 1
	if prev != nil && prev.Level == a.Level {
		a.HoursInLevel = prev.HoursInLevel + 1
	}

	return a
}

// escalate aplica la persistencia: como máximo un nivel por regla y nunca
// por debajo del raw. Solo escala desde un nivel que la hora actual cumple
// (raw >= from). Después aplica la confirmación por planitud: sin confirmar,
// un nivel >= NARANJA queda en AMARILLO; la excepción del ROJO por umbral
// (raw ROJO no se limita) solo rige si la versión no activa
// confirm_rojo_with_planitud. Las rachas siguen contando sobre el raw
// GUARDADO, así que una hora limitada no borra la duración acumulada: la
// primera hora con planitud confirmada recupera el nivel. Devuelve el
// nivel calculado, la regla que lo fijó y los motivos.
func escalate(i float64, dPl *float64, raw entities.Level, runAmarillo, runNaranja int, p entities.ThresholdParams,
) (entities.Level, entities.Rule, []entities.Reason) {
	candidate, run, from := entities.LevelNormal, 0, entities.LevelNormal
	naranjaAfter, amarilloAfter := p.EscalationHours(entities.LevelNaranja), p.EscalationHours(entities.LevelAmarillo)
	switch {
	case naranjaAfter != nil && raw.AtLeast(entities.LevelNaranja) && runNaranja >= *naranjaAfter:
		candidate, run, from = entities.LevelRojo, runNaranja, entities.LevelNaranja
	case amarilloAfter != nil && raw.AtLeast(entities.LevelAmarillo) && runAmarillo >= *amarilloAfter:
		candidate, run, from = entities.LevelNaranja, runAmarillo, entities.LevelAmarillo
	}

	var reasons []entities.Reason
	if raw.AtLeast(entities.LevelAmarillo) {
		reasons = append(reasons, entities.Reason{
			Code: entities.ReasonLeakThreshold, Level: raw, I: i, Threshold: p.CutFor(raw),
		})
	}

	calculated := entities.MaxLevel(raw, candidate)

	if limitedByPlanitud(calculated, raw, dPl, p) {
		reasons = append(reasons, entities.Reason{
			Code: entities.ReasonNaranjaNotConfirmed, Level: entities.LevelAmarillo, From: calculated,
			DPl: copyFloat(dPl), DPlMin: *p.DPlMin,
		})
		return entities.LevelAmarillo, entities.RuleThreshold, reasons
	}

	if calculated.Rank() > raw.Rank() {
		r := entities.Reason{Code: entities.ReasonLeakPersistence, Level: calculated, Hours: run, From: from}
		if p.IsWindow() {
			r.WindowHours = *p.PersistenceWindowHours
		}
		return calculated, entities.RulePersistence, append(reasons, r)
	}
	return calculated, entities.RuleThreshold, reasons
}

// limitedByPlanitud indica si la confirmación por planitud baja la hora a
// AMARILLO. Con confirm_rojo_with_planitud no hay excepción: cualquier nivel
// >= NARANJA (por valor o por duración) necesita planitud confirmada. Sin
// ella, un raw ROJO nunca se limita (comportamiento anterior).
func limitedByPlanitud(calculated, raw entities.Level, dPl *float64, p entities.ThresholdParams) bool {
	if !calculated.AtLeast(entities.LevelNaranja) {
		return false
	}
	if raw == entities.LevelRojo && !p.ConfirmRojoWithPlanitud {
		return false
	}
	return !p.IsNaranjaConfirmed(dPl)
}

// runAtLeast cuenta las horas consecutivas, terminando en la actual, con
// raw >= level (0 si la actual no lo cumple).
func runAtLeast(raw entities.Level, history []entities.RuleEvaluation, level entities.Level) int {
	if !raw.AtLeast(level) {
		return 0
	}
	n := 1
	for _, h := range history {
		if !h.RawLevel.AtLeast(level) {
			break
		}
		n++
	}
	return n
}

// countInWindow cuenta las horas con raw >= level entre la actual y las
// window − 1 horas calculadas anteriores (persistencia WINDOW). A diferencia
// de runAtLeast, una hora bajo el corte no reinicia el conteo.
func countInWindow(raw entities.Level, history []entities.RuleEvaluation, level entities.Level, window int) int {
	n := 0
	if raw.AtLeast(level) {
		n++
	}
	for k := 0; k < len(history) && k < window-1; k++ {
		if history[k].RawLevel.AtLeast(level) {
			n++
		}
	}
	return n
}

// belowRun cuenta las horas consecutivas, terminando en la actual, con
// I < amarillo − exit_margin (con los parámetros de la versión actual). Una
// hora anterior solo cuenta si además su raw GUARDADO es NORMAL: si se subió
// el umbral, una hora que fue AMARILLO con su versión no se reinterpreta como
// "baja". Con una sola versión es equivalente (I < corte < amarillo ⇒ NORMAL).
func belowRun(i float64, history []entities.RuleEvaluation, p entities.ThresholdParams) int {
	cutoff := p.ExitCutoff()
	if !(i < cutoff) {
		return 0
	}
	n := 1
	for _, h := range history {
		if h.RawLevel != entities.LevelNormal || !(h.IndicatorValue < cutoff) {
			break
		}
		n++
	}
	return n
}

func copyFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
