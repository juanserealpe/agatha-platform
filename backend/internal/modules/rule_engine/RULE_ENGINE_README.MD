# Modulo: Rule Engine

**Responsabilidad**: interpretar cada hora calculada del índice I(24 h) de Processing con **umbrales versionados por punto** (Amarillo, Naranja, Rojo) y **reglas de persistencia e histéresis**, guardar el resultado en `rule_evaluations` y publicar **cada hora** de 24 h a Alert Management (`rule_engine.level_evaluated`, v4), incluidas las de nivel NORMAL y las `NO_CALCULABLE`. Processing **mide**; el Rule Engine **interpreta** (es dueño del **nivel del indicador**: umbral + persistencia + episodio); Alert Management (`modules/alert_management`) **decide** (alerta única, estabilización, atención, silencio, clasificación). El `CLEARED` del episodio **no cierra la alerta**: para Alert Management es una evaluación más.

Todo en **UTC** (`America/Bogota` solo en logs). Hoy hay un solo tipo de evento: `FUGA`. Especificación vigente: `backend/AGATHA_Modulo_Rule_Engine_v2.docx` (umbrales por API, simulación, planitud, persistencia por ventana, evento v3).

## Estructura

```
rule_engine/
├── contract/                 # superficie pública (solo stdlib): tópico + LevelEvaluated (v4)
├── domain/
│   ├── errors.go             # AppError del módulo
│   ├── entities/             # Level/Rule/EpisodeEvent/ReasonCode, ThresholdParams/Patch/Version, RuleEvaluation, Reason, ThresholdSegment
│   ├── repositories/         # ThresholdRepository (+ ThresholdWriter transaccional), RuleEvaluationRepository
│   └── services/             # lógica PURA: RawLevel, Assess (persistencia + histéresis), ClipTimeline
├── application/
│   ├── ports/                # Clock, LevelEventPublisher, IndicatorSource (feature_evaluations: I y d_pl, solo lectura)
│   └── services/             # ThresholdService, EvaluationService (EvaluateIndicator), BackfillService, CatchUpService, SimulationService, LevelEvaluatedFromRow
└── infrastructure/
    ├── events/               # IndicatorEvaluatedSubscriber → EvaluateIndicator
    ├── http/                 # ThresholdHandler (/api/rule-engine) + DTOs y ParsePatch
    ├── scheduling/           # CatchUpScheduler (al arrancar + cada hora en el minuto 50)
    └── persistence/
        ├── schema.go         # Migrate (AutoMigrate + btree_gist + EXCLUDE) y SeedInitialThresholds
        ├── models/ mappers/  # filas propias + modelo de lectura de feature_evaluations
        ├── repositories/     # GORM (+ tests de integración)
        └── memory/           # mismos repos en memoria (tests y prueba de punta a punta)
```

## Tablas

### `rule_threshold_versions` — umbrales versionados (solo se agrega)

| Columna | Por qué |
|---|---|
| `id` BIGSERIAL PK | La referencia cada `rule_evaluations.threshold_version_id`: qué umbrales se usaron en cada hora. |
| `measuring_point_id` BIGINT | Sin herencia global: cada punto tiene su propia línea de tiempo. Sin FK (`devices` es de Ingestion). |
| `event_type` TEXT (`'FUGA'`) | Permite reglas por tipo de evento sin otra tabla. |
| `version` INT | Incremental por punto + evento; `UNIQUE (measuring_point_id, event_type, version)`. |
| `amarillo`, `naranja`, `rojo` NUMERIC(5,4) | Cortes de I. `CHECK 0 < amarillo < naranja < rojo <= 1`. |
| `min_hours` INT | Horas calculadas seguidas con raw ≥ AMARILLO para abrir un episodio (evita abrir por una hora aislada). |
| `escalate_amarillo_after_hours` INT NULL | Racha raw ≥ AMARILLO que sube a NARANJA por persistencia. NULL = sin escalamiento. |
| `escalate_naranja_after_hours` INT NULL | Racha raw ≥ NARANJA que sube a ROJO. NULL = sin escalamiento. |
| `exit_margin` NUMERIC(5,4) | Histéresis: para salir hay que bajar de `amarillo − exit_margin` (evita abrir/cerrar en el borde). `CHECK 0 <= exit_margin < amarillo`. |
| `exit_hours` INT | Horas calculadas seguidas bajo la banda de salida que cierran el episodio. |
| `confirm_naranja_with_planitud` BOOLEAN (`false`) | Si `true`, NARANJA (por umbral o persistencia) solo se asigna con `d_pl ≥ d_pl_min`; si no, la hora queda en AMARILLO con `NARANJA_NOT_CONFIRMED`. `CHECK NOT confirm OR d_pl_min IS NOT NULL`. |
| `confirm_rojo_with_planitud` BOOLEAN (`false`) | Si `true`, ROJO (por valor **o** por duración) también exige `d_pl ≥ d_pl_min`: desaparece la excepción del ROJO por umbral y una hora sin planitud confirmada queda en AMARILLO con `NARANJA_NOT_CONFIRMED` (`from` puede ser ROJO). `CHECK NOT confirm_rojo OR confirm_naranja_with_planitud` (no hay Rojo confirmado sin Naranja confirmado). Aditiva: las versiones existentes quedan en `false` y se comportan igual. |
| `d_pl_min` NUMERIC(5,2) NULL | % mínimo de registros con planitud alta de las últimas 24 h (propuesto 10–12). `CHECK` 0–100. |
| `persistence_mode` TEXT (`'STRICT'`) | `STRICT` = racha seguida (comportamiento v1). `WINDOW` = horas sobre el corte en una ventana móvil. `CHECK`: con `WINDOW`, las dos columnas siguientes son obligatorias y `min ≤ ventana`. |
| `persistence_window_hours` INT NULL | Para `WINDOW`: tamaño de la ventana en horas calculadas (propuesto 24). |
| `persistence_min_hours` INT NULL | Para `WINDOW`: horas sobre el corte dentro de la ventana que escalan un nivel (propuesto 18). |
| `valid_from`, `valid_to` TIMESTAMPTZ | Vigencia `[valid_from, valid_to)`; `valid_to` NULL = vigente. `CHECK valid_to IS NULL OR valid_to > valid_from`. |
| `reason`, `created_by`, `created_at` | Auditoría de cada cambio. |

Las 5 columnas v2 tienen defaults que reproducen el comportamiento anterior: las versiones existentes no cambian. Restricciones: los `CHECK` de arriba, `min_hours >= 1`, `exit_hours >= 1`, `escalate_* IS NULL OR >= 1`; **`EXCLUDE USING gist (measuring_point_id WITH =, event_type WITH =, tstzrange(valid_from, valid_to) WITH &&)`** (`ex_rule_threshold_versions_no_overlap`, requiere `btree_gist`): la BD garantiza que nunca hay dos versiones vigentes a la vez. Índice "vigente en t": `ix_rule_threshold_versions_effective (measuring_point_id, event_type, valid_from DESC)`.

Versión aplicable a la hora t: `valid_from <= t AND (valid_to IS NULL OR valid_to > t)`.

### `rule_evaluations` — una fila por hora CALCULADA de 24 h

