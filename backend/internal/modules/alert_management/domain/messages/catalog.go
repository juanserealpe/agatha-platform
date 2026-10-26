// Package messages es el catálogo estático de textos de la línea de tiempo
// (sección 6.2): código → título y cuerpo con variables entre llaves. Las
// reglas devuelven código y variables; el texto se renderiza acá y se guarda
// tal como se mostró, con MessageCatalogVersion. Lenguaje de campo: nunca
// "I", "umbral" ni "persistencia".
package messages

import (
	"fmt"
	"regexp"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// MessageCatalogVersion se guarda en cada mensaje. Subirla al cambiar un texto.
const MessageCatalogVersion = 1

// LowConfidenceMark se agrega con low_confidence = true (sección 6.1).
const LowConfidenceMark = "Sensor en ajuste: resultado preliminar."

// Códigos de mensaje (sección 6.1).
const (
	CodeOpen                 = "OPEN"
	CodeUpNaranjaPersist     = "UP_NARANJA_PERSIST"
	CodeUpNaranjaFast        = "UP_NARANJA_FAST"
	CodeUpRojo               = "UP_ROJO"
	CodeReactivated          = "REACTIVATED"
	CodeNewAfterClassified   = "NEW_AFTER_CLASSIFIED"
	CodeUpDuringIntervention = "UP_DURING_INTERVENTION"
	CodeNoCalcStart          = "NO_CALC_START"
	CodeNoCalcEnd            = "NO_CALC_END"
	CodeDowngrade            = "DOWNGRADE"
	CodeRojoHeld             = "ROJO_HELD"
	CodeOscillating          = "OSCILLATING"
	CodeAlmostNormal         = "ALMOST_NORMAL"
	CodeOneHourNormal        = "ONE_HOUR_NORMAL"
	CodeBackAbnormal         = "BACK_ABNORMAL"
	CodeDowngradeStart       = "DOWNGRADE_START"
	CodeCountdown            = "COUNTDOWN"
	CodeTrend                = "TREND"
	CodeSummary              = "SUMMARY"
	CodeEndNormal            = "END_NORMAL"
	CodeClassified           = "CLASSIFIED"
	CodeLabeled              = "LABELED"
)

// Variantes de un mismo código (el código guardado no cambia).
const (
	VariantAmarillo = "AMARILLO"
	VariantNaranja  = "NARANJA"
	VariantRojo     = "ROJO"
	VariantPersist  = "PERSIST"
)

// Template es un texto del catálogo.
type Template struct {
	Code  string
	Kind  entities.MessageKind
	Title string
	Body  string
	// Vars son las variables que el código entrega (una prueba verifica que
	// el texto no use otras).
	Vars []string
}

// Key arma la clave de un código con su variante ("" = sin variante).
func Key(code, variant string) string {
	if variant == "" {
		return code
	}
	return code + "/" + variant
}

var catalog = map[string]Template{
	Key(CodeOpen, VariantAmarillo): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva: ruido anormal en el sensor {pk}",
		Body:  "Sensor {pk}: el sensor escucha un ruido mayor de lo normal desde las {hora_inicio}. Por ahora no se requiere salir a campo.",
		Vars:  []string{"pk", "hora_inicio"},
	},
	Key(CodeOpen, VariantNaranja): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva: ruido fuerte en el sensor {pk}",
		Body:  "Sensor {pk}: ruido fuerte en la tubería desde las {hora_inicio}. Programar recorrido a la zona {zona}.",
		Vars:  []string{"pk", "hora_inicio", "zona"},
	},
	Key(CodeOpen, VariantRojo): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva: ruido muy fuerte en el sensor {pk}",
		Body:  "Sensor {pk}: ruido fuerte en la tubería desde las {hora_inicio}. Enviar cuadrilla a la zona {zona}.",
		Vars:  []string{"pk", "hora_inicio", "zona"},
	},
	Key(CodeUpNaranjaPersist, ""): {
		Code: CodeUpNaranjaPersist, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Naranja",
		Body:  "La alerta {codigo} subió a Naranja: el ruido anormal lleva {N} h seguidas sin bajar. Programar recorrido de la zona {zona}.",
		Vars:  []string{"codigo", "N", "zona"},
	},
	Key(CodeUpNaranjaFast, ""): {
		Code: CodeUpNaranjaFast, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Naranja",
		Body:  "La alerta {codigo} subió a Naranja: el ruido aumentó rápidamente. Programar recorrido de la zona {zona}.",
		Vars:  []string{"codigo", "zona"},
	},
	Key(CodeUpRojo, ""): {
		Code: CodeUpRojo, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Rojo",
		Body:  "La alerta {codigo} subió a Rojo: ruido muy fuerte. Enviar cuadrilla a la zona {zona} y avisar al centro de control.",
		Vars:  []string{"codigo", "zona"},
	},
	Key(CodeUpRojo, VariantPersist): {
		Code: CodeUpRojo, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Rojo",
		Body:  "La alerta {codigo} subió a Rojo: ruido fuerte sostenido {N} h. Enviar cuadrilla a la zona {zona} y avisar al centro de control.",
		Vars:  []string{"codigo", "N", "zona"},
	},
	// El kind de REACTIVATED depende de si supera el máximo: lo fija la regla.
	Key(CodeReactivated, ""): {
		Code: CodeReactivated, Kind: entities.KindUpdate,
		Title: "La alerta {codigo} se reactiva",
		Body:  "El ruido volvió {horas} h después de terminar. La alerta {codigo} se reactiva en {nivel}.",
		Vars:  []string{"horas", "codigo", "nivel"},
	},
	Key(CodeNewAfterClassified, ""): {
		Code: CodeNewAfterClassified, Kind: entities.KindNotification,
		Title: "Alerta nueva en el sensor {pk}",
		Body:  "Alerta nueva: la anterior ({codigo_anterior}) ya había sido clasificada como {clasificacion}.",
		Vars:  []string{"pk", "codigo_anterior", "clasificacion"},
	},
	Key(CodeUpDuringIntervention, ""): {
		Code: CodeUpDuringIntervention, Kind: entities.KindUpdate,
		Title: "Subió durante la intervención",
		Body:  "El ruido subió a {nivel} durante la intervención registrada (sin sonido).",
		Vars:  []string{"nivel"},
	},
	Key(CodeNoCalcStart, ""): {
		Code: CodeNoCalcStart, Kind: entities.KindUpdate,
		Title: "Sin datos suficientes",
		Body:  "No hay datos suficientes para evaluar desde las {hora}; la alerta sigue en {nivel}.",
		Vars:  []string{"hora", "nivel"},
	},
	Key(CodeNoCalcEnd, ""): {
		Code: CodeNoCalcEnd, Kind: entities.KindUpdate,
		Title: "Vuelve a haber datos",
		Body:  "Vuelve a haber datos; la alerta sigue en {nivel}.",
		Vars:  []string{"nivel"},
	},
	Key(CodeDowngradeStart, ""): {
		Code: CodeDowngradeStart, Kind: entities.KindUpdate,
		Title: "El ruido bajó",
		Body:  "El ruido bajó a nivel {nivel_hora}. Si se mantiene así {horas} h, la alerta bajará a {nivel_inferior}.",
		Vars:  []string{"nivel_hora", "horas", "nivel_inferior"},
	},
	Key(CodeDowngrade, ""): {
		Code: CodeDowngrade, Kind: entities.KindUpdate,
		Title: "La alerta bajó a {nivel_inferior}",
		Body:  "El ruido se mantuvo por debajo de {nivel} durante {horas} h; la alerta baja a {nivel_inferior}. Máximo alcanzado: {max}.",
		Vars:  []string{"nivel", "horas", "nivel_inferior", "max"},
	},
	Key(CodeRojoHeld, ""): {
		Code: CodeRojoHeld, Kind: entities.KindUpdate,
		Title: "Sigue en rojo hasta que se clasifique",
		Body:  "El ruido bajó hace {horas} h, pero la alerta sigue en rojo hasta que alguien la clasifique.",
		Vars:  []string{"horas"},
	},
	Key(CodeCountdown, ""): {
		Code: CodeCountdown, Kind: entities.KindUpdate,
		Title: "Puede subir a {nivel_siguiente}",
		Body:  "Lleva {N} h seguidas. Si sigue {horas} h más, pasa a {nivel_siguiente}.",
		Vars:  []string{"N", "horas", "nivel_siguiente"},
	},
	Key(CodeTrend, ""): {
		Code: CodeTrend, Kind: entities.KindUpdate,
		Title: "El ruido está {direccion}",
		Body:  "El ruido está {direccion}. La alerta sigue en {nivel} (lleva {N} h).",
		Vars:  []string{"direccion", "nivel", "N"},
	},
	Key(CodeAlmostNormal, ""): {
		Code: CodeAlmostNormal, Kind: entities.KindUpdate,
		Title: "Casi normal",
		// El evento v3 no trae exit_hours del Rule Engine: el texto no lo cita.
		Body: "El ruido volvió casi a lo normal; la alerta terminará si sigue así.",
		Vars: []string{},
	},
	Key(CodeOneHourNormal, ""): {
		Code: CodeOneHourNormal, Kind: entities.KindUpdate,
		Title: "1 h con ruido normal",
		Body:  "1 h con ruido normal.",
		Vars:  []string{},
	},
	Key(CodeBackAbnormal, ""): {
		Code: CodeBackAbnormal, Kind: entities.KindUpdate,
		Title: "El ruido anormal volvió",
		Body:  "El ruido anormal volvió; sigue en {nivel}.",
		Vars:  []string{"nivel"},
	},
	Key(CodeOscillating, ""): {
		Code: CodeOscillating, Kind: entities.KindUpdate,
		Title: "El ruido oscila",
		Body:  "El ruido oscila alrededor del límite desde las {hora}. No se informará cada subida y bajada.",
		Vars:  []string{"hora"},
	},
	Key(CodeSummary, ""): {
		Code: CodeSummary, Kind: entities.KindSummary,
		Title: "Sin cambios",
		Body:  "Sin cambios: sigue en {nivel}, lleva {N} h, ruido {tendencia}.",
		Vars:  []string{"nivel", "N", "tendencia"},
	},
	Key(CodeEndNormal, ""): {
		Code: CodeEndNormal, Kind: entities.KindEnd,
		Title: "La alerta {codigo} terminó",
		Body:  "La alerta terminó: el ruido volvió a lo normal. Duró {N} h; nivel máximo {max}. Si vuelve en las próximas {horas} h y no se ha clasificado, se reactivará esta misma alerta.",
		Vars:  []string{"codigo", "N", "max", "horas"},
	},
	Key(CodeClassified, ""): {
		Code: CodeClassified, Kind: entities.KindManagement,
		Title: "Clasificada como {clasificacion}",
		Body:  "Clasificada como {clasificacion} desde {origen}.",
		Vars:  []string{"clasificacion", "origen"},
	},
	Key(CodeLabeled, ""): {
		Code: CodeLabeled, Kind: entities.KindManagement,
		Title: "Etiqueta: {etiqueta}",
		Body:  "Etiqueta: {etiqueta}, desde {origen}.",
		Vars:  []string{"etiqueta", "origen"},
	},
}

