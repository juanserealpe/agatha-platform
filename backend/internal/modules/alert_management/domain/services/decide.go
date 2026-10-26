// Package services contiene la lógica PURA de Alert Management: Decide
// (tabla de transiciones del ciclo de la alerta + reglas de mensaje en orden).
// Sin IO ni estado en memoria: todo lo que recuerda viaja en la alerta
// (runtime) y en el estado de procesamiento.
package services

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

// ErrNoticeMismatch: la alerta activa no es del punto/evento de la hora (error de programación).
var ErrNoticeMismatch = errors.New("alert_management: la alerta activa no corresponde al punto o evento de la hora")

// DecideInput es todo lo que Decide necesita para una hora.
type DecideInput struct {
	State entities.ProcessingState
	// Active es la alerta ACTIVA del (punto, evento), nil si no hay.
	Active *entities.Alert
	// Segment es el color vigente de Active.
	Segment *entities.Segment
	// Last es la última alerta finalizada (State.LastAlertID): candidata a reactivación.
	Last   *entities.Alert
	Notice entities.LevelNotice
	Policy entities.Policy
	Device entities.Device
}

// Outcome es lo que pasó con la hora.
type Outcome string

const (
	OutcomeNoAlert     Outcome = "NO_ALERT"
	OutcomeOpened      Outcome = "OPENED"
	OutcomeReactivated Outcome = "REACTIVATED"
	OutcomeUpdated     Outcome = "UPDATED"
	OutcomeFinished    Outcome = "FINISHED"
)

// Decision es el resultado de Decide. El caso de uso lo persiste en una
// transacción: alerta, cierre/apertura de color, mensajes y estado.
type Decision struct {
	State entities.ProcessingState
	// Alert es la alerta tocada en la hora (nil = ninguna). IsNew = se crea
	// (sin ID todavía); si no, se actualiza (también al reactivarse).
	Alert *entities.Alert
	IsNew bool
	// CloseSegmentAt cierra el color vigente; NewSegment abre otro (el caso de
	// uso asigna AlertID y Seq).
	CloseSegmentAt *time.Time
	NewSegment     *entities.Segment
	// Messages van al color nuevo si NewSegment != nil; si no, al vigente.
	Messages []entities.Message
	Outcome  Outcome
}

// Decide aplica una hora del Rule Engine. El caso de uso garantiza el orden y
// filtra duplicados (evaluated_at <= State.LastEvaluatedAt).
func Decide(in DecideInput) (Decision, error) {
	n := in.Notice
	if in.Active != nil && (in.Active.MeasuringPointID != n.MeasuringPointID || in.Active.EventType != n.EventType) {
		return Decision{}, ErrNoticeMismatch
	}

	at := n.EvaluatedAt.UTC()
	d := Decision{State: in.State, Outcome: OutcomeNoAlert}
	firstHour := in.State.LastEvaluatedAt == nil
	d.State.MeasuringPointID, d.State.EventType = n.MeasuringPointID, n.EventType
	d.State.LastEvaluatedAt = &at

	b := builder{in: in, at: at}

	if !n.Calculated() {
		d.State.LastStatus = entities.StatusNoCalculable
		if in.Active != nil {
			b.noCalc(&d)
		}
		return d, b.err
	}
	d.State.LastStatus = entities.StatusCalculated

	if in.Active == nil {
		b.withoutAlert(&d, firstHour)
	} else {
		b.withAlert(&d)
	}
	return d, b.err
}

// builder arma los mensajes de una hora y acumula el primer error de render.
type builder struct {
	in  DecideInput
	at  time.Time
	err error
}