| Columna | Por qué |
|---|---|
| `measuring_point_id`, `event_type`, `evaluated_at` | `UNIQUE` (`ux_rule_evaluations_point_event_hour`): idempotencia y clave de los eventos. |
| `indicator_value` NUMERIC | I(24 h) de la hora (copia de `feature_evaluations.i`). |
| `raw_level` | Nivel que marca el umbral, sin reglas. **Las rachas se cuentan sobre este valor guardado**: un cambio de umbral no reinterpreta el pasado. |
| `computed_level` | `max(raw, persistencia)`, **sin** histéresis ni racha de entrada: lo que sostiene la evidencia en esta hora. Alert Management sube y estabiliza con este valor (el HOLD es un artefacto del episodio). |
| `level` | Nivel del episodio (persistencia, histéresis, racha de entrada). En el evento: `episode_level`. |
| `rule` | `THRESHOLD` \| `PERSISTENCE` \| `HOLD` \| `NONE`: por qué tiene ese `level`. |
| `in_episode`, `hours_above`, `hours_in_level` | Estado para la hora siguiente (no hay estado en memoria) y contexto para Alert Management. |
| `run_amarillo`, `run_naranja` INT (0) | Horas sobre cada corte con las que se evaluó la persistencia (racha en STRICT, conteo en la ventana en WINDOW). Viajan en el evento v3 (`runs`). |
| `below_run` INT (0) | Horas seguidas bajo la salida de AMARILLO, incluida esta. Evento v3. |
| `d_pl` DOUBLE PRECISION NULL | % de planitud alta de la hora (copia de `feature_evaluations.d_pl`). Evento v3 y confirmación de NARANJA. |
| `threshold_version_id` FK | Versión usada (trazabilidad; la vigente en `evaluated_at`). |
| `low_confidence` | Copia de Processing (línea base de confianza baja). |
| `episode_event` | `OPENED` (esta hora abrió el episodio) \| `CLEARED` (lo cerró) \| `NONE`. |
| `reasons` JSONB | Motivos de `computed_level` (mismo formato que el evento): `LEAK_THRESHOLD {i, threshold}` si raw ≥ AMARILLO, `LEAK_PERSISTENCE {hours, from, window_hours?}` si la persistencia subió un nivel y `NARANJA_NOT_CONFIRMED {from, d_pl, d_pl_min}` si la planitud no confirmó NARANJA. `[]` si NORMAL. |
| `created_at` | Auditoría. |

`CHECK` en los enums. Índice `ix_rule_evaluations_point_event_recent (measuring_point_id, event_type, evaluated_at DESC)` para leer las últimas K horas (el único también lo cubriría; se dejó explícito según la especificación).

La fila contiene **todo** lo que lleva su `level_evaluated` (umbrales y salidas por la FK de la versión): `services.LevelEvaluatedFromRow(row, params, occurredAt)` reconstruye el mismo evento (base del reconciliador de Alert Management, AM-19). Las filas anteriores a las columnas v3 quedan con `run_* = 0`, `below_run = 0` y `d_pl = NULL` (para completarlas, `-rebuild` fuera de producción). Las horas `NO_CALCULABLE` **no generan fila**. Los dos esquemas los crea `persistence.Migrate` (ver "Migración").

## Versionado y no retroactividad

- **Nunca** `UPDATE` ni `DELETE` de una versión. La única modificación permitida es cerrar la abierta (`valid_to = valid_from` de la nueva) **en la misma transacción** que inserta la nueva (`SELECT … FOR UPDATE NOWAIT` de la abierta + `UPDATE` + `INSERT`).
- **Concurrencia**: dos creaciones a la vez sobre el mismo punto ⇒ la segunda **no espera**: `NOWAIT` falla con `55P03` (o el `UNIQUE`/`EXCLUDE` con `23505`/`23P01`), el repositorio lo envuelve en `repositories.ErrConflict` y el caso de uso responde `ErrThresholdConflict` (`CONFLICT`, HTTP 409). El cliente reintenta.
- **Nunca retroactivo**: `valid_from >= now` (cero ⇒ now; una fecha futura se permite y deja la actual vigente hasta entonces).
- **Única excepción — el seed**: la primera vez que se migra (tabla vacía), cada punto de `devices` recibe la **versión 1** con la plantilla, `valid_from = MIN(raw_measurements.timestamp)` del punto (`now()` si no tiene datos), `reason = "valor inicial beta"`, `created_by = "system"`. Es retroactiva **a propósito**: sin ella el backfill no tendría umbrales para el histórico.
- **Segunda excepción — `-reset-versions`**: reconstruir el historial de versiones de **un** punto (ver Backfill). Deliberada, acotada a un punto y bloqueada con `AGATHA_ENV=production`; es la única forma de aplicar una versión al histórico.
- **Punto nuevo (primer uso)**: si llega una hora de un punto sin ninguna versión, se crea la versión 1 con la plantilla y `valid_from = now` (`"valor inicial beta (primer uso)"`). Esa hora (anterior a now) **no se interpreta** (`NO_THRESHOLDS`, Warn) y el backfill tampoco podrá interpretar horas anteriores a now para ese punto. Si se necesita su histórico, sembrar su versión 1 a mano con la misma regla del seed **antes** de habilitar el módulo.

## Parámetros (plantilla, todos beta)

| Parámetro | Valor |
|---|---|
| `amarillo` / `naranja` / `rojo` | **0,16** / 0,25 / 0,35 |
| `min_hours` | 2 |
| `escalate_amarillo_after_hours` | 12 |
| `escalate_naranja_after_hours` | 6 |
| `exit_margin` | 0,02 (salida bajo 0,14) |
| `exit_hours` | 2 |
| `confirm_naranja_with_planitud` / `confirm_rojo_with_planitud` / `d_pl_min` | false / false / NULL |
| `persistence_mode` | STRICT (ventana NULL) |

**Amarillo 0,16 es la v2 de la especificación** (salida 0,14; el resto igual) y desde el 2026-10-26 es **la plantilla**: con `rule_threshold_versions` vacía, el seed crea la v1 de cada punto con 0,16 desde su primera medición, así el histórico entero se interpreta con ese corte (la API no puede crear versiones retroactivas). Los escenarios de los tests siguen escritos sobre el de referencia (0,18, salida 0,16) con `referenceParams()`.

`entities.DefaultThresholdParams()` es la única fuente: la usan el seed, el primer uso y la primera versión creada por el caso de uso. Los valores se redondean a 4 decimales **antes** de validar.

## Algoritmo (`domain/services.Assess`, puro)

Niveles `NORMAL < AMARILLO < NARANJA < ROJO`. Para cada hora calculada, con su I y su `d_pl`, la versión vigente en su `evaluated_at` y las últimas K filas (K = máx. de los parámetros de horas, incluida `persistence_window_hours` en WINDOW, + 1):

1. **Raw**: ROJO si I ≥ rojo; NARANJA si ≥ naranja; AMARILLO si ≥ amarillo; si no NORMAL.
2. **Persistencia** (sobre el `raw_level` guardado):
   - `STRICT`: `runAmarillo`/`runNaranja` = horas consecutivas, terminando en la actual, con raw ≥ AMARILLO / ≥ NARANJA. Umbral: `escalate_*_after_hours`.
   - `WINDOW`: `runAmarillo`/`runNaranja` = horas con raw ≥ corte entre la actual y las `persistence_window_hours − 1` calculadas anteriores (una hora bajo el corte **no reinicia** el conteo). Umbral: `persistence_min_hours` para los dos escalamientos; un `escalate_*` NULL sigue desactivando el suyo.
   - Candidato ROJO si raw ≥ NARANJA y `runNaranja ≥ umbral`; si no NARANJA si raw ≥ AMARILLO y `runAmarillo ≥ umbral` (nunca escala una hora bajo el corte). `nivelCalculado = max(raw, candidato)` (como mucho **un** nivel por regla). `rule = PERSISTENCE` si sube sobre el raw; si no `THRESHOLD`.
   - **Planitud** (`confirm_naranja_with_planitud`): si `nivelCalculado ≥ NARANJA`, el raw **no** es ROJO y `d_pl` es NULL o `< d_pl_min` ⇒ `nivelCalculado = AMARILLO`, `rule = THRESHOLD`, motivo `NARANJA_NOT_CONFIRMED`. Frena también la persistencia NARANJA → ROJO (si NARANJA no está confirmado, no se escala sobre él); un ROJO por umbral no se toca. Es el único caso en que `computed_level` queda **bajo** el raw.
   - **Planitud también en ROJO** (`confirm_rojo_with_planitud`): la excepción `raw ≠ ROJO` **solo** rige con la opción en `false`. Con `true`, cualquier `nivelCalculado ≥ NARANJA` (por valor o por duración) con `d_pl` NULL o `< d_pl_min` baja a AMARILLO, con el mismo motivo `NARANJA_NOT_CONFIRMED` y `from` = el nivel que no se confirmó (puede ser ROJO). Las rachas y la persistencia siguen contando sobre el `raw_level` **guardado**, no sobre el `computed_level`: una hora limitada no borra la duración acumulada, y la primera hora siguiente con `d_pl ≥ d_pl_min` recupera el nivel, incluido un ROJO por duración. `d_pl` NULL = no confirmado.
