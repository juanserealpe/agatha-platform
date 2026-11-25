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

// v11: cada texto dice QUÉ cambia. Un nivel siempre es el de la alerta y
// ningún título queda sin sujeto ("Sigue en Rojo", "Sin cambios").
func TestCatalog_SaysWhatChanges(t *testing.T) {
	vague := []string{"cambio de nivel", "completar el cierre", "se mantiene así.", "máximo alcanzado",
		"sigue en {nivel}", "sigue en {nivel_enviado}", "se reactiva en {nivel}", "nivel máximo:"}
	vagueTitles := []string{"Sigue en Rojo", "Puede subir a {nivel_siguiente}", "Sin cambios", "Casi normal",
		"Sin datos suficientes", "Vuelve a haber datos", "Volvió a la normalidad",
		"Clasificada como {clasificacion}", "Etiqueta: {etiqueta}", "Resumen del día {n} de la alerta"}
	for key, tpl := range catalog {
		text := strings.ToLower(tpl.Title + " " + tpl.Body)
		for _, v := range vague {
			if strings.Contains(text, v) {
				t.Errorf("%s no dice qué cambia (%q): %s", key, v, text)
			}
		}
		if slices.Contains(vagueTitles, tpl.Title) {
			t.Errorf("%s tiene un título sin sujeto: %q", key, tpl.Title)
		}
	}
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
	text, err := Render(Key(CodeCountdown, ""), map[string]string{"N": "9", "horas": "3", "nivel": "Amarillo", "nivel_siguiente": "Naranja", "corte": "0,16", "valor": "0,19"})
	if err != nil {
		t.Fatal(err)
	}
	if text.Body != "El índice de anomalía acústica lleva 9 h por encima del límite de Amarillo (0,16) y ahora está en 0,19. Si sigue así 3 h más, la alerta sube a Naranja." || text.Title != "La alerta puede subir a Naranja" {
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

// Todo mensaje del procesamiento horario de FUGA cita el índice de anomalía acústica
// (los de movimiento y temperatura citan su propio dato: §8.3).
func TestCatalog_ProcessingMessagesCiteTheIndex(t *testing.T) {
	for key, tpl := range catalog {
		if tpl.Kind == entities.KindManagement || IsMotionKey(key) {
			continue
		}
		if !strings.Contains(strings.ToLower(tpl.Body), "índice") {
			t.Errorf("%s no cita el índice de anomalía acústica: %q", key, tpl.Body)
		}
	}
}

// Ningún texto da un veredicto de fuga: nunca dice si es o no una fuga.
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

// v7 §8.3 / v8 §10.1: los TEMP_* citan un dato ("Dato: …"); los de
// movimiento llevan el número en la frase (TestCatalog_MotionV8Language
// prohíbe "Dato:"); el fin cita la duración. Ninguno usa las palabras de una
// interpretación.
func TestCatalog_MotionMessagesCiteDataAndStayNeutral(t *testing.T) {
	banned := []string{"umbral", "persistencia", "cierre", "cerrado", "reapertura", "flujo de gas"}
	motion := 0
	for key, tpl := range catalog {
		if !IsMotionKey(key) {
			continue
		}
		motion++
		if strings.HasPrefix(key, "TEMP_") && !strings.Contains(tpl.Body, "Dato:") {
			t.Errorf("%s no cita un dato: %q", key, tpl.Body)
		}
		if key == Key(CodeEndNormal, VariantMotion) && !strings.Contains(tpl.Body, "{duracion}") {
			t.Errorf("%s no cita la duración: %q", key, tpl.Body)
		}
		text := strings.ToLower(tpl.Title + " " + tpl.Body)
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s usa %q: %s", key, b, text)
			}
		}
	}
	// v8: 4 variantes de MOV_OPEN, 3 de MOV_DISPLACED, MOV_LEVEL_UP,
	// MOV_LEVEL_OBSERVED, MOV_SLOW_ONGOING, 3 de MOV_DAILY, MOV_RETURN, los 4
	// TEMP_* y END_NORMAL/MOVIMIENTO; sin los del acople (LEAK_JOINED, MOV_JOINED, PART_NORMAL).
	if motion != 19 {
		t.Fatalf("códigos de movimiento = %d, se esperaban 19", motion)
	}
}

// FUGA y MOVIMIENTO no se combinan: no quedan los códigos del acople.
func TestCatalog_NoCouplingCodes(t *testing.T) {
	for key := range catalog {
		for _, code := range []string{"LEAK_JOINED", "MOV_JOINED", "PART_NORMAL"} {
			if strings.HasPrefix(key, code) {
				t.Fatalf("sigue la clave %s", key)
			}
		}
	}
}

// v7: UP_DURING_INTERVENTION ya no existe.
func TestCatalog_NoInterventionCode(t *testing.T) {
	for key := range catalog {
		if strings.HasPrefix(key, "UP_DURING_INTERVENTION") {
			t.Fatalf("sigue la clave %s", key)
		}
	}
}

func TestFormatDegreesAndDuration(t *testing.T) {
	for v, want := range map[float64]string{0.08: "+0,08°", -0.115: "-0,12°", 0: "0,00°", 0.035: "+0,04°", -0.07: "-0,07°"} {
		if got := FormatDegrees(v); got != want {
			t.Errorf("FormatDegrees(%v) = %q, se esperaba %q", v, got, want)
		}
	}
	for h, want := range map[int]string{1: "1 h", 23: "23 h", 47: "47 h", 48: "2 días", 53: "2 días y 5 h", 120: "5 días"} {
		if got := FormatDuration(h); got != want {
			t.Errorf("FormatDuration(%d) = %q, se esperaba %q", h, got, want)
		}
	}
}

func TestRender_ManagementWithTimeRecordAndObservation(t *testing.T) {
	text, err := Render(Key(CodeClassified, ""), map[string]string{
		"clasificacion": "Falsa alerta", "origen": "desde la app", "fecha": "21/10", "hora": "14:23",
		"registro": " Hecha en el celular a las 13:50.", "observacion": " Observación: «maniobra de TGI»",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "Registrada desde la app el 21/10 a las 14:23. Hecha en el celular a las 13:50. Observación: «maniobra de TGI»"
	if text.Body != want || text.Kind != entities.KindManagement {
		t.Fatalf("body = %q", text.Body)
	}
}

// v10 retiró los mensajes de la confirmación por planitud.
func TestCatalog_PlanitudMessagesWereRetired(t *testing.T) {
	if MessageCatalogVersion < 10 {
		t.Fatalf("MessageCatalogVersion = %d, se esperaba 10 o más", MessageCatalogVersion)
	}
	for key, tpl := range catalog {
		for _, retired := range []string{"NO_LEAK_SHAPE", "SIN_FIRMA", "CONFIRMADA"} {
			if strings.Contains(key, retired) {
				t.Errorf("la clave %s debía retirarse", key)
			}
		}
		if strings.Contains(tpl.Body, "{d_pl") || strings.Contains(strings.Join(tpl.Vars, ","), "d_pl") {
			t.Errorf("%s todavía cita la planitud", key)
		}
	}
}
