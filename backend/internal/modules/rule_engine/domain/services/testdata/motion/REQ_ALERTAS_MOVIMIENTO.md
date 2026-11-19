# AGATHA · Requerimiento: alertas por movimiento combinadas con fuga

**Versión:** 1.0 · 19 nov 2026 · **Punto de prueba:** MP-11556 (PK34+470)
**Módulos:** Rule Engine y Alert Management, más el rebuild del histórico. Push, API y frontend van en la entrega 2 (§13).
**Anexos:** `motion_ref.py` (implementación de referencia), `motion_input.csv` (datos de entrada), `motion_expected.csv` (salida esperada hora por hora). Escenarios gráficos: artifact "Escenarios ruido y movimiento".

---

## 1. Resumen

Un movimiento brusco del sensor es una anomalía por sí mismo y genera alerta, aunque no tenga que ver con una fuga. El ruido (fuga) y el movimiento comparten **una sola alerta por punto**:

- Abre la alerta lo primero que aparezca, ruido o movimiento brusco. Lo otro se une a esa misma alerta, en los dos sentidos.
- El color lo da el ruido. El movimiento solo nunca pasa de Amarillo.
- La alerta finaliza cuando todo está normal a la vez: el ruido, el sensor quieto 24 h y la temperatura variando otra vez (si se había estabilizado).

La temperatura se usa como contexto: si se estabiliza o vuelve a variar después de un salto del sensor, se informa. Los mensajes dicen **lo que se midió**. Que el gasoducto se haya cerrado es una interpretación: en el MP-11556 coincidió las dos veces, pero no es una certeza y solo TGI la confirma.

| Decisión | Regla |
|---|---|
| Lento es normal, brusco no | Un movimiento repartido en horas o días no alerta, solo se marca en los datos, sea en un eje o en los dos. Alerta un salto de pitch y roll juntos, o un eje que cambia 0,04° o más en 3 h o menos |
| Fuga ↔ movimiento | Se acoplan en la misma alerta en cualquier orden: movimiento → ruido o ruido → movimiento |
| Temperatura | Informa si se estabiliza o vuelve a variar después de un salto. No abre ni cierra la alerta por sí sola |
| Textos neutrales | Ningún mensaje afirma un cierre o una reapertura del gasoducto |
| La gestión no silencia | Clasificar o etiquetar no detiene ningún mensaje |
| La gestión queda en la línea de tiempo | La clasificación y la etiqueta generan un mensaje con la fecha y hora en que se guardaron |
| Puntos nuevos | Modo sombra: se calcula y se guarda, pero no se publica ni abre alertas |
| Rebuild | Recalcula el movimiento de todo el histórico y reconstruye todas las alertas desde cero, gestión incluida (§11) |

---

## 2. Contexto y evidencia

Convención: **[H]** hecho confirmado por TGI · **[D]** patrón observado en los datos · **[Hip]** hipótesis · **[V]** requiere validación del cliente.

| Qué | Grado | Detalle |
|---|---|---|
| Maniobras de agosto y octubre | [H] | 25 ago cierre, 27 ago reapertura, 20 oct cierre, 25 oct reapertura |
| Salto conjunto de pitch y roll en cada maniobra | [D] | 0,07–0,16° en 15–90 min, los dos ejes juntos. Lo normal es menos de 0,01° |
| Sensor desplazado mientras duró cada corte | [D] | En el 100 % de los registros. Al final vuelve a ±0,02° del nivel previo |
| Temperatura estable después del salto | [D] | Con flujo cambia en mediana 0,08 °C/h; en los dos cortes bajó de 0,03 °C/h. Se ve unas 16 h después del salto |
| Temperatura que vuelve a variar | [D] | 1–2 h después del regreso del sensor |
| Cambio brusco de un solo eje | [D] | 12 y 13 oct: roll +0,04–0,045° en menos de 3 h. Causa desconocida [V] |
| Movimiento lento | [D] | Pitch de −3,06° a −3,23° en 4 meses (−0,0008°/día); roll +0,05° en 11 h el 19–20 oct. El cliente lo considera normal |
| Cambio normal hora a hora | [D] | En 3 h: 95 % de las horas sin cambio en pitch, 89 % en roll; máximo 0,02° (pitch) y 0,03° (roll) |
| Que un cierre siempre mueva el sensor | [Hip] | Solo 2 casos en un equipo. Otro montaje o tramo puede comportarse distinto |
| Que sin flujo la temperatura siempre se estabilice | [Hip] | Puede fallar con otro clima, otra profundidad o poco caudal |

Ninguna regla de movimiento indica una fuga.

---

## 3. Alcance de esta entrega

| Módulo | Entra |
|---|---|
| Rule Engine | `motion_threshold_versions`, `motion_evaluations`, `MotionAssess` puro (§5), evento `rule_engine.motion_evaluated` v1, scheduler, catch-up, backfill, `simulate` y API de versiones (§6) |
| Alert Management | Una alerta ACTIVA por punto, suscripción y catch-up de MOVIMIENTO, `DecideMotion`, cambios acotados en `Decide` de FUGA, mensajes de gestión con su hora, catálogo v7, rebuild intercalado (§7, §8, §11) |

**No cambia:** Ingestion, Processing y Baseline; la ecuación del índice, los umbrales de FUGA, la planitud, `rule_evaluations` y `level_evaluated`; la API de gestión y su inmutabilidad.

**Fuera de esta entrega:** subeventos clasificables; varios tramos de temperatura estable por alerta (se guarda el último, y los mensajes cuentan todos); alerta por temperatura estable sin movimiento; subir de nivel cuando coinciden ruido y movimiento (§14); línea base de movimiento en el módulo Baseline (§14).

Mientras no llegue la entrega 2, Alert Management publica `alert_notification` y `alert_changed` con el contrato actual y `event_type` = MOVIMIENTO cuando el mensaje viene de ese flujo. La app ya acepta MOVEMENT.

---

## 4. Conceptos y nombres

### 4.1 Estado del movimiento (`motion_state`, por hora)

| Valor | Significado |
|---|---|
| NORMAL | Sin cambio relevante |
| SALTO | Pitch y roll cambiaron juntos y de golpe en la última hora |
| DESPLAZADO | El sensor sigue fuera de la posición que tenía antes de un salto |
| RETORNO | En esta hora el sensor volvió a su posición anterior |
| BRUSCO | Un eje cambió 0,04° o más en 3 h o menos |

### 4.2 Estado de la temperatura (`temp_state`, por hora)

Solo se calcula con `temp_mode = MOVIMIENTO_Y_TEMPERATURA`. Con `SOLO_MOVIMIENTO` es NULL.

| Valor | Significado |
|---|---|
| TEMP_VARIABLE | La temperatura varía como siempre (estado normal) |
| VERIFICANDO | Hubo un salto del sensor; se espera a ver si la temperatura se estabiliza |
| TEMP_ESTABLE | Después del salto, la temperatura casi no cambia |
| RETORNO_TEMP_ESTABLE | El sensor volvió, pero la temperatura sigue estable |
| SIN_ESTABILIZAR | Pasaron 36 h desde el salto y la temperatura nunca se estabilizó |

Los nombres describen la medición, no el estado del gasoducto.

---

## 5. Rule Engine: algoritmo `MotionAssess`

`MotionAssess` es una función pura. Recibe los registros crudos de una ventana, el estado de la hora anterior y los parámetros, y devuelve la fila de la hora. Con los parámetros de §5.2 debe reproducir exactamente `motion_expected.csv`.

### 5.1 Entrada y convenciones

- **Registros:** `raw_measurements` del punto, métricas 1 (pitch), 2 (roll), 19 (deriva) y 20 (temperatura), agrupadas por `timestamp`. Solo cuenta un registro que tenga pitch **y** roll.
- **Hora t:** `evaluated_at` es el fin de la ventana; la hora t cubre [t − 1 h, t), igual que `feature_evaluations`. El equipo transmite cada 15 min (:00:42, :15:42, :30:42, :45:42), así que una hora completa tiene 4 registros.
- **Mediana:** con un número par de valores, el promedio de los dos centrales (como `pandas.median`).
- **Nivel de la hora** (`pitch_now`, `roll_now`, `temp_now`): mediana de [t − 1 h, t).
- **Todo en UTC.** Resolución del equipo: 0,01°. Al comparar con un umbral, usar `>=` / `<` como dice cada regla, con tolerancia de 1e-9.
- **Orden:** las horas de un punto se evalúan en orden ascendente, una a la vez (lock por punto).