3. **Fuera de episodio**: raw ≥ AMARILLO y racha **seguida** ≥ `min_hours` (siempre STRICT, también en WINDOW) ⇒ abre (`level = nivelCalculado`, `OPENED`, `hours_above` = esa racha); si no `NORMAL`/`NONE`/`NONE`.
4. **Dentro de episodio**: raw ≥ AMARILLO ⇒ `level = nivelCalculado` (sube o baja). raw NORMAL ⇒ `belowRun` = horas seguidas con I < `amarillo − exit_margin` **y raw guardado NORMAL**; si `belowRun ≥ exit_hours` cierra (`NORMAL`, `rule NONE`, `CLEARED`); si no `HOLD` (conserva el nivel).
5. `hours_above` = horas calculadas desde la primera hora de la racha de entrada, incluida la actual (la hora de cierre cuenta: es la última del episodio; 0 fuera de episodio). `hours_in_level` = horas seguidas con el mismo `level`.
6. `computed_level = nivelCalculado` en **todas** las horas (dentro o fuera de episodio, también en HOLD) y `reasons` lo explican: `LEAK_THRESHOLD` (`level` = raw, `i`, `threshold` = corte del raw); si la persistencia subió, `LEAK_PERSISTENCE` (`level` = nivel ganado, `hours` = racha o conteo, `from` = AMARILLO o NARANJA, `window_hours` en WINDOW); si la planitud no confirmó, `NARANJA_NOT_CONFIRMED` (`level` = AMARILLO, `from` = nivel no confirmado, `d_pl`, `d_pl_min`).
7. `run_amarillo`, `run_naranja` y `below_run` se guardan siempre (también fuera de episodio) para el evento v3.

**Salidas por nivel** (`ThresholdParams.ExitFor`): `corte − exit_margin`, redondeado a 4 decimales ⇒ 0,14 / 0,23 / 0,33 con la plantilla (0,16 / 0,23 / 0,33 en el escenario de referencia). El Rule Engine solo usa la de AMARILLO para cerrar el episodio; las tres viajan en el evento (`exit`) para la histéresis por nivel de Alert Management. Si hacen falta márgenes distintos por nivel, se agregan columnas a `rule_threshold_versions`.

**Huecos**: una hora `NO_CALCULABLE` no tiene fila, así que las rachas cuentan horas **calculadas**: un hueco **pausa** las rachas, no las reinicia.

**Cambio de umbral**: no recalcula horas pasadas. Las rachas usan el `raw_level` guardado; por eso `belowRun` exige raw guardado NORMAL (si se subiera `amarillo`, una hora que fue AMARILLO con su versión no se reinterpreta como "baja" para cerrar el episodio).

### Escenario de referencia (amarillo 0,18, `referenceParams()` en `level_assessor_test.go`)

| I por hora | level (episode) | computed | rule | episode_event | Nota |
|---|---|---|---|---|---|
| 0,12 · 0,14 | NORMAL | NORMAL | NONE | NONE | |
| 0,19 | NORMAL | AMARILLO | NONE | NONE | racha de entrada pendiente (1 < 2) |
| 0,20 | AMARILLO | AMARILLO | THRESHOLD | **OPENED** | abre, `hours_above` 2 |
| 0,21 | AMARILLO | AMARILLO | THRESHOLD | NONE | |
| 0,27 | NARANJA | NARANJA | THRESHOLD | NONE | |
| 0,22 | AMARILLO | AMARILLO | THRESHOLD | NONE | baja dentro del episodio |
| 12 h entre 0,18 y 0,24 | AMARILLO → NARANJA | AMARILLO → NARANJA | THRESHOLD → PERSISTENCE | NONE | la racha ≥ AMARILLO viene desde la 0,19: llega a 12 en la 7.ª hora del bloque; la 12.ª es NARANJA/PERSISTENCE |
| hueco NO_CALCULABLE | — | — | — | — | sin fila (se reenvía como `NO_CALCULABLE`); la racha sigue |
| 0,17 | NARANJA | NORMAL | HOLD | NONE | banda de histéresis: el episodio conserva NARANJA, el computed no |
| 0,15 · 0,14 | NARANJA · NORMAL | NORMAL | HOLD · NONE | NONE · **CLEARED** | la 2.ª cumple `exit_hours` |
| 0,20 · 0,21 | NORMAL · AMARILLO | AMARILLO | NONE · THRESHOLD | NONE · **OPENED** | episodio nuevo en la 2.ª hora |

`level_assessor_v2_test.go` (casos del documento v2, sintéticos, más los de `confirm_rojo_with_planitud`: ROJO por valor con `d_pl` 4,6 y mínimo 10 ⇒ AMARILLO con `from` ROJO; con la opción en `false` queda igual que antes; 6 h sobre NARANJA con planitud baja ⇒ AMARILLO y la hora siguiente con planitud confirmada ⇒ ROJO por duración): R7 con planitud nunca NARANJA; fuga con `d_pl` 15 llega igual; planitud frena la persistencia pero no el ROJO por umbral; racha de 8 h y persistencia 12 → 6 ⇒ la siguiente es NARANJA/PERSISTENCE; oscilación 0,17/0,17/0,17/0,15 con WINDOW 18 de 24 ⇒ NARANJA en la hora 23 (STRICT nunca escala).

> **Nota sobre el caso 10 del documento**: el patrón 0,17/0,17/0,15 tiene 2 de cada 3 horas sobre el corte ⇒ como máximo **16 de 24**: con WINDOW 18 de 24 **nunca** escala. El test usa 3 de cada 4 (exactamente 18 de 24).

Además: salto directo a ROJO (desde NORMAL respeta `min_hours`; dentro de un episodio sube de inmediato), persistencia NARANJA → ROJO a las 6 h, nunca dos niveles por persistencia, escalamiento NULL ⇒ nunca PERSISTENCE, 12 h literales desde una racha nueva ⇒ la 12.ª es la primera PERSISTENCE.

## Evaluación horaria (`EvaluationService.EvaluateIndicator`)

Entrada: `IndicatorEvaluatedNotice` (vista propia del evento de Processing, traducida en bootstrap). Solo `WindowHours == 24`; las de 6 h, un estado desconocido o `CALCULATED` sin I se ignoran (`Debug`, `IGNORED`).

**Hora `NO_CALCULABLE` (24 h)**: no crea fila. Bajo el lock del punto, si la hora ya tiene fila (`DUPLICATE`) o hay filas posteriores (tardía, `Warn`) no se reenvía; si no, publica `level_evaluated` con `status = NO_CALCULABLE` (`OutcomeNoCalculable`). En backfill nunca se reenvía. Qué hacer con ella (pausar la estabilización) lo decide Alert Management.

**Hora `CALCULATED` (24 h)**:

1. Si ya existe la fila (punto, evento, hora) ⇒ nada (`DUPLICATE`; no se republica).
2. Versión vigente en `evaluated_at` (sin ninguna ⇒ primer uso, ver arriba) y las últimas K filas anteriores.
2b. `d_pl` de la hora: el evento de Processing no lo trae, así que se lee de `feature_evaluations` (`IndicatorSource.GetDPl`, índice único punto/ventana/hora). El backfill y la simulación ya lo traen en cada hora (`Notice.DPlLoaded`). Un error de lectura falla la hora (`ErrConsultingIndicators`), igual que los demás errores de BD.
3. `Assess` + `INSERT … ON CONFLICT DO NOTHING` en una TX. Las horas de un punto se serializan con un lock en memoria (la siguiente necesita la fila de la anterior).
4. Post-commit publica `level_evaluated` (`LevelEvaluatedFromRow`) **en todas las horas**, también NORMAL y HOLD. Un fallo al publicar **nunca** falla la evaluación.
5. Si hay filas posteriores del punto (llegada tardía o desordenada) ⇒ se guarda **sin publicar** + `Warn`. Las filas posteriores no se recalculan (para eso, `-rebuild`).
6. Log `Info`: punto, hora, I, raw, computed, level, rule, versión y `episode_event`.