// message renderiza la clave. kind vacío ⇒ el del catálogo.
func (b *builder) message(alert *entities.Alert, key string, params map[string]string, kind entities.MessageKind, level entities.Level, withValue bool) entities.Message {
	text, err := messages.Render(key, params)
	if err != nil && b.err == nil {
		b.err = err
	}
	if b.in.Notice.LowConfidence && text.Kind != entities.KindManagement {
		text = messages.WithLowConfidence(text)
	}
	if kind == "" {
		kind = text.Kind
	}
	m := entities.Message{
		EvaluatedAt:     b.at,
		Kind:            kind,
		Code:            text.Code,
		Level:           level,
		Title:           text.Title,
		Body:            text.Body,
		Params:          params,
		CatalogVersion:  messages.MessageCatalogVersion,
		PolicyVersionID: b.in.Policy.ID,
	}
	if withValue {
		v := b.in.Notice.Value()
		m.IndicatorValue = &v
	}
	at := b.at
	alert.Runtime.LastMessageAt = &at
	return m
}

func (b *builder) noCalc(d *Decision) {
	a := cloneAlert(*b.in.Active)
	a.HoursNoData++
	if !a.Runtime.InNoCalc {
		a.Runtime.InNoCalc = true
		d.Messages = append(d.Messages, b.message(&a, messages.Key(messages.CodeNoCalcStart, ""), map[string]string{
			"hora": messages.FormatHour(b.at), "nivel": messages.LevelName(a.CurrentLevel),
		}, "", a.CurrentLevel, false))
	}
	d.Alert = &a
	d.Outcome = OutcomeUpdated
}

// withoutAlert: sigue la racha pendiente (fecha inicio) y abre o reactiva con
// el OPENED del Rule Engine. Sin estado previo y en mitad de un episodio
// (módulo habilitado tarde), abre igual.
func (b *builder) withoutAlert(d *Decision, firstHour bool) {
	n := b.in.Notice
	pending := d.State.PendingRunStartedAt
	if n.RawLevel.AlertLevel() {
		if pending == nil {
			at := b.at
			d.State.PendingRunStartedAt = &at
		}
	} else {
		d.State.PendingRunStartedAt = nil
	}

	opens := n.EpisodeEvent == entities.EpisodeOpened || (firstHour && n.InEpisode && n.EpisodeEvent != entities.EpisodeCleared)
	if !opens {
		return
	}

	started := b.runStart(pending)
	d.State.PendingRunStartedAt = nil
	level := entities.MaxLevel(n.ComputedLevel, entities.LevelAmarillo)
	p := b.in.Policy

	last := b.in.Last
	withinWindow := last != nil && last.EndedAt != nil && last.State == entities.AlertFinished &&
		b.at.Sub(*last.EndedAt) < time.Duration(p.MergeWindowHours)*time.Hour
	if withinWindow && (!last.Management.Classified() || !p.ReactivateOnlyIfUnclassified) {
		b.reactivate(d, *last, level, started)
		return
	}

	a := entities.Alert{
		MeasuringPointID:         n.MeasuringPointID,
		EventType:                n.EventType,
		State:                    entities.AlertActive,
		StartedAt:                started,
		OpenedAt:                 b.at,
		CurrentLevel:             level,
		MaxLevel:                 level,
		OpeningIndicatorValue:    n.Value(),
		LastIndicatorValue:       n.Value(),
		LastEvaluatedAt:          b.at,
		HoursCalculated:          max(n.HoursAbove, 1),
		LowConfidenceAtOpen:      n.LowConfidence,
		PolicyVersionID:          p.ID,
		ThresholdVersionIDAtOpen: n.ThresholdVersionID,
		OpenedByRuleEvaluationID: n.RuleEvaluationID,
		Management:               entities.Management{Status: entities.StatusUnclassified},
		Runtime: entities.Runtime{
			PrevZone: entities.ZoneAbove,
			Recent:   []entities.HourValue{{At: b.at, I: n.Value()}},
		},
	}

	if withinWindow {
		d.Messages = append(d.Messages, b.message(&a, messages.Key(messages.CodeNewAfterClassified, ""), map[string]string{
			"pk": b.in.Device.PK, "codigo_anterior": last.Code,
			"clasificacion": messages.ClassificationName(last.Management.Classification),
		}, "", level, true))
	} else {
		d.Messages = append(d.Messages, b.message(&a, messages.Key(messages.CodeOpen, string(level)), map[string]string{
			"pk": b.in.Device.PK, "hora_inicio": messages.FormatHour(started), "zona": b.in.Device.Zone,
		}, "", level, true))
	}

	d.Alert = &a
	d.IsNew = true
	d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentOpen, Level: level, StartedAt: started}
	d.Outcome = OutcomeOpened
}

