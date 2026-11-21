package messages

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// Formato de los textos de movimiento del catálogo v8 (REQ_MOVIMIENTO_UMBRALES_V2
// §10). Reproduce messages_v2_ref.py: grados con signo y dos decimales
// redondeados medio alejándose de cero sobre el valor de 3 decimales
// (−0,105 → −0,11), velocidades con tres, duraciones con las horas
// redondeadas medio hacia arriba, horas y fechas de Colombia.

// roundHalfUp redondea medio alejándose de cero a dec decimales, sobre el
// valor llevado antes a 6 decimales (como Decimal(str(round(v, 6))) de la referencia).
func roundHalfUp(v float64, dec int) float64 {
	v6, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 6, 64), 64)
	p := math.Pow(10, float64(dec))
	r := math.Floor(math.Abs(v6)*p+0.5+1e-9) / p
	if v6 < 0 {
		return -r
	}
	return r
}

// FormatDegrees son grados con signo y dos decimales, con coma: +0,08°;
// 0 se escribe 0,00° sin signo.
func FormatDegrees(v float64) string {
	r := roundHalfUp(v, 2)
	if math.Abs(r) < 0.005 {
		return "0,00°"
	}
	s := strconv.FormatFloat(r, 'f', 2, 64)
	if r > 0 {
		s = "+" + s
	}
	return strings.Replace(s, ".", ",", 1) + "°"
}

// FormatDegreesPtr es FormatDegrees con "s/d" si falta el dato.
func FormatDegreesPtr(v *float64) string {
	if v == nil {
		return "s/d"
	}
	return FormatDegrees(*v)
}

// FormatDegreesAbs son grados sin signo (un límite: "menos de 0,04°").
func FormatDegreesAbs(v float64) string {
	return strings.Replace(strconv.FormatFloat(math.Abs(roundHalfUp(v, 2)), 'f', 2, 64), ".", ",", 1) + "°"
}

// FormatRatePerDay es una velocidad con tres decimales: "-0,027° por día".
func FormatRatePerDay(v float64) string {
	r := roundHalfUp(v, 3)
	if math.Abs(r) < 0.0005 {
		return "0,000° por día"
	}
	s := strconv.FormatFloat(r, 'f', 3, 64)
	if r > 0 {
		s = "+" + s
	}
	return strings.Replace(s, ".", ",", 1) + "° por día"
}

// FormatWindow es una ventana: "1 hora", "3 horas", "3 días", "7 días".
func FormatWindow(h int) string {
	if h < 24 {
		if h == 1 {
			return "1 hora"
		}
		return strconv.Itoa(h) + " horas"
	}
	if d := h / 24; d != 1 {
		return strconv.Itoa(d) + " días"
	}
	return "1 día"
}

// FormatSpan es una duración con las horas redondeadas medio hacia arriba:
// "23 h", "1 día y 2 h", "10 días y 8 h".
func FormatSpan(hours float64) string {
	h := int(hours + 0.5)
	d, r := h/24, h%24
	if d == 0 {
		return strconv.Itoa(r) + " h"
	}
	s := strconv.Itoa(d) + " día"
	if d > 1 {
		s += "s"
	}
	if r > 0 {
		s += " y " + strconv.Itoa(r) + " h"
	}
	return s
}

// JoinList une con comas y "y": "a", "a y b", "a, b y c".
func JoinList(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " y " + xs[len(xs)-1]
}

// Between es "Entre las HH:MM y las HH:MM del dd/mm" o, si cambia el día,
// "Entre las HH:MM del dd/mm y las HH:MM del dd/mm".
func Between(a, b time.Time) string {
	if FormatDate(a) == FormatDate(b) {
		return "Entre las " + FormatClock(a) + " y las " + FormatClock(b) + " del " + FormatDate(b)
	}
	return "Entre las " + FormatHour(a) + " y las " + FormatHour(b)
}

// LevelTitle es el nivel con mayúscula inicial ("Amarillo", "Naranja", "Rojo").
func LevelTitle(l entities.Level) string { return LevelName(l) }

// ObservedLevelText es {calculado}: vacío si el nivel calculado no supera al enviado.
func ObservedLevelText(calc, sent entities.Level) string {
	if calc.Rank() <= sent.Rank() {
		return ""
	}
	return " Por su magnitud correspondería a " + LevelName(calc) +
		"; ese nivel todavía no está activo para este sensor, por eso la alerta se emite en " + LevelName(sent) + "."
}

