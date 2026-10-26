package messages

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`\{([a-zA-Z_]+)\}`)

func placeholders(s string) []string {
	var out []string
	for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// Un texto no puede usar una variable que su código no declara (y entrega).
func TestCatalog_TextsOnlyUseDeclaredVariables(t *testing.T) {
	for key, tpl := range catalog {
		for _, v := range append(placeholders(tpl.Title), placeholders(tpl.Body)...) {
			if !slices.Contains(tpl.Vars, v) {
				t.Errorf("%s usa {%s}, que no está en Vars %v", key, v, tpl.Vars)
			}
		}
	}
}

// Lenguaje de campo: nunca "I", "umbral" ni "persistencia".
func TestCatalog_FieldLanguage(t *testing.T) {
	for key, tpl := range catalog {
		text := strings.ToLower(tpl.Title + " " + tpl.Body)
		for _, banned := range []string{"umbral", "persistencia", " i ", "i(24"} {
			if strings.Contains(" "+text+" ", banned) {
				t.Errorf("%s usa %q: %s", key, banned, text)
			}
		}
	}
}

func TestRender_FailsOnMissingVariable(t *testing.T) {
	if _, err := Render(Key(CodeCountdown, ""), map[string]string{"N": "9"}); err == nil {
		t.Fatal("se esperaba error: falta nivel_siguiente")
	}
	if _, err := Render("NO_EXISTE", nil); err == nil {
		t.Fatal("se esperaba error: código desconocido")
	}
}

func TestRender_ReplacesAndMarksLowConfidence(t *testing.T) {
	text, err := Render(Key(CodeCountdown, ""), map[string]string{"N": "9", "horas": "3", "nivel_siguiente": "Naranja"})
	if err != nil {
		t.Fatal(err)
	}
	if text.Body != "Lleva 9 h seguidas. Si sigue 3 h más, pasa a Naranja." {
		t.Fatalf("body = %q", text.Body)
	}
	if got := WithLowConfidence(text).Body; !strings.HasSuffix(got, LowConfidenceMark) {
		t.Fatalf("sin marca preliminar: %q", got)
	}
}

func TestCatalog_EveryCodeHasKind(t *testing.T) {
	for key, tpl := range catalog {
		if tpl.Kind == "" || tpl.Title == "" || tpl.Body == "" {
			t.Errorf("%s incompleto: %+v", key, tpl)
		}
	}
}