// runStart es la fecha inicio: la primera hora de la racha si se vio antes de
// esta hora; si no, la que se deduce de hours_above.
func (b *builder) runStart(pending *time.Time) time.Time {
	if pending != nil && pending.Before(b.at) {
		return *pending
	}
	return b.at.Add(-time.Duration(max(b.in.Notice.HoursAbove, 1)-1) * time.Hour)
}

// reactivate: misma alerta, vuelve a ACTIVA con un color nuevo con el nivel
// que tenía al finalizar (o el nuevo, si es mayor). Notifica solo si supera su máximo.
func (b *builder) reactivate(d *Decision, last entities.Alert, computed entities.Level, started time.Time) {
	n := b.in.Notice
	a := cloneAlert(last)
	hoursSinceEnd := int(b.at.Sub(*a.EndedAt).Hours())
	previousMax := a.MaxLevel
	level := entities.MaxLevel(a.CurrentLevel, computed)

	at := b.at
	a.State = entities.AlertActive
	a.EndedAt = nil
	a.ReactivationCount++
	a.LastReactivatedAt = &at
	a.CurrentLevel = level
	a.MaxLevel = entities.MaxLevel(previousMax, level)
	a.LastIndicatorValue = n.Value()
	a.LastEvaluatedAt = b.at
	a.HoursCalculated += max(n.HoursAbove, 1)
	a.BelowLevelHours = 0
	a.Runtime = entities.Runtime{
		PrevZone: entities.ZoneAbove,
		Recent:   []entities.HourValue{{At: b.at, I: n.Value()}},
	}

	kind := entities.KindUpdate
	if level.Rank() > previousMax.Rank() {
		kind = entities.KindNotification
	}
	d.Messages = append(d.Messages, b.message(&a, messages.Key(messages.CodeReactivated, ""), map[string]string{
		"horas": strconv.Itoa(hoursSinceEnd), "codigo": a.Code, "nivel": messages.LevelName(level),
	}, kind, level, true))

	d.Alert = &a
	d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentReactivation, Level: level, StartedAt: started}
	d.State.LastAlertID = nil
	d.Outcome = OutcomeReactivated
}

// withAlert: una hora calculada con la alerta activa.
func (b *builder) withAlert(d *Decision) {
	n := b.in.Notice
	p := b.in.Policy
	a := cloneAlert(*b.in.Active)
	v := n.Value()

	a.LastIndicatorValue = v
	a.LastEvaluatedAt = b.at
	a.HoursCalculated++
	d.State.PendingRunStartedAt = nil
	wasNoCalc := a.Runtime.InNoCalc
	a.Runtime.InNoCalc = false
	d.Alert = &a
	d.Outcome = OutcomeUpdated

	if n.EpisodeEvent == entities.EpisodeCleared {
		at := b.at
		a.State = entities.AlertFinished
		a.EndedAt = &at
		d.CloseSegmentAt = &at
		d.Messages = append(d.Messages, b.message(&a, messages.Key(messages.CodeEndNormal, ""), map[string]string{
			"codigo": a.Code, "N": strconv.Itoa(a.HoursCalculated), "max": messages.LevelName(a.MaxLevel),
			"horas": strconv.Itoa(p.MergeWindowHours),
		}, "", a.CurrentLevel, true))
		id := a.ID
		d.State.LastAlertID = &id
		d.Outcome = OutcomeFinished
		return
	}

	zone, prevZone := b.track(&a)

	// Regla 1 (siempre se emite): subida de nivel.
	primary := false
	current, computed := a.CurrentLevel, n.ComputedLevel
	switch {
	case computed.Rank() > current.Rank():
		b.up(d, &a, computed)
		primary = true
	case computed.Rank() < current.Rank():
		a.BelowLevelHours++
	default:
		a.BelowLevelHours = 0
	}

	// Reglas 2 a 9: gana la primera que aplica. Tras una subida, solo 2 a 4.
	if wasNoCalc {
		d.Messages = append(d.Messages, b.message(&a, messages.Key(messages.CodeNoCalcEnd, ""), map[string]string{
			"nivel": messages.LevelName(a.CurrentLevel),
		}, "", a.CurrentLevel, true))
		return
	}
	if b.downgrade(d, &a) {
		return
	}
	if b.oscillation(d, &a) {
		return
	}
	if primary {
		return
	}
	if !a.Runtime.Oscillating {
		if b.band(d, &a, zone, prevZone) || b.downgradeStart(d, &a) {
			return
		}
	}
	if b.countdown(d, &a) || b.trend(d, &a) {
		return
	}
	b.summary(d, &a)
}

