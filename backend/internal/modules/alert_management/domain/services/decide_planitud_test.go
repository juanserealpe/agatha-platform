package services

import (
	"slices"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

// planitud fija el d_pl de la hora y el mínimo de la versión; limitedFrom != ""
// ⇒ el Rule Engine limitó la hora a Amarillo (NARANJA_NOT_CONFIRMED con ese from).
func planitud(dPl, dPlMin float64, limitedFrom entities.Level) func(*entities.LevelNotice) {
	return func(n *entities.LevelNotice) {
		n.DPl, n.DPlMin, n.LimitedFrom = &dPl, &dPlMin, limitedFrom
	}
}

func messageOf(w *world, code string) []entities.Message {
	var out []entities.Message
	for _, m := range w.messages {
		if m.Code == code {
			out = append(out, m)
		}
	}
	return out
}

// Abre en Amarillo porque la hora quedó limitada: el aviso cita el nivel que
// daría el índice solo y la firma de fuga frente al mínimo.
func TestDecide_OpensLimitedWithoutLeakSignature(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.36, R, A, none, inEpisode(false), planitud(4.6, 10, R)),
		calc(1, 0.36, R, A, opened, hoursAbove(2), planitud(4.6, 10, R)),
	)
	opens := messageOf(w, messages.CodeOpen)
	if len(opens) != 1 {
		t.Fatalf("mensajes = %v", w.codes())
	}
	m := opens[0]
	want := "Por el índice de anomalía acústica la alerta estaría en Rojo desde las 05:00 del 24/08, pero en las últimas 24 h solo el 4,6 % de las mediciones tuvo el patrón de sonido típico de una fuga (se necesita al menos 10 %). Dato: 0,36 (Rojo desde 0,35; lo normal es menos de 0,16)."
	if m.Title != "Alerta nueva en PK34+500: nivel Amarillo" || m.Body != want || m.Kind != entities.KindNotification || m.Level != A {
		t.Fatalf("OPEN sin firma = %+v", m)
	}
	if m.Params["from"] != "Rojo" || m.Params["d_pl"] != "4,6 %" || m.Params["d_pl_min"] != "10 %" || m.CatalogVersion != messages.MessageCatalogVersion {
		t.Fatalf("params = %v", m.Params)
	}
	a := w.alerts[123]
	if a.CurrentLevel != A || a.LeakSignature == nil || a.LeakSignature.Confirmed || a.LeakSignature.DPlMin != 10 ||
		a.LeakSignature.DPl == nil || *a.LeakSignature.DPl != 4.6 {
		t.Fatalf("alerta = %+v firma = %+v", a.CurrentLevel, a.LeakSignature)
	}
}

// Sin hora limitada la apertura no cambia (texto v5).
func TestDecide_OpenNotLimitedKeepsTheClassicText(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false), planitud(30, 10, "")),
		calc(1, 0.18, A, A, opened, hoursAbove(2), planitud(30, 10, "")),
	)
	m := messageOf(w, messages.CodeOpen)[0]
	if m.Body != "El índice de anomalía acústica está por encima de lo normal desde las 05:00 del 24/08. Dato: 0,18 (lo normal es menos de 0,16)." {
		t.Fatalf("body = %q", m.Body)
	}
	if s := w.alerts[123].LeakSignature; s == nil || !s.Confirmed {
		t.Fatalf("firma = %+v: d_pl 30 >= 10 y sin limitar debía confirmar", s)
	}
}

// NO_LEAK_SHAPE sale una vez por racha de horas limitadas, no es aviso y la
// racha se reinicia con una hora que no está limitada.
func TestDecide_NoLeakShapeOncePerStreak(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.27, O, A, none, planitud(4.6, 10, O)),
		calc(3, 0.27, O, A, none, planitud(5, 10, O)),
		calc(4, 0.27, O, A, none, planitud(6, 10, O)),
		calc(5, 0.19, A, A, none, planitud(30, 10, "")),
		calc(6, 0.27, O, A, none, planitud(3, 10, O)),
	)
	got := w.codeAt(messages.CodeNoLeakShape)
	if len(got) != 2 || !got[0].Equal(h(2)) || !got[1].Equal(h(6)) {
		t.Fatalf("NO_LEAK_SHAPE en %v; se esperaba h2 y h6 (mensajes %v)", got, w.codes())
	}
	m := messageOf(w, messages.CodeNoLeakShape)[0]
	want := "Por el índice de anomalía acústica la alerta estaría en Naranja, pero en las últimas 24 h solo el 4,6 % de las mediciones tuvo el patrón de sonido típico de una fuga (se necesita al menos 10 %). Dato: 0,27 (Naranja desde 0,25)."
	if m.Kind != entities.KindUpdate || m.Title != "Se mantiene en Amarillo" || m.Body != want {
		t.Fatalf("NO_LEAK_SHAPE = %+v", m)
	}
	if w.notifications() != 1 {
		t.Fatalf("notificaciones = %d; solo la apertura (%v)", w.notifications(), w.codes())
	}
}