// uniqueDates devuelve las fechas dd/mm sin repetir, en orden.
func uniqueDates(ts []time.Time) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range ts {
		if f := FormatDate(t); !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// ---- historial (§10.3) ----

type histStep struct {
	At      time.Time `json:"at"`
	Delta   float64   `json:"delta"`
	InAlert bool      `json:"in_alert"`
}

type histAxis struct {
	Net      *float64   `json:"net"`
	Pattern  string     `json:"pattern"`
	Steps    int        `json:"steps"`
	StepList []histStep `json:"step_list"`
	Coverage float64    `json:"coverage"`
}

type histAxes struct {
	Pitch histAxis `json:"pitch"`
	Roll  histAxis `json:"roll"`
}

type motionHistory struct {
	D7     histAxes `json:"d7"`
	D30    histAxes `json:"d30"`
	Normal []struct {
		WindowHours int     `json:"window_hours"`
		AmarilloDeg float64 `json:"amarillo_deg"`
	} `json:"normal"`
	PreviousEpisodes []struct {
		OpenedAt time.Time `json:"opened_at"`
		Kind     string    `json:"kind"`
	} `json:"previous_episodes"`
}

// kindPhrase es el tipo de un episodio anterior en {alertas_90}.
var kindPhrase = map[string]string{
	entities.MotionKindSalto:     "cambio brusco de pitch y roll",
	entities.MotionKindHueco:     "cambio durante pérdida de datos",
	entities.MotionKindRapido:    "cambio rápido",
	entities.MotionKindAcumulado: "cambio lento acumulado",
}

func axisHistoryText(name string, x histAxis) string {
	switch x.Pattern {
	case "SIN_DATOS":
		return name + " sin datos suficientes"
	case "QUIETO":
		return name + " estable (" + FormatDegreesPtr(x.Net) + ")"
	case "CONTINUO":
		return name + " " + FormatDegreesPtr(x.Net) + ", de forma continua"
	}
	var with, without []time.Time
	for _, s := range x.StepList {
		if s.InAlert {
			with = append(with, s.At)
		} else {
			without = append(without, s.At)
		}
	}
	var det []string
	if con := uniqueDates(with); len(con) > 0 {
		det = append(det, "con alerta: "+JoinList(con))
	}
	if sin := uniqueDates(without); len(sin) > 0 {
		prefix := ""
		if len(det) > 0 {
			prefix = "sin alerta: "
		}
		det = append(det, prefix+JoinList(sin))
	}
	cuantos := "en " + strconv.Itoa(x.Steps) + " cambios"
	if x.Steps == 1 {
		cuantos = "en un cambio"
	}
	if x.Steps > len(x.StepList) && len(det) > 0 {
		det[0] = "los últimos, " + det[0]
	}
	return name + " " + FormatDegreesPtr(x.Net) + ", " + cuantos + " (" + strings.Join(det, "; ") + ")"
}

// MotionHistoryText es {historial}: lo que pasó antes de la alerta en 7 y 30
// días por eje, lo normal del sensor, las alertas de los últimos 90 días y la
// falta de datos. Vacío si no hay historial (evento v1).
func MotionHistoryText(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	var h motionHistory
	b, _ := json.Marshal(raw)
	if json.Unmarshal(b, &h) != nil {
		return ""
	}
	s := "Antes de esta alerta. Últimos 7 días: " + axisHistoryText("pitch", h.D7.Pitch) + "; " + axisHistoryText("roll", h.D7.Roll) +
		". Últimos 30 días: " + axisHistoryText("pitch", h.D30.Pitch) + "; " + axisHistoryText("roll", h.D30.Roll) + "."
	normal := map[int]float64{}
	var windows []int
	for _, n := range h.Normal {
		if _, ok := normal[n.WindowHours]; !ok {
			windows = append(windows, n.WindowHours)
		}
		normal[n.WindowHours] = n.AmarilloDeg
	}
	if len(windows) > 0 {
		sortInts(windows)
		ns := make([]string, len(windows))
		for i, w := range windows {
			ns[i] = FormatDegreesAbs(normal[w]) + " en " + FormatWindow(w)
		}
		s += " Lo normal en este sensor: menos de " + JoinList(ns) + "."
	}
	if n := len(h.PreviousEpisodes); n > 0 {
		items := make([]string, n)
		for i, e := range h.PreviousEpisodes {
			items[i] = FormatDate(e.OpenedAt) + " " + kindPhrase[e.Kind]
		}
		s += " Alertas de movimiento en los últimos 90 días: " + strconv.Itoa(n) + " (" + strings.Join(items, "; ") + ")."
	} else {
		s += " Sin alertas de movimiento en los últimos 90 días."
	}
	for _, d := range []struct {
		days int
		cov  float64
	}{{7, h.D7.Pitch.Coverage}, {30, h.D30.Pitch.Coverage}} {
		if d.cov < 0.8 {
			s += " En los últimos " + strconv.Itoa(d.days) + " días faltan datos en el " +
				strconv.Itoa(int(math.RoundToEven((1-d.cov)*100))) + " % de las horas."
		}
	}
	return s
}

func sortInts(xs []int) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// CumulativeText es {acumulado} (MOV_CUMULATIVE en la hora t).
func CumulativeText(t time.Time, r entities.MotionReason) string {
	s := "El " + r.Str("axis") + " acumula " + FormatDegreesPtr(r.Num("delta")) + " en " + FormatWindow(r.Int("span_hours")) + " (del "
	if since := r.Time("since"); since != nil {
		s += FormatDate(*since)
	}
	s += " al " + FormatDate(t)
	if r.Str("pattern") == "ESCALONES" {
		var ats []time.Time
		if list, ok := r.Data["step_list"].([]any); ok {
			for _, x := range list {
				m, _ := x.(map[string]any)
				if at := (entities.MotionReason{Data: m}).Time("at"); at != nil {
					ats = append(ats, *at)
				}
			}
		}
		s += "), en " + strconv.Itoa(r.Int("steps")) + " cambios pequeños (" + JoinList(uniqueDates(ats)) + ")."
	} else {
		s += ", de forma continua"
		if rate := r.Num("rate_per_day_3d"); rate != nil {
			s += "; en los últimos 3 días, " + FormatRatePerDay(*rate) + ")."
		} else {
			s += ")"
		}
	}
	normal := 0.0
	if v := r.Num("amarillo_deg"); v != nil {
		normal = *v
	}
	return s + " En este sensor lo normal es menos de " + FormatDegreesAbs(normal) + " en " + FormatWindow(r.Int("window_hours")) + "."
}

// ReasonDataJSON es el data del motivo como JSON (params["datos"]: permite
// regenerar el texto).
func ReasonDataJSON(r *entities.MotionReason) string {
	if r == nil || r.Data == nil {
		return "{}"
	}
	b, err := json.Marshal(r.Data)
	if err != nil {
		return "{}"
	}
	return string(b)
}