### 5.2 Parámetros (`motion_threshold_versions`, v1 del MP-11556)

| Parámetro | Valor | Uso |
|---|---|---|
| `alerts_enabled` | true (puntos nuevos: false) | false = modo sombra |
| `temp_mode` | MOVIMIENTO_Y_TEMPERATURA (nuevos: SOLO_MOVIMIENTO) | Con SOLO_MOVIMIENTO no se ejecuta el paso 7 |
| `records_per_hour` / `min_records_hour` | 4 / 3 | Con menos de 3 registros la hora es NO_CALCULABLE |
| `jump_deg` / `deriva_jump_deg` | 0,03 / 0,03 | Salto (paso 3) |
| `shift_deg` / `shift_window_hours` / `shift_min_records` | 0,04 / 2 / 6 | Desplazamiento sostenido (paso 5) |
| `base_from_hours` / `base_to_hours` / `base_min_records` | 24 / 6 / 36 | Base [t − 24 h, t − 6 h) |
| `gap_min_hours` / `gap_pre_hours` / `gap_post_minutes` | 1 / 2 / 45 | Hueco (paso 4) |
| `gap_pre_min_records` / `gap_post_min_records` | 3 / 2 | Hueco |
| `return_fraction` | 0,5 | Retorno: recuperó al menos la mitad del desplazamiento (paso 6) |
| `shift_suppress_after_return_hours` | 24 | Sin desplazamiento nuevo en las 24 h siguientes a un retorno |
| `abrupt_deg` / `abrupt_hours` | 0,04 / 3 | Cambio brusco (paso 8) |
| `abrupt_suppress_after_joint_hours` | 6 | Sin cambio brusco en las 6 h siguientes a un desplazamiento o retorno |
| `slow_drift_deg_24h` | 0,04 | Solo marca `slow_drift_flag` |
| `still_hours` / `still_min_calculated` / `still_range_deg` | 24 / 20 / 0,02 | Quieto (paso 9) |
| `temp_flat_rate` / `temp_flat_hours` / `temp_flat_min_rates` | 0,03 °C/h / 6 / 5 | Temperatura estable |
| `temp_flow_rate` / `temp_flow_hours` | 0,08 °C/h / 2 | Temperatura variable |
| `stabilize_max_hours` | 36 | VERIFICANDO → SIN_ESTABILIZAR |
| `still_stable_first_hours` / `still_stable_every_hours` | 6 / 12 | Aviso "la temperatura sigue estable" tras el retorno |
| `trend_deg_7d` | 0,04 | Solo marca `trend_flag` |

### 5.3 Variables de la hora t

| Variable | Definición exacta | Nula si |
|---|---|---|
| `n_records`, `coverage` | Registros en [t − 1 h, t); `coverage` = n / 4 | — |
| `dp_1h`, `dr_1h` | Nivel − mediana de [t − 3 h, t − 1 h) | Menos de 4 registros en esa ventana |
| `base_pitch`, `base_roll` | Mediana de [t − 24 h, t − 6 h) | Menos de 36 registros |
| `dp_6h`, `dr_6h` | Nivel − base | Base nula |
| `shift_sign` | +1 si **todos** los registros de [t − 2 h, t) tienen pitch − base_pitch ≥ 0,04 y roll − base_roll ≥ 0,04; −1 si todos ≤ −0,04 en los dos; si no, 0 | 0 si la base es nula o hay menos de 6 registros |
| `deriva_max_1h` | Máximo de deriva en [t − 1 h, t) | Sin deriva |
| `jump` | `deriva_max_1h` ≥ 0,03, **o** \|dp_1h\| ≥ 0,03 y \|dr_1h\| ≥ 0,03 con el mismo signo | — |
| `abrupt_delta` | Para k = 1, 2, 3 y cada eje: nivel(t) − nivel(t − k h), solo entre horas calculadas. Se queda el de mayor valor absoluto entre los que llegan a 0,04; guarda eje, delta y k | Ninguno llega a 0,04 |
| `dp_24h`, `dr_24h` | Nivel − nivel de la hora t − 24 h | Esa hora fue NO_CALCULABLE |
| `slow_drift_flag` | \|dp_24h\| ≥ 0,04 o \|dr_24h\| ≥ 0,04. Solo se guarda | false si falta alguno |
| `trend_flag` | \|nivel − nivel de t − 168 h\| ≥ 0,04 en algún eje. Solo se guarda | false si esa hora no se calculó |
| `temp_rate(h)` | \|temp_now(h) − temp_now(h − 1 h)\| | Falta alguna de las dos |
| `temp_rate_max_6h` | Máximo de `temp_rate` en t, t − 1 h, …, t − 5 h | Ninguna disponible |
| `temp_flat` | Al menos 5 de esas 6 tasas existen y el máximo < 0,03 | — |
| `temp_flow` | `temp_rate(t)` ≥ 0,08 y `temp_rate(t − 1 h)` ≥ 0,08 | false si falta alguna |

### 5.4 Estado que pasa de una hora a la siguiente

Se guarda en la fila y se lee de la fila anterior, sin memoria entre horas: `displaced`, `ref_pitch`, `ref_roll`, `shift_sign`, `shift_started_at`, `shift_level_pitch`, `shift_level_roll`, `gap_from`, `gap_to`, `last_joint_at`, `last_return_at`, `return_started_at`, `temp_state`, `temp_since`, `temp_stable_from`, `temp_stable_to`, `still_stable_msgs`, `in_episode`, `episode_opened_at`, `abrupt_active` y `pending_gaps`.

### 5.5 Pasos de cada hora, en este orden

1. **Cobertura.** Si `n_records` < 3, la hora es NO_CALCULABLE: se guarda la fila con el estado copiado de la anterior y `episode_event` = NONE. Si en esta hora tocaba evaluar un hueco, pasa a `pending_gaps`. No se ejecuta ningún otro paso. Las cuentas por reloj (36 h, 6 h, 12 h) siguen corriendo y se revisan en la próxima hora calculada.
2. **Variables** de §5.3.
3. **Salto.** Si `jump`, motivo `MOV_JUMP {dp, dr, deriva}`.
4. **Hueco.** Un hueco es un par de registros consecutivos `prev`, `f` con `f − prev` ≥ 1 h. Se evalúa en la primera hora t > f + 45 min, o en la siguiente calculada si esa no lo es.
   - pre = mediana de (prev − 2 h, prev], con al menos 3 registros.
   - post = mediana de [f, f + 45 min], con al menos 2 registros.
   - Si post − pre ≥ 0,04 en los dos ejes (o ≤ −0,04 en los dos) y `displaced` es falso: `displaced` = true, `ref` = pre, `shift_level` = post, `shift_sign` = el signo, `shift_started_at` = `gap_from` = prev, `gap_to` = f, `last_joint_at` = t. Motivo `MOV_SHIFT_GAP {dp, dr, gap_from, gap_to}`. Cuenta como desplazamiento nuevo.
5. **Desplazamiento sostenido.** Si `displaced` es falso, `shift_sign` ≠ 0, no hubo retorno en las últimas 24 h **y hubo al menos una hora con `jump` en [t − 6 h, t]** (el desplazamiento tiene que haber sido brusco; uno lento en los dos ejes es normal y no abre nada):
   - `displaced` = true, `ref` = base, `shift_level` = nivel de t, `last_joint_at` = t.
   - `shift_started_at` es el primer registro de [t − 6 h, t) con deriva ≥ 0,03. Si no hay, el primer registro de la primera hora con `jump` en [t − 6 h, t]. Si tampoco hay, el primer registro de [t − 6 h, t) con los dos ejes a 0,03° o más de la base en el sentido del signo.
   - Motivo `MOV_SHIFT {dp, dr, started_at}`. Cuenta como desplazamiento nuevo.
6. **Retorno.** Si `displaced` ya era verdadero antes de esta hora y, en los dos ejes, `signo × (shift_level − nivel)` ≥ 0,5 × `signo × (shift_level − ref)`:
   - `displaced` = false, `last_joint_at` = `last_return_at` = t.
   - `return_started_at` es el primer registro de [t − 6 h, t) con los dos ejes a 0,03° o más de `shift_level`, hacia la referencia. Si no hay, t − 1 h.
   - Motivo `MOV_RETURN {dp, dr, started_at}`.
   - El retorno se mide contra la referencia y no por el signo: sirve igual si otro sensor está montado al revés.
