package entities

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

// motion-v2 (REQ_MOVIMIENTO_UMBRALES_V2): la tabla de lo normal por
// dispositivo, el nivel calculado / enviado, el historial y los episodios.
//
// Los tipos de valor de este archivo (MotionRuleHit, MotionStep,
// MotionHistory...) llevan tags JSON porque viajan tal cual como jsonb
// (motion_evaluations.rules_hit / history) y en el evento v2: su forma JSON ES
// el contrato (§6.6, §7.2).

// MotionShortMaxHours: una fila con window_hours <= 24 es una regla corta
// (cambio rápido); con más, una regla larga (acumulado desde el reposo).
const MotionShortMaxHours = 24

// MotionRuleMaxHours es la ventana más larga permitida (30 días).
const MotionRuleMaxHours = 720

// MotionMinAmarilloDeg: amarillo_deg debe superar la resolución del equipo.
const MotionMinAmarilloDeg = 0.01

// MotionHistoryDays son las ventanas del historial (§6.6).
var MotionHistoryDays = [2]int{7, 30}

// Ejes de una fila de la tabla.
const (
	MotionAxisAny   = "ANY"
	MotionAxisPitch = "PITCH"
	MotionAxisRoll  = "ROLL"
)

// MotionSendMode decide si un nivel calculado se envía (ACTIVE) o solo se
// registra (OBSERVATION) y la alerta sale con el nivel activo inferior.
type MotionSendMode string

const (
	MotionSendActive      MotionSendMode = "ACTIVE"
	MotionSendObservation MotionSendMode = "OBSERVATION"
)

// MotionRuleKind distingue las reglas cortas de las largas.
type MotionRuleKind string

const (
	MotionRuleShort MotionRuleKind = "SHORT"
	MotionRuleLong  MotionRuleKind = "LONG"
)

// MotionEpisodeKind es el tipo de episodio (§6.4).
type MotionEpisodeKind string

const (
	MotionEpisodeSalto     MotionEpisodeKind = "SALTO"
	MotionEpisodeHueco     MotionEpisodeKind = "HUECO"
	MotionEpisodeRapido    MotionEpisodeKind = "RAPIDO"
	MotionEpisodeAcumulado MotionEpisodeKind = "ACUMULADO"
)

// Patrones del historial y del acumulado (§6.6).
const (
	MotionPatternSinDatos  = "SIN_DATOS"
	MotionPatternQuieto    = "QUIETO"
	MotionPatternEscalones = "ESCALONES"
	MotionPatternUnEscalon = "UN_ESCALON"
	MotionPatternContinuo  = "CONTINUO"
)

// MotionRule es una fila de motion_threshold_rules: "en window_hours lo normal
// es moverse menos de amarillo_deg". Naranja y Rojo son opcionales.
type MotionRule struct {
	WindowHours int      `json:"window_hours"`
	Axis        string   `json:"axis"`
	AmarilloDeg float64  `json:"amarillo_deg"`
	NaranjaDeg  *float64 `json:"naranja_deg"`
	RojoDeg     *float64 `json:"rojo_deg"`
}

// Kind dice si la fila es corta o larga.
func (r MotionRule) Kind() MotionRuleKind {
	if r.WindowHours <= MotionShortMaxHours {
		return MotionRuleShort
	}
	return MotionRuleLong
}

// DefaultMotionRules es la tabla de §5.3 (v1 del MP-11556 y plantilla).
func DefaultMotionRules() []MotionRule {
	f := func(v float64) *float64 { return &v }
	return []MotionRule{
		{WindowHours: 1, Axis: MotionAxisAny, AmarilloDeg: 0.04, RojoDeg: f(0.20)},
		{WindowHours: 3, Axis: MotionAxisAny, AmarilloDeg: 0.04, NaranjaDeg: f(0.10)},
		{WindowHours: 72, Axis: MotionAxisAny, AmarilloDeg: 0.08},
		{WindowHours: 168, Axis: MotionAxisAny, AmarilloDeg: 0.10, NaranjaDeg: f(0.20)},
		{WindowHours: 720, Axis: MotionAxisAny, AmarilloDeg: 0.15, NaranjaDeg: f(0.25)},
	}
}

// CloneMotionRules copia la tabla (los punteros también).
func CloneMotionRules(rules []MotionRule) []MotionRule {
	if rules == nil {
		return nil
	}
	out := make([]MotionRule, len(rules))
	for i, r := range rules {
		out[i] = r
		if r.NaranjaDeg != nil {
			v := *r.NaranjaDeg
			out[i].NaranjaDeg = &v
		}
		if r.RojoDeg != nil {
			v := *r.RojoDeg
			out[i].RojoDeg = &v
		}
	}
	return out
}

// MotionThreshold es un umbral: un grado no nulo de una fila con su nivel.
type MotionThreshold struct {
	Key         string // "{window_hours}:{axis}:{nivel}" (clave de long_hold)
	WindowHours int
	Kind        MotionRuleKind
	Axis        string
	Deg         float64
	Level       Level
}

