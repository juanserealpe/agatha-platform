// Package messages es el catálogo estático de textos de la línea de tiempo
// (sección 6.2): código → título y cuerpo con variables entre llaves. Las
// reglas devuelven código y variables; el texto se renderiza acá y se guarda
// tal como se mostró, con MessageCatalogVersion. Lenguaje de campo: nunca
// "I", "umbral" ni "persistencia".
package messages

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// MessageCatalogVersion se guarda en cada mensaje. Subirla al cambiar un texto.
// v2: {pk} sin "sensor" delante (devices.name ya lo trae).
// v3: el cuerpo no repite el título y ningún texto dice qué hacer (sin zona).
// v4: el cuerpo cita el índice de ruido y el límite que explica el mensaje.
// v5: los avisos (banner y push) empiezan con una frase simple de qué pasa y
// terminan con el dato ("Dato: …"); el resto de los mensajes no cambia.
// v6: confirmación por planitud. Se agregan OPEN/SIN_FIRMA (la alerta abre en
// Amarillo porque la hora quedó limitada), NO_LEAK_SHAPE (se mantiene en
// Amarillo) y las variantes *_CONFIRMADA de las subidas (anteponen la firma de
// fuga). La "firma de fuga" es el % de registros de las últimas 24 h con
// planitud alta (d_pl) frente al mínimo (d_pl_min).
// v7: movimiento y temperatura (REQ_ALERTAS_MOVIMIENTO §8): MOV_*, TEMP_* y
// END_NORMAL/MOVIMIENTO; CLASSIFIED y LABELED con su fecha y hora; se elimina
// UP_DURING_INTERVENTION. Los demás textos de fuga no cambian. FUGA y
// MOVIMIENTO son alertas separadas (no hay LEAK_JOINED, MOV_JOINED ni
// PART_NORMAL); los TEMP_* salen también en la alerta de FUGA vigente.
// v8: movimiento con la tabla de lo normal (REQ_MOVIMIENTO_UMBRALES_V2 §10):
// el número va en la frase (sin "Dato:"), lo normal de ESE sensor, nivel
// calculado vs. enviado, historial al abrir, MOV_LEVEL_UP / MOV_LEVEL_OBSERVED,
// MOV_SLOW_ONGOING, MOV_DAILY y el fin con el cambio total. MOV_OPEN tiene las
// variantes SALTO, HUECO, RAPIDO y ACUMULADO. Los textos de FUGA y TEMP_* no cambian.
const MessageCatalogVersion = 8

// LowConfidenceMark se agrega con low_confidence = true (sección 6.1).
const LowConfidenceMark = "Sensor en ajuste: resultado preliminar."

// Códigos de mensaje (sección 6.1).
const (
	CodeOpen               = "OPEN"
	CodeUpNaranjaPersist   = "UP_NARANJA_PERSIST"
	CodeUpNaranjaFast      = "UP_NARANJA_FAST"
	CodeUpRojo             = "UP_ROJO"
	CodeReactivated        = "REACTIVATED"
	CodeNewAfterClassified = "NEW_AFTER_CLASSIFIED"
	CodeNoCalcStart        = "NO_CALC_START"
	CodeNoCalcEnd          = "NO_CALC_END"
	CodeDowngrade          = "DOWNGRADE"
	CodeRojoHeld           = "ROJO_HELD"
	CodeOscillating        = "OSCILLATING"
	CodeAlmostNormal       = "ALMOST_NORMAL"
	CodeOneHourNormal      = "ONE_HOUR_NORMAL"
	CodeBackAbnormal       = "BACK_ABNORMAL"
	CodeDowngradeStart     = "DOWNGRADE_START"
	CodeCountdown          = "COUNTDOWN"
	CodeTrend              = "TREND"
	CodeSummary            = "SUMMARY"
	CodeEndNormal          = "END_NORMAL"
	CodeClassified         = "CLASSIFIED"
	CodeLabeled            = "LABELED"
	// CodeNoLeakShape: la hora quedó limitada a Amarillo por la planitud (una
	// vez por racha de horas limitadas).
	CodeNoLeakShape = "NO_LEAK_SHAPE"

	// Movimiento y temperatura (v7).
	CodeMovOpen           = "MOV_OPEN"
	CodeMovDisplaced      = "MOV_DISPLACED"
	CodeMovReturn         = "MOV_RETURN"
	CodeTempStabilized    = "TEMP_STABILIZED"
	CodeTempVariesAgain   = "TEMP_VARIES_AGAIN"
	CodeTempNotStabilized = "TEMP_NOT_STABILIZED"
	CodeTempStillStable   = "TEMP_STILL_STABLE"

	// Movimiento v8.
	CodeMovLevelUp       = "MOV_LEVEL_UP"
	CodeMovLevelObserved = "MOV_LEVEL_OBSERVED"
	CodeMovSlowOngoing   = "MOV_SLOW_ONGOING"
	CodeMovDaily         = "MOV_DAILY"
)