// track actualiza la memoria de las reglas: zona, cambios de zona (oscilación)
// y valores recientes (tendencia).
func (b *builder) track(a *entities.Alert) (zone, prevZone string) {
	n := b.in.Notice
	p := b.in.Policy
	rt := &a.Runtime

	zone = zoneOf(n)
	prevZone = rt.PrevZone
	if prevZone != "" && (prevZone == entities.ZoneAbove) != (zone == entities.ZoneAbove) {
		at := b.at
		rt.Flips = append(rt.Flips, at)
		rt.LastFlipAt = &at
	}
	window := time.Duration(p.OscillationWindowHours) * time.Hour
	kept := rt.Flips[:0]
	for _, f := range rt.Flips {
		if b.at.Sub(f) < window {
			kept = append(kept, f)
		}
	}
	rt.Flips = kept
	rt.PrevZone = zone

	lookback := time.Duration(p.TrendLookbackHours) * time.Hour
	recent := []entities.HourValue{}
	for _, hv := range rt.Recent {
		if b.at.Sub(hv.At) <= lookback {
			recent = append(recent, hv)
		}
	}
	rt.Recent = append(recent, entities.HourValue{At: b.at, I: n.Value()})
	return zone, prevZone
}

func zoneOf(n entities.LevelNotice) string {
	switch {
	case n.RawLevel.AlertLevel():
		return entities.ZoneAbove
	case n.BelowRun > 0:
		return entities.ZoneBelow
	default:
		return entities.ZoneBand
	}
}

func (b *builder) up(d *Decision, a *entities.Alert, level entities.Level) {
	n := b.in.Notice
	p := b.in.Policy
	at := b.at

	a.CurrentLevel = level
	a.MaxLevel = entities.MaxLevel(a.MaxLevel, level)
	a.BelowLevelHours = 0
	a.Runtime.CountdownSentFor = ""
	a.Runtime.RojoHeldSent = false
	d.CloseSegmentAt = &at
	d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentUp, Level: level, StartedAt: at}

	zone := b.in.Device.Zone
	switch {
	case a.InIntervention() && level.Rank() <= p.InterventionSilentMaxLevel.Rank():
		d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeUpDuringIntervention, ""), map[string]string{
			"nivel": messages.LevelName(level),
		}, "", level, true))
	case level == entities.LevelRojo && n.PersistenceHours > 0:
		d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeUpRojo, messages.VariantPersist), map[string]string{
			"codigo": a.Code, "N": strconv.Itoa(n.PersistenceHours), "zona": zone,
		}, "", level, true))
	case level == entities.LevelRojo:
		d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeUpRojo, ""), map[string]string{
			"codigo": a.Code, "zona": zone,
		}, "", level, true))
	case n.PersistenceHours > 0:
		d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeUpNaranjaPersist, ""), map[string]string{
			"codigo": a.Code, "N": strconv.Itoa(n.PersistenceHours), "zona": zone,
		}, "", level, true))
	default:
		d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeUpNaranjaFast, ""), map[string]string{
			"codigo": a.Code, "zona": zone,
		}, "", level, true))
	}
}