Un error (BD) se registra en el subscriber y la hora queda sin interpretar: el evento es una pista; el catch-up la completa en su próxima pasada (si no hay horas posteriores ya interpretadas).

## Eventos

### Consumidos

| Tópico | Publicador | Notice propio | Suscripción | Acción |
|---|---|---|---|---|
| `processing.indicator_evaluated` | Processing (`EvaluatePending`, post-commit) | `services.IndicatorEvaluatedNotice` | `rule_engine.indicator_evaluated` | `EvaluateIndicator` |

Best-effort: pueden perderse (overflow, reinicio) o duplicarse. `feature_evaluations` es la fuente de verdad; el backfill de Processing no publica.

### Publicados (`contract/`, `SchemaVersion = 4`)

**`rule_engine.level_evaluated`** — `LevelEvaluated`, una por **cada** hora de 24 h que llega en vivo. Reemplaza a `leak_threshold_exceeded` / `leak_threshold_cleared` (v1, sin consumidores): Alert Management necesita también las horas NORMAL para contar la estabilización.

```json
{
  "rule_evaluation_id": 4821,
  "measuring_point_id": 11556,
  "event_type": "FUGA",
  "evaluated_at": "2026-10-15T17:00:00Z",
  "status": "CALCULATED",
  "indicator_value": 0.22,
  "raw_level": "AMARILLO",
  "computed_level": "NARANJA",
  "episode_level": "NARANJA",
  "in_episode": true,
  "episode_event": "NONE",
  "reasons": [
    { "code": "LEAK_THRESHOLD",   "level": "AMARILLO", "data": { "i": 0.22, "threshold": 0.16 } },
    { "code": "LEAK_PERSISTENCE", "level": "NARANJA",  "data": { "hours": 12, "from": "AMARILLO" } }
  ],
  "thresholds": { "amarillo": 0.16, "naranja": 0.25, "rojo": 0.35 },
  "exit":       { "amarillo": 0.14, "naranja": 0.23, "rojo": 0.33 },
  "threshold_version_id": 3,
  "hours_above": 14,
  "runs": { "amarillo": 12, "naranja": 0 },
  "escalation": { "amarillo_after_hours": 12, "naranja_after_hours": 6 },
  "below_run": 0,
  "d_pl": 17.4,
  "d_pl_min": 10,
  "persistence_mode": "STRICT",
  "low_confidence": false,
  "occurred_at": "2026-10-15T17:35:04Z",
  "schema_version": 4
}
```

| Campo | Significado |
|---|---|
| `status` | `CALCULATED` o `NO_CALCULABLE`. Con `NO_CALCULABLE`: `rule_evaluation_id = 0`, `indicator_value = null`, niveles y `episode_event` vacíos, sin `reasons` ni umbrales; trae `no_calc_reason` de Processing. |
| `raw_level` / `computed_level` / `episode_level` | Umbral solo / `max(raw, persistencia)` / nivel del episodio (con HOLD). Alert Management sube y estabiliza con `computed_level`. |
| `in_episode`, `episode_event` | Estado del episodio; `OPENED` en la hora que lo abre, `CLEARED` en la que lo cierra, `NONE` en el resto. |
| `reasons[]` | Por qué `computed_level` (ver Algoritmo, paso 6). |
| `thresholds`, `exit` | Cortes y salidas (`corte − exit_margin`) de la versión usada (`threshold_version_id`). |
| `hours_above` | Horas calculadas del episodio, incluida esta (la de cierre cuenta; 0 fuera de episodio). |
| `runs.amarillo`, `runs.naranja` (v3) | Horas de racha (STRICT) o en la ventana (WINDOW) sobre cada corte. Base del aviso "si sigue N h más, pasa a…". |
| `escalation.*_after_hours` (v3) | Umbral **efectivo** con el que se comparan los `runs` (`escalate_*` en STRICT, `persistence_min_hours` en WINDOW); `null` = desactivado. Faltan para subir: `escalation − runs`. |
| `below_run` (v3) | Horas seguidas bajo la salida de AMARILLO; con `exit_hours` da "se cerrará si sigue así". |
| `d_pl` (v3) | Evidencia de planitud de la hora, en porcentaje tal como se guarda (`null` si Processing no la calculó). Viaja en **toda** hora calculada, no solo dentro del motivo `NARANJA_NOT_CONFIRMED`. |
| `d_pl_min` (v4) | `d_pl_min` de la versión vigente en la hora, también en toda hora calculada (se **omite** si la versión no lo define). Junto con `d_pl` permite explicar la limitación por planitud sin leer los `reasons`. Go: `contract.LevelEvaluated.DPlMin *float64`. |
| `persistence_mode` (v3) | `STRICT` o `WINDOW`, para textos de cuenta regresiva coherentes. |

**v3 y v4 son aditivas**: los campos v2 no cambian; un consumidor v2 o v3 sigue funcionando. v4 solo agrega `d_pl_min`; además, con `confirm_rojo_with_planitud`, el motivo `NARANJA_NOT_CONFIRMED` puede traer `from = ROJO`. Con `NO_CALCULABLE`: `runs` en 0, `escalation` y `d_pl` null, `d_pl_min` omitido, `persistence_mode` vacío.

No se publica en el **backfill**, en **duplicados** ni en **llegadas tardías** (RE-06). **Clave de idempotencia para consumidores: `(measuring_point_id, event_type, evaluated_at)`.** Se publica por el puerto `LevelEventPublisher`; el adaptador al bus (`busLevelEventPublisher`) vive en `internal/bootstrap`. Lo consume Alert Management (suscripción `alert_management.level_evaluated`); su catch-up del minuto 55 relee `rule_evaluations` para las horas cuyo evento se perdió.

### Pendiente (fase 2, RE-07)

Rachas en tiempo de reloj con `hueco_max` (P1). La oscilación alrededor del corte (P2) ya tiene respuesta con `persistence_mode = WINDOW`, que **no está activa** en ninguna versión: activarla solo después de validarla con `simulate`.

## Casos de uso de umbrales (`ThresholdService`)

| Caso de uso | Comportamiento |
|---|---|
| `CreateThresholdVersion(cmd)` | Valida (orden, rangos, planitud, ventana, motivo/autor, `valid_from` no pasado; `AppError` `INVALID_INPUT`), copia de la vigente los campos que el `Patch` no informa (`entities.Some(...)`; `Some[*int](nil)` deja un escalamiento en NULL), cierra la abierta y crea la nueva en una TX. Sin versión previa parte de la plantilla. |
| `CreateThresholdVersionBulk(cmd)` | Igual para varios puntos en **una** TX (si uno falla, ninguno). `PointIDs` vacío ⇒ todos los puntos con alguna versión. |
| `GetCurrentThresholds(point, event)` | Versión vigente ahora (`NOT_FOUND` si no hay), con `PreviousVersionID`. |
| `GetThresholdHistory(point, event)` | Todas las versiones, ascendentes, con `PreviousVersionID` (no es columna: se deriva del orden). |
| `GetThresholdTimeline(point, event, from, to)` | Tramos que se solapan con `[from, to)`, recortados: `tramo.from = GREATEST(valid_from, from)`, `tramo.to = LEAST(COALESCE(valid_to, to), to)`. |
| `EnsureInitialVersion(point, event)` | Primer uso (lo invoca `EvaluateIndicator`). |

## API HTTP (`infrastructure/http`, prefijo `/api/rule-engine`)

