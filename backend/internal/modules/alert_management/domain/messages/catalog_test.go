package messages

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
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
	if _, err := Render(Key(CodeCountdown, ""), map[string]string{"N": "9", "corte": "0,16", "valor": "0,19"}); err == nil {
		t.Fatal("se esperaba error: falta nivel_siguiente")
	}
	if _, err := Render("NO_EXISTE", nil); err == nil {
		t.Fatal("se esperaba error: código desconocido")
	}
}

func TestRender_ReplacesAndMarksLowConfidence(t *testing.T) {
	text, err := Render(Key(CodeCountdown, ""), map[string]string{"N": "9", "horas": "3", "nivel_siguiente": "Naranja", "corte": "0,16", "valor": "0,19"})
	if err != nil {
		t.Fatal(err)
	}
	if text.Body != "El índice de ruido lleva 9 h por encima de 0,16 (ahora en 0,19); faltan 3 h para el cambio de nivel." || text.Title != "Puede subir a Naranja" {
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
	params := sampleParams()
	params["pk"] = name
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

// El índice se muestra en decimal con coma: 2 decimales como mínimo, 3 si
// hacen falta (0,158 no se confunde con el corte 0,16).
func TestFormatValue(t *testing.T) {
	for v, want := range map[float64]string{0.16: "0,16", 0.2: "0,20", 0.158: "0,158", 0.1734: "0,173", 1: "1,00", 0.4: "0,40"} {
		if got := FormatValue(v); got != want {
			t.Errorf("FormatValue(%v) = %q, se esperaba %q", v, got, want)
		}
	}
}

// Todo mensaje del procesamiento horario cita el índice de ruido.
func TestCatalog_ProcessingMessagesCiteTheIndex(t *testing.T) {
	for key, tpl := range catalog {
		if tpl.Kind == entities.KindManagement {
			continue
		}
		if !strings.Contains(strings.ToLower(tpl.Body), "índice") {
			t.Errorf("%s no cita el índice de ruido: %q", key, tpl.Body)
		}
	}
}

// El porcentaje usa coma y un decimal como máximo: 4,6 % / 10 %.
func TestFormatPercent(t *testing.T) {
	for v, want := range map[float64]string{4.6: "4,6 %", 10: "10 %", 0: "0 %", 12.34: "12,3 %", 99.96: "100 %", 7.06: "7,1 %", 100: "100 %"} {
		if got := FormatPercent(v); got != want {
			t.Errorf("FormatPercent(%v) = %q, se esperaba %q", v, got, want)
		}
	}
}

// Ningún texto da un veredicto de fuga: se habla de "firma de fuga" (el % de
// registros con planitud alta frente al mínimo), nunca de si es o no una fuga.
func TestCatalog_NeverGivesALeakVerdict(t *testing.T) {
	banned := []string{"es una fuga", "no es una fuga", "parece", "forma de fuga", "es fuga", "hay una fuga", "no hay fuga"}
	for key, tpl := range catalog {
		text := strings.ToLower(tpl.Title + " " + tpl.Body)
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s da un veredicto (%q): %s", key, b, text)
			}
		}
	}
}

func TestCatalog_NoLeakSignatureTexts(t *testing.T) {
	params := map[string]string{
		"pk": "PK34+500", "hora_inicio": "12:00 del 14/11", "from": "Rojo", "d_pl": "4,6 %", "d_pl_min": "10 %",
		"valor": "0,36", "corte": "0,35", "normal": "0,16",
	}
	open, err := Render(Key(CodeOpen, VariantNoLeakSignature), params)
	if err != nil {
		t.Fatal(err)
	}
	if open.Kind != entities.KindNotification || open.Code != CodeOpen || open.Title != "Alerta nueva en PK34+500: nivel Amarillo" ||
		open.Body != "Por el índice de ruido la alerta estaría en Rojo desde las 12:00 del 14/11, pero solo el 4,6 % de los registros de las últimas 24 h tiene firma de fuga (se necesita 10 %). Dato: 0,36 (Rojo desde 0,35; lo normal es menos de 0,16)." {
		t.Fatalf("OPEN sin firma = %+v", open)
	}
	shape, err := Render(Key(CodeNoLeakShape, ""), params)
	if err != nil {
		t.Fatal(err)
	}
	if shape.Kind != entities.KindUpdate || shape.Title != "Se mantiene en Amarillo" ||
		shape.Body != "Por el índice de ruido la alerta estaría en Rojo, pero solo el 4,6 % de los registros de las últimas 24 h tiene firma de fuga (se necesita 10 %). Dato: 0,36 (Rojo desde 0,35)." {
		t.Fatalf("NO_LEAK_SHAPE = %+v", shape)
	}
}

// La subida con la planitud recién confirmada antepone la firma de fuga y no
// cambia el título ni el código guardado.
func TestCatalog_UpConfirmedVariantsPrependTheSignature(t *testing.T) {
	prefix := "El 12 % de los registros de las últimas 24 h tiene firma de fuga (mínimo 10 %). "
	params := map[string]string{"codigo": "ALR-000123", "N": "12", "normal": "0,16", "valor": "0,27", "valor_anterior": "0,20",
		"corte": "0,25", "d_pl": "12 %", "d_pl_min": "10 %"}
	for _, c := range []struct{ code, variant string }{
		{CodeUpNaranjaPersist, ""}, {CodeUpNaranjaFast, ""}, {CodeUpRojo, ""}, {CodeUpRojo, VariantPersist},
	} {
		plain, err := Render(Key(c.code, c.variant), params)
		if err != nil {
			t.Fatalf("%s/%s: %v", c.code, c.variant, err)
		}
		confirmed, err := Render(Key(c.code, ConfirmedVariant(c.variant)), params)
		if err != nil {
			t.Fatalf("%s/%s confirmada: %v", c.code, c.variant, err)
		}
		if confirmed.Code != c.code || confirmed.Title != plain.Title || confirmed.Kind != plain.Kind || confirmed.Body != prefix+plain.Body {
			t.Errorf("%s/%s confirmada = %+v (simple %+v)", c.code, c.variant, confirmed, plain)
		}
	}
}