// Variantes de un mismo código (el código guardado no cambia).
const (
	VariantAmarillo = "AMARILLO"
	VariantNaranja  = "NARANJA"
	VariantRojo     = "ROJO"
	VariantPersist  = "PERSIST"
	// VariantNoLeakSignature: OPEN con la hora limitada a Amarillo por la planitud.
	VariantNoLeakSignature = "SIN_FIRMA"
	// VariantConfirmed se agrega a la variante de una subida (UP_*) cuando la
	// hora anterior estaba limitada y la planitud acaba de confirmarse.
	VariantConfirmed = "CONFIRMADA"

	// Variantes de movimiento (v8: las de episode_kind).
	VariantJump       = "SALTO"     // MOV_OPEN / MOV_DISPLACED por salto o desplazamiento
	VariantGap        = "HUECO"     // por un hueco de datos
	VariantAbrupt     = "RAPIDO"    // cambio rápido de un eje
	VariantCumulative = "ACUMULADO" // movimiento lento acumulado
	VariantMotion     = "MOVIMIENTO"
	// MOV_DAILY sin datos en las últimas 24 h o con datos que no alcanzan.
	VariantNoData  = "SIN_DATOS"
	VariantPartial = "PARCIAL"
)

// ConfirmedVariant es la variante "con firma de fuga confirmada" de la
// variante de una subida ("" ⇒ CONFIRMADA; PERSIST ⇒ PERSIST_CONFIRMADA).
func ConfirmedVariant(variant string) string {
	if variant == "" {
		return VariantConfirmed
	}
	return variant + "_" + VariantConfirmed
}

