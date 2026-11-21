# Movimiento del sensor: reglas, alertas y base de datos

Estado al 2026-11-21. Describe lo que está implementado en el código (`backend/internal/modules`), no lo que pide la especificación original (`REQ_ALERTAS_MOVIMIENTO.md`, en `rule_engine/domain/services/testdata/motion/`). Donde el código se aparta de la especificación, manda el código.

---

## 1. Panorama

El movimiento corre en dos módulos, igual que la fuga:

```
raw_measurements (pitch=1, roll=2, deriva=19, temperatura=20)
        |
        v
RULE ENGINE  -- MotionAssess (puro) cada hora, minuto :40 UTC
        |      guarda 1 fila por punto y hora en motion_evaluations
        |      publica rule_engine.motion_evaluated (solo LIVE/CATCH_UP y alerts_enabled = true)
        v
ALERT MANAGEMENT -- DecideMotion (alerta de MOVIMIENTO)
                 -- DecideTemperature (copia los TEMP_* a la alerta de FUGA activa)
        |
        v
alerts / alert_level_segments / alert_messages / alert_processing_state
```

Decisión de diseño vigente (2026-11-19): **FUGA y MOVIMIENTO nunca se combinan**. Si se solapan en el tiempo son DOS alertas distintas, cada una con sus colores, mensajes y gestión. Lo único que comparten es la temperatura: los mensajes `TEMP_*` salen en la alerta de MOVIMIENTO y también en la de FUGA si está activa a esa hora.

---

## 2. Rule Engine: cómo se evalúa una hora

### 2.1 Entrada

- La hora `t` cubre `[t - 1 h, t)`.
- Se leen los registros crudos de `[t - 169 h, t)` (una semana más una hora), agrupados por timestamp. Solo cuentan los registros que traen pitch Y roll.
- El estado de la hora anterior (fila previa de `motion_evaluations`) y los parámetros de la versión vigente (`motion_threshold_versions`).
- `MotionAssess` es una función pura, sin reloj ni base de datos. Reproduce el script de referencia `motion_ref.py` (hay pruebas golden contra él).

### 2.2 Variables principales

Todo nivel de una hora es la **mediana** de pitch, roll y temperatura de esa hora.

| Variable | Qué es |
|---|---|
| `pitch_now`, `roll_now`, `temp_now` | Mediana de la hora `t` |
| `dp_1h`, `dr_1h` | Nivel de `t` menos mediana de `[t - 3 h, t - 1 h)` (necesita 4 registros o más) |
| `deriva_max_1h` | Máximo de la deriva (métrica 19) en la hora |
| `base_pitch`, `base_roll` | Mediana de `[t - 24 h, t - 6 h)` (necesita 36 registros o más) |
| `dp_6h`, `dr_6h` | Nivel actual menos la base |
| `dp_24h`, `dr_24h` | Nivel actual menos nivel de `t - 24 h` |
| `temp_rate` | `abs(temp_now(t) - temp_now(t - 1 h))`, en °C/h |
| `temp_rate_max_6h` | Máximo de `temp_rate` en las últimas 6 h |

### 2.3 Reglas, en el orden en que se aplican

**1) Cobertura.** Si la hora tiene menos de `min_records_hour` (3) registros es `NO_CALCULABLE`. No se evalúa nada; los huecos de datos que tocaba evaluar quedan pendientes para la próxima hora calculada. Si el sensor estaba desplazado, el estado sigue en `DESPLAZADO`.

**2) Salto (`MOV_JUMP`).** Hay salto si se cumple alguna:
- `deriva_max_1h >= deriva_jump_deg` (0,03°), o
- `abs(dp_1h) >= jump_deg` (0,03°) **y** `abs(dr_1h) >= jump_deg` **y** los dos con el mismo signo.

Es decir: pitch y roll se mueven JUNTOS y de golpe.