7. **Temperatura**, solo con MOVIMIENTO_Y_TEMPERATURA (ver §5.6).
8. **Cambio brusco.** El movimiento lento es normal; solo alerta un cambio rápido de un eje que no es un salto conjunto.
   - `abrupt` se cumple si existe `abrupt_delta`, `jump` es falso, `displaced` es falso, no hubo desplazamiento nuevo en esta hora y t − `last_joint_at` > 6 h (o nunca hubo).
   - Si `abrupt` y `abrupt_active` era falso: motivo `MOV_ABRUPT {axis, delta, hours}`. Solo al empezar: las horas siguientes del mismo cambio no lo repiten.
   - `abrupt_active` = `abrupt`. Las horas NO_CALCULABLE no lo tocan.
9. **Quieto.** `still` se cumple si en las horas t − 23 h … t hay al menos 20 calculadas, ninguna con `jump`, y el rango (máx − mín) del nivel es ≤ 0,02 en los dos ejes.
10. **Episodio** (ver §5.7).

### 5.6 Estado de la temperatura (paso 7)

Se parte del estado al empezar la hora y las transiciones se aplican en este orden:

| Si está | Y | Pasa a | Motivo |
|---|---|---|---|
| TEMP_VARIABLE | Hubo un desplazamiento nuevo (paso 4 o 5) | VERIFICANDO | `TEMP_VERIFYING` |
| VERIFICANDO | Retorno | TEMP_VARIABLE | Ninguno |
| VERIFICANDO | `temp_flat` | TEMP_ESTABLE; `temp_stable_from` = `shift_started_at` | `TEMP_STABILIZED {temp_rate_max_6h, temp_stable_from}` |
| VERIFICANDO | t − `shift_started_at` ≥ 36 h | SIN_ESTABILIZAR | `TEMP_NOT_STABILIZED {hours}` |
| SIN_ESTABILIZAR | Retorno | TEMP_VARIABLE | Ninguno |
| TEMP_ESTABLE | Retorno | RETORNO_TEMP_ESTABLE; `still_stable_msgs` = 0 | (el `MOV_RETURN` del paso 6) |
| TEMP_ESTABLE o RETORNO_TEMP_ESTABLE, ya aplicada la fila anterior | `temp_flow` | TEMP_VARIABLE; `temp_stable_to` = `return_started_at` si venía de RETORNO_TEMP_ESTABLE, si no t − 2 h | `TEMP_VARIES_AGAIN {rate, temp_stable_from, temp_stable_to}` |
| RETORNO_TEMP_ESTABLE sin `temp_flow` | horas desde `temp_since` ≥ 6 + 12 × `still_stable_msgs` | Sigue igual; `still_stable_msgs` + 1 | `TEMP_STILL_STABLE {hours, temp_rate_max_6h}` |

`temp_since` = t cada vez que cambia el estado. El retorno y la vuelta de la temperatura pueden ocurrir en la misma hora (así pasó en agosto y en octubre); por eso las dos últimas filas se evalúan después de la del retorno.

### 5.7 Episodio de MOVIMIENTO (paso 10)

- **OPENED:** fuera de episodio y en la hora hubo `jump`, un desplazamiento nuevo o `MOV_ABRUPT`. `episode_opened_at` = t.
- **CLEARED:** dentro de episodio y se cumplen tres cosas:
  - `still`;
  - `temp_state` en TEMP_VARIABLE, SIN_ESTABILIZAR o NULL;
  - t − `episode_opened_at` ≥ 24 h.

  Si `displaced` seguía verdadero, pasa a falso y se agrega `MOV_NEW_POSITION`: la posición actual queda como la normal. Si estaba en SIN_ESTABILIZAR, pasa a TEMP_VARIABLE. Se agrega `MOV_STILL {hours: 24}`.
- **`computed_level`:** AMARILLO dentro del episodio, si no NORMAL.
- **`motion_state`:** el primero que aplique entre DESPLAZADO (`displaced` al final de la hora), RETORNO, SALTO, BRUSCO (`abrupt_active`) y NORMAL. En una hora NO_CALCULABLE: DESPLAZADO si `displaced`, si no NULL.

---

## 6. Rule Engine: datos, evento y ejecución

### 6.1 `motion_threshold_versions`

- **Columnas:** `id`, `measuring_point_id`, `version`, los parámetros de §5.2 (NUMERIC(6,4) para grados y °C/h, INT para horas y registros, TEXT `temp_mode`, BOOLEAN `alerts_enabled`), `valid_from`, `valid_to`, `reason`, `created_by`, `created_at`.
- **Restricciones:** `UNIQUE (measuring_point_id, version)`; `EXCLUDE USING gist (measuring_point_id WITH =, tstzrange(valid_from, valid_to) WITH &&)`; `CHECK temp_mode IN ('MOVIMIENTO_Y_TEMPERATURA','SOLO_MOVIMIENTO')`, `0 < jump_deg <= shift_deg`, `0 < return_fraction <= 1`, horas y registros ≥ 1, `min_records_hour <= records_per_hour`.
- **Seed:** con la tabla vacía, cada punto de `devices` recibe la v1 con `valid_from` = su primera medición. El MP-11556, con los valores de §5.2; el resto, con `alerts_enabled = false` y `SOLO_MOVIMIENTO`. Un punto nuevo sin versión recibe la v1 de punto nuevo con `valid_from` = now.
- **Versionado:** igual que `rule_threshold_versions`. Solo se agregan versiones, nunca retroactivas, y la abierta se cierra en la misma transacción con `FOR UPDATE NOWAIT` (409 si hay conflicto).

### 6.2 `motion_evaluations` (una fila por punto y hora, también NO_CALCULABLE)

| Grupo | Columnas |
|---|---|
| Clave | `id`, `measuring_point_id`, `evaluated_at`, `status` (CALCULATED \| NO_CALCULABLE), `n_records`, `coverage` |
| Variables | `pitch_now`, `roll_now`, `temp_now`, `dp_1h`, `dr_1h`, `base_pitch`, `base_roll`, `dp_6h`, `dr_6h`, `shift_sign`, `deriva_max_1h`, `jump`, `abrupt`, `dp_24h`, `dr_24h`, `slow_drift_flag`, `trend_flag`, `temp_rate`, `temp_rate_max_6h`, `temp_flat`, `temp_flow`, `still` |
| Estado | Los de §5.4 (`pending_gaps` en jsonb) |
| Resultado | `motion_state`, `in_episode`, `episode_opened_at`, `episode_event` (OPENED \| CLEARED \| NONE), `computed_level` (NORMAL \| AMARILLO), `reasons` (jsonb `[{code, data}]`, formato de FUGA) |
| Trazabilidad | `threshold_version_id` (FK), `algorithm_version` (`motion-v1`), `trigger` (LIVE \| CATCH_UP \| BACKFILL), `input_max_ts`, `created_at` |

`UNIQUE (measuring_point_id, evaluated_at)` y `CHECK` en los enums.

Códigos de `reasons`:
- **Movimiento:** `MOV_JUMP`, `MOV_SHIFT`, `MOV_SHIFT_GAP`, `MOV_RETURN`, `MOV_ABRUPT`, `MOV_STILL`, `MOV_NEW_POSITION`.
- **Temperatura:** `TEMP_VERIFYING`, `TEMP_STABILIZED`, `TEMP_NOT_STABILIZED`, `TEMP_VARIES_AGAIN`, `TEMP_STILL_STABLE`.

### 6.3 Evento `rule_engine.motion_evaluated` v1

Uno por punto y hora evaluada en vivo (LIVE o CATCH_UP), también NORMAL y NO_CALCULABLE, después del commit. No se publica en backfill, en duplicados, en horas tardías ni con `alerts_enabled = false`.

