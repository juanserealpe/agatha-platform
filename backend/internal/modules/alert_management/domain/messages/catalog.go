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
// v2: {pk} sin "sensor" delante (devices.name ya lo trae).
// v3: el cuerpo no repite el título y ningún texto dice qué hacer (sin zona).
const MessageCatalogVersion = 3

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

// catalog: el título dice qué pasó (y la criticidad); el cuerpo agrega el
// detalle sin repetir el título. Ningún texto indica qué hacer: solo el
// estado de la alerta (lo verifican las pruebas).
var catalog = map[string]Template{
	Key(CodeOpen, VariantAmarillo): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Amarillo",
		Body:  "Ruido mayor de lo normal desde las {hora_inicio}.",
		Vars:  []string{"pk", "hora_inicio"},
	},
	Key(CodeOpen, VariantNaranja): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Naranja",
		Body:  "Ruido fuerte en la tubería desde las {hora_inicio}.",
		Vars:  []string{"pk", "hora_inicio"},
	},
	Key(CodeOpen, VariantRojo): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Rojo",
		Body:  "Ruido muy fuerte en la tubería desde las {hora_inicio}.",
		Vars:  []string{"pk", "hora_inicio"},
	},
	Key(CodeUpNaranjaPersist, ""): {
		Code: CodeUpNaranjaPersist, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Naranja",
		Body:  "El ruido anormal lleva {N} h seguidas sin bajar.",
		Vars:  []string{"codigo", "N"},
	},
	Key(CodeUpNaranjaFast, ""): {
		Code: CodeUpNaranjaFast, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Naranja",
		Body:  "El ruido aumentó rápidamente.",
		Vars:  []string{"codigo"},
	},
	Key(CodeUpRojo, ""): {
		Code: CodeUpRojo, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Rojo",
		Body:  "Ruido muy fuerte en la tubería.",
		Vars:  []string{"codigo"},
	},
	Key(CodeUpRojo, VariantPersist): {
		Code: CodeUpRojo, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Rojo",
		Body:  "Ruido fuerte sostenido durante {N} h.",
		Vars:  []string{"codigo", "N"},
	},
	// El kind de REACTIVATED depende de si supera el máximo: lo fija la regla.
	Key(CodeReactivated, ""): {
		Code: CodeReactivated, Kind: entities.KindUpdate,
		Title: "La alerta {codigo} se reactiva en {nivel}",
		Body:  "El ruido volvió {horas} h después de terminar.",
		Vars:  []string{"horas", "codigo", "nivel"},
	},
	Key(CodeNewAfterClassified, ""): {
		Code: CodeNewAfterClassified, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel {nivel}",
		Body:  "La anterior ({codigo_anterior}) ya había sido clasificada como {clasificacion}.",
		Vars:  []string{"pk", "nivel", "codigo_anterior", "clasificacion"},
	},
	Key(CodeUpDuringIntervention, ""): {
		Code: CodeUpDuringIntervention, Kind: entities.KindUpdate,
		Title: "Subió a {nivel} durante la intervención",
		Body:  "Por la intervención registrada, esta subida no se notifica.",
		Vars:  []string{"nivel"},
	},
	Key(CodeNoCalcStart, ""): {
		Code: CodeNoCalcStart, Kind: entities.KindUpdate,
		Title: "Sin datos suficientes",
		Body:  "No se puede evaluar desde las {hora}; la alerta sigue en {nivel}.",
		Vars:  []string{"hora", "nivel"},
	},
	Key(CodeNoCalcEnd, ""): {
		Code: CodeNoCalcEnd, Kind: entities.KindUpdate,
		Title: "Vuelve a haber datos",
		Body:  "La alerta sigue en {nivel}.",
		Vars:  []string{"nivel"},
	},
	Key(CodeDowngradeStart, ""): {
		Code: CodeDowngradeStart, Kind: entities.KindUpdate,
		Title: "El ruido bajó a nivel {nivel_hora}",
		Body:  "Si se mantiene así {horas} h, la alerta bajará a {nivel_inferior}.",
		Vars:  []string{"nivel_hora", "horas", "nivel_inferior"},
	},
	Key(CodeDowngrade, ""): {
		Code: CodeDowngrade, Kind: entities.KindUpdate,
		Title: "La alerta bajó a {nivel_inferior}",
		Body:  "El ruido se mantuvo por debajo de {nivel} durante {horas} h. Máximo alcanzado: {max}.",
		Vars:  []string{"nivel", "horas", "nivel_inferior", "max"},
	},
	Key(CodeRojoHeld, ""): {
		Code: CodeRojoHeld, Kind: entities.KindUpdate,
		Title: "Sigue en Rojo",
		Body:  "El ruido bajó hace {horas} h, pero una alerta roja sin clasificar no baja de nivel.",
		Vars:  []string{"horas"},
	},
	Key(CodeCountdown, ""): {
		Code: CodeCountdown, Kind: entities.KindUpdate,
		Title: "Puede subir a {nivel_siguiente}",
		Body:  "Lleva {N} h seguidas; faltan {horas} h para el cambio de nivel.",
		Vars:  []string{"N", "horas", "nivel_siguiente"},
	},
	Key(CodeTrend, ""): {
		Code: CodeTrend, Kind: entities.KindUpdate,
		Title: "El ruido está {direccion}",
		Body:  "La alerta sigue en {nivel} y lleva {N} h.",
		Vars:  []string{"direccion", "nivel", "N"},
	},
	Key(CodeAlmostNormal, ""): {
		Code: CodeAlmostNormal, Kind: entities.KindUpdate,
		Title: "Casi normal",
		// El evento v3 no trae exit_hours del Rule Engine: el texto no lo cita.
		Body: "La alerta terminará si el ruido se mantiene así.",
		Vars: []string{},
	},
	Key(CodeOneHourNormal, ""): {
		Code: CodeOneHourNormal, Kind: entities.KindUpdate,
		Title: "1 h con ruido normal",
		Body:  "La alerta sigue en {nivel} hasta completar el cierre.",
		Vars:  []string{"nivel"},
	},
	Key(CodeBackAbnormal, ""): {
		Code: CodeBackAbnormal, Kind: entities.KindUpdate,
		Title: "El ruido anormal volvió",
		Body:  "La alerta sigue en {nivel}.",
		Vars:  []string{"nivel"},
	},
	Key(CodeOscillating, ""): {
		Code: CodeOscillating, Kind: entities.KindUpdate,
		Title: "El ruido oscila",
		Body:  "Sube y baja alrededor del límite desde las {hora}; no se informará cada cambio.",
		Vars:  []string{"hora"},
	},
	Key(CodeSummary, ""): {
		Code: CodeSummary, Kind: entities.KindSummary,
		Title: "Sin cambios",
		Body:  "Sigue en {nivel}, lleva {N} h, ruido {tendencia}.",
		Vars:  []string{"nivel", "N", "tendencia"},
	},
	Key(CodeEndNormal, ""): {
		Code: CodeEndNormal, Kind: entities.KindEnd,
		Title: "La alerta {codigo} terminó",
		Body:  "El ruido volvió a lo normal. Duró {N} h; nivel máximo {max}. Si vuelve en las próximas {horas} h y sigue sin clasificar, se reactivará esta misma alerta.",
		Vars:  []string{"codigo", "N", "max", "horas"},
	},
	Key(CodeClassified, ""): {
		Code: CodeClassified, Kind: entities.KindManagement,
		Title: "Clasificada como {clasificacion}",
		Body:  "Registrada desde {origen}.",
		Vars:  []string{"clasificacion", "origen"},
	},
	Key(CodeLabeled, ""): {
		Code: CodeLabeled, Kind: entities.KindManagement,
		Title: "Etiqueta: {etiqueta}",
		Body:  "Registrada desde {origen}.",
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