**3) Hueco de datos (`MOV_SHIFT_GAP`).** Un hueco es un par de registros consecutivos separados por `gap_min_hours` (1 h) o más. Se evalúa en la hora que contiene `fin del hueco + 45 min`. Compara la mediana de las 2 h antes del hueco (mínimo 3 registros) con la de los 45 min después (mínimo 2 registros). Si los dos ejes cambiaron `shift_deg` (0,04°) o más en el mismo sentido y el sensor no estaba desplazado, pasa a **desplazado** con referencia = posición antes del hueco.

**4) Desplazamiento sostenido (`MOV_SHIFT`).** Si TODOS los registros de las últimas 2 h (mínimo 6) están a `shift_deg` (0,04°) o más de la base, en los dos ejes y en el mismo sentido, el sensor pasa a **desplazado**. Dos condiciones extra:
- Tiene que haber habido al menos una hora con salto en `[t - 6 h, t]`. Un corrimiento lento de los dos ejes es normal y NO abre nada.
- No se dispara si hubo un retorno hace menos de `shift_suppress_after_return_hours` (24 h).

El inicio del desplazamiento (`shift_started_at`) se busca en las últimas 6 h: primero el registro con deriva alta, si no la primera hora con salto, si no el primer registro a `jump_deg` de la base.

**5) Retorno (`MOV_RETURN`).** Estando desplazado (y no en la misma hora en que se desplazó), si el sensor recuperó al menos `return_fraction` (50 %) del desplazamiento en los dos ejes, vuelve a no desplazado.

**6) Cambio brusco de un eje (`MOV_ABRUPT`).** Si pitch o roll cambió `abrupt_deg` (0,04°) o más frente a alguna de las últimas `abrupt_hours` (3) horas. Solo cuenta si NO hay salto en la hora, NO está desplazado y no hubo un evento conjunto (desplazamiento o retorno) en las últimas 6 h. El motivo sale una sola vez por racha (`abrupt_active`).

**7) Temperatura** (solo con `temp_mode = MOVIMIENTO_Y_TEMPERATURA`). Ver 2.4.

**8) Quieto (`MOV_STILL`).** El sensor está quieto si en las últimas 24 h hay al menos 20 horas calculadas, ninguna con salto, y el rango (máx - mín) de pitch y de roll es 0,02° o menos.

**9) Episodio.** Es lo que se traduce en alerta:
- **Abre** (`OPENED`) con salto, desplazamiento nuevo (sostenido o por hueco) o cambio brusco nuevo.
- **Cierra** (`CLEARED`) cuando el sensor está quieto 24 h, pasaron al menos 24 h desde que abrió y la temperatura está en `TEMP_VARIABLE`, `SIN_ESTABILIZAR` o el modo es `SOLO_MOVIMIENTO`. Si al cerrar seguía desplazado, sale `MOV_NEW_POSITION`: la posición actual pasa a ser la normal.
- Dentro del episodio `computed_level = AMARILLO`; fuera, `NORMAL`. **El movimiento nunca pasa de Amarillo.**

Señales informativas que se guardan pero no disparan nada: `slow_drift_flag` (0,04° en 24 h) y `trend_flag` (0,04° en 7 días).

### 2.4 Máquina de estados de la temperatura

La idea: si el sensor se movió y la temperatura de la tubería deja de variar, puede ser una señal de interés (pasó antes en este punto), pero NO está confirmado.

| Estado | Cómo se llega | Sale |
|---|---|---|
| `TEMP_VARIABLE` | Inicial, o la temperatura volvió a variar | Desplazamiento nuevo pasa a `VERIFICANDO` (`TEMP_VERIFYING`) |
| `VERIFICANDO` | El sensor se desplazó | Temperatura plana pasa a `TEMP_ESTABLE` (`TEMP_STABILIZED`); 36 h sin aplanarse pasa a `SIN_ESTABILIZAR` (`TEMP_NOT_STABILIZED`); retorno vuelve a `TEMP_VARIABLE` |
| `TEMP_ESTABLE` | Temperatura plana | Retorno del sensor pasa a `RETORNO_TEMP_ESTABLE`; temperatura fluye pasa a `TEMP_VARIABLE` (`TEMP_VARIES_AGAIN`) |
| `RETORNO_TEMP_ESTABLE` | Sensor volvió pero la temperatura sigue plana | `TEMP_STILL_STABLE` a las 6 h y luego cada 12 h; temperatura fluye pasa a `TEMP_VARIABLE` (`TEMP_VARIES_AGAIN`) |
| `SIN_ESTABILIZAR` | 36 h desplazado sin aplanarse | Retorno o cierre del episodio vuelve a `TEMP_VARIABLE` |