```json
{
  "motion_evaluation_id": 1173,
  "measuring_point_id": 11556,
  "event_type": "MOVIMIENTO",
  "evaluated_at": "2026-08-26T03:00:00Z",
  "status": "CALCULATED",
  "motion_state": "DESPLAZADO",
  "temp_state": "TEMP_ESTABLE",
  "temp_mode": "MOVIMIENTO_Y_TEMPERATURA",
  "computed_level": "AMARILLO",
  "in_episode": true,
  "episode_event": "NONE",
  "episode_opened_at": "2026-08-25T12:00:00Z",
  "shift": { "pitch": 0.08, "roll": 0.13, "started_at": "2026-08-25T11:15:42Z", "gap_from": null, "gap_to": null },
  "temp_stable": { "from": "2026-08-25T11:15:42Z", "to": null },
  "reasons": [ { "code": "TEMP_STABILIZED", "data": { "temp_rate_max_6h": 0.025, "temp_stable_from": "2026-08-25T11:15:42Z" } } ],
  "threshold_version_id": 1,
  "occurred_at": "2026-08-26T03:40:05Z",
  "schema_version": 1
}
```

`shift.pitch` y `shift.roll` = nivel actual − referencia mientras `displaced`; si no, null. Clave de idempotencia del consumidor: `(measuring_point_id, evaluated_at)`.

### 6.4 Ejecución

- **Scheduler** `MotionScheduler`: al arrancar y cada hora en el minuto `RULE_ENGINE_MOTION_MINUTE` (40). Por cada punto con versión, evalúa en orden las horas desde la última fila + 1 h hasta el inicio de la hora actual: trigger LIVE para la última, CATCH_UP para las anteriores. No depende de Processing.
- **Punto sin filas:** se saltea con un Warn ("correr rule_engine -backfill -motion").
- **Datos que llegan después del minuto 40:** la hora ya guardada no se recalcula. Un salto ocurrido sin datos lo detecta la regla de hueco.
- **Backfill:** `rule_engine -backfill -motion [-point] [-from] [-to] [-rebuild]`. No publica, es idempotente y retoma. `-rebuild` borra `motion_evaluations` del punto o rango y recalcula. Bloqueado con `AGATHA_ENV=production`. Sin filas, arranca desde la primera medición.
- **Simulate:** `POST /api/rule-engine/points/{id}/motion-thresholds/simulate` con `{patch, from, to}`. En memoria, sin escribir. Devuelve los episodios (apertura, cierre, motivos, tramo de temperatura estable) y la comparación con la versión vigente.
- **API de versiones:** `GET /api/rule-engine/points/{id}/motion-thresholds`, `GET …/history`, `POST …` (`patch`, `valid_from`, `reason`, `created_by`), con los mismos errores que FUGA.

### 6.5 Código

- `domain/services/motion_assess.go`: `MotionAssess(window []RawRecord, prev MotionState, p MotionParams, t time.Time) MotionEvaluation`. Puro y sin reloj.
- `domain/entities`: `MotionParams`, `MotionState`, `MotionEvaluation`, enums.
- `application/ports`: `RawMeasurementSource` (lee las métricas 1, 2, 19 y 20 de `raw_measurements` en [t − 169 h, t), solo lectura, modelo propio), `MotionEvaluationRepository`, `MotionThresholdRepository`, `MotionEventPublisher`.
- `application/services`: `MotionEvaluationService`, `MotionBackfillService`, `MotionSimulationService`, `MotionThresholdService`.
- `contract/`: `TopicMotionEvaluated` + `MotionEvaluated`, solo stdlib.

---

## 7. Alert Management: acople fuga ↔ movimiento

Alert Management procesa dos flujos por punto, FUGA (`level_evaluated`) y MOVIMIENTO (`motion_evaluated`), sobre la misma alerta activa: abre el primero, el otro se une, y la alerta finaliza cuando se cierra el último episodio abierto. Una alerta en la que nunca participa el movimiento se comporta exactamente como hoy.

### 7.1 Datos

| Tabla | Cambio |
|---|---|
| `alerts` | Nuevas: `events text[] NOT NULL` (default `ARRAY[event_type]`; valores FUGA, MOVIMIENTO), `motion_state`, `temp_state`, `temp_stable_from`, `temp_stable_to`, `last_motion_evaluated_at`. `event_type` queda como origen (lo que abrió la alerta) |
| Índice | `ux_alerts_one_active` pasa a `UNIQUE (measuring_point_id) WHERE state = 'ACTIVA'`. La migración aborta si algún punto tiene dos activas |
| `alert_processing_state` | Nueva columna `in_episode boolean NOT NULL DEFAULT false`. Se usa una fila (punto, MOVIMIENTO) junto a la de FUGA |
| `alert_messages` | Sin cambios de esquema; `catalog_version` = 7 en los mensajes nuevos |

### 7.2 Concurrencia y orden

- Un lock por punto serializa los dos flujos, en vivo, en el catch-up y en el backfill.
- Cada flujo tiene su propio `last_evaluated_at`. Un duplicado o una hora vieja de ese flujo no hace nada.
- Para decidir el fin, cada flujo lee el `in_episode` del otro tal como quedó en su última hora procesada.
- **Backfill y rebuild:** por cada hora t ascendente, primero FUGA y después MOVIMIENTO. En vivo el orden puede variar; la única diferencia posible es una reactivación en vez de una unión, sin perder mensajes.

### 7.3 Flujo FUGA: cambios en `Decide`

1. **OPENED con una alerta ACTIVA en el punto** (abierta o sostenida por movimiento):
   - Agrega FUGA a `events` y marca `in_episode` = true en FUGA.
   - Emite `LEAK_JOINED` (Notificación). Si `computed_level` supera el `current_level`, abre un color nuevo con ese nivel y el mismo mensaje lo lleva; no se emite además un `UP_*`.
   - `started_at` no cambia.
2. **CLEARED con MOVIMIENTO en episodio:** no finaliza. Marca `in_episode` = false en FUGA y emite `PART_NORMAL` variante ruido (Actualización).
3. **Horas de FUGA con la alerta activa y FUGA fuera de episodio** (la alerta sigue por el movimiento): solo se aplican la bajada de nivel (6 h por debajo, nunca por debajo de AMARILLO) y `ROJO_HELD`. No se emiten `ALMOST_NORMAL`, `TREND`, `COUNTDOWN`, `SUMMARY` ni `NO_CALC_*`.
4. **Reactivación:** se busca la última alerta FINALIZADA del punto, sin importar su origen. El resto de la regla no cambia (menos de 24 h y Sin clasificar).
5. **Mantenimiento:** se elimina `UP_DURING_INTERVENTION`. Con `MANTENIMIENTO_PROGRAMADO`, las subidas emiten su `UP_*` normal (Notificación).

### 7.4 Flujo MOVIMIENTO: `DecideMotion` (nuevo, puro)

| Llega | Condición | Acción | Mensaje |
|---|---|---|---|
| NO_CALCULABLE | — | Actualiza `last_evaluated_at` | Ninguno |
| `OPENED` | Sin alerta ACTIVA; la última FINALIZADA del punto terminó hace menos de 24 h y está Sin clasificar | Reactiva esa alerta. Agrega MOVIMIENTO a `events` | `MOV_OPEN` (Notificación) |
| `OPENED` | Sin alerta ACTIVA, en otro caso | Alerta nueva: `event_type` = MOVIMIENTO, `events` = {MOVIMIENTO}, `current_level` = `max_level` = AMARILLO, `opened_at` = t, `started_at` según §7.5 | `MOV_OPEN` (Notificación) |
| `OPENED` | Con alerta ACTIVA | Agrega MOVIMIENTO a `events`; el color no cambia | `MOV_JOINED` (Actualización) |
| `MOV_SHIFT`, `MOV_SHIFT_GAP` o `MOV_ABRUPT` en una hora que no es OPENED | Alerta ACTIVA | — | `MOV_DISPLACED` con su variante (Actualización) |
| `TEMP_STABILIZED` | Alerta ACTIVA | `temp_stable_from` = el del motivo; `temp_stable_to` = NULL | `TEMP_STABILIZED` (Notificación) |
| `MOV_RETURN` | Alerta ACTIVA | — | `MOV_RETURN` (Actualización) |
| `TEMP_VARIES_AGAIN` | Alerta ACTIVA | `temp_stable_to` = el del motivo | `TEMP_VARIES_AGAIN` (Notificación) |
| `TEMP_NOT_STABILIZED` | Alerta ACTIVA | — | `TEMP_NOT_STABILIZED` (Actualización) |
| `TEMP_STILL_STABLE` | Alerta ACTIVA | — | `TEMP_STILL_STABLE` (Actualización) |
| `CLEARED` | FUGA en episodio | Marca `in_episode` = false en MOVIMIENTO | `PART_NORMAL` variante sensor (Actualización) |
| `CLEARED` | FUGA fuera de episodio | FINALIZADA, `ended_at` = t | `END_NORMAL` variante movimiento |
| Cualquier hora calculada | Alerta ACTIVA | Copia `motion_state`, `temp_state` y `last_motion_evaluated_at` | — |

