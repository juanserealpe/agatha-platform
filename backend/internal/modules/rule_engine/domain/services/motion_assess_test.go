package services

import (
	"math"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// Tests sintéticos de CA-6. Cada serie arranca en t0 (00:00:42 UTC) con un
// registro cada 15 min; h es la hora desde t0 (float) de cada registro.

var synthT0 = time.Date(2026, 1, 1, 0, 0, 42, 0, time.UTC)

const (
	baseP = -3.00
	baseR = 25.00
)

// sample devuelve pitch, roll, deriva y temp del registro en la hora h; ok =
// false deja un hueco.
type sample func(h float64) (p, r, der, temp float64, ok bool)

func synth(hoursN int, f sample) []entities.RawRecord {
	var out []entities.RawRecord
	for i := 0; i < hoursN*4; i++ {
		h := float64(i) / 4
		p, r, der, temp, ok := f(h)
		if !ok {
			continue
		}
		p, r, temp = round2(p), round2(r), round2(temp)
		out = append(out, entities.RawRecord{
			Timestamp: synthT0.Add(time.Duration(i) * 15 * time.Minute),
			Pitch:     fp(p), Roll: fp(r), Deriva: fp(der), Temp: fp(temp),
		})
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// hourAt es la hora en punto t0 + h (las horas evaluadas son :00:00).
func hourAt(h int) time.Time { return synthT0.Truncate(time.Hour).Add(time.Duration(h) * time.Hour) }

// recAt es el timestamp del registro de la hora h (con sus :00:42).
func recAt(h float64) time.Time { return synthT0.Add(time.Duration(h * float64(time.Hour))) }

func byHour(evs []entities.MotionEvaluation) map[time.Time]entities.MotionEvaluation {
	m := make(map[time.Time]entities.MotionEvaluation, len(evs))
	for _, e := range evs {
		m[e.EvaluatedAt] = e
	}
	return m
}

func reasonOf(e entities.MotionEvaluation, code entities.ReasonCode) (entities.MotionReason, bool) {
	for _, r := range e.Reasons {
		if r.Code == code {
			return r, true
		}
	}
	return entities.MotionReason{}, false
}

func hoursWith(evs []entities.MotionEvaluation, code entities.ReasonCode) []time.Time {
	var out []time.Time
	for _, e := range evs {
		if e.HasReason(code) {
			out = append(out, e.EvaluatedAt)
		}
	}
	return out
}

// rampTemp es una temperatura que sube 0,1 °C/h: siempre TEMP_VARIABLE.
func rampTemp(h float64) float64 { return 20 + 0.1*h }

// jumpAt48 mueve los dos ejes +0,10° en h = 48 (deriva 0,05 en ese registro).
func jumpAt48(h float64) (float64, float64, float64) {
	if h < 48 {
		return baseP, baseR, 0
	}
	der := 0.0
	if h == 48 {
		der = 0.05
	}
	return baseP + 0.10, baseR + 0.10, der
}

func TestMotionAssess_TempNotStabilizedAt36h(t *testing.T) {
	recs := synth(100, func(h float64) (float64, float64, float64, float64, bool) {
		p, r, der := jumpAt48(h)
		return p, r, der, rampTemp(h), true
	})
	evs := runMotion(recs, entities.DefaultMotionParams())
	m := byHour(evs)

	shift := m[hourAt(50)]
	if !shift.HasReason(entities.ReasonMovShift) || !shift.HasReason(entities.ReasonTempVerifying) {
		t.Fatalf("50 h: motivos %+v, se esperaba MOV_SHIFT y TEMP_VERIFYING", shift.Reasons)
	}
	if got := shift.ShiftStartedAt(); got == nil || !got.Equal(recAt(48)) {
		t.Fatalf("shift_started_at = %v, se esperaba %v", got, recAt(48))
	}
	// 48:00:42 + 36 h = 84:00:42: la primera hora evaluada que llega es 85:00.
	got := hoursWith(evs, entities.ReasonTempNotStabilized)
	if len(got) != 1 || !got[0].Equal(hourAt(85)) {
		t.Fatalf("TEMP_NOT_STABILIZED en %v, se esperaba solo en %v", got, hourAt(85))
	}
	r, _ := reasonOf(m[hourAt(85)], entities.ReasonTempNotStabilized)
	if r.Hours != 36 {
		t.Fatalf("hours = %d, se esperaba 36", r.Hours)
	}
	if m[hourAt(84)].State.TempState != entities.TempVerificando {
		t.Fatalf("84 h: temp_state = %s, se esperaba VERIFICANDO", m[hourAt(84)].State.TempState)
	}
}

func TestMotionAssess_TempStillStableAt6_18_30h(t *testing.T) {
	recs := synth(110, func(h float64) (float64, float64, float64, float64, bool) {
		switch {
		case h < 48:
			return baseP, baseR, 0, rampTemp(h), true
		case h < 70:
			p, r, der := jumpAt48(h)
			return p, r, der, rampTemp(48), true
		default: // retorno en 70 h, la temperatura sigue estable
			der := 0.0
			if h == 70 {
				der = 0.05
			}
			return baseP, baseR, der, rampTemp(48), true
		}
	})
	evs := runMotion(recs, entities.DefaultMotionParams())
	m := byHour(evs)

	if len(hoursWith(evs, entities.ReasonTempStabilized)) != 1 {
		t.Fatalf("TEMP_STABILIZED en %v, se esperaba una vez", hoursWith(evs, entities.ReasonTempStabilized))
	}
	ret := hoursWith(evs, entities.ReasonMovReturn)
	if len(ret) != 1 || !ret[0].Equal(hourAt(71)) {
		t.Fatalf("MOV_RETURN en %v, se esperaba en %v", ret, hourAt(71))
	}
	if s := m[hourAt(71)].State.TempState; s != entities.TempRetornoEstable {
		t.Fatalf("71 h: temp_state = %s, se esperaba RETORNO_TEMP_ESTABLE", s)
	}
	got := hoursWith(evs, entities.ReasonTempStillStable)
	want := []time.Time{hourAt(77), hourAt(89), hourAt(101)}
	if len(got) != len(want) {
		t.Fatalf("TEMP_STILL_STABLE en %v, se esperaba en %v", got, want)
	}
	for i, h := range []int{6, 18, 30} {
		if !got[i].Equal(want[i]) {
			t.Fatalf("TEMP_STILL_STABLE #%d en %v, se esperaba %v", i, got[i], want[i])
		}
		r, _ := reasonOf(m[got[i]], entities.ReasonTempStillStable)
		if r.Hours != h {
			t.Fatalf("TEMP_STILL_STABLE #%d hours = %d, se esperaba %d", i, r.Hours, h)
		}
	}
	// Mientras la temperatura sigue estable, el episodio no se cierra.
	if !m[hourAt(109)].State.InEpisode {
		t.Fatal("109 h: el episodio se cerró con RETORNO_TEMP_ESTABLE")
	}
}

func TestMotionAssess_GapEvaluatedAtNextCalculatedHour(t *testing.T) {
	// Último registro antes del hueco: 50:45:42. Primero después: f = 56:30:42,
	// ya desplazado +0,10°. Después de f faltan 57:30:42 y 57:45:42, así que la
	// hora de evaluación del hueco (floor(f + 45 min) + 1 h = 58:00) es NO_CALCULABLE.
	recs := synth(80, func(h float64) (float64, float64, float64, float64, bool) {
		switch {
		case h <= 50.75:
			return baseP, baseR, 0, rampTemp(h), true
		case h < 56.5, h == 57.5, h == 57.75:
			return 0, 0, 0, 0, false
		default:
			return baseP + 0.10, baseR + 0.08, 0, rampTemp(h), true
		}
	})
	evs := runMotion(recs, entities.DefaultMotionParams())
	m := byHour(evs)

	h58 := m[hourAt(58)]
	if h58.Status != entities.MotionStatusNoCalculable || len(h58.State.PendingGaps) != 1 {
		t.Fatalf("58 h: status %s, pending %v; se esperaba NO_CALCULABLE con el hueco pendiente",
			h58.Status, h58.State.PendingGaps)
	}
	got := hoursWith(evs, entities.ReasonMovShiftGap)
	if len(got) != 1 || !got[0].Equal(hourAt(59)) {
		t.Fatalf("MOV_SHIFT_GAP en %v, se esperaba en %v", got, hourAt(59))
	}
	h59 := m[hourAt(59)]
	r, _ := reasonOf(h59, entities.ReasonMovShiftGap)
	if !r.GapFrom.Equal(recAt(50.75)) || !r.GapTo.Equal(recAt(56.5)) {
		t.Fatalf("hueco %v → %v, se esperaba %v → %v", r.GapFrom, r.GapTo, recAt(50.75), recAt(56.5))
	}
	if *r.DP != 0.1 || *r.DR != 0.08 {
		t.Fatalf("dp/dr = %v/%v, se esperaba 0.1/0.08", *r.DP, *r.DR)
	}
	if len(h59.State.PendingGaps) != 0 || h59.ShiftStartedAt() == nil || !h59.ShiftStartedAt().Equal(recAt(50.75)) {
		t.Fatalf("59 h: pending %v, shift_started_at %v", h59.State.PendingGaps, h59.ShiftStartedAt())
	}
}

func TestMotionAssess_NoNewShiftWithin24hAfterReturn(t *testing.T) {
	// Sube en 48 h, vuelve en 54 h y vuelve a subir en 57 h para quedarse.
	recs := synth(100, func(h float64) (float64, float64, float64, float64, bool) {
		up := (h >= 48 && h < 54) || h >= 57
		der := 0.0
		if h == 48 || h == 54 || h == 57 {
			der = 0.05
		}
		if up {
			return baseP + 0.10, baseR + 0.10, der, rampTemp(h), true
		}
		return baseP, baseR, der, rampTemp(h), true
	})
	evs := runMotion(recs, entities.DefaultMotionParams())

	ret := hoursWith(evs, entities.ReasonMovReturn)
	if len(ret) != 1 {
		t.Fatalf("MOV_RETURN en %v, se esperaba uno", ret)
	}
	rt := ret[0]
	if got := hoursWith(evs, entities.ReasonMovShift); len(got) != 1 || !got[0].Before(rt) {
		t.Fatalf("MOV_SHIFT en %v, se esperaba solo el anterior al retorno %v", got, rt)
	}
	// La regla de desplazamiento sí se cumplía dentro de las 24 h: la supresión fue la que actuó.
	signed := false
	for _, e := range evs {
		if e.EvaluatedAt.After(rt) && e.EvaluatedAt.Sub(rt) < 24*time.Hour && e.ShiftSignNow != 0 {
			signed = true
			if e.State.Displaced {
				t.Fatalf("%v: desplazado dentro de las 24 h siguientes al retorno", e.EvaluatedAt)
			}
		}
	}
	if !signed {
		t.Fatal("ninguna hora tuvo shift_sign ≠ 0 tras el retorno: el test no prueba la supresión")
	}
}

// shiftAndStay sube los dos ejes en 48 h y se queda; temperatura en rampa.
func shiftAndStay(hoursN int) []entities.RawRecord {
	return synth(hoursN, func(h float64) (float64, float64, float64, float64, bool) {
		p, r, der := jumpAt48(h)
		return p, r, der, rampTemp(h), true
	})
}

func TestMotionAssess_NewPositionWhenClearingDisplaced(t *testing.T) {
	p := entities.DefaultMotionParams()
	p.TempMode = entities.TempModeMotionOnly
	evs := runMotion(shiftAndStay(90), p)

	cleared := []entities.MotionEvaluation{}
	for _, e := range evs {
		if e.EpisodeEvent == entities.EpisodeEventCleared {
			cleared = append(cleared, e)
		}
	}
	if len(cleared) != 1 {
		t.Fatalf("CLEARED %d veces, se esperaba una", len(cleared))
	}
	c := cleared[0]
	if !c.HasReason(entities.ReasonMovNewPosition) || !c.HasReason(entities.ReasonMovStill) {
		t.Fatalf("motivos al cerrar %+v, se esperaba MOV_NEW_POSITION y MOV_STILL", c.Reasons)
	}
	if c.State.Displaced || c.MotionState != entities.MotionNormal || c.ComputedLevel != entities.LevelNormal {
		t.Fatalf("al cerrar: displaced %v, motion_state %s, level %s", c.State.Displaced, c.MotionState, c.ComputedLevel)
	}
	if prev := byHour(evs)[c.EvaluatedAt.Add(-time.Hour)]; !prev.State.Displaced {
		t.Fatal("la hora anterior al cierre ya no estaba desplazada")
	}
}

func TestMotionAssess_MotionOnlyHasNoTempState(t *testing.T) {
	p := entities.DefaultMotionParams()
	p.TempMode = entities.TempModeMotionOnly
	evs := runMotion(shiftAndStay(90), p)

	for _, e := range evs {
		if e.State.TempState != "" {
			t.Fatalf("%v: temp_state = %s, se esperaba vacío", e.EvaluatedAt, e.State.TempState)
		}
		for _, r := range e.Reasons {
			switch r.Code {
			case entities.ReasonTempVerifying, entities.ReasonTempStabilized, entities.ReasonTempNotStabilized,
				entities.ReasonTempVariesAgain, entities.ReasonTempStillStable:
				t.Fatalf("%v: motivo de temperatura %s con SOLO_MOVIMIENTO", e.EvaluatedAt, r.Code)
			}
		}
	}
	if len(hoursWith(evs, entities.ReasonMovShift)) != 1 {
		t.Fatal("se esperaba el MOV_SHIFT: el movimiento se sigue evaluando")
	}
}

func TestMotionAssess_AbruptOncePerChange(t *testing.T) {
	// Roll sube 0,05° en 48 h y se queda; pitch quieto, sin deriva.
	recs := synth(80, func(h float64) (float64, float64, float64, float64, bool) {
		r := baseR
		if h >= 48 {
			r += 0.05
		}
		return baseP, r, 0, rampTemp(h), true
	})
	evs := runMotion(recs, entities.DefaultMotionParams())
	m := byHour(evs)

	got := hoursWith(evs, entities.ReasonMovAbrupt)
	if len(got) != 1 || !got[0].Equal(hourAt(49)) {
		t.Fatalf("MOV_ABRUPT en %v, se esperaba solo en %v", got, hourAt(49))
	}
	r, _ := reasonOf(m[hourAt(49)], entities.ReasonMovAbrupt)
	if r.Hit == nil || r.Hit.Axis != "roll" || r.Hit.Delta != 0.05 || r.Hit.SpanHours != 1 || r.Hit.WindowHours != 1 {
		t.Fatalf("MOV_ABRUPT %+v, se esperaba roll +0.05 en 1 h", r)
	}
	for _, h := range []int{49, 50, 51} {
		if e := m[hourAt(h)]; !e.Abrupt || e.MotionState != entities.MotionBrusco {
			t.Fatalf("%d h: abrupt %v, motion_state %s; el cambio sigue activo", h, e.Abrupt, e.MotionState)
		}
	}
	if m[hourAt(52)].Abrupt {
		t.Fatal("52 h: abrupt sigue activo pasadas 3 h")
	}
	if m[hourAt(49)].EpisodeEvent != entities.EpisodeEventOpened {
		t.Fatalf("49 h: episode_event %s, se esperaba OPENED", m[hourAt(49)].EpisodeEvent)
	}
}

func TestMotionAssess_SlowMovementHasNoEpisode(t *testing.T) {
	// Sube 0,05° en 11 h (de 48 a 59 h), con resolución 0,01°: en un eje o en
	// los dos a la vez. Ninguno abre episodio.
	cases := []struct {
		name      string
		pitchToo  bool
		wantShift bool // con los dos ejes la regla del desplazamiento sí se cumple
	}{
		{name: "un eje (roll)"},
		{name: "los dos ejes", pitchToo: true, wantShift: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recs := synth(100, func(h float64) (float64, float64, float64, float64, bool) {
				d := 0.05 * math.Min(math.Max((h-48)/11, 0), 1)
				p := baseP
				if tc.pitchToo {
					p += d
				}
				return p, baseR + d, 0, rampTemp(h), true
			})
			evs := runMotion(recs, entities.DefaultMotionParams())

			drift, signed := false, false
			for _, e := range evs {
				if e.State.InEpisode || len(e.Reasons) > 0 || e.State.Displaced {
					t.Fatalf("%v: in_episode %v, displaced %v, motivos %+v; un movimiento lento no abre episodio",
						e.EvaluatedAt, e.State.InEpisode, e.State.Displaced, e.Reasons)
				}
				drift = drift || e.SlowDriftFlag
				signed = signed || e.ShiftSignNow != 0
			}
			if !drift {
				t.Fatal("se esperaba slow_drift_flag en alguna hora")
			}
			// Con los dos ejes, shift_sign se cumple: lo que lo frena es que no hubo salto.
			if signed != tc.wantShift {
				t.Fatalf("shift_sign ≠ 0 en alguna hora = %v, se esperaba %v", signed, tc.wantShift)
			}
		})
	}
}

// CA-8: un salto conjunto mayor que closure_max_deg (0,20°) no se queda en el
// tope del patrón conjunto; uno de 0,15° sí.
func TestMotionAssessV2_JointJumpAboveClosureMax(t *testing.T) {
	for _, tc := range []struct {
		jump float64
		want entities.Level
	}{{0.25, entities.LevelRojo}, {0.15, entities.LevelAmarillo}} {
		recs := synth(120, func(h float64) (float64, float64, float64, float64, bool) {
			if h < 48 {
				return baseP, baseR, 0, rampTemp(h), true
			}
			der := 0.0
			if h == 48 {
				der = 0.05
			}
			return baseP + tc.jump, baseR + tc.jump, der, rampTemp(h), true
		})
		max := entities.LevelNormal
		opened := 0
		for _, e := range runMotion(recs, entities.DefaultMotionParams()) {
			max = entities.MaxLevel(max, e.TableLevel)
			if e.EpisodeEvent == entities.EpisodeEventOpened {
				opened++
			}
			if e.ComputedLevel != entities.LevelNormal && e.ComputedLevel != entities.LevelAmarillo {
				t.Fatalf("+%.2f: computed_level %s con Rojo en OBSERVATION", tc.jump, e.ComputedLevel)
			}
		}
		if opened != 1 || max != tc.want {
			t.Errorf("salto conjunto +%.2f°: %d episodios, table_level máximo %s; se esperaba 1 y %s", tc.jump, opened, max, tc.want)
		}
	}
}

// CA-11: una versión nueva con el episodio abierto no baja ep_level ni lo cierra.
func TestMotionAssessV2_NewVersionKeepsEpisodeLevel(t *testing.T) {
	// Pitch −0,25° en h = 48: Rojo por la fila de 1 h.
	recs := synth(80, func(h float64) (float64, float64, float64, float64, bool) {
		p := baseP
		if h >= 48 {
			p -= 0.25
		}
		return p, baseR, 0, rampTemp(h), true
	})
	strict := entities.DefaultMotionParams()
	lax := entities.DefaultMotionParams()
	lax.Rules = []entities.MotionRule{{WindowHours: 1, Axis: entities.MotionAxisAny, AmarilloDeg: 0.30}}
	state := entities.InitialMotionState(strict)
	switchAt := hourAt(50)
	for h := hourAt(1); !h.After(hourAt(72)); h = h.Add(time.Hour) {
		p := strict
		if !h.Before(switchAt) {
			p = lax
		}
		from, _ := entities.MotionWindow(h)
		var win []entities.RawRecord
		for _, r := range recs {
			if !r.Timestamp.Before(from) && r.Timestamp.Before(h) {
				win = append(win, r)
			}
		}
		e := MotionAssess(win, state, p, h, nil)
		state = e.State
		if h.Equal(hourAt(49)) && (e.EpisodeEvent != entities.EpisodeEventOpened || e.TableLevel != entities.LevelRojo) {
			t.Fatalf("49 h: %s %s; se esperaba OPENED en ROJO", e.EpisodeEvent, e.TableLevel)
		}
		if h.After(switchAt) && h.Before(hourAt(73)) {
			if !e.State.InEpisode || e.TableLevel != entities.LevelRojo {
				t.Fatalf("%v: in_episode %v, table_level %s; la versión nueva no baja el nivel ni cierra antes de 24 h",
					h, e.State.InEpisode, e.TableLevel)
			}
		}
	}
}