// confirmedPrefix se antepone al cuerpo de las subidas con la planitud recién confirmada.
const confirmedPrefix = "El {d_pl} de los registros de las últimas 24 h tiene firma de fuga (mínimo {d_pl_min}). "

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
// detalle sin repetir el título y cita el dato que lo explica. Los avisos
// (KindNotification y REACTIVATED) se leen de un vistazo en un banner o un
// push: primero una frase simple y al final "Dato: …". El dato es el índice de
// ruido de la hora ({valor}) frente al límite normal ({normal}, corte de
// Amarillo), al corte del nivel ({corte}) o a la salida ({cierre}). Ningún
// texto indica qué hacer: solo el estado de la alerta (lo verifican las pruebas).
var catalog = map[string]Template{
	Key(CodeOpen, VariantAmarillo): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Amarillo",
		Body:  "El índice de ruido está por encima de lo normal desde las {hora_inicio}. Dato: {valor} (lo normal es menos de {normal}).",
		Vars:  []string{"pk", "hora_inicio", "valor", "normal"},
	},
	Key(CodeOpen, VariantNaranja): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Naranja",
		Body:  "El índice de ruido está muy por encima de lo normal desde las {hora_inicio}. Dato: {valor} (Naranja desde {corte}; lo normal es menos de {normal}).",
		Vars:  []string{"pk", "hora_inicio", "valor", "corte", "normal"},
	},
	Key(CodeOpen, VariantRojo): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Rojo",
		Body:  "El índice de ruido es demasiado alto desde las {hora_inicio}. Dato: {valor} (Rojo desde {corte}; lo normal es menos de {normal}).",
		Vars:  []string{"pk", "hora_inicio", "valor", "corte", "normal"},
	},
	// Sin "seguidas": con persistencia WINDOW {N} es un conteo en la ventana.
	Key(CodeUpNaranjaPersist, ""): {
		Code: CodeUpNaranjaPersist, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Naranja",
		Body:  "El índice de ruido lleva {N} h por encima de lo normal. Dato: {valor} (lo normal es menos de {normal}).",
		Vars:  []string{"codigo", "N", "normal", "valor"},
	},
	Key(CodeUpNaranjaFast, ""): {
		Code: CodeUpNaranjaFast, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Naranja",
		Body:  "El índice de ruido está subiendo más rápido de lo normal. Dato: pasó de {valor_anterior} a {valor} (Naranja desde {corte}).",
		Vars:  []string{"codigo", "valor_anterior", "valor", "corte"},
	},
	Key(CodeUpRojo, ""): {
		Code: CodeUpRojo, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Rojo",
		Body:  "El índice de ruido siguió subiendo y ya es demasiado alto. Dato: pasó de {valor_anterior} a {valor} (Rojo desde {corte}).",
		Vars:  []string{"codigo", "valor_anterior", "valor", "corte"},
	},
	// Rojo por duración: las horas se cuentan sobre el corte de Naranja.
	Key(CodeUpRojo, VariantPersist): {
		Code: CodeUpRojo, Kind: entities.KindNotification,
		Title: "La alerta {codigo} subió a Rojo",
		Body:  "El índice de ruido lleva {N} h muy por encima de lo normal. Dato: {valor} (Naranja desde {corte}).",
		Vars:  []string{"codigo", "N", "corte", "valor"},
	},
	// El kind de REACTIVATED depende de si supera el máximo: lo fija la regla.
	Key(CodeReactivated, ""): {
		Code: CodeReactivated, Kind: entities.KindUpdate,
		Title: "La alerta {codigo} se reactiva en {nivel}",
		Body:  "El índice de ruido volvió a subir {horas} h después de normalizarse. Dato: {valor} (lo normal es menos de {normal}).",
		Vars:  []string{"horas", "codigo", "nivel", "valor", "normal"},
	},
	// La hora quedó limitada a Amarillo: {from} es el nivel que daría el índice
	// solo y {corte}, su corte. d_pl / d_pl_min ya vienen como porcentaje.
	Key(CodeOpen, VariantNoLeakSignature): {
		Code: CodeOpen, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel Amarillo",
		Body:  "Por el índice de ruido la alerta estaría en {from} desde las {hora_inicio}, pero solo el {d_pl} de los registros de las últimas 24 h tiene firma de fuga (se necesita {d_pl_min}). Dato: {valor} ({from} desde {corte}; lo normal es menos de {normal}).",
		Vars:  []string{"pk", "hora_inicio", "from", "d_pl", "d_pl_min", "valor", "corte", "normal"},
	},
	Key(CodeNoLeakShape, ""): {
		Code: CodeNoLeakShape, Kind: entities.KindUpdate,
		Title: "Se mantiene en Amarillo",
		Body:  "Por el índice de ruido la alerta estaría en {from}, pero solo el {d_pl} de los registros de las últimas 24 h tiene firma de fuga (se necesita {d_pl_min}). Dato: {valor} ({from} desde {corte}).",
		Vars:  []string{"from", "d_pl", "d_pl_min", "valor", "corte"},
	},
	Key(CodeNewAfterClassified, ""): {
		Code: CodeNewAfterClassified, Kind: entities.KindNotification,
		Title: "Alerta nueva en {pk}: nivel {nivel}",
		Body:  "El índice de ruido otra vez está fuera de lo normal; la alerta anterior ({codigo_anterior}) se clasificó como {clasificacion}. Dato: {valor} (lo normal es menos de {normal}).",
		Vars:  []string{"pk", "nivel", "codigo_anterior", "clasificacion", "valor", "normal"},
	},
	// Sin cálculo en la hora: cita el último índice calculado.
	Key(CodeNoCalcStart, ""): {
		Code: CodeNoCalcStart, Kind: entities.KindUpdate,
		Title: "Sin datos suficientes",
		Body:  "No se puede evaluar desde las {hora}; la alerta sigue en {nivel}. Último índice de ruido: {valor_anterior}.",
		Vars:  []string{"hora", "nivel", "valor_anterior"},
	},
	Key(CodeNoCalcEnd, ""): {
		Code: CodeNoCalcEnd, Kind: entities.KindUpdate,
		Title: "Vuelve a haber datos",
		Body:  "Índice de ruido en {valor}; la alerta sigue en {nivel}.",
		Vars:  []string{"nivel", "valor"},
	},
	Key(CodeDowngradeStart, ""): {
		Code: CodeDowngradeStart, Kind: entities.KindUpdate,
		Title: "El ruido bajó a nivel {nivel_hora}",
		Body:  "Índice de ruido en {valor}, por debajo del límite de {nivel} ({corte}). Si se mantiene así {horas} h, la alerta bajará a {nivel_inferior}.",
		Vars:  []string{"nivel_hora", "horas", "nivel_inferior", "valor", "nivel", "corte"},
	},
	Key(CodeDowngrade, ""): {
		Code: CodeDowngrade, Kind: entities.KindUpdate,
		Title: "La alerta bajó a {nivel_inferior}",
		Body:  "El índice de ruido se mantuvo por debajo del límite de {nivel} ({corte}) durante {horas} h; ahora está en {valor}. Máximo alcanzado: {max}, con índice de hasta {valor_max}.",
		Vars:  []string{"nivel", "horas", "nivel_inferior", "max", "corte", "valor", "valor_max"},
	},
	Key(CodeRojoHeld, ""): {
		Code: CodeRojoHeld, Kind: entities.KindUpdate,
		Title: "Sigue en Rojo",
		Body:  "El índice de ruido lleva {horas} h por debajo del límite de Rojo ({corte}) y está en {valor}, pero una alerta roja sin clasificar no baja de nivel.",
		Vars:  []string{"horas", "corte", "valor"},
	},
	// {corte}: el del nivel actual, sobre el que se cuentan las {N} h.
	Key(CodeCountdown, ""): {
		Code: CodeCountdown, Kind: entities.KindUpdate,
		Title: "Puede subir a {nivel_siguiente}",
		Body:  "El índice de ruido lleva {N} h por encima de {corte} (ahora en {valor}); faltan {horas} h para el cambio de nivel.",
		Vars:  []string{"N", "horas", "nivel_siguiente", "corte", "valor"},
	},
	Key(CodeTrend, ""): {
		Code: CodeTrend, Kind: entities.KindUpdate,
		Title: "El ruido está {direccion}",
		Body:  "El índice de ruido pasó de {valor_anterior} a {valor} en {horas} h. La alerta sigue en {nivel} y lleva {N} h.",
		Vars:  []string{"direccion", "nivel", "N", "valor_anterior", "valor", "horas"},
	},
	// Banda de cierre: bajo el corte de Amarillo, todavía sobre su salida.
	Key(CodeAlmostNormal, ""): {
		Code: CodeAlmostNormal, Kind: entities.KindUpdate,
		Title: "Casi normal",
		// El evento v3 no trae exit_hours del Rule Engine: el texto no lo cita.
		Body: "Índice de ruido en {valor}, ya por debajo de {normal}; la alerta terminará si baja de {cierre} y se mantiene así.",
		Vars: []string{"valor", "normal", "cierre"},
	},
	Key(CodeOneHourNormal, ""): {
		Code: CodeOneHourNormal, Kind: entities.KindUpdate,
		Title: "1 h con ruido normal",
		Body:  "Índice de ruido en {valor}, por debajo de {cierre}. La alerta sigue en {nivel} hasta completar el cierre.",
		Vars:  []string{"nivel", "valor", "cierre"},
	},
	Key(CodeBackAbnormal, ""): {
		Code: CodeBackAbnormal, Kind: entities.KindUpdate,
		Title: "El ruido anormal volvió",
		Body:  "Índice de ruido en {valor}; lo normal es menos de {normal}. La alerta sigue en {nivel}.",
		Vars:  []string{"nivel", "valor", "normal"},
	},
	Key(CodeOscillating, ""): {
		Code: CodeOscillating, Kind: entities.KindUpdate,
		Title: "El ruido oscila",
		Body:  "El índice de ruido cruzó {cambios} veces el límite normal ({normal}) desde las {hora}; ahora está en {valor}. No se informará cada cambio.",
		Vars:  []string{"hora", "cambios", "normal", "valor"},
	},
	Key(CodeSummary, ""): {
		Code: CodeSummary, Kind: entities.KindSummary,
		Title: "Sin cambios",
		Body:  "Sigue en {nivel}, lleva {N} h, ruido {tendencia}. Índice de ruido en {valor} (lo normal, menos de {normal}); el máximo de la alerta fue {valor_max}.",
		Vars:  []string{"nivel", "N", "tendencia", "valor", "normal", "valor_max"},
	},
	Key(CodeEndNormal, ""): {
		Code: CodeEndNormal, Kind: entities.KindEnd,
		Title: "La alerta {codigo} terminó",
		Body:  "El índice de ruido volvió a {valor} (lo normal, menos de {normal}). Duró {N} h; nivel máximo {max}, con índice de hasta {valor_max}. Si vuelve en las próximas {horas} h y sigue sin clasificar, se reactivará esta misma alerta.",
		Vars:  []string{"codigo", "N", "max", "horas", "valor", "normal", "valor_max"},
	},
	// Gestión (v7): la fecha y hora en que se guardó; {registro} solo si el
	// celular la hizo más de 5 min antes; {observacion} si vino con el paso.
	Key(CodeClassified, ""): {
		Code: CodeClassified, Kind: entities.KindManagement,
		Title: "Clasificada como {clasificacion}",
		Body:  "Registrada {origen} el {fecha} a las {hora}.{registro}{observacion}",
		Vars:  []string{"clasificacion", "origen", "fecha", "hora", "registro", "observacion"},
	},
	Key(CodeLabeled, ""): {
		Code: CodeLabeled, Kind: entities.KindManagement,
		Title: "Etiqueta: {etiqueta}",
		Body:  "Registrada {origen} el {fecha} a las {hora}.{registro}{observacion}",
		Vars:  []string{"etiqueta", "origen", "fecha", "hora", "registro", "observacion"},
	},

	// Movimiento (v8, REQ_MOVIMIENTO_UMBRALES_V2 §10.2). Describen el estado
	// medido con el número en la frase; no dicen qué hacer ni interpretan la
	// causa. {entre}, {calculado}, {historial} y {acumulado} los arma
	// motion_v8.go; {historial} trae su salto de línea delante.
	Key(CodeMovOpen, VariantJump): {
		Code: CodeMovOpen, Kind: entities.KindNotification,
		Title: "Cambio brusco de inclinación en pitch y roll",
		Body:  "{entre} el pitch y el roll cambiaron juntos y de golpe: pitch {dp}, roll {dr} en 1 h. En este sensor lo normal es menos de 0,01° por hora.{calculado}{historial}",
		Vars:  []string{"entre", "dp", "dr", "calculado", "historial"},
	},
	Key(CodeMovOpen, VariantGap): {
		Code: CodeMovOpen, Kind: entities.KindNotification,
		Title: "Cambio de inclinación durante una pérdida de datos",
		Body:  "El sensor no transmitió entre las {gap_from} y las {gap_to} ({duracion}). Al volver los datos, el pitch estaba {dp} y el roll {dr} respecto a antes de la pérdida de datos.{calculado}{historial}",
		Vars:  []string{"gap_from", "gap_to", "duracion", "dp", "dr", "calculado", "historial"},
	},
	Key(CodeMovOpen, VariantAbrupt): {
		Code: CodeMovOpen, Kind: entities.KindNotification,
		Title: "Cambio rápido de inclinación",
		Body:  "{entre} el {eje} cambió {delta} ({horas}). En este sensor lo normal es menos de {normal} en {ventana}.{calculado}{historial}",
		Vars:  []string{"entre", "eje", "delta", "horas", "normal", "ventana", "calculado", "historial"},
	},
	Key(CodeMovOpen, VariantCumulative): {
		Code: CodeMovOpen, Kind: entities.KindNotification,
		Title: "Cambio lento de inclinación acumulado",
		Body:  "{acumulado}{calculado}{historial}",
		Vars:  []string{"acumulado", "calculado", "historial"},
	},
	Key(CodeMovLevelUp, ""): {
		Code: CodeMovLevelUp, Kind: entities.KindNotification,
		Title: "La alerta de movimiento subió a {nivel}",
		Body:  "El {eje} ya cambió {delta} en {span}. En este sensor, {nivel_calc} es desde {deg} en {ventana}.",
		Vars:  []string{"nivel", "eje", "delta", "span", "nivel_calc", "deg", "ventana"},
	},
	Key(CodeMovLevelObserved, ""): {
		Code: CodeMovLevelObserved, Kind: entities.KindUpdate,
		Title: "El cambio de inclinación sigue aumentando",
		Body:  "El {eje} ya cambió {delta} en {span}. Por su magnitud correspondería a {nivel_calc} (desde {deg} en {ventana}); ese nivel todavía no está activo para este sensor, por eso la alerta sigue en {nivel_enviado}.",
		Vars:  []string{"eje", "delta", "span", "nivel_calc", "deg", "ventana", "nivel_enviado"},
	},
	Key(CodeMovSlowOngoing, ""): {
		Code: CodeMovSlowOngoing, Kind: entities.KindUpdate,
		Title: "Además, cambio lento acumulado",
		Body:  "{acumulado}",
		Vars:  []string{"acumulado"},
	},
	Key(CodeMovDaily, ""): {
		Code: CodeMovDaily, Kind: entities.KindSummary,
		Title: "Resumen del día {n} de la alerta",
		Body:  "Últimas 24 h: pitch {dp_dia}, roll {dr_dia}. Desde el inicio del movimiento: pitch {dp_total}, roll {dr_total}.{sin_cambios}{cobertura}",
		Vars:  []string{"n", "dp_dia", "dr_dia", "dp_total", "dr_total", "sin_cambios", "cobertura"},
	},
	Key(CodeMovDaily, VariantNoData): {
		Code: CodeMovDaily, Kind: entities.KindSummary,
		Title: "Resumen del día {n} de la alerta",
		Body:  "Sin datos del sensor en las últimas 24 h.",
		Vars:  []string{"n"},
	},
	Key(CodeMovDaily, VariantPartial): {
		Code: CodeMovDaily, Kind: entities.KindSummary,
		Title: "Resumen del día {n} de la alerta",
		Body:  "Con datos en {n_horas} de las últimas 24 h; no alcanza para calcular el cambio del día.{total}",
		Vars:  []string{"n", "n_horas", "total"},
	},
	Key(CodeMovDisplaced, VariantJump): {
		Code: CodeMovDisplaced, Kind: entities.KindUpdate,
		Title: "El sensor sigue en la nueva posición",
		Body:  "No ha vuelto a su posición desde las {inicio}. Pitch {dp}, roll {dr}.",
		Vars:  []string{"inicio", "dp", "dr"},
	},
	Key(CodeMovDisplaced, VariantGap): {
		Code: CodeMovDisplaced, Kind: entities.KindUpdate,
		Title: "El sensor sigue en la nueva posición",
		Body:  "No ha vuelto a su posición desde las {inicio}. Pitch {dp}, roll {dr}.",
		Vars:  []string{"inicio", "dp", "dr"},
	},
	Key(CodeMovDisplaced, VariantAbrupt): {
		Code: CodeMovDisplaced, Kind: entities.KindUpdate,
		Title: "El sensor se volvió a mover",
		Body:  "{entre} el {eje} cambió {delta}.",
		Vars:  []string{"entre", "eje", "delta"},
	},
	// §8.1 dice "Pasó lo mismo en cierres anteriores", pero §8.3 prohíbe
	// "cierre" en los textos de movimiento y temperatura: frase neutral.
	Key(CodeTempStabilized, ""): {
		Code: CodeTempStabilized, Kind: entities.KindNotification,
		Title: "La temperatura de la tubería se estabilizó",
		Body:  "Desde que se movió el sensor ({hora_inicio} del {fecha_inicio}) la temperatura casi no cambia. Pasó lo mismo antes en este punto, pero no está confirmado. Dato: máximo {rate} °C por hora (normal: más de 0,08).",
		Vars:  []string{"hora_inicio", "fecha_inicio", "rate"},
	},
	Key(CodeMovReturn, ""): {
		Code: CodeMovReturn, Kind: entities.KindUpdate,
		Title: "El sensor volvió a su posición",
		Body:  "Regresó a las {hora} a la posición que tenía antes del movimiento. Pitch {dp}, roll {dr}.",
		Vars:  []string{"hora", "dp", "dr"},
	},
	Key(CodeTempVariesAgain, ""): {
		Code: CodeTempVariesAgain, Kind: entities.KindNotification,
		Title: "La temperatura de la tubería volvió a variar",
		Body:  "Estuvo estable {duracion} ({desde} – {hasta}).{sin_retorno} Dato: {rate} °C en una hora.",
		Vars:  []string{"duracion", "desde", "hasta", "sin_retorno", "rate"},
	},
	Key(CodeTempNotStabilized, ""): {
		Code: CodeTempNotStabilized, Kind: entities.KindUpdate,
		Title: "La temperatura no se estabilizó",
		Body:  "El sensor lleva {horas} h fuera de su posición y la temperatura sigue variando como siempre. Dato: pitch {dp}, roll {dr}.",
		Vars:  []string{"horas", "dp", "dr"},
	},
	Key(CodeTempStillStable, ""): {
		Code: CodeTempStillStable, Kind: entities.KindUpdate,
		Title: "La temperatura sigue estable",
		Body:  "El sensor volvió hace {horas} h, pero la temperatura de la tubería sigue casi quieta. Dato: máximo {rate} °C por hora (normal: más de 0,08).",
		Vars:  []string{"horas", "rate"},
	},
	Key(CodeEndNormal, VariantMotion): {
		Code: CodeEndNormal, Kind: entities.KindEnd,
		Title: "Volvió a la normalidad",
		Body:  "La inclinación lleva {quieto}. Duró {duracion}{reactivacion}. Cambio total desde el inicio del movimiento: pitch {dp_total}, roll {dr_total}. Nivel máximo: {nivel_enviado}{por_magnitud}.{tramo}{nueva_posicion}",
		Vars:  []string{"quieto", "duracion", "reactivacion", "dp_total", "dr_total", "nivel_enviado", "por_magnitud", "tramo", "nueva_posicion"},
	},
}