> **Sin autenticación de usuarios (temporal).** La especificación pide token y `created_by` tomado del token; mientras no exista auth, `created_by` viaja en el **cuerpo** y los endpoints de escritura **no están protegidos**. No exponer `rule_engine` fuera de la red de confianza hasta tener auth. Al agregarla: quitar `created_by` del cuerpo (`CreateVersionRequest`) y tomarlo del token en el handler.

| Método y ruta | Descripción | Respuestas |
|---|---|---|
| `GET /points/{id}/thresholds?event_type=FUGA` | Versión vigente ahora | 200 · 400 id inválido · 404 sin versión |
| `GET /points/{id}/thresholds/history` | Todas las versiones, ascendentes, con autor y motivo | 200 |
| `GET /points/{id}/thresholds/timeline?from=&to=` | Tramos recortados a `[from, to)` (RFC3339, obligatorios) | 200 · 400 rango inválido |
| `POST /points/{id}/thresholds` | Versión nueva con los campos que cambian (`patch`) | 201 · 400 · 409 |
| `POST /thresholds/bulk` | Misma versión para varios puntos, todo o nada (`point_ids` **obligatorio**: la API no aplica "todos los puntos") | 201 · 400 · 409 |
| `POST /points/{id}/thresholds/simulate` | Histórico con los valores propuestos, sin guardar ni publicar | 200 · 400 · 404 |

**Cuerpo del POST**: `event_type` (opcional, `FUGA`), `patch` (obligatorio), `valid_from` (RFC3339 o `null` = ahora; pasado ⇒ 400), `reason` (obligatorio), `created_by` (obligatorio, **temporal**). Bulk: lo mismo + `point_ids`. Campos desconocidos (raíz o `patch`) ⇒ 400. Cuerpo máximo 64 KiB.

**`patch`**: campo ausente ⇒ se copia de la vigente; `null` solo en los anulables (`escalate_amarillo_after_hours`, `escalate_naranja_after_hours`, `d_pl_min`, `persistence_window_hours`, `persistence_min_hours`) y los deja en NULL (un escalamiento NULL se desactiva). Campos: `amarillo`, `naranja`, `rojo`, `min_hours`, `escalate_*`, `exit_margin`, `exit_hours`, `confirm_naranja_with_planitud`, `confirm_rojo_with_planitud` (exige la de Naranja, si no ⇒ 400), `d_pl_min`, `persistence_mode`, `persistence_window_hours`, `persistence_min_hours`. La respuesta de la versión (y la de `simulate`) incluye `confirm_rojo_with_planitud`, y `simulate` aplica la misma regla de Rojo.

**Errores**: validación ⇒ 400 `INVALID_INPUT` sin escribir nada; concurrencia ⇒ 409 `CONFLICT` (reintentar); BD ⇒ 500.

**Respuesta de una versión**: `id`, `version`, `previous_version_id`, `measuring_point_id`, `event_type`, los parámetros, `exit {amarillo, naranja, rojo}` (corte − `exit_margin`), `valid_from`, `valid_to`, `reason`, `created_by`, `created_at`.

### Crear una versión nueva (ejemplo)

Amarillo 0,16 ya viene en la v1 (plantilla). Un cambio posterior se crea así (rige desde ahora, nunca hacia atrás):

```bash
curl -X POST "$AGATHA_API/api/rule-engine/points/11556/thresholds" \
  -H 'Content-Type: application/json' \
  -d '{"event_type": "FUGA", "patch": {"escalate_amarillo_after_hours": 6}, "valid_from": null,
       "reason": "Persistencia AMARILLO a 6 h",
       "created_by": "jvarona@unicauca.edu.co"}'
```

Antes: correr `simulate` con el mismo `patch` y comparar contra los eventos conocidos. Otros parches: `{"escalate_amarillo_after_hours": 6}`, `{"escalate_naranja_after_hours": null}`, `{"confirm_naranja_with_planitud": true, "d_pl_min": 12}`, `{"persistence_mode": "WINDOW", "persistence_window_hours": 24, "persistence_min_hours": 18}`.

### Simulación (`SimulationService`)

```json
POST /api/rule-engine/points/11556/thresholds/simulate
{ "patch": { "amarillo": 0.18 }, "from": "2026-07-08T00:00:00Z", "to": "2026-10-22T00:00:00Z" }

200 OK
{
  "measuring_point_id": 11556, "event_type": "FUGA", "from": "...", "to": "...",
  "thresholds": { "amarillo": 0.18, "...": "parámetros simulados" },
  "hours_evaluated": 2270,
  "episodes": [ { "opened_at": "2026-08-24T17:00:00Z", "cleared_at": "2026-08-25T20:00:00Z",
                  "max_level": "NARANJA", "first_naranja_at": "2026-08-25T03:00:00Z", "first_rojo_at": null, "hours": 29 } ],
  "summary": { "episodes": 6, "by_max_level": { "AMARILLO": 2, "NARANJA": 2, "ROJO": 2 } },
  "compared_to_current": { "current_version_id": 1, "current_episodes": 5, "new_episodes": 1, "removed_episodes": 0 }
}
```

- Base del parche: la versión **vigente ahora**. Los parámetros propuestos se aplican a **todo** el rango (no a la línea de tiempo real); `compared_to_current` corre el mismo rango con la vigente sola.
- Reusa **exactamente** `EvaluateIndicatorWithoutPublishing` sobre un **sandbox** en memoria nuevo por corrida (`SandboxFactory`, cableado en bootstrap con `persistence/memory`): **no escribe** en ninguna tabla ni publica.
- Arranca **sin historial** en `from`: las primeras horas no ven rachas previas al rango.
- Episodio = de la fila `OPENED` a la `CLEARED` (incluida); `cleared_at = null` si sigue abierto al final; `hours` = `hours_above` de la última fila. Un `CLEARED` sin `OPENED` en el rango se ignora.
- `new_episodes` / `removed_episodes`: episodios de un lado que no se solapan con ninguno del otro.
- Rango obligatorio, `from < to`, máximo 366 días (`ErrSimulationRangeTooLong`).

## Catch-up (`CatchUpService`, red de seguridad en vivo)

Hay horas que Processing guarda en `feature_evaluations` **sin** que llegue su evento: un lote insertado en parte (Processing no publica nada), un evento perdido por el bus (overflow, reinicio) o un error de BD al consumirlo. Igual que el reconciliador de Processing, el Rule Engine tiene su red de seguridad:

- `CatchUpScheduler` corre al arrancar (cubre la caída) y cada hora en el minuto `RULE_ENGINE_CATCHUP_MINUTE` (50, después del 35 de Processing). Solo en `cmd/platform` con `rule_engine` habilitado.
- Por punto de `feature_evaluations`: lee las horas `CALCULATED` de 24 h **posteriores a la última fila** de `rule_evaluations` y las interpreta con `EvaluateIndicator` (mismo lock por punto que el subscriber) **publicando** `level_evaluated` (`Trigger = CATCH_UP`): Alert Management las recibe, tarde pero en orden. Si el evento llega después, es `DUPLICATE` y no se republica.
- Un punto **sin ninguna fila** se saltea (`Warn`: "correr rule_engine -backfill"): el catch-up nunca publica el histórico entero.
- Solo avanza: un hueco en medio del histórico (hora H fallida y H+1 ya interpretada) queda tardío; se arregla con `-rebuild`. Las `NO_CALCULABLE` no se reenvían (no hay fila que leer).
- Un error detiene ese punto (el orden importa), no a los demás; la próxima pasada retoma. Costo en régimen: 2 consultas por punto por hora.

## Backfill (`cmd/rule_engine -backfill`)

```
rule_engine -backfill [-point 11556] [-from 2026-07-09T00:00:00Z] [-to 2026-10-19T00:00:00Z] [-rebuild]
rule_engine -backfill -rebuild -reset-versions -point 11556 -confirm 11556
```