// downgrade (regla 3): downgrade_hours por debajo del nivel actual bajan un
// nivel; el rojo sin clasificar no baja (un solo ROJO_HELD).
func (b *builder) downgrade(d *Decision, a *entities.Alert) bool {
	p := b.in.Policy
	current := a.CurrentLevel
	if current.Rank() <= entities.LevelAmarillo.Rank() || a.BelowLevelHours < p.DowngradeHours {
		return false
	}

	if current == entities.LevelRojo && p.RojoRequiresClassificationToDowngrade && !a.Management.Classified() {
		if a.Runtime.RojoHeldSent {
			return false
		}
		a.Runtime.RojoHeldSent = true
		d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeRojoHeld, ""), map[string]string{
			"horas": strconv.Itoa(p.DowngradeHours),
		}, "", current, true))
		return true
	}

	lower := current.Below()
	at := b.at
	a.CurrentLevel = lower
	a.BelowLevelHours = 0
	a.Runtime.CountdownSentFor = ""
	d.CloseSegmentAt = &at
	d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentDown, Level: lower, StartedAt: at}
	d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeDowngrade, ""), map[string]string{
		"nivel": messages.LevelName(current), "horas": strconv.Itoa(p.DowngradeHours),
		"nivel_inferior": messages.LevelName(lower), "max": messages.LevelName(a.MaxLevel),
	}, "", lower, true))
	return true
}

// oscillation (regla 4): oscillation_flips entradas y salidas de la banda de
// cierre en oscillation_window_hours ⇒ un OSCILLATING y silencio de las reglas
// 5 y 6 hasta que pase una ventana entera sin cambios.
func (b *builder) oscillation(d *Decision, a *entities.Alert) bool {
	p := b.in.Policy
	rt := &a.Runtime
	window := time.Duration(p.OscillationWindowHours) * time.Hour

	if rt.Oscillating {
		if rt.LastFlipAt == nil || b.at.Sub(*rt.LastFlipAt) >= window {
			rt.Oscillating = false
		}
		return false
	}
	if len(rt.Flips) < p.OscillationFlips {
		return false
	}
	rt.Oscillating = true
	d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeOscillating, ""), map[string]string{
		"hora": messages.FormatHour(rt.Flips[0]),
	}, "", a.CurrentLevel, true))
	return true
}

// band (regla 5): entra a la banda de cierre / lleva 1 h bajo la salida /
// vuelve sobre el umbral.
func (b *builder) band(d *Decision, a *entities.Alert, zone, prevZone string) bool {
	n := b.in.Notice
	var key string
	params := map[string]string{}
	switch {
	case zone == entities.ZoneBand && prevZone == entities.ZoneAbove:
		key = messages.Key(messages.CodeAlmostNormal, "")
	case zone == entities.ZoneBelow && n.BelowRun == 1:
		key = messages.Key(messages.CodeOneHourNormal, "")
	case zone == entities.ZoneAbove && (prevZone == entities.ZoneBand || prevZone == entities.ZoneBelow):
		key = messages.Key(messages.CodeBackAbnormal, "")
		params["nivel"] = messages.LevelName(a.CurrentLevel)
	default:
		return false
	}
	d.Messages = append(d.Messages, b.message(a, key, params, "", a.CurrentLevel, true))
	return true
}

// downgradeStart (regla 6): primera hora por debajo del nivel actual.
func (b *builder) downgradeStart(d *Decision, a *entities.Alert) bool {
	current := a.CurrentLevel
	if current.Rank() <= entities.LevelAmarillo.Rank() || a.BelowLevelHours != 1 {
		return false
	}
	d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeDowngradeStart, ""), map[string]string{
		"nivel_hora":     messages.LevelName(b.in.Notice.ComputedLevel),
		"horas":          strconv.Itoa(b.in.Policy.DowngradeHours),
		"nivel_inferior": messages.LevelName(current.Below()),
	}, "", current, true))
	return true
}