En una misma hora pueden salir varios mensajes; se guardan en el orden de la tabla. Un `MOV_JUMP` suelto dentro de un episodio abierto no emite nada.

### 7.5 Inicio y variante de una alerta abierta por movimiento

- **`started_at`:**
  - con `MOV_ABRUPT`: t − (`hours` − 1) h;
  - con `MOV_SHIFT_GAP`: el fin de la hora que contiene `gap_from`;
  - en otro caso: t.
- **Variante de `MOV_OPEN`:** sale del primer motivo presente en este orden: `MOV_SHIFT_GAP` (hueco), `MOV_SHIFT` o `MOV_JUMP` (salto), `MOV_ABRUPT` (brusco).

### 7.6 Gestión: no silencia y queda en la línea de tiempo

- **No silencia:** ninguna regla de los dos flujos lee la clasificación ni la etiqueta, salvo las de hoy (`ROJO_HELD`, la bajada del rojo y la reactivación). Una alerta clasificada sigue generando todos sus mensajes y Notificaciones.
- **Al clasificar:** cuando `POST /{id}/classification` responde `SAVED`, en la misma transacción se inserta un mensaje kind GESTION, código `CLASSIFIED`.
  - `evaluated_at` = `classified_at` (hora del servidor, UTC, al segundo).
  - `segment_id` = el color vigente en ese momento.
  - `params`: `classification`, `origin`, `device_identifier`, `recorded_at`, `observation`.
- **Al etiquetar:** igual con `POST /{id}/label`: código `LABELED` y `evaluated_at` = `labeled_at`. En `params` van `label` y `other_text` en vez de `classification`.
- **Sin mensaje:** un 409, un reenvío (`ALREADY_RECEIVED`) y una observación suelta no generan mensaje GESTION.
- **Orden:** `GET /{id}` ordena los mensajes de cada color por `evaluated_at`. Una clasificación de las 14:23 queda entre los mensajes horarios de las 14:00 y las 15:00.
- **Publicación:** clasificar no publica `alert_notification`. Sí publica `alert_changed` `CLASSIFIED` / `LABELED`, como hoy.

### 7.7 Código

- `domain/services/decide_motion.go`: `DecideMotion(alert *Alert, motion MotionNotice, otherInEpisode bool, lastFinished *Alert, policy Policy) Decision`.
- `domain/services/decide.go`: los cambios de §7.3, con `motionInEpisode bool` como entrada nueva.
- `entities.MotionNotice`, `MotionEvaluatedSubscriber` (traducción en `internal/bootstrap`) y `MotionSource` (lee `motion_evaluations` con un modelo propio, solo lectura).
- `ManagementService`: inserta el mensaje GESTION en la misma transacción del paso guardado.
- Catch-up del minuto 55: lee también `motion_evaluations` desde el `last_evaluated_at` de la fila (punto, MOVIMIENTO).

---

## 8. Catálogo v7

`MessageCatalogVersion` pasa a 7. Se agregan los códigos de movimiento, los textos de gestión y una variante de `END_NORMAL`. Se elimina `UP_DURING_INTERVENTION`. Los demás códigos de fuga no cambian.

Reglas de siempre:
- La frase simple primero y el dato al final.
- El cuerpo no repite el título.
- Nunca se dice qué hacer ni se usa "umbral" o "persistencia".
- Hora de Colombia y fecha dd/mm.
- Grados con signo y dos decimales (+0,08°); °C/h con tres decimales si hace falta.
- **Ningún texto afirma que el gasoducto se cerró o se reabrió.**

### 8.1 Movimiento y temperatura

| Código (variante) | Tipo | Título | Cuerpo |
|---|---|---|---|
| `MOV_OPEN` (salto) | Notificación | Movimiento brusco del sensor | Pitch y roll se inclinaron juntos y de golpe a las {hora} del {fecha}. Dato: pitch {dp}, roll {dr} en 1 h (lo normal es menos de 0,01°). |
| `MOV_OPEN` (hueco) | Notificación | El sensor cambió de posición sin datos | Al volver los datos, a las {hora} del {fecha}, el sensor está en otra posición. Estuvo {duracion_hueco} sin transmitir. Dato: pitch {dp}, roll {dr}. |
| `MOV_OPEN` (brusco) | Notificación | Movimiento brusco del sensor | El sensor se movió de golpe hacia un solo lado a las {hora} del {fecha}. Dato: {eje} {delta} en {horas} h (lo normal es menos de 0,03° en 3 h). |
| `MOV_JOINED` | Actualización | El sensor se movió | Ocurrió durante la alerta por ruido, a las {hora} del {fecha}. Dato: pitch {dp}, roll {dr}. |
| `MOV_DISPLACED` (salto / hueco / brusco) | Actualización | El sensor sigue en la nueva posición | No ha vuelto a su posición desde las {hora_inicio}. Dato: pitch {dp}, roll {dr}. (Brusco: "El sensor se volvió a mover hacia un solo lado a las {hora}. Dato: {eje} {delta} en {horas} h.") |
| `TEMP_STABILIZED` | Notificación | La temperatura de la tubería se estabilizó | Desde que se movió el sensor ({hora_inicio} del {fecha_inicio}) la temperatura casi no cambia. Pasó lo mismo en cierres anteriores, pero no está confirmado. Dato: máximo {rate} °C por hora (normal: más de 0,08). |
| `MOV_RETURN` | Actualización | El sensor volvió a su posición | Regresó a las {hora} a la posición que tenía antes de las {hora_inicio} del {fecha_inicio}. Dato: pitch {dp}, roll {dr}. |
| `TEMP_VARIES_AGAIN` | Notificación | La temperatura de la tubería volvió a variar | Estuvo estable {duracion} ({desde} – {hasta}).{sin_retorno} Dato: {rate} °C en una hora. |
| `TEMP_NOT_STABILIZED` | Actualización | La temperatura no se estabilizó | El sensor lleva {horas} h fuera de su posición y la temperatura sigue variando como siempre. Dato: pitch {dp}, roll {dr}. |
| `TEMP_STILL_STABLE` | Actualización | La temperatura sigue estable | El sensor volvió hace {horas} h, pero la temperatura de la tubería sigue casi quieta. Dato: máximo {rate} °C por hora (normal: más de 0,08). |
| `LEAK_JOINED` | Notificación | Ruido alto durante una alerta de movimiento | El índice de ruido está {nivel_texto} desde las {hora} del {fecha}. {contexto} Dato: {i} (lo normal es menos de {normal}). |
| `PART_NORMAL` (ruido) | Actualización | El ruido volvió a lo normal | La alerta sigue abierta porque {motivo}. Dato: {i}. |
| `PART_NORMAL` (sensor) | Actualización | El sensor está estable | Lleva 24 h quieto. La alerta sigue abierta por el ruido ({nivel}). Dato: {i}. |
| `END_NORMAL` (movimiento) | Mismo kind que hoy | Volvió a la normalidad | Duró {duracion}, nivel máximo {max}.{tramo}{nueva_posicion} |

Variables:
- **`{nivel_texto}`:** "por encima de lo normal" (AMARILLO), "alto" (NARANJA), "demasiado alto" (ROJO).
- **`{contexto}`**, la primera que aplique:
  - "El sensor se movió y la temperatura está estable desde las {hora} del {fecha}." (TEMP_ESTABLE o RETORNO_TEMP_ESTABLE)
  - "La temperatura volvió a variar hace {n} h." (menos de 24 h desde `TEMP_VARIES_AGAIN`)
  - "El sensor se movió a las {hora} del {fecha}."
- **`{motivo}`:**
  - "el sensor se movió y la temperatura sigue estable" (TEMP_ESTABLE);
  - "el sensor volvió pero la temperatura sigue estable" (RETORNO_TEMP_ESTABLE);
  - "el sensor sigue fuera de su posición" (`displaced`);
  - "el sensor no lleva 24 h quieto" (resto).