- Fuente: `feature_evaluations` (`window_hours = 24`, `status = CALCULATED`) por el puerto de solo lectura `IndicatorSource` (`repositories.IndicatorReader`, modelo propio).
- Por punto (todos si no hay `-point`), en orden ascendente de `evaluated_at`, en lotes (`RULE_ENGINE_BACKFILL_BATCH_SIZE`, 500), con **el mismo caso de uso** en modo sin publicación (`EvaluateIndicatorWithoutPublishing`): **nunca publica**. Usa la versión vigente en cada hora (varias versiones en el rango ⇒ cada una en su tramo).
- Idempotente: las horas ya interpretadas se saltean (`dupl.`). Un error aborta; repetirlo retoma.
- `-rebuild`: borra `rule_evaluations` del rango/punto y recalcula. **Bloqueado con `AGATHA_ENV=production`**. Útil tras llegadas tardías (las horas posteriores no se recalculan solas). Recomendado sin `-to` (rebuild hasta el final), para que ninguna hora quede calculada sobre un historial viejo.
- `-reset-versions` (solo con `-backfill -rebuild`): reconstruye el historial de versiones de **un** punto para interpretar **todo** el histórico con los parámetros vigentes hoy. Es la **segunda excepción deliberada a la no retroactividad** de los umbrales (la primera es la v1 sembrada), acotada a un punto y **bloqueada con `AGATHA_ENV=production`** (`ErrRebuildForbidden`, `EqualFold` + `TrimSpace`).
  - **Guardas** (rechaza antes de escribir nada): exige `-point` (nunca todos los puntos), `-confirm` igual a `-point`, `-rebuild`, y el historial completo (sin `-from` ni `-to`); rechaza también si el punto tiene versiones programadas a futuro, para no descartarlas en silencio.
  - **Qué hace**, en **una** transacción antes de recalcular: bloquea las versiones FUGA del punto, toma la **vigente hoy** (versión N), borra **todas** las filas de `rule_evaluations` del punto (la FK a la versión es `RESTRICT`: van **antes** que las versiones), borra **todas** las versiones FUGA del punto e inserta una única **versión 1** con los parámetros de la N, `valid_from` = primera medición del punto en `raw_measurements` (la misma expresión del seed; `now()` si no tiene), `valid_to NULL`, `reason = "Reconstrucción: parámetros de la versión N aplicados al histórico"`, `created_by = "system"`. Si algo falla, rollback y no queda nada escrito. Después corre el rebuild normal (recalcula todo el historial).
  - **Informe**: cuántas versiones se colapsaron, de qué versión salieron los parámetros y el nuevo `valid_from`.
  - **Procedimiento**: (1) **backup** de `rule_threshold_versions` y `rule_evaluations` (la operación no se deshace; se pierde el historial de versiones del punto); (2) **exportar la gestión de los operadores** de Alert Management del punto, porque su `-rebuild` posterior borra `alerts`, `alert_messages` y la gestión; (3) fuera de production, correr el comando de arriba; (4) correr `alert_management -backfill -rebuild -point <id>`. Es la **única** forma de aplicar una versión al histórico: crear una versión por la API nunca es retroactivo.
- Log de progreso por lote (horas por nivel, episodios abiertos y cerrados) y tabla final por punto.
- ~4 consultas por hora (posición, versión, historial, insert): con BD remota, del orden de minutos por punto-año. Tope de la corrida: 2 h.

**Orden de inicialización**: `baseline -backfill` → `processing -backfill` → `rule_engine -backfill` (migra, siembra la v1 con amarillo 0,16 si `rule_threshold_versions` está vacía e interpreta) → habilitar `rule_engine` en `AGATHA_MODULES` (el catch-up completa, publicando, las horas calculadas entre el backfill y el arranque). Si el módulo ya corrió en vivo antes del backfill, las horas históricas serán "tardías" respecto de las vivas: usá `-rebuild` fuera de producción.

## Movimiento (`REQ_ALERTAS_MOVIMIENTO` v1.0, entrega 1)

Flujo independiente de FUGA: lee `raw_measurements` (no depende de Processing), interpreta el movimiento del sensor (pitch, roll, deriva) y la temperatura de la tubería hora a hora, guarda **una fila por punto y hora** en `motion_evaluations` (también `NO_CALCULABLE`) y publica `rule_engine.motion_evaluated` v1. La especificación y sus anexos están en `domain/services/testdata/motion/` (`REQ_ALERTAS_MOVIMIENTO.md`, `motion_ref.py`, `motion_input.csv`, `motion_expected.csv`). Lo consume Alert Management (`alert_management.motion_evaluated`; ver su README).

**Export de FUGA para el test de §12.3** (`internal/bootstrap/motion_alerts_test.go`): con `testdata/motion/rule_evaluations_11556.csv` el test corre también con el histórico real de FUGA. Se genera desde `psql` (solo lectura):

```sql
\copy (SELECT r.id, r.measuring_point_id, to_char(r.evaluated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS evaluated_at,
  r.indicator_value, r.raw_level, r.computed_level, r.level, r.in_episode, r.hours_above, r.run_amarillo, r.run_naranja,
  r.below_run, r.threshold_version_id, r.low_confidence, r.episode_event, r.reasons::text AS reasons, r.d_pl, v.d_pl_min,
  v.amarillo, v.naranja, v.rojo, v.exit_margin, v.escalate_amarillo_after_hours, v.escalate_naranja_after_hours,
  v.persistence_mode, v.persistence_min_hours
FROM rule_evaluations r JOIN rule_threshold_versions v ON v.id = r.threshold_version_id
WHERE r.measuring_point_id = 11556 AND r.event_type = 'FUGA' ORDER BY r.evaluated_at) TO 'rule_evaluations_11556.csv' WITH CSV HEADER
```

### Algoritmo (`domain/services.MotionAssess`, puro)

`MotionAssess(window []RawRecord, prev MotionState, p MotionParams, t) MotionEvaluation`: la hora t cubre [t − 1 h, t) y la ventana es [t − 169 h, t). Reproduce `motion_ref.py` (§5): cobertura, variables, hueco, desplazamiento sostenido (solo si hubo una hora con `jump` en [t − 6 h, t]), retorno, salto, cambio brusco, quieto, temperatura y episodio. Donde el texto y la referencia difieren, manda la referencia (§12.1): la tolerancia 1e-9 solo en cambio brusco y quieto, `dp_1h` exige 4 registros, la búsqueda del inicio usa 6 h fijas.

- Los niveles y saltos de las horas anteriores **se recalculan desde la ventana**: el estado que viaja entre filas es solo el de §5.4 (`entities.MotionState`, incluidos los `pending_gaps`).
- `MotionStateValue` es el enum de `motion_state` (`MotionState` es el estado que se arrastra).
- `MotionReason.Data()` arma el `data` canónico de cada código (grados redondeados a 3 decimales, fechas RFC 3339); lo usan el mapper y el evento.

### Tablas

- `motion_threshold_versions` (§6.1): un parámetro por columna (nombres = `entities.MotionParamFields`, la tabla única que usan el parche, la API, el seed y la validación); grados y °C/h en `NUMERIC(6,4)`, horas y registros `INT`, `temp_mode` TEXT, `alerts_enabled` BOOLEAN. `UNIQUE (punto, versión)`, `EXCLUDE USING gist` de vigencias (`ex_motion_threshold_versions_no_overlap`) y `CHECK`s de §6.1. Mismo versionado que FUGA (solo se agrega, nunca retroactiva, `FOR UPDATE NOWAIT` ⇒ 409).
- `motion_evaluations` (§6.2): clave, variables, estado, resultado y trazabilidad (`threshold_version_id` FK, `algorithm_version = motion-v1`, `trigger` LIVE | CATCH_UP | BACKFILL, `input_max_ts`). `UNIQUE (punto, evaluated_at)`. Grados en `double precision` para que el estado vuelva idéntico. Nombres que difieren de §6.2 porque §5.3 y §5.4 repiten nombres: `shift_sign` es el del estado y `shift_sign_hour` el de la hora; `gap_from`/`gap_to` son los del estado y el hueco evaluado en la hora va en `gap_eval_from`, `gap_eval_to`, `dp_gap`, `dr_gap`. Columnas extra: `shift_records_2h`.
- **Seed** (`SeedInitialMotionThresholds`, solo con la tabla vacía): v1 de cada punto de `devices` con `valid_from` = su primera medición de pitch o roll. El MP-11556 (`MotionPilotPointID`) con los valores de §5.2 (`DefaultMotionParams`); el resto, la de punto nuevo (`NewPointMotionParams`: `alerts_enabled = false`, `SOLO_MOVIMIENTO`). Un punto de `devices` sin versión recibe esa v1 con `valid_from = now` en la pasada del scheduler.