// countdown (regla 7): faltan countdown_hours para subir por duración (una vez por nivel).
func (b *builder) countdown(d *Decision, a *entities.Alert) bool {
	n := b.in.Notice
	next := a.CurrentLevel.Above()
	if next == "" || a.Runtime.CountdownSentFor == a.CurrentLevel {
		return false
	}
	escalation, runs := n.EscalationAmarillo, n.RunsAmarillo
	if a.CurrentLevel == entities.LevelNaranja {
		escalation, runs = n.EscalationNaranja, n.RunsNaranja
	}
	if escalation == nil || *escalation-runs != b.in.Policy.CountdownHours {
		return false
	}
	a.Runtime.CountdownSentFor = a.CurrentLevel
	d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeCountdown, ""), map[string]string{
		"N": strconv.Itoa(runs), "horas": strconv.Itoa(b.in.Policy.CountdownHours), "nivel_siguiente": messages.LevelName(next),
	}, "", a.CurrentLevel, true))
	return true
}

// trend (regla 8): la tendencia cambia (±trend_delta frente a trend_lookback_hours antes).
func (b *builder) trend(d *Decision, a *entities.Alert) bool {
	trend := trendAt(a.Runtime.Recent, b.at, b.in.Policy)
	if trend == "" || trend == a.Runtime.LastTrend {
		return false
	}
	a.Runtime.LastTrend = trend
	if trend == entities.TrendStable {
		return false
	}
	direction := "subiendo"
	if trend == entities.TrendDown {
		direction = "bajando"
	}
	d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeTrend, ""), map[string]string{
		"direccion": direction, "nivel": messages.LevelName(a.CurrentLevel), "N": strconv.Itoa(a.HoursCalculated),
	}, "", a.CurrentLevel, true))
	return true
}

func trendAt(recent []entities.HourValue, at time.Time, p entities.Policy) string {
	ref := at.Add(-time.Duration(p.TrendLookbackHours) * time.Hour)
	var current, previous *float64
	for i := range recent {
		switch {
		case recent[i].At.Equal(at):
			current = &recent[i].I
		case recent[i].At.Equal(ref):
			previous = &recent[i].I
		}
	}
	if current == nil || previous == nil {
		return ""
	}
	diff := math.Round((*current-*previous)*100) / 100
	switch {
	case diff >= p.TrendDelta-1e-9:
		return entities.TrendUp
	case diff <= -p.TrendDelta+1e-9:
		return entities.TrendDown
	default:
		return entities.TrendStable
	}
}

// summary (regla 9): summary_every_hours sin mensajes.
func (b *builder) summary(d *Decision, a *entities.Alert) {
	last := a.Runtime.LastMessageAt
	if last == nil || b.at.Sub(*last) < time.Duration(b.in.Policy.SummaryEveryHours)*time.Hour {
		return
	}
	trend := "estable"
	switch a.Runtime.LastTrend {
	case entities.TrendUp:
		trend = "subiendo"
	case entities.TrendDown:
		trend = "bajando"
	}
	d.Messages = append(d.Messages, b.message(a, messages.Key(messages.CodeSummary, ""), map[string]string{
		"nivel": messages.LevelName(a.CurrentLevel), "N": strconv.Itoa(a.HoursCalculated), "tendencia": trend,
	}, "", a.CurrentLevel, true))
}

// cloneAlert copia la alerta sin compartir los slices del runtime.
func cloneAlert(a entities.Alert) entities.Alert {
	a.Runtime.Flips = append([]time.Time(nil), a.Runtime.Flips...)
	a.Runtime.Recent = append([]entities.HourValue(nil), a.Runtime.Recent...)
	return a
}