- **`{sin_retorno}`:** " El sensor no volvió a su posición anterior." si la temperatura volvió a variar sin `MOV_RETURN`; si no, vacío.
- **`{tramo}`:** " Temperatura estable del {desde} al {hasta}." si la alerta tiene `temp_stable_to`; si no, vacío.
- **`{nueva_posicion}`:** " La posición actual del sensor queda como la normal." si hubo `MOV_NEW_POSITION`; si no, vacío.

### 8.2 Gestión

| Código | Kind | Título | Cuerpo |
|---|---|---|---|
| `CLASSIFIED` | GESTION | Clasificada como {clasificacion} | Registrada {origen} el {fecha} a las {hora}.{registro}{observacion} |
| `LABELED` | GESTION | Etiqueta: {etiqueta} | Registrada {origen} el {fecha} a las {hora}.{registro}{observacion} |

- **`{clasificacion}`:** Confirmada · Falsa alerta.
- **`{etiqueta}`:** Fuga confirmada en campo · Movimiento de tierra · Maquinaria ajena · Clima extremo · Tránsito pesado · Mantenimiento programado · Otro: {other_text}.
- **`{origen}`:** "desde la web" · "desde la app".
- **`{registro}`:** " Hecha en el celular a las {hora_registro}." solo si `recorded_at` difiere en más de 5 min de la hora guardada.
- **`{observacion}`:** " Observación: «{texto}»" si vino con el paso.

### 8.3 Pruebas del catálogo

- Cada código declara sus variables y ningún texto usa una que no declara.
- Todo mensaje de movimiento o de temperatura cita un dato.
- Ninguno repite el título, dice qué hacer ni usa "umbral", "persistencia", "cierre", "cerrado", "reapertura" o "flujo de gas".
- Con `temp_mode = SOLO_MOVIMIENTO` no sale ningún código `TEMP_*`.

---

## 9. Combinaciones y escenarios

### 9.1 Matriz

| Ruido alto | Movimiento brusco | Temperatura estable | Resultado | Escenarios |
|---|---|---|---|---|
| Sí | — | — | Alerta por ruido, como hoy | 1 |
| — | Sí | — | Alerta de movimiento, Amarillo | 2, 9, 14, 15 |
| — | Sí | Sí | Alerta de movimiento, Amarillo, con el contexto de la temperatura | 4, 10, 11, 12 |
| Sí | Sí | — | Una alerta con los dos; el color lo da el ruido | 6, 7 |
| Sí | Sí | Sí | Una alerta con los dos y el contexto de la temperatura | 5, 8, 13 |
| — | Lento | — | Nada (solo una marca en los datos) | 3 |
| — | — | Sí | Hoy nada (por decidir, §14) | 16 |
| Sí | Sí | — | El color no sube por la coincidencia (por decidir, §14) | 17 |

### 9.2 Escenarios

Cada fila es un test de `Decide` / `DecideMotion`. Tipos: N = Notificación, A = Actualización, G = Gestión, F = Fin. Las líneas de tiempo con los textos completos están en el artifact "Escenarios ruido y movimiento".

| # | Escenario | Mensajes en orden | Finaliza |
|---|---|---|---|
| 1 | Solo ruido | Igual que hoy | Con el CLEARED de FUGA |
| 2 | Cambio brusco de un eje | `MOV_OPEN` brusco (N) → `END_NORMAL` (F) | 24 h quieto y 24 h desde la apertura |
| 3 | Movimiento lento (deriva de días; 19–20 oct) | Ninguno | — |
| 4 | Salto, temperatura estable y regreso, sin ruido | `MOV_OPEN` salto (N) → `MOV_DISPLACED` (A) → `TEMP_STABILIZED` (N) → `MOV_RETURN` (A) + `TEMP_VARIES_AGAIN` (N) → `END_NORMAL` (F) | 24 h quieto después del regreso |
| 5 | Ruido y después salto (agosto, real) | `OPEN` → `UP_NARANJA` → `UP_ROJO` → `MOV_JOINED` (A) → `MOV_DISPLACED` (A) → `PART_NORMAL` ruido (A) → `TEMP_STABILIZED` (N) → `LEAK_JOINED` (N) → `MOV_RETURN` (A) + `TEMP_VARIES_AGAIN` (N) → `PART_NORMAL` sensor (A) → `END_NORMAL` (F) | Con el fin del ruido |
| 6 | Movimiento brusco y después ruido | `MOV_OPEN` (N) → `LEAK_JOINED` (N) → `UP_NARANJA` (N) → `PART_NORMAL` sensor (A) → `END_NORMAL` (F) | Con el fin del ruido |
| 7 | Ruido y movimiento en la misma hora | `OPEN` (N) + `MOV_JOINED` (A) → `PART_NORMAL` ruido (A) → `END_NORMAL` (F) | Cuando el sensor lleva 24 h quieto |
| 8 | Ruido mientras la temperatura está estable | `MOV_OPEN` (N) → `TEMP_STABILIZED` (N) → `LEAK_JOINED` (N, contexto de temperatura) → `UP_NARANJA` (N) → `PART_NORMAL` ruido (A) → `DOWNGRADE` (A) → `MOV_RETURN` (A) + `TEMP_VARIES_AGAIN` (N) → `END_NORMAL` (F) | 24 h después del regreso |
| 9 | Salto sin que la temperatura se estabilice | `MOV_OPEN` (N) → `TEMP_NOT_STABILIZED` (A) a las 36 h + `END_NORMAL` (F) | A las 36 h; la posición nueva queda como normal |
| 10 | El sensor vuelve y la temperatura sigue estable | `MOV_OPEN` → `TEMP_STABILIZED` → `MOV_RETURN` → `TEMP_STILL_STABLE` (A) a las 6, 18, 30 h… → `TEMP_VARIES_AGAIN` (N) → `END_NORMAL` | 24 h después de que la temperatura vuelve a variar |
| 11 | La temperatura vuelve a variar y el sensor no regresa | `MOV_OPEN` → `TEMP_STABILIZED` → `TEMP_VARIES_AGAIN` (con "el sensor no volvió") → `END_NORMAL` (con nueva posición) | 24 h quieto |
| 12 | Salto durante pérdida de datos (20–21 oct, real) | `MOV_OPEN` hueco (N) → `TEMP_STABILIZED` (N) → `MOV_RETURN` (A) + `TEMP_VARIES_AGAIN` (N) → `END_NORMAL` (F) | 24 h después del regreso |
| 13 | Clasificada (Falsa alerta + Mantenimiento) y después ruido | `MOV_OPEN` → `CLASSIFIED` (G, a su hora) → `LABELED` (G, a su hora) → `TEMP_STABILIZED` → `LEAK_JOINED` → `UP_NARANJA` → `MOV_RETURN` + `TEMP_VARIES_AGAIN` → `END_NORMAL` | Igual que sin clasificar |
| 14 | Finaliza y vuelve a moverse antes de 24 h | `MOV_OPEN` → `END_NORMAL` → `MOV_OPEN` (reactivada si está Sin clasificar; alerta nueva si está clasificada) → `END_NORMAL` | 24 h después del segundo cambio |
| 15 | Sin datos con la alerta de movimiento abierta | Ningún mensaje del movimiento durante el hueco | Se atrasa hasta tener 20 horas con datos en 24 |
| 16 | Temperatura estable sin movimiento (por decidir) | Hoy ninguno | — |
| 17 | Ruido y movimiento coinciden (por decidir) | Igual que el 6; el color no sube | — |

Además, se prueban sin escenario gráfico: punto en modo sombra (sin evento ni alerta) y punto con `SOLO_MOVIMIENTO` (sin `TEMP_*`; un salto se cierra al quedar quieto en la posición nueva, y el regreso abre otra alerta).

---

## 10. Otros dispositivos y tramos

Nada de lo aprendido en el MP-11556 se asume en un punto nuevo.

| Diferencia posible | Cómo se cubre |
|---|---|
| El sensor se inclina al revés | El retorno se mide contra la referencia, no por el signo |
| El sensor se mueve más en condiciones normales | Umbrales por punto, calibrados en modo sombra |
| Una maniobra no mueve el sensor | El punto queda en `SOLO_MOVIMIENTO` |
| La temperatura se comporta distinto (otro clima, profundidad o caudal) | `temp_mode` en `SOLO_MOVIMIENTO` hasta verificarlo; parámetros `temp_*` versionados |

**Puesta en marcha de un punto nuevo:**