// init agrega a cada subida su variante con la firma de fuga confirmada: el
// mismo título y código, con la frase de la firma antes del cuerpo.
func init() {
	for _, k := range []string{
		Key(CodeUpNaranjaPersist, ""), Key(CodeUpNaranjaFast, ""), Key(CodeUpRojo, ""), Key(CodeUpRojo, VariantPersist),
	} {
		tpl := catalog[k]
		variant := ""
		if i := strings.IndexByte(k, '/'); i >= 0 {
			variant = k[i+1:]
		}
		tpl.Body = confirmedPrefix + tpl.Body
		tpl.Vars = append(append([]string(nil), tpl.Vars...), "d_pl", "d_pl_min")
		catalog[Key(tpl.Code, ConfirmedVariant(variant))] = tpl
	}
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

// FormatValue es un índice (o un corte) en decimal con coma: 2 decimales y
// un tercero solo si hace falta, para que 0,158 no se lea como el corte 0,16.
func FormatValue(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimSuffix(s, "0")
	return strings.Replace(s, ".", ",", 1)
}

// FormatPercent es un porcentaje (0 a 100) con coma y un decimal como máximo:
// 4,6 → "4,6 %"; 10 → "10 %".
func FormatPercent(v float64) string {
	s := strconv.FormatFloat(math.Round(v*10)/10, 'f', 1, 64)
	s = strings.TrimSuffix(s, ".0")
	return strings.Replace(s, ".", ",", 1) + " %"
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

// FormatClock es la hora de Colombia "15:04"; FormatDate, la fecha "02/01".
func FormatClock(t time.Time) string { return t.In(colombia).Format("15:04") }

func FormatDate(t time.Time) string { return t.In(colombia).Format("02/01") }

// FormatRate es una tasa en °C/h: dos decimales y un tercero si hace falta.
func FormatRate(v float64) string { return FormatValue(v) }

// FormatDuration son horas en lenguaje de campo: "23 h", "2 días y 3 h".
func FormatDuration(hours int) string {
	if hours < 48 {
		return strconv.Itoa(max(hours, 0)) + " h"
	}
	days, rest := hours/24, hours%24
	if rest == 0 {
		return strconv.Itoa(days) + " días"
	}
	return strconv.Itoa(days) + " días y " + strconv.Itoa(rest) + " h"
}

// OriginPhrase es {origen} de los mensajes de gestión.
func OriginPhrase(o entities.Origin) string {
	if o == entities.OriginMobile {
		return "desde la app"
	}
	return "desde la web"
}

// labelPhrases son las etiquetas como las cita el catálogo (§8.2).
var labelPhrases = map[entities.Label]string{
	entities.LabelLeakConfirmed:      "Fuga confirmada en campo",
	entities.LabelGroundMovement:     "Movimiento de tierra",
	entities.LabelThirdPartyMachines: "Maquinaria ajena",
	entities.LabelExtremeWeather:     "Clima extremo",
	entities.LabelHeavyTraffic:       "Tránsito pesado",
	entities.LabelScheduledWork:      "Mantenimiento programado",
}

// LabelPhrase es {etiqueta} de LABELED ("Otro: <texto>" para OTRO).
func LabelPhrase(l entities.Label, otherText string) string {
	if l == entities.LabelOther {
		return "Otro: " + otherText
	}
	if p, ok := labelPhrases[l]; ok {
		return p
	}
	return string(l)
}

// IsMotionKey indica si la clave es de un mensaje de movimiento o de
// temperatura (incluido END_NORMAL/MOVIMIENTO).
func IsMotionKey(key string) bool {
	code, variant, _ := strings.Cut(key, "/")
	switch code {
	case CodeMovOpen, CodeMovDisplaced, CodeMovReturn, CodeTempStabilized, CodeTempVariesAgain,
		CodeTempNotStabilized, CodeTempStillStable, CodeMovLevelUp, CodeMovLevelObserved, CodeMovSlowOngoing, CodeMovDaily:
		return true
	case CodeEndNormal:
		return variant == VariantMotion
	}
	return false
}