// La apertura ya informó la limitación: la misma racha no repite el mensaje.
func TestDecide_NoLeakShapeNotRepeatedAfterLimitedOpen(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.36, R, A, none, inEpisode(false), planitud(4.6, 10, R)),
		calc(1, 0.36, R, A, opened, hoursAbove(2), planitud(4.6, 10, R)),
		calc(2, 0.36, R, A, none, planitud(4.6, 10, R)),
		calc(3, 0.36, R, A, none, planitud(4.6, 10, R)),
	)
	if got := w.codeAt(messages.CodeNoLeakShape); len(got) != 0 {
		t.Fatalf("NO_LEAK_SHAPE en %v; la apertura ya lo informó", got)
	}
}

// Una hora sin cálculo no corta la racha ni repite el mensaje.
func TestDecide_NoLeakShapeSurvivesNoCalcHour(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.27, O, A, none, planitud(4.6, 10, O)),
		noCalc(3),
		calc(4, 0.27, O, A, none, planitud(4.6, 10, O)),
	)
	if got := w.codeAt(messages.CodeNoLeakShape); len(got) != 1 {
		t.Fatalf("NO_LEAK_SHAPE en %v", got)
	}
	if s := w.alerts[123].LeakSignature; s == nil || s.Confirmed {
		t.Fatalf("firma tras la hora sin datos = %+v", s)
	}
}

// Sube con la planitud recién confirmada: la subida antepone la firma de fuga.
func TestDecide_RiseRightAfterLimitedPrependsTheSignature(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.36, R, A, none, inEpisode(false), planitud(4.6, 10, R)),
		calc(1, 0.36, R, A, opened, hoursAbove(2), planitud(4.6, 10, R)),
		calc(2, 0.36, R, R, none, planitud(12, 10, "")),
	)
	ups := messageOf(w, messages.CodeUpRojo)
	if len(ups) != 1 {
		t.Fatalf("mensajes = %v", w.codes())
	}
	m := ups[0]
	want := "En las últimas 24 h, el 12 % de las mediciones tuvo el patrón de sonido típico de una fuga (mínimo 10 %). El índice de anomalía acústica siguió subiendo y ya es demasiado alto. Dato: pasó de 0,36 a 0,36 (Rojo desde 0,35)."
	if m.Title != "La alerta ALR-000123 subió a Rojo" || m.Body != want || m.Params["d_pl"] != "12 %" || m.Params["d_pl_min"] != "10 %" {
		t.Fatalf("UP_ROJO = %+v", m)
	}
	if s := w.alerts[123].LeakSignature; s == nil || !s.Confirmed || *s.DPl != 12 {
		t.Fatalf("firma = %+v", s)
	}
}

// Rojo por duración con la planitud recién confirmada usa la variante PERSIST.
func TestDecide_PersistRiseAfterLimitedPrependsTheSignature(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.27, O, A, none, inEpisode(false), planitud(4.6, 10, O)),
		calc(1, 0.27, O, A, opened, hoursAbove(2), planitud(4.6, 10, O)),
		calc(2, 0.27, O, R, none, persistence(6), planitud(40, 10, "")),
	)
	m := messageOf(w, messages.CodeUpRojo)
	if len(m) != 1 || m[0].Body != "En las últimas 24 h, el 40 % de las mediciones tuvo el patrón de sonido típico de una fuga (mínimo 10 %). El índice de anomalía acústica lleva 6 h muy por encima de lo normal. Dato: 0,27 (Naranja desde 0,25)." {
		t.Fatalf("mensajes = %+v", m)
	}
}

// Una subida cuya hora anterior no estaba limitada conserva el texto de siempre.
func TestDecide_RiseWithoutPreviousLimitKeepsTheText(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false), planitud(30, 10, "")),
		calc(1, 0.18, A, A, opened, hoursAbove(2), planitud(30, 10, "")),
		calc(2, 0.27, O, O, none, planitud(30, 10, "")),
	)
	m := messageOf(w, messages.CodeUpNaranjaFast)
	if len(m) != 1 || m[0].Body != "El índice de anomalía acústica está subiendo más rápido de lo normal. Dato: pasó de 0,18 a 0,27 (Naranja desde 0,25)." {
		t.Fatalf("mensajes = %+v", m)
	}
}

// Sin mínimo de planitud en la versión no hay firma de fuga ni mensajes nuevos.
func TestDecide_NoPlanitudMinimumMeansNoLeakSignature(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.19, A, A, none),
	)
	if s := w.alerts[123].LeakSignature; s != nil {
		t.Fatalf("firma = %+v; se esperaba nil", s)
	}
	if slices.Contains(w.codes(), messages.CodeNoLeakShape) {
		t.Fatal("NO_LEAK_SHAPE sin planitud")
	}
}
