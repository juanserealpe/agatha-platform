package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
)

func TestThresholdParams_Validate(t *testing.T) {
	valid := DefaultThresholdParams()
	if err := valid.Validate(); err != nil {
		t.Fatalf("los valores por defecto deben ser válidos: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ThresholdParams)
		want   error
	}{
		{"amarillo = 0", func(p *ThresholdParams) { p.Amarillo = 0 }, domain.ErrThresholdOrder},
		{"amarillo = naranja", func(p *ThresholdParams) { p.Amarillo = p.Naranja }, domain.ErrThresholdOrder},
		{"naranja > rojo", func(p *ThresholdParams) { p.Naranja = 0.40 }, domain.ErrThresholdOrder},
		{"rojo > 1", func(p *ThresholdParams) { p.Rojo = 1.01 }, domain.ErrThresholdOrder},
		{"margen negativo", func(p *ThresholdParams) { p.ExitMargin = -0.01 }, domain.ErrExitMarginRange},
		{"margen = amarillo", func(p *ThresholdParams) { p.ExitMargin = p.Amarillo }, domain.ErrExitMarginRange},
		{"min_hours 0", func(p *ThresholdParams) { p.MinHours = 0 }, domain.ErrHoursRange},
		{"exit_hours 0", func(p *ThresholdParams) { p.ExitHours = 0 }, domain.ErrHoursRange},
		{"escalamiento 0", func(p *ThresholdParams) { p.EscalateNaranjaAfterHours = intPtr(0) }, domain.ErrHoursRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := DefaultThresholdParams()
			tt.mutate(&p)
			if err := p.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, se esperaba %v", err, tt.want)
			}
		})
	}

	t.Run("rojo = 1 y escalamientos NULL son válidos", func(t *testing.T) {
		p := DefaultThresholdParams()
		p.Rojo = 1
		p.EscalateAmarilloAfterHours, p.EscalateNaranjaAfterHours = nil, nil
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestThresholdParams_NormalizeRoundsToColumnPrecision(t *testing.T) {
	p := DefaultThresholdParams()
	p.Amarillo, p.Naranja = 0.18004, 0.18001 // distintos, pero iguales en NUMERIC(5,4)

	n := p.Normalize()
	if n.Amarillo != 0.18 || n.Naranja != 0.18 {
		t.Fatalf("normalize = %v / %v", n.Amarillo, n.Naranja)
	}
	if err := n.Validate(); !errors.Is(err, domain.ErrThresholdOrder) {
		t.Fatalf("tras redondear deben quedar iguales e inválidos: %v", err)
	}
}

func TestThresholdPatch_CopiesOmittedFields(t *testing.T) {
	base := DefaultThresholdParams()

	got := ThresholdPatch{
		Rojo:                      Some(0.40),
		EscalateNaranjaAfterHours: Some[*int](nil), // explícitamente sin escalamiento
	}.ApplyTo(base)

	if got.Rojo != 0.40 || got.EscalateNaranjaAfterHours != nil {
		t.Fatalf("campos informados no aplicados: %+v", got)
	}
	if got.Amarillo != base.Amarillo || got.Naranja != base.Naranja || got.MinHours != base.MinHours ||
		got.ExitMargin != base.ExitMargin || got.ExitHours != base.ExitHours ||
		got.EscalateAmarilloAfterHours == nil || *got.EscalateAmarilloAfterHours != 12 {
		t.Fatalf("campos omitidos no copiados: %+v", got)
	}

	// La copia no comparte punteros con la base.
	*got.EscalateAmarilloAfterHours = 99
	if *base.EscalateAmarilloAfterHours != 12 {
		t.Fatal("ApplyTo comparte el puntero del escalamiento con la base")
	}
}

func TestThresholdParams_HistoryDepth(t *testing.T) {
	p := DefaultThresholdParams()
	if got := p.HistoryDepth(); got != 13 { // máx(2, 12, 6, 2) + 1
		t.Fatalf("K = %d, se esperaba 13", got)
	}

	p.EscalateAmarilloAfterHours, p.EscalateNaranjaAfterHours = nil, nil
	p.ExitHours = 5
	if got := p.HistoryDepth(); got != 6 {
		t.Fatalf("K = %d, se esperaba 6", got)
	}
}

func TestThresholdVersion_IsEffectiveAt(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	closed := ThresholdVersion{ValidFrom: from, ValidTo: &to}
	if !closed.IsEffectiveAt(from) || closed.IsEffectiveAt(to) || closed.IsEffectiveAt(from.Add(-time.Second)) {
		t.Fatal("una versión cerrada aplica en [valid_from, valid_to)")
	}

	open := ThresholdVersion{ValidFrom: from}
	if !open.IsEffectiveAt(from.Add(1000 * time.Hour)) {
		t.Fatal("una versión vigente aplica desde valid_from en adelante")
	}
}

// Histéresis por nivel (RE-04): la salida de cada nivel es su corte − exit_margin,
// redondeada a 4 decimales para que 0,18 − 0,02 sea exactamente 0,16.
func TestThresholdParams_CutAndExitPerLevel(t *testing.T) {
	p := DefaultThresholdParams()

	tests := []struct {
		level     Level
		cut, exit float64
	}{
		{LevelAmarillo, 0.16, 0.14},
		{LevelNaranja, 0.25, 0.23},
		{LevelRojo, 0.35, 0.33},
		{LevelNormal, 0, 0},
	}
	for _, tt := range tests {
		if got := p.CutFor(tt.level); got != tt.cut {
			t.Errorf("CutFor(%s) = %v, se esperaba %v", tt.level, got, tt.cut)
		}
		if got := p.ExitFor(tt.level); got != tt.exit {
			t.Errorf("ExitFor(%s) = %v, se esperaba %v", tt.level, got, tt.exit)
		}
	}
	if p.ExitCutoff() != p.ExitFor(LevelAmarillo) {
		t.Fatal("ExitCutoff es la salida de AMARILLO")
	}
}

func TestThresholdParams_ValidatePlanitudAndWindow(t *testing.T) {
	ok := DefaultThresholdParams()
	f := func(v float64) *float64 { return &v }

	tests := []struct {
		name   string
		mutate func(p *ThresholdParams)
		want   error
	}{
		{"plantilla", func(*ThresholdParams) {}, nil},
		{"planitud sin d_pl_min", func(p *ThresholdParams) { p.ConfirmNaranjaWithPlanitud = true }, domain.ErrPlanitudConfig},
		{"rojo con planitud sin naranja", func(p *ThresholdParams) { p.ConfirmRojoWithPlanitud, p.DPlMin = true, f(10) }, domain.ErrRojoPlanitudConfig},
		{"rojo con planitud y naranja", func(p *ThresholdParams) {
			p.ConfirmNaranjaWithPlanitud, p.ConfirmRojoWithPlanitud, p.DPlMin = true, true, f(10)
		}, nil},
		{"naranja sin rojo", func(p *ThresholdParams) { p.ConfirmNaranjaWithPlanitud, p.DPlMin = true, f(10) }, nil},
		{"d_pl_min > 100", func(p *ThresholdParams) { p.DPlMin = f(101) }, domain.ErrPlanitudConfig},
		{"planitud válida", func(p *ThresholdParams) { p.ConfirmNaranjaWithPlanitud, p.DPlMin = true, f(12) }, nil},
		{"modo desconocido", func(p *ThresholdParams) { p.PersistenceMode = "ROLLING" }, domain.ErrPersistenceConfig},
		{"WINDOW sin horas", func(p *ThresholdParams) { p.PersistenceMode = PersistenceWindow }, domain.ErrPersistenceConfig},
		{"WINDOW min > ventana", func(p *ThresholdParams) {
			p.PersistenceMode, p.PersistenceWindowHours, p.PersistenceMinHours = PersistenceWindow, intPtr(6), intPtr(7)
		}, domain.ErrPersistenceConfig},
		{"WINDOW válida", func(p *ThresholdParams) {
			p.PersistenceMode, p.PersistenceWindowHours, p.PersistenceMinHours = PersistenceWindow, intPtr(24), intPtr(18)
		}, nil},
		{"ventana 0 en STRICT", func(p *ThresholdParams) { p.PersistenceWindowHours = intPtr(0) }, domain.ErrPersistenceConfig},
	}
	for _, tt := range tests {
		p := ok
		tt.mutate(&p)
		if got := p.Normalize().Validate(); got != tt.want {
			t.Errorf("%s: Validate() = %v, se esperaba %v", tt.name, got, tt.want)
		}
	}
}

func TestThresholdParams_NormalizeModeAndDPlMin(t *testing.T) {
	p := DefaultThresholdParams()
	p.PersistenceMode = " window "
	v := 12.345
	p.DPlMin = &v
	n := p.Normalize()
	if n.PersistenceMode != PersistenceWindow || *n.DPlMin != 12.35 {
		t.Fatalf("mode=%q d_pl_min=%v; se esperaba WINDOW y 12.35", n.PersistenceMode, *n.DPlMin)
	}
	if v != 12.345 {
		t.Fatal("Normalize no debe modificar el puntero original")
	}

	p.PersistenceMode = ""
	if p.Normalize().PersistenceMode != PersistenceStrict {
		t.Fatal("modo vacío ⇒ STRICT (versiones previas a la columna)")
	}
}

func TestThresholdParams_EscalationHoursByMode(t *testing.T) {
	p := DefaultThresholdParams()
	if *p.EscalationHours(LevelAmarillo) != 12 || *p.EscalationHours(LevelNaranja) != 6 {
		t.Fatal("STRICT usa escalate_*_after_hours")
	}

	p.PersistenceMode, p.PersistenceWindowHours, p.PersistenceMinHours = PersistenceWindow, intPtr(24), intPtr(18)
	p.EscalateNaranjaAfterHours = nil
	if *p.EscalationHours(LevelAmarillo) != 18 || p.EscalationHours(LevelNaranja) != nil {
		t.Fatal("WINDOW usa persistence_min_hours y respeta el escalamiento desactivado")
	}
	if p.HistoryDepth() != 25 {
		t.Fatalf("K = %d, se esperaba 25 (ventana 24 + 1)", p.HistoryDepth())
	}
}

func TestThresholdPatch_NewFields(t *testing.T) {
	base := DefaultThresholdParams()
	min := 12.0
	got := ThresholdPatch{
		ConfirmNaranjaWithPlanitud: Some(true),
		ConfirmRojoWithPlanitud:    Some(true),
		DPlMin:                     Some(&min),
		PersistenceMode:            Some(PersistenceWindow),
		PersistenceWindowHours:     Some(intPtr(24)),
		PersistenceMinHours:        Some(intPtr(18)),
	}.ApplyTo(base)

	if !got.ConfirmRojoWithPlanitud {
		t.Fatalf("confirm_rojo_with_planitud no aplicado: %+v", got)
	}
	if !got.ConfirmNaranjaWithPlanitud || *got.DPlMin != 12 || got.PersistenceMode != PersistenceWindow ||
		*got.PersistenceWindowHours != 24 || *got.PersistenceMinHours != 18 {
		t.Fatalf("parche no aplicado: %+v", got)
	}
	min = 99
	if *got.DPlMin != 12 {
		t.Fatal("ApplyTo comparte el puntero de d_pl_min con el parche")
	}

	if again := (ThresholdPatch{}).ApplyTo(got); !again.ConfirmNaranjaWithPlanitud || !again.ConfirmRojoWithPlanitud || again.PersistenceMode != PersistenceWindow {
		t.Fatal("un parche vacío copia los campos nuevos de la base")
	}
}