// MotionThresholds despliega la tabla en umbrales, ordenados (estable) por
// ventana: los cortos quedan antes que los largos, como en la referencia.
func MotionThresholds(rules []MotionRule) []MotionThreshold {
	var out []MotionThreshold
	for _, r := range rules {
		for _, x := range []struct {
			lvl Level
			deg *float64
		}{{LevelAmarillo, &r.AmarilloDeg}, {LevelNaranja, r.NaranjaDeg}, {LevelRojo, r.RojoDeg}} {
			if x.deg == nil {
				continue
			}
			out = append(out, MotionThreshold{
				Key:         strconv.Itoa(r.WindowHours) + ":" + r.Axis + ":" + string(x.lvl),
				WindowHours: r.WindowHours, Kind: r.Kind(), Axis: r.Axis, Deg: *x.deg, Level: x.lvl,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].WindowHours < out[j].WindowHours })
	return out
}

// NormalOf es el amarillo_deg de la primera fila con esa ventana que cubre el
// eje ("pitch" | "roll"); nil si no hay.
func NormalOf(rules []MotionRule, windowHours int, axis string) *float64 {
	for _, r := range rules {
		if r.WindowHours == windowHours && (r.Axis == MotionAxisAny || r.Axis == upperAxis(axis)) {
			v := r.AmarilloDeg
			return &v
		}
	}
	return nil
}

func upperAxis(a string) string {
	switch a {
	case "pitch":
		return MotionAxisPitch
	case "roll":
		return MotionAxisRoll
	}
	return a
}

// SentLevel es el nivel que se envía: el más alto ACTIVO que no supera el
// calculado. Amarillo siempre está activo.
func (p MotionParams) SentLevel(calc Level) Level {
	best := LevelNormal
	for _, l := range []struct {
		lvl    Level
		active bool
	}{{LevelAmarillo, true}, {LevelNaranja, p.SendNaranja == MotionSendActive}, {LevelRojo, p.SendRojo == MotionSendActive}} {
		if l.active && l.lvl.Rank() <= calc.Rank() && l.lvl.Rank() > best.Rank() {
			best = l.lvl
		}
	}
	return best
}

// MotionRuleHit es un umbral que cuenta en la hora (rules_hit, §6.2.5).
// Since solo en los largos.
type MotionRuleHit struct {
	WindowHours int            `json:"window_hours"`
	Kind        MotionRuleKind `json:"kind"`
	Level       Level          `json:"level"`
	Deg         float64        `json:"deg"`
	Axis        string         `json:"axis"`
	Delta       float64        `json:"delta"`
	SpanHours   int            `json:"span_hours"`
	Since       *time.Time     `json:"since,omitempty"`
}

// MotionStep es un cambio del historial o del acumulado. InAlert solo en el
// historial (en MOV_CUMULATIVE se omite).
type MotionStep struct {
	At      time.Time `json:"at"`
	Delta   float64   `json:"delta"`
	InAlert *bool     `json:"in_alert,omitempty"`
}

// MotionAxisHistory es el historial de un eje en D días (§6.6).
type MotionAxisHistory struct {
	Net          *float64     `json:"net"`
	Range        *float64     `json:"range"`
	RatePerDay   *float64     `json:"rate_per_day"`
	Pattern      string       `json:"pattern"`
	Steps        int          `json:"steps"`
	StepsInAlert int          `json:"steps_in_alert"`
	StepList     []MotionStep `json:"step_list"`
	Coverage     float64      `json:"coverage"`
}

// MotionAxesHistory son los dos ejes de una ventana del historial.
type MotionAxesHistory struct {
	Pitch MotionAxisHistory `json:"pitch"`
	Roll  MotionAxisHistory `json:"roll"`
}

// MotionNormalRow es una fila de 7 o 30 días de la tabla, para el mensaje.
type MotionNormalRow struct {
	WindowHours int     `json:"window_hours"`
	Axis        string  `json:"axis"`
	AmarilloDeg float64 `json:"amarillo_deg"`
}

// MotionPreviousEpisode es un episodio anterior en el historial.
type MotionPreviousEpisode struct {
	OpenedAt  time.Time         `json:"opened_at"`
	ClearedAt time.Time         `json:"cleared_at"`
	Kind      MotionEpisodeKind `json:"kind"`
	Level     Level             `json:"level"`
}

// MotionHistory es el historial de la hora OPENED (§6.6): lo que pasó antes
// del movimiento (hasta tb).
type MotionHistory struct {
	Until            time.Time               `json:"until"`
	Kind             MotionEpisodeKind       `json:"kind"`
	D7               MotionAxesHistory       `json:"d7"`
	D30              MotionAxesHistory       `json:"d30"`
	Normal           []MotionNormalRow       `json:"normal"`
	PreviousEpisodes []MotionPreviousEpisode `json:"previous_episodes"`
}

// MotionEpisode es un episodio cerrado del punto: lo usa el historial
// (in_alert y previous_episodes). Lo arma EpisodeFromCleared.
type MotionEpisode struct {
	OpenedAt  time.Time
	ClearedAt time.Time
	Kind      MotionEpisodeKind
	Level     Level // nivel calculado máximo
}

// EpisodeFromCleared arma el episodio de una hora CLEARED (ok = false si la
// hora no es CLEARED). Level sale de MOV_STILL.max_level (AMARILLO en las
// filas motion-v1, que no lo traen).
func EpisodeFromCleared(e MotionEvaluation) (MotionEpisode, bool) {
	if e.EpisodeEvent != EpisodeEventCleared || e.State.EpisodeOpenedAt == nil {
		return MotionEpisode{}, false
	}
	ep := MotionEpisode{OpenedAt: e.State.EpisodeOpenedAt.UTC(), ClearedAt: e.EvaluatedAt.UTC(), Kind: e.EpisodeKind, Level: LevelAmarillo}
	for _, r := range e.Reasons {
		if r.Code == ReasonMovStill && r.Level != "" {
			ep.Level = r.Level
		}
	}
	return ep, true
}

// String para logs y diffs.
func (h MotionRuleHit) String() string {
	return fmt.Sprintf("%d:%s:%s %s %+.3f/%dh", h.WindowHours, h.Kind, h.Level, h.Axis, h.Delta, h.SpanHours)
}