- **Plana**: al menos 5 tasas en las últimas 6 h y todas menores a 0,03 °C/h.
- **Fluye**: las últimas 2 tasas son 0,08 °C/h o más.

### 2.5 Estado del movimiento de la hora (`motion_state`)

En este orden de prioridad: `DESPLAZADO` > `RETORNO` > `SALTO` > `BRUSCO` > `NORMAL`. Vacío (NULL) en una hora `NO_CALCULABLE` sin desplazamiento.

### 2.6 Parámetros (versión v1 del MP-11556)

Viven en `motion_threshold_versions`, versionados con vigencia `[valid_from, valid_to)` y un `EXCLUDE` que impide que dos versiones del mismo punto se solapen.

| Parámetro | Valor | Parámetro | Valor |
|---|---|---|---|
| `alerts_enabled` | true | `temp_mode` | `MOVIMIENTO_Y_TEMPERATURA` |
| `records_per_hour` | 4 | `min_records_hour` | 3 |
| `jump_deg` | 0,03 | `deriva_jump_deg` | 0,03 |
| `shift_deg` | 0,04 | `shift_window_hours` / `shift_min_records` | 2 / 6 |
| `base_from_hours` / `base_to_hours` / `base_min_records` | 24 / 6 / 36 | `gap_min_hours` | 1 |
| `gap_pre_hours` / `gap_post_minutes` | 2 / 45 | `gap_pre_min_records` / `gap_post_min_records` | 3 / 2 |
| `return_fraction` | 0,5 | `shift_suppress_after_return_hours` | 24 |
| `abrupt_deg` / `abrupt_hours` | 0,04 / 3 | `abrupt_suppress_after_joint_hours` | 6 |
| `still_hours` / `still_min_calculated` / `still_range_deg` | 24 / 20 / 0,02 | `slow_drift_deg_24h` | 0,04 |
| `temp_flat_rate` / `temp_flat_hours` / `temp_flat_min_rates` | 0,03 / 6 / 5 | `temp_flow_rate` / `temp_flow_hours` | 0,08 / 2 |
| `stabilize_max_hours` | 36 | `still_stable_first_hours` / `still_stable_every_hours` | 6 / 12 |
| `trend_deg_7d` | 0,04 | | |

Los demás puntos reciben la v1 de "punto nuevo": mismos valores pero `alerts_enabled = false` (modo sombra) y `SOLO_MOVIMIENTO`. En modo sombra se evalúa y se guarda, pero no se publica evento ni se crean alertas.

API (sin autenticación):

- `GET  /api/rule-engine/points/{id}/motion-thresholds` (vigente)
- `GET  /api/rule-engine/points/{id}/motion-thresholds/history`
- `POST /api/rule-engine/points/{id}/motion-thresholds` (versión nueva; los campos no enviados se copian de la vigente)
- `POST /api/rule-engine/points/{id}/motion-thresholds/simulate`

### 2.7 Cuándo corre

- **LIVE**: scheduler propio a los `:40` de cada hora UTC (`RULE_ENGINE_MOTION_MINUTE`).
- **CATCH_UP**: completa horas faltantes.
- **BACKFILL**: histórico por CLI. No publica eventos.

El evento `rule_engine.motion_evaluated` se publica después del commit, por cada punto y hora LIVE/CATCH_UP con `alerts_enabled = true`, incluidas las horas `NORMAL` y `NO_CALCULABLE`. Clave de idempotencia: `(measuring_point_id, evaluated_at)`.

---

## 3. Alert Management: cómo se convierte en alerta