### Ejecución

| Trigger | Acción | Componente |
|---|---|---|
| `MotionScheduler` (al arrancar + minuto `RULE_ENGINE_MOTION_MINUTE`, 40) | Da la v1 de punto nuevo a los puntos de `devices` sin versión; por cada punto con versión evalúa, en orden, desde la última fila + 1 h hasta el inicio de la hora actual (LIVE la última, CATCH_UP las anteriores) y publica. Punto sin filas ⇒ Warn "correr rule_engine -backfill -motion" | `MotionEvaluationService.RunLive` |
| `cmd/rule_engine -backfill -motion [-point] [-from] [-to] [-rebuild]` | Histórico con la MISMA evaluación, `trigger = BACKFILL`, sin publicar. Retoma desde la última fila; sin filas arranca en la primera medición (o en `-from`). `-rebuild` borra [from, to) y recalcula (bloqueado con `AGATHA_ENV=production`; avisa si quedan filas posteriores a `-to`) | `MotionBackfillService.Backfill` |
| `GET /api/rule-engine/points/{id}/motion-thresholds`, `GET …/history`, `POST …` (`patch`, `valid_from`, `reason`, `created_by`) | Versiones de movimiento; mismos errores que FUGA (400, 404, 409). `params` va como objeto por nombre | `MotionHandler` → `MotionThresholdService` |
| `POST /api/rule-engine/points/{id}/motion-thresholds/simulate` (`{patch, from, to}`) | En memoria, sin escribir: episodios (apertura, cierre, motivos, tramo de temperatura estable) con los parámetros propuestos y con los vigentes, y la comparación (`new_episodes`, `removed_episodes`) | `MotionHandler` → `MotionSimulationService` |

Las horas se evalúan bajo un lock por punto. El backfill y el catch-up leen `raw_measurements` **por tramos de 168 horas** (una consulta por tramo, con su ventana de 169 h) y cortan la ventana de cada hora en memoria: el resultado es idéntico a una consulta por hora. Un `Save` que choca con una fila existente (`ON CONFLICT DO NOTHING`) sigue desde el estado de ESA fila.

### Evento `rule_engine.motion_evaluated` v1 (`contract/motion_evaluated.go`)

Uno por punto y hora evaluada en vivo (LIVE o CATCH_UP), también NORMAL y NO_CALCULABLE, después del commit. No se publica en backfill, en duplicados ni con `alerts_enabled = false` (modo sombra). Campos de §6.3: `shift.pitch`/`shift.roll` = nivel actual − referencia (3 decimales) mientras `displaced`; `episode_opened_at` dentro del episodio y en la hora de CLEARED; `temp_stable` según las reglas de la fila. Clave de idempotencia: `(measuring_point_id, evaluated_at)`. Adaptador al bus: `busMotionEventPublisher` en `internal/bootstrap/rule_engine.go`.

## Migración

`persistence.Migrate` (desde `NewRuleEngineModule`, idempotente, en cada arranque como los demás módulos):

0. **Conversión v1 → v2** (`upgradeRuleEvaluationsToV2`, una TX, solo si existe la columna `emitted`): agrega `computed_level`, `episode_event` y `reasons`; `episode_event` = `CLEARED` si lo era, `OPENED` si era `EXCEEDED` y la hora calculada anterior no estaba en episodio (`LAG`), si no `NONE`; `computed_level` = `level` si `rule` ∈ {THRESHOLD, PERSISTENCE}, si no `raw_level` (exacto mientras `min_hours ≤ escalate_*`); `reasons = '[]'` (las filas v1 no guardaron la racha: para completarlas, `-rebuild` fuera de producción). Borra `emitted` y su `CHECK`.
1. `AutoMigrate` de ambos modelos: tablas, `CHECK`, `UNIQUE`, índices y la FK (tags).
2. SQL idempotente para lo que GORM no expresa: `CREATE EXTENSION IF NOT EXISTS btree_gist` y el `EXCLUDE` (dentro de un `DO $$ … IF NOT EXISTS (pg_constraint) …`). En Supabase `btree_gist` está disponible; el rol debe poder crear extensiones.
3. `SeedInitialThresholds`: solo si `rule_threshold_versions` está vacía (y existen `devices` y `raw_measurements`).
4. `migrateMotion`: `AutoMigrate` de `motion_threshold_versions` y `motion_evaluations` + `EXCLUDE` de vigencias de movimiento. Después, `SeedInitialMotionThresholds` (solo con la tabla vacía).

## Errores (`domain/errors.go`)

`INVALID_INPUT`: `ErrThresholdOrder`, `ErrExitMarginRange`, `ErrHoursRange`, `ErrPlanitudConfig`, `ErrRojoPlanitudConfig`, `ErrPersistenceConfig`, `ErrInvalidEventType`, `ErrMissingAuthorship`, `ErrValidFromInPast`, `ErrValidFromNotAfterCurrent`, `ErrInvalidTimeRange`, `ErrSimulationRangeTooLong`, `ErrInvalidBody`, `ErrInvalidPatch`, `ErrNoPoints`, `ErrRebuildForbidden`, `ErrResetRequiresRebuild`, `ErrResetRequiresPoint`, `ErrResetNotConfirmed`, `ErrResetFullHistory`, `ErrResetFutureVersions`, `ErrResetUnavailable`. `CONFLICT` (409): `ErrThresholdConflict`. `INVALID_MEASURING_POINT_ID`: `ErrInvalidMeasuringPointID`. `NOT_FOUND`: `ErrThresholdsNotFound`. IO: `ErrConsultingThresholds` (`CONSULTING_THRESHOLDS`), `ErrPersistingThresholds` (`PERSISTING_THRESHOLDS`), `ErrConsultingRuleEvaluations` (`CONSULTING_RULE_EVALUATIONS`), `ErrPersistingRuleEvaluations` (`PERSISTING_RULE_EVALUATIONS`), `ErrConsultingIndicators` (`CONSULTING_EVALUATIONS`). Una hora ignorada, duplicada, tardía o sin umbrales **no** es error.

## Reglas de Import (Aislamiento)

No importa `modules/processing` (ni su `contract/`), `modules/ingestion`, `modules/baseline` ni `shared/eventbus` (`depguard` `rule-engine-isolation`). `contract/` es solo stdlib (`rule-engine-contract-stdlib-only`). Lee `feature_evaluations` con su propio modelo (acople de esquema, como Processing con Baseline). Toda comunicación con el bus va por puertos con adaptadores en `internal/bootstrap`.

## Cableado (Composition Root)

`bootstrap.NewRuleEngineModule(platform)` (`internal/bootstrap/rule_engine.go`): `Migrate` + seed → repos GORM + `IndicatorReader` + `busLevelEventPublisher` → `ThresholdService`, `EvaluationService` (con `IndicatorReader` para el `d_pl`), `BackfillService`, `CatchUpService`, `SimulationService` (sandbox `memorySandbox`) → `RuleEngineModule{Thresholds, Simulator, Evaluator, Backfiller, CatchUp, NewScheduler}`. `NewApplication` agrega el `CatchUpScheduler` a los Runners. `RegisterRoutes(mux)` monta `ThresholdHandler`; `NewApplication` lo agrega a las rutas cuando `rule_engine` está habilitado. `SubscribeIndicatorEvaluated()` traduce `processing/contract.IndicatorEvaluated` → `IndicatorEvaluatedNotice` y lo entrega al subscriber; `NewApplication` lo llama antes de arrancar los schedulers.