// Text es un mensaje renderizado.
type Text struct {
	Code  string
	Kind  entities.MessageKind
	Title string
	Body  string
}

var placeholderRe = regexp.MustCompile(`\{([a-zA-Z_]+)\}`)

// Render arma el texto de la clave con las variables. Falta una variable o
// la clave no existe ⇒ error (es un error de programación: lo atrapan las pruebas).
func Render(key string, params map[string]string) (Text, error) {
	tpl, ok := catalog[key]
	if !ok {
		return Text{}, fmt.Errorf("messages: clave desconocida %q", key)
	}
	var missing error
	replace := func(s string) string {
		return placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
			name := m[1 : len(m)-1]
			v, ok := params[name]
			if !ok && missing == nil {
				missing = fmt.Errorf("messages: %s sin la variable %q", key, name)
			}
			return v
		})
	}
	text := Text{Code: tpl.Code, Kind: tpl.Kind, Title: replace(tpl.Title), Body: replace(tpl.Body)}
	return text, missing
}

// WithLowConfidence agrega la marca de resultado preliminar.
func WithLowConfidence(t Text) Text {
	t.Body += " " + LowConfidenceMark
	return t
}

// LevelName es el nivel en lenguaje de campo.
func LevelName(l entities.Level) string {
	switch l {
	case entities.LevelAmarillo:
		return "Amarillo"
	case entities.LevelNaranja:
		return "Naranja"
	case entities.LevelRojo:
		return "Rojo"
	default:
		return "normal"
	}
}

// ClassificationName es la clasificación en lenguaje de campo.
func ClassificationName(c entities.Classification) string {
	if c == entities.ClassificationFalseAlarm {
		return "Falsa alerta"
	}
	return "Confirmada"
}

// colombia es la zona en la que se muestran las horas (RN-11).
var colombia = loadColombia()

func loadColombia() *time.Location {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		// Colombia no tiene horario de verano.
		return time.FixedZone("America/Bogota", -5*60*60)
	}
	return loc
}

// Colombia es la zona en la que se muestran las fechas (RN-11).
func Colombia() *time.Location { return colombia }

// FormatHour es una hora en lenguaje de campo, en hora de Colombia: "11:00 del 24/08".
func FormatHour(t time.Time) string {
	return t.In(colombia).Format("15:04 del 02/01")
}