### 3.1 Flujo (`ProcessMotion`)

- Mismo lock por punto que FUGA. Cada flujo tiene su fila en `alert_processing_state` con `(measuring_point_id, event_type)`: una hora igual o anterior a la última procesada del flujo no hace nada.
- Si en vivo faltan horas (evento perdido), se leen de `motion_evaluations` antes de procesar la recibida.
- La **política** (`alert_policy_versions`) es la de FUGA: mismas reglas de reactivación y de atraso.
- Por cada hora: `applyMotion` (alerta de MOVIMIENTO) y después `applyTemperature` (TEMP_* sobre la FUGA activa, si hay).

### 3.2 Reglas de `DecideMotion`

1. Hora `NO_CALCULABLE`: solo actualiza el estado del flujo. No genera mensajes.
2. **Abrir**: si no hay alerta de MOVIMIENTO activa y la hora trae `episode_event = OPENED` (o es la primera hora del flujo y ya está en episodio):
   - Si la última alerta de MOVIMIENTO FINALIZADA terminó hace menos de `merge_window_hours` y está sin clasificar, se **reactiva** esa misma alerta (segmento `REACTIVATION`, las horas de pausa no suman a la duración).
   - Si no, se crea una alerta nueva: `event_type = MOVIMIENTO`, nivel **AMARILLO**, segmento `OPEN`.
   - Mensaje `MOV_OPEN` con variante según el primer motivo: `HUECO` (`MOV_SHIFT_GAP`), `SALTO` (`MOV_SHIFT` o `MOV_JUMP`), `BRUSCO` (`MOV_ABRUPT`).
   - `started_at`: con hueco, el fin de la hora que contiene el inicio del hueco; con brusco, `t - (horas - 1) h`; si no, `t`.
3. **Seguir**: con la alerta activa, un `OPENED` repetido no abre otra. Se copia `motion_state` y `temp_state` a la alerta. Si aparece un desplazamiento o brusco nuevo sale `MOV_DISPLACED`.
4. **Temperatura**, en este orden: `TEMP_STABILIZED`, `MOV_RETURN`, `TEMP_VARIES_AGAIN`, `TEMP_NOT_STABILIZED`, `TEMP_STILL_STABLE`. Actualiza `temp_stable_from` / `temp_stable_to` de la alerta.
5. **Cerrar**: con `episode_event = CLEARED` la alerta pasa a FINALIZADA y sale `END_NORMAL/MOVIMIENTO` (con el tramo de temperatura estable y la nota de nueva posición si corresponde).

Importante: **una alerta de MOVIMIENTO siempre es Amarillo**. No sube a Naranja ni a Rojo, no tiene cuenta regresiva, resumen ni bajadas de nivel.

### 3.3 Reglas de `DecideTemperature` (sobre la alerta de FUGA)

- Solo si hay una alerta de FUGA ACTIVA del punto y la hora de movimiento trae algún `TEMP_*`.
- Replica `TEMP_STABILIZED`, `TEMP_VARIES_AGAIN`, `TEMP_NOT_STABILIZED` y `TEMP_STILL_STABLE` con el mismo tipo de mensaje.
- Actualiza `temp_state` y `temp_stable_from/to` de la alerta de FUGA.
- NUNCA cambia su color, su estado ni su fin, y NUNCA emite `MOV_*` (ni siquiera `MOV_RETURN`).
- La notificación sale con el `event_type` de la alerta: una copia TEMP_* en FUGA sale como FUGA.

---

## 4. Cómo están las alertas en la BD

### 4.1 Tablas del Rule Engine (movimiento)

**`motion_threshold_versions`**: una fila por versión de parámetros de un punto. Columnas: `id`, `measuring_point_id`, `version`, los 35 parámetros de la tabla 2.6, `valid_from`, `valid_to` (NULL = vigente), `reason`, `created_by`, `created_at`.

**`motion_evaluations`**: una fila por punto y hora, también las `NO_CALCULABLE`. Es la fuente de verdad del movimiento.