1. **Alta:** v1 con `alerts_enabled = false` y `SOLO_MOVIMIENTO`. Calcula y guarda, sin publicar.
2. **Sombra, 30 días o más:** `simulate` sobre el periodo. Cada episodio sin una maniobra o trabajo conocido de TGI cuenta como falsa alarma.
3. **Ajuste:** si hubo falsas, subir `jump_deg`, `shift_deg` y `abrupt_deg` por encima del mayor cambio normal observado.
4. **Habilitar:** versión con `alerts_enabled = true`.
5. **Primera maniobra conocida:** si aparecen el salto, el desplazamiento y la temperatura estable, versión con `MOVIMIENTO_Y_TEMPERATURA`.

---

## 11. Rebuild del histórico

### 11.1 Qué hay que recalcular

| Tabla | Acción | Por qué |
|---|---|---|
| `rule_evaluations` (FUGA) | **No se toca** | Ya se reconstruyó el 16 nov con la versión única de planitud |
| `motion_evaluations` | Se calcula todo el histórico, desde la primera medición | Es nueva |
| `alerts`, `alert_level_segments`, `alert_messages`, `alert_processing_state` | Se reconstruyen con los dos flujos intercalados | Cambia la unión de alertas y aparecen las de movimiento |
| Gestión (`alerts.classification`, `label`, `alert_observations`) | **Se borra** | La gestión actual (ALR-000021, 022 y 023) era de prueba |

### 11.2 Guarda del rebuild

`alert_management -backfill -rebuild` borra todo lo del punto, gestión incluida, como hoy. Se agrega una guarda: si alguna alerta del punto tiene clasificación, etiqueta u observaciones, el comando aborta sin escribir, salvo que se pase `-drop-management`. Antes de borrar imprime cuántas alertas, mensajes, clasificaciones y observaciones va a eliminar.

### 11.3 Pasos (desde tu PC hacia Supabase, `AGATHA_ENV` distinto de `production`)

1. Respaldo (opcional):

   ```bash
   pg_dump "$DATABASE_URL" -n public -Fc -f respaldo_antes_movimiento.dump
   ```

2. Desplegar en el VPS las dos partes (Rule Engine y Alert Management). La migración crea las tablas de movimiento, siembra la v1 del MP-11556 desde su primera medición y cambia el índice de alerta activa.
3. Detener el backend en el VPS para que los schedulers no escriban durante el rebuild:

   ```bash
   ssh <vps> 'cd <carpeta del proyecto> && docker compose stop <servicio backend>'
   ```

4. Calcular el movimiento del histórico:

   ```bash
   go run ./cmd/rule_engine -backfill -motion -point 11556
   ```

5. Revisar en la base que estén los eventos de §12.2:

   ```sql
   select evaluated_at at time zone 'America/Bogota' as hora_col, motion_state, temp_state, episode_event,
          jsonb_path_query_array(reasons, '$[*].code') as motivos
   from motion_evaluations
   where measuring_point_id = 11556 and (reasons <> '[]'::jsonb or episode_event <> 'NONE')
   order by evaluated_at;
   ```

6. Reconstruir las alertas, borrando la gestión de prueba:

   ```bash
   go run ./cmd/alert_management -backfill -rebuild -drop-management -point 11556
   ```

7. Levantar el backend:

   ```bash
   ssh <vps> 'cd <carpeta del proyecto> && docker compose start <servicio backend>'
   ```

8. Verificar:

   ```sql
   select code, event_type, events, state, started_at at time zone 'America/Bogota' as inicio,
          ended_at at time zone 'America/Bogota' as fin, max_level, management_status,
          temp_stable_from at time zone 'America/Bogota' as temp_estable_desde,
          temp_stable_to at time zone 'America/Bogota' as temp_estable_hasta
   from alerts a where measuring_point_id = 11556 order by started_at;
   ```

Los códigos de alerta cambian (ALR-000024 en adelante). Un push viejo en el celular abrirá "alerta no encontrada".

---

## 12. Criterios de aceptación

### 12.1 Prueba dorada del Rule Engine

Cargar `motion_input.csv` (11 556 registros), correr `MotionAssess` hora por hora con §5.2 y comparar con `motion_expected.csv`: 3 182 horas, del 8 jul 06:00Z al 17 nov 19:00Z, de las que 2 918 son CALCULATED y 264 NO_CALCULABLE.

- **Deben ser iguales:** `status`, `motion_state`, `temp_state`, `in_episode`, `episode_event`, `computed_level`, `shift_started_at`, `temp_stable_from` y `temp_stable_to`.
- **Numéricos:** diferencia ≤ 1e-6.
- **`reasons`:** el mismo conjunto de códigos por hora, con los mismos datos; el orden puede variar.
- **Nombres del CSV que difieren de §5.3:**
  - `temp_rate_last` = `temp_rate(t)`;
  - `reference_pitch` / `reference_roll` = `ref_*`, solo mientras `displaced`;
  - `shift_records_2h` = registros en [t − 2 h, t).
- **Diferencias:** si el agente cree que una diferencia es un error de la referencia, la reporta en vez de ajustar el test.
- **Texto vs. referencia:** donde difieran, manda `motion_ref.py`. La tolerancia de 1e-9 se aplica solo al cambio brusco y al quieto; `dp_1h` exige 4 registros y la ventana de búsqueda del inicio es de 6 h fijas, no parametrizadas.

### 12.2 Eventos de movimiento esperados (hora Colombia)

| Hora evaluada | Motivos | Estado | Episodio |
|---|---|---|---|
| 25 ago 07:00 | `MOV_JUMP` (deriva 0,05) | SALTO · TEMP_VARIABLE | OPENED |
| 25 ago 09:00 | `MOV_SHIFT` (+0,06° / +0,115°, inicio 06:15), `TEMP_VERIFYING` | DESPLAZADO · VERIFICANDO | — |
| 25 ago 22:00 | `TEMP_STABILIZED` (0,025 °C/h, desde 06:15) | DESPLAZADO · TEMP_ESTABLE | — |
| 27 ago 14:00 | `MOV_RETURN` (inicio 12:45), `MOV_JUMP`, `TEMP_VARIES_AGAIN` (25 ago 06:15 – 27 ago 12:45) | RETORNO · TEMP_VARIABLE | — |
| 28 ago 16:00 | `MOV_STILL` | NORMAL · TEMP_VARIABLE | CLEARED |
| 12 oct 01:00 | `MOV_ABRUPT` (roll +0,04° en 3 h) | BRUSCO | OPENED |
| 13 oct 01:00 | `MOV_STILL` | NORMAL | CLEARED |
| 13 oct 05:00 | `MOV_ABRUPT` (roll +0,04° en 3 h) | BRUSCO | OPENED |
| 14 oct 05:00 | `MOV_STILL` | NORMAL | CLEARED |
| 21 oct 09:00 | `MOV_SHIFT_GAP` (+0,10° / +0,08°, hueco 20 oct 09:15 – 21 oct 08:00), `MOV_JUMP`, `TEMP_VERIFYING` | DESPLAZADO · VERIFICANDO | OPENED |
| 21 oct 14:00 | `TEMP_STABILIZED` (0,015 °C/h, desde 20 oct 09:15) | DESPLAZADO · TEMP_ESTABLE | — |
| 25 oct 20:00 | `MOV_RETURN` (inicio 18:30), `MOV_JUMP`, `TEMP_VARIES_AGAIN` (20 oct 09:15 – 25 oct 18:30) | RETORNO · TEMP_VARIABLE | — |
| 26 oct 20:00 | `MOV_STILL` | NORMAL · TEMP_VARIABLE | CLEARED |

Además hay `MOV_JUMP` sueltos el 25 ago 08:00, el 27 ago 13:00 y el 25 oct 19:00, dentro de episodios abiertos. No hay ningún episodio del 27 oct al 17 nov.

### 12.3 Alertas esperadas tras el rebuild (hora Colombia)

