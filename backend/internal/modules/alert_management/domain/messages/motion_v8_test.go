package messages

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// §10.1: grados con signo y dos decimales, medio alejándose de cero sobre el
// valor de 3 decimales (0,285 se guarda en binario como 0,28499…: igual da 0,29).
func TestFormatDegrees_HalfUpOnThreeDecimals(t *testing.T) {
	for v, want := range map[float64]string{
		0.12: "+0,12°", -0.25: "-0,25°", 0: "0,00°", -0.004: "0,00°", -0.105: "-0,11°", 0.285: "+0,29°",
		-0.285: "-0,29°", 1.005: "+1,01°", 0.035: "+0,04°", -0.115: "-0,12°", 0.08: "+0,08°",
	} {
		if got := FormatDegrees(v); got != want {
			t.Errorf("FormatDegrees(%v) = %q, se esperaba %q", v, got, want)
		}
	}
	if got := FormatDegreesAbs(-0.04); got != "0,04°" {
		t.Errorf("FormatDegreesAbs = %q", got)
	}
	if got := FormatDegreesPtr(nil); got != "s/d" {
		t.Errorf("FormatDegreesPtr(nil) = %q", got)
	}
	for v, want := range map[float64]string{-0.0267: "-0,027° por día", 0.0002: "0,000° por día", 0.0125: "+0,013° por día"} {
		if got := FormatRatePerDay(v); got != want {
			t.Errorf("FormatRatePerDay(%v) = %q, se esperaba %q", v, got, want)
		}
	}
}

func TestFormatSpanWindowAndList(t *testing.T) {
	for h, want := range map[float64]string{23: "23 h", 22.75: "23 h", 24: "1 día", 26.5: "1 día y 3 h", 55: "2 días y 7 h", 248: "10 días y 8 h"} {
		if got := FormatSpan(h); got != want {
			t.Errorf("FormatSpan(%v) = %q, se esperaba %q", h, got, want)
		}
	}
	for h, want := range map[int]string{1: "1 hora", 3: "3 horas", 72: "3 días", 168: "7 días", 361: "15 días", 720: "30 días"} {
		if got := FormatWindow(h); got != want {
			t.Errorf("FormatWindow(%d) = %q, se esperaba %q", h, got, want)
		}
	}
	if got := JoinList([]string{"12/10", "13/10", "19/10"}); got != "12/10, 13/10 y 19/10" {
		t.Errorf("JoinList = %q", got)
	}
	a := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)
	if got := Between(a, a.Add(3*time.Hour)); got != "Entre las 22:00 del 11/10 y las 01:00 del 12/10" {
		t.Errorf("Between = %q", got)
	}
}

// La regex de Render solo reconoce [a-zA-Z_]: una variable con dígitos
// ({dp_24h}) quedaría sin reemplazar sin error.
func TestCatalog_VariableNamesAreRenderable(t *testing.T) {
	name := regexp.MustCompile(`^[a-zA-Z_]+$`)
	any := regexp.MustCompile(`\{[^}]*\}`)
	for key, tpl := range catalog {
		for _, v := range tpl.Vars {
			if !name.MatchString(v) {
				t.Errorf("%s declara %q, que Render no reemplaza", key, v)
			}
		}
		for _, m := range any.FindAllString(tpl.Title+tpl.Body, -1) {
			if !placeholderRe.MatchString(m) {
				t.Errorf("%s usa %s, que Render no reemplaza", key, m)
			}
		}
	}
}

// §10.5: ningún texto de movimiento dice qué hacer ni interpreta la causa.
func TestCatalog_MotionV8Language(t *testing.T) {
	banned := []string{"umbral", "regla", "persistencia", "cierre", "cerrado", "reapertura", "flujo de gas", "terreno",
		"deslizamiento", "maniobra", "revisar", "verificar", "recomend", "debe"}
	for key, tpl := range catalog {
		code, _, _ := strings.Cut(key, "/")
		if !IsMotionKey(key) || strings.HasPrefix(code, "TEMP_") {
			continue
		}
		text := strings.ToLower(tpl.Title + " " + tpl.Body)
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s usa %q", key, b)
			}
		}
		if strings.Contains(tpl.Body, "Dato:") {
			t.Errorf("%s usa \"Dato:\": en movimiento el número va en la frase", key)
		}
	}
	if s := ObservedLevelText(entities.LevelNaranja, entities.LevelNaranja); s != "" {
		t.Errorf("{calculado} con el nivel activo = %q", s)
	}
}