| Grupo | Columnas |
|---|---|
| Identidad | `id`, `measuring_point_id`, `evaluated_at`, `status`, `n_records`, `coverage` |
| Variables | `pitch_now`, `roll_now`, `temp_now`, `dp_1h`, `dr_1h`, `base_pitch`, `base_roll`, `dp_6h`, `dr_6h`, `shift_records_2h`, `shift_sign_hour`, `deriva_max_1h`, `jump`, `abrupt`, `dp_24h`, `dr_24h`, `slow_drift_flag`, `trend_flag`, `temp_rate`, `temp_rate_max_6h`, `temp_flat`, `temp_flow`, `still` |
| Hueco evaluado | `gap_eval_from`, `gap_eval_to`, `dp_gap`, `dr_gap` |
| Estado que pasa a la hora siguiente | `displaced`, `ref_pitch`, `ref_roll`, `shift_sign`, `shift_started_at`, `shift_level_pitch`, `shift_level_roll`, `gap_from`, `gap_to`, `last_joint_at`, `last_return_at`, `return_started_at`, `temp_state`, `temp_since`, `temp_stable_from`, `temp_stable_to`, `still_stable_msgs`, `in_episode`, `episode_opened_at`, `abrupt_active`, `pending_gaps` |
| Resultado | `motion_state`, `episode_event` (`OPENED` / `CLEARED` / `NONE`), `computed_level` (`NORMAL` / `AMARILLO`), `reasons` (lista `{code, data}`) |
| Trazabilidad | `threshold_version_id`, `algorithm_version` (`motion-v1`), `trigger` (`LIVE` / `CATCH_UP` / `BACKFILL`), `input_max_ts`, `created_at` |

### 4.2 Tablas de Alert Management (FUGA y MOVIMIENTO comparten las mismas)

**`alerts`**: una fila por alerta. El modelo es el mismo para FUGA y MOVIMIENTO.

| Columna | Uso |
|---|---|
| `id`, `code` | `code` es generado: `ALR-000123` |
| `measuring_point_id`, `event_type` | `FUGA` o `MOVIMIENTO` |
| `state` | `ACTIVA` o `FINALIZADA` (puede volver a ACTIVA por reactivación) |
| `motion_state`, `temp_state`, `temp_stable_from`, `temp_stable_to`, `last_motion_evaluated_at` | Contexto del movimiento. En MOVIMIENTO se copia de la última hora; en FUGA solo se llena la temperatura cuando le llega un TEMP_* |
| `started_at`, `opened_at`, `ended_at` | `ended_at` NULL si y solo si está ACTIVA |
| `reactivation_count`, `last_reactivated_at` | Reactivaciones |
| `current_level`, `max_level` | `AMARILLO` / `NARANJA` / `ROJO` (en MOVIMIENTO siempre `AMARILLO`) |
| `opening_indicator_value`, `last_indicator_value`, `max_indicator_value`, `last_evaluated_at` | Índice de ruido (FUGA) |
| `hours_calculated`, `hours_no_data`, `below_level_hours` | Contadores |
| `low_confidence_at_open`, `runtime` (jsonb) | `runtime` guarda la memoria interna (en MOVIMIENTO, `motion_shift_started_at`) |
| `leak_d_pl`, `leak_d_pl_min`, `leak_confirmed` | Firma de fuga por planitud (solo FUGA) |
| `policy_version_id`, `threshold_version_id_at_open`, `opened_by_rule_evaluation_id` | Trazabilidad |
| `management_status`, `classification`, `label`, `label_other_text`, `classified_*`, `labeled_*` | Gestión del operador (inmutable una vez escrita) |
| `source` | `LIVE` / `CATCH_UP` / `BACKFILL` |

Restricciones clave:

- `ux_alerts_one_active`: UNA sola alerta ACTIVA por `(measuring_point_id, event_type)`. Por eso un punto puede tener a la vez una FUGA activa y un MOVIMIENTO activo, pero nunca dos del mismo tipo.
- `chk_alerts_motion_state`: `NORMAL`, `SALTO`, `DESPLAZADO`, `RETORNO`, `BRUSCO` o NULL.
- `chk_alerts_temp_state`: `TEMP_VARIABLE`, `VERIFICANDO`, `TEMP_ESTABLE`, `RETORNO_TEMP_ESTABLE`, `SIN_ESTABILIZAR` o NULL.
- Trigger `trg_alerts_management_immutable`: la clasificación y la etiqueta no cambian, y las alertas no se borran (salvo `-rebuild` con `SET LOCAL agatha.alerts_rebuild = 'on'`).

**`alert_level_segments`**: los colores (tramos) de la alerta: `alert_id`, `seq`, `opened_by` (`OPEN` / `UP` / `DOWN` / `REACTIVATION`), `level`, `started_at`, `ended_at`. Una alerta de MOVIMIENTO tiene un segmento `OPEN` y uno `REACTIVATION` por cada reactivación, todos Amarillo.

**`alert_messages`**: la línea de tiempo. `alert_id`, `segment_id`, `evaluated_at`, `kind`, `code`, `level`, `indicator_value`, `title`, `body` (texto ya renderizado), `params` (jsonb), `catalog_version` (hoy 7), `policy_version_id`. Único por `(alert_id, evaluated_at, code)`.

**`alert_observations`**: observaciones del operador.

**`alert_processing_state`**: cursor por `(measuring_point_id, event_type)`: `last_evaluated_at`, `last_alert_id`, `last_status`, `pending_run_started_at`.

**`alert_policy_versions`**: política versionada (reactivación, resumen, cuenta regresiva, etc.). El movimiento usa la de FUGA.

---

## 5. Qué tipos de alertas y mensajes salen

### 5.1 Tipos de alerta

| `event_type` | Origen | Niveles posibles | Abre por |
|---|---|---|---|
| `FUGA` | Índice de ruido (`rule_evaluations`) | Amarillo, Naranja, Rojo | Índice sobre el umbral (con confirmación por planitud) |
| `MOVIMIENTO` | `motion_evaluations` | Solo Amarillo | Salto, desplazamiento (sostenido o por hueco) o cambio brusco |

### 5.2 Tipos de mensaje (`alert_messages.kind`)

- `NOTIFICACION`: avisos (banner y push).
- `ACTUALIZACION`: seguimiento en la línea de tiempo.
- `RESUMEN`: resumen periódico (solo FUGA).
- `FIN`: cierre de la alerta.
- `GESTION`: clasificación y etiqueta del operador.

### 5.3 Mensajes de una alerta de MOVIMIENTO

| Código / variante | Tipo | Título | Cuándo |
|---|---|---|---|
| `MOV_OPEN/SALTO` | NOTIFICACION | Movimiento brusco del sensor | Abre por salto o desplazamiento sostenido |
| `MOV_OPEN/HUECO` | NOTIFICACION | El sensor cambió de posición sin datos | Abre por hueco de datos |
| `MOV_OPEN/BRUSCO` | NOTIFICACION | Movimiento brusco del sensor | Abre por cambio brusco de un eje |
| `MOV_DISPLACED/SALTO`, `/HUECO` | ACTUALIZACION | El sensor sigue en la nueva posición | Desplazamiento nuevo con la alerta ya abierta |
| `MOV_DISPLACED/BRUSCO` | ACTUALIZACION | El sensor sigue en la nueva posición | Cambio brusco nuevo con la alerta ya abierta |
| `MOV_RETURN` | ACTUALIZACION | El sensor volvió a su posición | Retorno |
| `TEMP_STABILIZED` | NOTIFICACION | La temperatura de la tubería se estabilizó | `VERIFICANDO` pasa a `TEMP_ESTABLE` |
| `TEMP_VARIES_AGAIN` | NOTIFICACION | La temperatura de la tubería volvió a variar | Fin del tramo estable |
| `TEMP_NOT_STABILIZED` | ACTUALIZACION | La temperatura no se estabilizó | 36 h desplazado sin aplanarse |
| `TEMP_STILL_STABLE` | ACTUALIZACION | La temperatura sigue estable | Sensor volvió pero la temperatura sigue plana (6 h y cada 12 h) |
| `END_NORMAL/MOVIMIENTO` | FIN | Volvió a la normalidad | Episodio `CLEARED` |
| `CLASSIFIED`, `LABELED` | GESTION | Clasificada como ... / Etiqueta: ... | Acción del operador |

