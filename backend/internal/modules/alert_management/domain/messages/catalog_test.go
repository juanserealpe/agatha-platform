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
	if text.Body != "Lleva 9 h seguidas; faltan 3 h para el cambio de nivel." || text.Title != "Puede subir a Naranja" {
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

// El nombre del dispositivo ya trae "Sensor …" (devices.name): ningún texto lo
// antepone de nuevo ("Sensor Sensor MP-11556 PK34-470").
func TestCatalog_DeviceNameIsNotPrefixedAgain(t *testing.T) {
	name := "Sensor MP-11556 PK34-470"
	params := map[string]string{
		"pk": name, "nivel": "Naranja", "hora_inicio": "05:00 del 24/08", "codigo": "ALR-000123", "N": "3",
		"codigo_anterior": "ALR-000122", "clasificacion": "Falsa alerta",
	}
	for key, tpl := range catalog {
		if !slices.Contains(tpl.Vars, "pk") {
			continue
		}
		text, err := Render(key, params)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if all := strings.ToLower(text.Title + " " + text.Body); strings.Contains(all, "sensor sensor") {
			t.Errorf("%s duplica el nombre: %q / %q", key, text.Title, text.Body)
		}
	}
}

// sampleParams tiene un valor para cada variable del catálogo.
func sampleParams() map[string]string {
	out := map[string]string{}
	for _, tpl := range catalog {
		for _, v := range tpl.Vars {
			out[v] = "<" + v + ">"
		}
	}
	return out
}

// Los mensajes informan el estado y la criticidad; nunca dicen qué hacer.
func TestCatalog_NeverTellsWhatToDo(t *testing.T) {
	banned := []string{"enviar", "cuadrilla", "programar", "recorrido", "avisar", "centro de control",
		"salir a campo", "se requiere", "revisar", "verificar", "zona"}
	for key, tpl := range catalog {
		text := strings.ToLower(tpl.Title + " " + tpl.Body)
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s indica una acción (%q): %s", key, b, text)
			}
		}
	}
}

// El cuerpo no repite el título: agrega información.
func TestCatalog_BodyDoesNotRepeatTitle(t *testing.T) {
	params := sampleParams()
	for key := range catalog {
		text, err := Render(key, params)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		title := strings.ToLower(strings.TrimRight(text.Title, ".:"))
		if strings.Contains(strings.ToLower(text.Body), title) {
			t.Errorf("%s repite el título en el cuerpo: %q / %q", key, text.Title, text.Body)
		}
	}
}

// Una alerta nueva dice su criticidad en el título.
func TestCatalog_NewAlertTitleSaysTheLevel(t *testing.T) {
	for variant, name := range map[string]string{VariantAmarillo: "Amarillo", VariantNaranja: "Naranja", VariantRojo: "Rojo"} {
		text, _ := Render(Key(CodeOpen, variant), sampleParams())
		if !strings.Contains(text.Title, name) {
			t.Errorf("OPEN/%s: el título no dice el nivel: %q", variant, text.Title)
		}
	}
	text, _ := Render(Key(CodeNewAfterClassified, ""), sampleParams())
	if !strings.Contains(text.Title, "<nivel>") {
		t.Errorf("NEW_AFTER_CLASSIFIED: el título no dice el nivel: %q", text.Title)
	}
}