| # | Origen | `events` | Inicio | Fin | Nivel máx. | Temperatura estable | Hoy |
|---|---|---|---|---|---|---|---|
| 1 | FUGA | FUGA | 22 jul 01:00 | 22 jul 18:00 | Amarillo | — | ALR-000016 |
| 2 | FUGA | FUGA | 23 jul 19:00 | 2 ago 23:00 | Rojo | — | ALR-000017 |
| 3 | FUGA | FUGA | 22 ago 14:00 | 23 ago 00:00 | Amarillo | — | ALR-000018 |
| 4 | FUGA | FUGA, MOVIMIENTO | 24 ago 05:00 | 30 ago 00:00 | Rojo | 25 ago 06:15 – 27 ago 12:45 | ALR-000019 + ALR-000020 unidas |
| 5 | FUGA | FUGA | 18 sep 19:00 | 19 sep 20:00 | Amarillo | — | ALR-000021 |
| 6 | FUGA | FUGA | 26 sep 18:00 | 27 sep 01:00 | Amarillo | — | ALR-000022 |
| 7 | MOVIMIENTO | MOVIMIENTO | 11 oct 23:00 | 14 oct 05:00 (1 reactivación) | Amarillo | — | Nueva |
| 8 | MOVIMIENTO | MOVIMIENTO | 20 oct 10:00 | 26 oct 20:00 | Amarillo | 20 oct 09:15 – 25 oct 18:30 | Nueva |
| 9 | FUGA | FUGA | 13 nov 14:00 | 16 nov 09:00 | Amarillo | — | ALR-000023 |

**Mensajes nuevos de la alerta 4:**

| Hora | Código |
|---|---|
| 25 ago 07:00 | `MOV_JOINED` |
| 25 ago 09:00 | `MOV_DISPLACED` |
| 25 ago 19:00 | `PART_NORMAL` ruido ("el sensor sigue fuera de su posición") |
| 25 ago 22:00 | `TEMP_STABILIZED` |
| 27 ago 08:00 | `LEAK_JOINED` (contexto de temperatura estable; sin subir, porque sigue en Rojo sin clasificar) |
| 27 ago 14:00 | `MOV_RETURN` y `TEMP_VARIES_AGAIN` |
| 28 ago 16:00 | `PART_NORMAL` sensor |
| 30 ago 00:00 | `END_NORMAL` |

Entre el 25 ago 19:00 y el 27 ago 08:00 no hay mensajes horarios de ruido y el color sigue en Rojo.

**Alerta 7:**

| Hora | Código |
|---|---|
| 12 oct 01:00 | `MOV_OPEN` brusco |
| 13 oct 01:00 | `END_NORMAL` |
| 13 oct 05:00 | Reactivación + `MOV_OPEN` brusco |
| 14 oct 05:00 | `END_NORMAL` |

**Alerta 8:**

| Hora | Código |
|---|---|
| 21 oct 09:00 | `MOV_OPEN` hueco |
| 21 oct 14:00 | `TEMP_STABILIZED` |
| 25 oct 20:00 | `MOV_RETURN` y `TEMP_VARIES_AGAIN` |
| 26 oct 20:00 | `END_NORMAL` |

**Alertas 5, 6 y 9:** los mismos colores y mensajes que hoy, todas Sin clasificar.

### 12.4 Otros criterios

| ID | Criterio |
|---|---|
| CA-1 | Las alertas 1, 2, 3, 5, 6 y 9 tienen los mismos colores, mensajes y Notificaciones que hoy |
| CA-2 | La base rechaza una segunda alerta ACTIVA en un punto |
| CA-3 | Clasificar a las 14:23:10 crea `CLASSIFIED` con `evaluated_at` = 14:23:10 dentro del color vigente; etiquetar después crea `LABELED` con su hora; un 409 o un reenvío no crea mensajes |
| CA-4 | Después de clasificar (Confirmada o Falsa alerta, con o sin etiqueta) siguen saliendo todos los mensajes de los dos flujos |
| CA-5 | Con `MANTENIMIENTO_PROGRAMADO`, una subida emite su `UP_*` como Notificación |
| CA-6 | Tests sintéticos de `MotionAssess` (ver lista abajo) |
| CA-7 | Modo sombra: con `alerts_enabled = false` hay filas, pero no eventos ni alertas |
| CA-8 | Idempotencia: reprocesar una hora o recibir un evento duplicado no crea filas ni mensajes |
| CA-9 | Tras el rebuild no queda gestión en ninguna alerta del punto: todas Sin clasificar y sin observaciones |
| CA-10 | Sin `-drop-management`, el rebuild aborta sin escribir si el punto tiene gestión; con `-drop-management` la borra y reconstruye |
| CA-11 | Las pruebas del catálogo de §8.3 |
| CA-12 | Los 17 escenarios de §9.2, más modo sombra y `SOLO_MOVIMIENTO`, como tests de `Decide` / `DecideMotion` |

Tests sintéticos de CA-6:
- `TEMP_NOT_STABILIZED` a las 36 h;
- `TEMP_STILL_STABLE` a las 6, 18 y 30 h;
- hueco evaluado en la siguiente hora calculada;
- sin desplazamiento nuevo en las 24 h siguientes a un retorno;
- `MOV_NEW_POSITION` al cerrar desplazado;
- `SOLO_MOVIMIENTO` sin `temp_state`;
- `MOV_ABRUPT` una sola vez por cambio;
- un movimiento lento (0,05° en 11 h) sin episodio, tanto en un eje como en los dos ejes a la vez.

---

## 13. Entrega 2 (no se implementa ahora)

| Dónde | Cambio |
|---|---|
| `alert_notification` / `alert_changed` | Suben a v2. Agregan `events`, `motion_state`, `temp_state`, `temp_stable_from` y `temp_stable_to`; `alert_changed` agrega `change` = `TEMP_STATE_CHANGED` |
| Push | `event`: LEAK, MOVEMENT o LEAK_AND_MOVEMENT según `events`. `kind`: NEW (`OPEN`, `MOV_OPEN`, `NEW_AFTER_CLASSIFIED`), LEVEL_UP (subidas y `LEAK_JOINED` con subida) e INFO, nuevo (`LEAK_JOINED` sin subida, `TEMP_STABILIZED`, `TEMP_VARIES_AGAIN`). `title` y `body` siempre |
| `MobilePushPolicy` | Deja de omitir FALSA_ALERTA. Sigue omitiendo las horas `late` |
| `/api/alerts` | Filtro `event` sobre `events`. El detalle agrega `events`, `motion_state`, `temp_state` y `temp_stable`. El Excel agrega Origen, Eventos y Temperatura estable |
| Web y app | Etiquetas Ruido / Movimiento, estado de la temperatura, franja del tramo estable en la línea de tiempo, mensajes GESTION, y `kind` INFO con `title`/`body` de respaldo |

Hasta la entrega 2, una alerta clasificada como Falsa alerta no envía push, porque el filtro está en Notifications. En la web sí aparece todo.

---

## 14. Riesgos, decisiones pendientes y preguntas

**Riesgos:**
- **Pocos casos:** un equipo, 2 maniobras con salto y 2 cambios bruscos de un eje.
- **Margen corto en el cambio brusco:** lo normal llega a 0,03° en 3 h y el umbral es 0,04°, un solo paso de resolución.
- **Cortes cortos:** si duran menos de unas 16 h, la temperatura no alcanza a estabilizarse y queda solo el movimiento con regreso.
- **Alertas largas:** un corte de 5 días deja la alerta activa 5 días. Si está en Rojo sin clasificar, sigue titilando.
- **Pérdida de datos:** un salto y su regreso dentro del mismo hueco no se ven.
- **Línea base acústica:** las alertas de movimiento también quedan excluidas de ella. Varios cortes largos en 90 días acercan el tope del 40 %.

**Por decidir (fuera de esta entrega):**
1. **Temperatura estable sin movimiento.** ¿Debe abrir una alerta Amarillo por sí sola? En 4 meses no pasó nunca con flujo (escenario 16).
2. **Coincidencia de ruido y movimiento.** ¿Sube un nivel? No hay casos para calibrarlo; la propuesta es no subir (escenario 17).
3. **Línea base de movimiento en Baseline:** percentiles del cambio en 1 h y 3 h por eje, sobre 90 días sin alertas, con el umbral fijo como piso. Con un solo equipo daría lo mismo que el fijo. La propuesta es empezar solo calculándola y guardándola, y decidir con 2 o 3 equipos.

**Preguntas para TGI y el fabricante:**
1. ¿TGI acepta que un corte largo mantenga la alerta activa varios días?
2. ¿36 h es razonable para "la temperatura no se estabilizó"? [V]
3. Registro de maniobras de cada tramo y qué pasó el 12–13 oct (cambios bruscos de roll de noche). [V]
4. Fabricante: qué representa la variable Deriva en el firmware. [V]