Si la alerta se reactiva, el `MOV_OPEN` de la reactivación se guarda como `NOTIFICACION` sobre la misma alerta.

En una alerta de FUGA activa solo pueden aparecer, por el movimiento, los cuatro `TEMP_*`.

### 5.4 Mensajes de una alerta de FUGA (referencia)

- Apertura: `OPEN/AMARILLO`, `OPEN/NARANJA`, `OPEN/ROJO`, `OPEN/SIN_FIRMA` (limitada a Amarillo por planitud), `NEW_AFTER_CLASSIFIED`.
- Subidas: `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `UP_ROJO/PERSIST` y sus variantes `*_CONFIRMADA` (firma de fuga recién confirmada).
- Reactivación: `REACTIVATED`.
- Seguimiento: `NO_LEAK_SHAPE`, `NO_CALC_START`, `NO_CALC_END`, `DOWNGRADE_START`, `DOWNGRADE`, `ROJO_HELD`, `COUNTDOWN`, `TREND`, `ALMOST_NORMAL`, `ONE_HOUR_NORMAL`, `BACK_ABNORMAL`, `OSCILLATING`, `SUMMARY`.
- Fin: `END_NORMAL`.
- Gestión: `CLASSIFIED`, `LABELED`.
- Por movimiento: `TEMP_STABILIZED`, `TEMP_VARIES_AGAIN`, `TEMP_NOT_STABILIZED`, `TEMP_STILL_STABLE`.

---

## 6. Consultas útiles

```sql
-- Alertas de movimiento de un punto
SELECT code, state, current_level, motion_state, temp_state, started_at, ended_at, reactivation_count
FROM alerts
WHERE measuring_point_id = 11556 AND event_type = 'MOVIMIENTO'
ORDER BY started_at;

-- Línea de tiempo de una alerta
SELECT evaluated_at, kind, code, title, body
FROM alert_messages
WHERE alert_id = (SELECT id FROM alerts WHERE code = 'ALR-000123')
ORDER BY evaluated_at, id;

-- Horas que abrieron o cerraron episodio de movimiento
SELECT evaluated_at, motion_state, temp_state, episode_event, reasons
FROM motion_evaluations
WHERE measuring_point_id = 11556 AND episode_event <> 'NONE'
ORDER BY evaluated_at;

-- FUGA y MOVIMIENTO solapados en el mismo punto
SELECT f.code AS fuga, m.code AS movimiento, f.started_at, m.started_at
FROM alerts f
JOIN alerts m ON m.measuring_point_id = f.measuring_point_id AND m.event_type = 'MOVIMIENTO'
WHERE f.event_type = 'FUGA'
  AND tstzrange(f.started_at, f.ended_at) && tstzrange(m.started_at, m.ended_at);
```

---

## 7. Archivos de referencia

| Tema | Archivo |
|---|---|
| Algoritmo (reglas) | `rule_engine/domain/services/motion_assess.go` |
| Entidades y valores v1 | `rule_engine/domain/entities/motion.go`, `motion_params.go` |
| Contrato del evento | `rule_engine/contract/motion_evaluated.go` |
| Esquema y seed de movimiento | `rule_engine/infrastructure/persistence/motion_schema.go` |
| API de umbrales | `rule_engine/infrastructure/http/motion_handler.go` |
| Decisión de la alerta | `alert_management/domain/services/decide_motion.go` |
| Flujo de procesamiento | `alert_management/application/services/processing_motion.go` |
| Textos (catálogo v7) | `alert_management/domain/messages/catalog.go` |
| Esquema de alertas | `alert_management/infrastructure/persistence/schema.go`, `models/models.go` |