| Entrypoint | Rule Engine |
|---|---|
| `cmd/platform` | Módulo `rule_engine` de `AGATHA_MODULES` (suscripción + HTTP `/api/rule-engine` + catch-up horario). **Requiere `processing`**. No está en el default |
| `cmd/rule_engine` | `-backfill [-point] [-from] [-to] [-rebuild] [-reset-versions -point N -confirm N]`; con `-motion`, el histórico de `motion_evaluations` (`-reset-versions` no aplica); sale al terminar. No suscribe al bus |

Movimiento: repos `MotionThresholdRepository`, `MotionEvaluationRepository` y `RawMeasurementReader` (solo lectura de `raw_measurements` y `devices`) + `busMotionEventPublisher` → `MotionThresholdService`, `MotionEvaluationService`, `MotionBackfillService`, `MotionSimulationService` y `NewMotionScheduler` (Runner de `NewApplication`); `RegisterRoutes` monta también `MotionHandler`.

Variables: `AGATHA_ENV` (`production` bloquea `-rebuild` y `-reset-versions`), `RULE_ENGINE_BACKFILL_BATCH_SIZE` (500, también el lote del catch-up), `RULE_ENGINE_CATCHUP_MINUTE` (50), `RULE_ENGINE_MOTION_MINUTE` (40).

## Tests

- `domain/services/level_assessor_test.go`: `RawLevel` en cada borde; escenario completo (con `OPENED`/`CLEARED`); persistencia AMARILLO→NARANJA (literal a las 12 h) y NARANJA→ROJO; salto directo a ROJO; nunca dos niveles; escalamiento NULL; histéresis; racha de salida consecutiva; salida con `raw_level` guardado tras un cambio de umbral; `computed_level` sin HOLD ni racha de entrada; `reasons` de umbral y de persistencia.
- `domain/services/timeline_test.go`, `domain/entities/threshold_version_test.go`: recorte de la línea de tiempo; validación, redondeo, parche, K, vigencia y cortes/salidas por nivel de la plantilla (0,14 / 0,23 / 0,33).
- `application/services/threshold_service_test.go`: crear cierra la anterior en la misma TX; campos omitidos se copian; fecha pasada / orden / rangos / autoría ⇒ validación sin escribir nada; fecha futura; bulk atómico y "todos los puntos"; timeline con varias versiones; primer uso.
- `application/services/evaluation_service_test.go`: ignora 6 h; `NO_CALCULABLE` reenviada sin fila (contrato) y no reenviada si ya hay fila, si es tardía o en backfill; una `level_evaluated` por cada hora calculada (contrato completo, HOLD con computed NORMAL, cierre); motivo de persistencia; duplicado sin fila ni republicación; llegada tardía sin publicar; hueco que pausa; versión vigente en cada `evaluated_at`; primer uso; fallos de publicación y de guardado; el modo sin publicación produce las mismas filas.
- `domain/services/level_assessor_v2_test.go`: planitud (bloquea, confirma, frena persistencia, no frena ROJO por umbral salvo con `confirm_rojo_with_planitud`, que lo frena y recupera la duración), cambio de persistencia con racha acumulada, WINDOW vs STRICT, `below_run`.
- `application/services/simulation_service_test.go`: episodio nuevo con amarillo 0,16, comparación con la vigente, episodio abierto al final, validaciones, no escribe versiones.
- `infrastructure/http/threshold_handler_test.go`: 201 con `previous_version_id` y v1 cerrada; `null` desactiva un escalamiento; planitud y ventana; rechazos 400 sin escribir; 409 por conflicto; bulk; 404; historial y timeline; simulate; ausente vs `null` en el parche.
- `infrastructure/persistence/mappers/mappers_test.go`: `reasons` ida y vuelta en JSON (`[]` sin motivos); columnas v2/v3 ida y vuelta; modo vacío ⇒ STRICT.
- `application/services/backfill_service_test.go`: orden y rachas por lotes, idempotencia, varias versiones, todos los puntos y rango, no publica, `-rebuild` bloqueado en production y recálculo fuera de ella.
- `application/services/backfill_reset_test.go`: `-reset-versions` colapsa las versiones en una v1 con los parámetros de la vigente y `valid_from` = primera medición (o now), recalcula con ella, no toca otros puntos; rechazos sin escribir (sin `-point`, `-confirm` ausente o distinto, sin `-rebuild`, con rango, production, sin repositorio, versiones a futuro); rollback si falla el insert; la FK obliga a borrar las filas antes que las versiones.
- `application/services/level_event_test.go`: `d_pl` y `d_pl_min` en toda hora calculada (también NORMAL), copias de los punteros, omisión de `d_pl_min` sin dato.
- `application/services/catch_up_service_test.go`: interpreta y publica en orden solo las horas posteriores a la última fila (las rachas siguen desde el backfill), idempotente, saltea puntos sin historial.
- `infrastructure/scheduling/catch_up_scheduler_test.go`: próximo minuto, minuto inválido ⇒ 50, corre al arrancar y termina al cancelar.
- `infrastructure/events/indicator_evaluated_subscriber_test.go`: delega al caso de uso y registra sus errores.
- `internal/bootstrap/rule_engine_test.go`: traducción del contrato y **de punta a punta con el bus real** (secuencia que cruza el umbral con ventanas de 6 h y un `NO_CALCULABLE` ⇒ filas correctas y una `level_evaluated` por hora de 24 h, en orden, con `OPENED`/`CLEARED`).
- `infrastructure/persistence/repositories/repositories_integration_test.go` (`-tags integration`, `DATABASE_URL`, **no corrido** con los cambios v2: falta cubrir `NOWAIT` ⇒ conflicto, los `CHECK` nuevos y `GetDPl`): EXCLUDE y CHECK (incluido `episode_event`), cierre + creación en la misma TX y rollback, timeline, idempotencia de `Save`, `computed_level`/`episode_event`/`reasons` ida y vuelta, posición, borrado.
- **Movimiento:**
  - `domain/services/motion_assess_golden_test.go`: prueba dorada (§12.1), las 3 182 horas de `motion_expected.csv` con igualdad exacta de estados y fechas, numéricos ≤ 1e-6 y los mismos motivos con sus datos.
  - `domain/services/motion_assess_test.go`: los sintéticos de CA-6 (`TEMP_NOT_STABILIZED` a las 36 h; `TEMP_STILL_STABLE` a las 6, 18 y 30 h; hueco evaluado en la siguiente hora calculada; sin desplazamiento nuevo 24 h después de un retorno; `MOV_NEW_POSITION`; `SOLO_MOVIMIENTO` sin `temp_state`; `MOV_ABRUPT` una vez por cambio; movimiento lento en un eje y en los dos sin episodio).
  - `application/services/motion_services_test.go`: backfill completo del MP-11556 por tramos igual a la referencia y sin publicar; retoma e idempotencia; rebuild; rebuild bloqueado en production; catch-up + LIVE con el evento de §6.3; repetir no crea filas ni eventos (CA-8); modo sombra con filas y sin eventos (CA-7); punto sin filas salteado; punto nuevo con v1 en sombra; versiones (copia, cierre, validaciones); simulación y comparación.
  - `infrastructure/persistence/mappers/motion_mappers_test.go`: ida y vuelta exacta de las 3 182 filas reales (estado, motivos y huecos pendientes) y de una versión; forma `[{code, data}]`.
  - `infrastructure/http/motion_handler_test.go` y `infrastructure/scheduling/motion_scheduler_test.go`: rutas, parche tipado, rechazos 400/404, simulate; minuto 40 y corrida al arrancar.
  - **No corrido contra PostgreSQL**: migración, `CHECK`s, `EXCLUDE`, seed y repositorios GORM de movimiento.
