# Modulo: Rule Engine

**Responsabilidad**: interpretar cada hora calculada del índice I(24 h) de Processing con **umbrales versionados por punto** (Amarillo, Naranja, Rojo) y **reglas de persistencia e histéresis**, guardar el resultado en `rule_evaluations` y publicar **cada hora** de 24 h a Alert Management (`rule_engine.level_evaluated`, v2), incluidas las de nivel NORMAL y las `NO_CALCULABLE`. Processing **mide**; el Rule Engine **interpreta** (es dueño del **nivel del indicador**: umbral + persistencia + episodio); Alert Management (todavía no existe) **decide** (alerta única, estabilización, atención, silencio, clasificación). El `CLEARED` del episodio **no cierra la alerta**: para Alert Management es una evaluación más.

Todo en **UTC** (`America/Bogota` solo en logs). Hoy hay un solo tipo de evento: `FUGA`.

## Estructura

```
rule_engine/
├── contract/                 # superficie pública (solo stdlib): tópico + LevelEvaluated (v2)
├── domain/
│   ├── errors.go             # AppError del módulo
│   ├── entities/             # Level/Rule/EpisodeEvent/ReasonCode, ThresholdParams/Patch/Version, RuleEvaluation, Reason, ThresholdSegment
│   ├── repositories/         # ThresholdRepository (+ ThresholdWriter transaccional), RuleEvaluationRepository
│   └── services/             # lógica PURA: RawLevel, Assess (persistencia + histéresis), ClipTimeline
├── application/
│   ├── ports/                # Clock, LevelEventPublisher, IndicatorSource (feature_evaluations, solo lectura)
│   └── services/             # ThresholdService, EvaluationService (EvaluateIndicator), BackfillService, LevelEvaluatedFromRow
└── infrastructure/
    ├── events/               # IndicatorEvaluatedSubscriber → EvaluateIndicator
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
| `valid_from`, `valid_to` TIMESTAMPTZ | Vigencia `[valid_from, valid_to)`; `valid_to` NULL = vigente. `CHECK valid_to IS NULL OR valid_to > valid_from`. |
| `reason`, `created_by`, `created_at` | Auditoría de cada cambio. |

Restricciones: los `CHECK` de arriba, `min_hours >= 1`, `exit_hours >= 1`, `escalate_* IS NULL OR >= 1`; **`EXCLUDE USING gist (measuring_point_id WITH =, event_type WITH =, tstzrange(valid_from, valid_to) WITH &&)`** (`ex_rule_threshold_versions_no_overlap`, requiere `btree_gist`): la BD garantiza que nunca hay dos versiones vigentes a la vez. Índice "vigente en t": `ix_rule_threshold_versions_effective (measuring_point_id, event_type, valid_from DESC)`.

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
| `threshold_version_id` FK | Versión usada (trazabilidad; la vigente en `evaluated_at`). |
| `low_confidence` | Copia de Processing (línea base de confianza baja). |
| `episode_event` | `OPENED` (esta hora abrió el episodio) \| `CLEARED` (lo cerró) \| `NONE`. |
| `reasons` JSONB | Motivos de `computed_level` (mismo formato que el evento): `LEAK_THRESHOLD {i, threshold}` si raw ≥ AMARILLO y `LEAK_PERSISTENCE {hours, from}` si la racha subió un nivel. `[]` si NORMAL. |
| `created_at` | Auditoría. |

`CHECK` en los enums. Índice `ix_rule_evaluations_point_event_recent (measuring_point_id, event_type, evaluated_at DESC)` para leer las últimas K horas (el único también lo cubriría; se dejó explícito según la especificación).

La fila contiene **todo** lo que lleva su `level_evaluated` (umbrales y salidas por la FK de la versión): `services.LevelEvaluatedFromRow(row, params, occurredAt)` reconstruye el mismo evento (base del reconciliador de Alert Management, AM-19). Las horas `NO_CALCULABLE` **no generan fila**. Los dos esquemas los crea `persistence.Migrate` (ver "Migración").

## Versionado y no retroactividad

- **Nunca** `UPDATE` ni `DELETE` de una versión. La única modificación permitida es cerrar la abierta (`valid_to = valid_from` de la nueva) **en la misma transacción** que inserta la nueva (`SELECT … FOR UPDATE` de la abierta + `UPDATE` + `INSERT`).
- **Nunca retroactivo**: `valid_from >= now` (cero ⇒ now; una fecha futura se permite y deja la actual vigente hasta entonces).
- **Única excepción — el seed**: la primera vez que se migra (tabla vacía), cada punto de `devices` recibe la **versión 1** con la plantilla, `valid_from = MIN(raw_measurements.timestamp)` del punto (`now()` si no tiene datos), `reason = "valor inicial beta"`, `created_by = "system"`. Es retroactiva **a propósito**: sin ella el backfill no tendría umbrales para el histórico.
- **Punto nuevo (primer uso)**: si llega una hora de un punto sin ninguna versión, se crea la versión 1 con la plantilla y `valid_from = now` (`"valor inicial beta (primer uso)"`). Esa hora (anterior a now) **no se interpreta** (`NO_THRESHOLDS`, Warn) y el backfill tampoco podrá interpretar horas anteriores a now para ese punto. Si se necesita su histórico, sembrar su versión 1 a mano con la misma regla del seed **antes** de habilitar el módulo.

## Parámetros (plantilla, todos beta)

| Parámetro | Valor |
|---|---|
| `amarillo` / `naranja` / `rojo` | 0,18 / 0,25 / 0,35 |
| `min_hours` | 2 |
| `escalate_amarillo_after_hours` | 12 |
| `escalate_naranja_after_hours` | 6 |
| `exit_margin` | 0,02 (salida bajo 0,16) |
| `exit_hours` | 2 |

`entities.DefaultThresholdParams()` es la única fuente: la usan el seed, el primer uso y la primera versión creada por el caso de uso. Los valores se redondean a 4 decimales **antes** de validar.

## Algoritmo (`domain/services.Assess`, puro)

Niveles `NORMAL < AMARILLO < NARANJA < ROJO`. Para cada hora calculada, con la versión vigente en su `evaluated_at` y las últimas K filas (K = máx. de los parámetros de horas + 1):

1. **Raw**: ROJO si I ≥ rojo; NARANJA si ≥ naranja; AMARILLO si ≥ amarillo; si no NORMAL.
2. **Persistencia** (sobre el `raw_level` guardado): `runAmarillo`/`runNaranja` = horas consecutivas, terminando en la actual, con raw ≥ AMARILLO / ≥ NARANJA. Candidato ROJO si `runNaranja ≥ escalate_naranja`; si no NARANJA si `runAmarillo ≥ escalate_amarillo`. `nivelCalculado = max(raw, candidato)` (como mucho **un** nivel por regla). `rule = PERSISTENCE` si sube sobre el raw; si no `THRESHOLD`.
3. **Fuera de episodio**: raw ≥ AMARILLO y `runAmarillo ≥ min_hours` ⇒ abre (`level = nivelCalculado`, `OPENED`, `hours_above = runAmarillo`); si no `NORMAL`/`NONE`/`NONE`.
4. **Dentro de episodio**: raw ≥ AMARILLO ⇒ `level = nivelCalculado` (sube o baja). raw NORMAL ⇒ `belowRun` = horas seguidas con I < `amarillo − exit_margin` **y raw guardado NORMAL**; si `belowRun ≥ exit_hours` cierra (`NORMAL`, `rule NONE`, `CLEARED`); si no `HOLD` (conserva el nivel).
5. `hours_above` = horas calculadas desde la primera hora de la racha de entrada, incluida la actual (la hora de cierre cuenta: es la última del episodio; 0 fuera de episodio). `hours_in_level` = horas seguidas con el mismo `level`.
6. `computed_level = nivelCalculado` en **todas** las horas (dentro o fuera de episodio, también en HOLD) y `reasons` lo explican: `LEAK_THRESHOLD` (`level` = raw, `i`, `threshold` = corte del raw) y, si la persistencia subió, `LEAK_PERSISTENCE` (`level` = nivel ganado, `hours` = racha, `from` = AMARILLO o NARANJA).

**Salidas por nivel** (`ThresholdParams.ExitFor`): `corte − exit_margin`, redondeado a 4 decimales ⇒ 0,16 / 0,23 / 0,33. El Rule Engine solo usa la de AMARILLO para cerrar el episodio; las tres viajan en el evento (`exit`) para la histéresis por nivel de Alert Management. Si hacen falta márgenes distintos por nivel, se agregan columnas a `rule_threshold_versions`.

**Huecos**: una hora `NO_CALCULABLE` no tiene fila, así que las rachas cuentan horas **calculadas**: un hueco **pausa** las rachas, no las reinicia.

**Cambio de umbral**: no recalcula horas pasadas. Las rachas usan el `raw_level` guardado; por eso `belowRun` exige raw guardado NORMAL (si se subiera `amarillo`, una hora que fue AMARILLO con su versión no se reinterpreta como "baja" para cerrar el episodio).

### Escenario de referencia (valores por defecto, `level_assessor_test.go`)

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

Además: salto directo a ROJO (desde NORMAL respeta `min_hours`; dentro de un episodio sube de inmediato), persistencia NARANJA → ROJO a las 6 h, nunca dos niveles por persistencia, escalamiento NULL ⇒ nunca PERSISTENCE, 12 h literales desde una racha nueva ⇒ la 12.ª es la primera PERSISTENCE.

## Evaluación horaria (`EvaluationService.EvaluateIndicator`)

Entrada: `IndicatorEvaluatedNotice` (vista propia del evento de Processing, traducida en bootstrap). Solo `WindowHours == 24`; las de 6 h, un estado desconocido o `CALCULATED` sin I se ignoran (`Debug`, `IGNORED`).

**Hora `NO_CALCULABLE` (24 h)**: no crea fila. Bajo el lock del punto, si la hora ya tiene fila (`DUPLICATE`) o hay filas posteriores (tardía, `Warn`) no se reenvía; si no, publica `level_evaluated` con `status = NO_CALCULABLE` (`OutcomeNoCalculable`). En backfill nunca se reenvía. Qué hacer con ella (pausar la estabilización) lo decide Alert Management.

**Hora `CALCULATED` (24 h)**:

1. Si ya existe la fila (punto, evento, hora) ⇒ nada (`DUPLICATE`; no se republica).
2. Versión vigente en `evaluated_at` (sin ninguna ⇒ primer uso, ver arriba) y las últimas K filas anteriores.
3. `Assess` + `INSERT … ON CONFLICT DO NOTHING` en una TX. Las horas de un punto se serializan con un lock en memoria (la siguiente necesita la fila de la anterior).
4. Post-commit publica `level_evaluated` (`LevelEvaluatedFromRow`) **en todas las horas**, también NORMAL y HOLD. Un fallo al publicar **nunca** falla la evaluación.
5. Si hay filas posteriores del punto (llegada tardía o desordenada) ⇒ se guarda **sin publicar** + `Warn`. Las filas posteriores no se recalculan (para eso, `-rebuild`).
6. Log `Info`: punto, hora, I, raw, computed, level, rule, versión y `episode_event`.

Un error (BD) se registra en el subscriber y la hora queda sin interpretar: el evento es una pista; el backfill puede completarla.

## Eventos

### Consumidos

| Tópico | Publicador | Notice propio | Suscripción | Acción |
|---|---|---|---|---|
| `processing.indicator_evaluated` | Processing (`EvaluatePending`, post-commit) | `services.IndicatorEvaluatedNotice` | `rule_engine.indicator_evaluated` | `EvaluateIndicator` |

Best-effort: pueden perderse (overflow, reinicio) o duplicarse. `feature_evaluations` es la fuente de verdad; el backfill de Processing no publica.

### Publicados (`contract/`, `SchemaVersion = 2`)

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
    { "code": "LEAK_THRESHOLD",   "level": "AMARILLO", "data": { "i": 0.22, "threshold": 0.18 } },
    { "code": "LEAK_PERSISTENCE", "level": "NARANJA",  "data": { "hours": 12, "from": "AMARILLO" } }
  ],
  "thresholds": { "amarillo": 0.18, "naranja": 0.25, "rojo": 0.35 },
  "exit":       { "amarillo": 0.16, "naranja": 0.23, "rojo": 0.33 },
  "threshold_version_id": 3,
  "hours_above": 14,
  "low_confidence": false,
  "occurred_at": "2026-10-15T17:35:04Z",
  "schema_version": 2
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

No se publica en el **backfill**, en **duplicados** ni en **llegadas tardías** (RE-06). **Clave de idempotencia para consumidores: `(measuring_point_id, event_type, evaluated_at)`.** Se publica por el puerto `LevelEventPublisher`; el adaptador al bus (`busLevelEventPublisher`) vive en `internal/bootstrap`. Hoy no hay suscriptores (Alert Management no existe): el bus los descarta.

### Pendiente (fase 2, RE-07)

Rachas en tiempo de reloj con `hueco_max` (P1) e histéresis en la racha (P2): hoy un I que oscila alrededor de 0,18 durante días nunca llega a NARANJA por persistencia (la racha se corta en cada hora bajo el corte).

## Casos de uso de umbrales (`ThresholdService`, sin HTTP)

| Caso de uso | Comportamiento |
|---|---|
| `CreateThresholdVersion(cmd)` | Valida (orden, rangos, motivo/autor, `valid_from` no pasado; `AppError` `INVALID_INPUT`), copia de la vigente los campos que el `Patch` no informa (`entities.Some(...)`; `Some[*int](nil)` deja un escalamiento en NULL), cierra la abierta y crea la nueva en una TX. Sin versión previa parte de la plantilla. |
| `CreateThresholdVersionBulk(cmd)` | Igual para varios puntos en **una** TX (si uno falla, ninguno). `PointIDs` vacío ⇒ todos los puntos con alguna versión. |
| `GetCurrentThresholds(point, event)` | Versión vigente ahora (`NOT_FOUND` si no hay). |
| `GetThresholdHistory(point, event)` | Todas las versiones, ascendentes. |
| `GetThresholdTimeline(point, event, from, to)` | Tramos que se solapan con `[from, to)`, recortados: `tramo.from = GREATEST(valid_from, from)`, `tramo.to = LEAST(COALESCE(valid_to, to), to)`. |
| `EnsureInitialVersion(point, event)` | Primer uso (lo invoca `EvaluateIndicator`). |

### Cómo cambiar umbrales HOY (sin endpoints)

Con una migración/script que respete **exactamente** las reglas: en **una** transacción, cerrar la abierta e insertar la nueva con `valid_from = now()` (o futura). Preferí invocar `ThresholdService.CreateThresholdVersion` desde un programa Go (`bootstrap.NewRuleEngineModule(platform).Thresholds`). Si se hace en SQL:

```sql
BEGIN;
SELECT * FROM rule_threshold_versions
 WHERE measuring_point_id = 11556 AND event_type = 'FUGA' AND valid_to IS NULL FOR UPDATE;
UPDATE rule_threshold_versions SET valid_to = now()
 WHERE measuring_point_id = 11556 AND event_type = 'FUGA' AND valid_to IS NULL;
INSERT INTO rule_threshold_versions (measuring_point_id, event_type, version, amarillo, naranja, rojo,
  min_hours, escalate_amarillo_after_hours, escalate_naranja_after_hours, exit_margin, exit_hours,
  valid_from, reason, created_by)
SELECT measuring_point_id, event_type, version + 1, amarillo, naranja, 0.40 /* rojo nuevo */,
  min_hours, escalate_amarillo_after_hours, escalate_naranja_after_hours, exit_margin, exit_hours,
  valid_to /* = now() del UPDATE */, 'motivo del cambio', 'quién'
  FROM rule_threshold_versions
 WHERE measuring_point_id = 11556 AND event_type = 'FUGA' ORDER BY version DESC LIMIT 1;
COMMIT;
```

Los `CHECK` y el `EXCLUDE` rechazan cualquier versión inválida o solapada. Nunca `UPDATE` de umbrales ni `valid_from` en el pasado.

### Endpoints previstos (NO implementados)

Prefijo `/api/rule-engine`, `POST` con token: `GET/POST /points/{id}/thresholds`, `GET /points/{id}/thresholds/history`, `POST /thresholds/bulk`, `GET /points/{id}/thresholds/timeline?from=&to=`. Solo hace falta agregar los handlers sobre `ThresholdService`. Dashboard: puerto `ThresholdTimelineProvider` y campo `thresholds` en la serie del indicador.

## Backfill (`cmd/rule_engine -backfill`)

```
rule_engine -backfill [-point 11556] [-from 2026-07-09T00:00:00Z] [-to 2026-10-19T00:00:00Z] [-rebuild]
```

- Fuente: `feature_evaluations` (`window_hours = 24`, `status = CALCULATED`) por el puerto de solo lectura `IndicatorSource` (`repositories.IndicatorReader`, modelo propio).
- Por punto (todos si no hay `-point`), en orden ascendente de `evaluated_at`, en lotes (`RULE_ENGINE_BACKFILL_BATCH_SIZE`, 500), con **el mismo caso de uso** en modo sin publicación (`EvaluateIndicatorWithoutPublishing`): **nunca publica**. Usa la versión vigente en cada hora (varias versiones en el rango ⇒ cada una en su tramo).
- Idempotente: las horas ya interpretadas se saltean (`dupl.`). Un error aborta; repetirlo retoma.
- `-rebuild`: borra `rule_evaluations` del rango/punto y recalcula. **Bloqueado con `AGATHA_ENV=production`**. Útil tras llegadas tardías (las horas posteriores no se recalculan solas). Recomendado sin `-to` (rebuild hasta el final), para que ninguna hora quede calculada sobre un historial viejo.
- Log de progreso por lote (horas por nivel, episodios abiertos y cerrados) y tabla final por punto.
- ~4 consultas por hora (posición, versión, historial, insert): con BD remota, del orden de minutos por punto-año. Tope de la corrida: 2 h.

**Orden de inicialización**: `baseline -backfill` → `processing -backfill` → `rule_engine -backfill` → habilitar `rule_engine` en `AGATHA_MODULES`. Si el módulo ya corrió en vivo antes del backfill, las horas históricas serán "tardías" respecto de las vivas: usá `-rebuild` fuera de producción.

## Migración

`persistence.Migrate` (desde `NewRuleEngineModule`, idempotente, en cada arranque como los demás módulos):

0. **Conversión v1 → v2** (`upgradeRuleEvaluationsToV2`, una TX, solo si existe la columna `emitted`): agrega `computed_level`, `episode_event` y `reasons`; `episode_event` = `CLEARED` si lo era, `OPENED` si era `EXCEEDED` y la hora calculada anterior no estaba en episodio (`LAG`), si no `NONE`; `computed_level` = `level` si `rule` ∈ {THRESHOLD, PERSISTENCE}, si no `raw_level` (exacto mientras `min_hours ≤ escalate_*`); `reasons = '[]'` (las filas v1 no guardaron la racha: para completarlas, `-rebuild` fuera de producción). Borra `emitted` y su `CHECK`.
1. `AutoMigrate` de ambos modelos: tablas, `CHECK`, `UNIQUE`, índices y la FK (tags).
2. SQL idempotente para lo que GORM no expresa: `CREATE EXTENSION IF NOT EXISTS btree_gist` y el `EXCLUDE` (dentro de un `DO $$ … IF NOT EXISTS (pg_constraint) …`). En Supabase `btree_gist` está disponible; el rol debe poder crear extensiones.
3. `SeedInitialThresholds`: solo si `rule_threshold_versions` está vacía (y existen `devices` y `raw_measurements`).

## Errores (`domain/errors.go`)

`INVALID_INPUT`: `ErrThresholdOrder`, `ErrExitMarginRange`, `ErrHoursRange`, `ErrInvalidEventType`, `ErrMissingAuthorship`, `ErrValidFromInPast`, `ErrValidFromNotAfterCurrent`, `ErrInvalidTimeRange`, `ErrNoPoints`, `ErrRebuildForbidden`. `INVALID_MEASURING_POINT_ID`: `ErrInvalidMeasuringPointID`. `NOT_FOUND`: `ErrThresholdsNotFound`. IO: `ErrConsultingThresholds` (`CONSULTING_THRESHOLDS`), `ErrPersistingThresholds` (`PERSISTING_THRESHOLDS`), `ErrConsultingRuleEvaluations` (`CONSULTING_RULE_EVALUATIONS`), `ErrPersistingRuleEvaluations` (`PERSISTING_RULE_EVALUATIONS`), `ErrConsultingIndicators` (`CONSULTING_EVALUATIONS`). Una hora ignorada, duplicada, tardía o sin umbrales **no** es error.

## Reglas de Import (Aislamiento)

No importa `modules/processing` (ni su `contract/`), `modules/ingestion`, `modules/baseline` ni `shared/eventbus` (`depguard` `rule-engine-isolation`). `contract/` es solo stdlib (`rule-engine-contract-stdlib-only`). Lee `feature_evaluations` con su propio modelo (acople de esquema, como Processing con Baseline). Toda comunicación con el bus va por puertos con adaptadores en `internal/bootstrap`.

## Cableado (Composition Root)

`bootstrap.NewRuleEngineModule(platform)` (`internal/bootstrap/rule_engine.go`): `Migrate` + seed → repos GORM + `IndicatorReader` + `busLevelEventPublisher` → `ThresholdService`, `EvaluationService`, `BackfillService` → `RuleEngineModule{Thresholds, Evaluator, Backfiller}`. `SubscribeIndicatorEvaluated()` traduce `processing/contract.IndicatorEvaluated` → `IndicatorEvaluatedNotice` y lo entrega al subscriber; `NewApplication` lo llama antes de arrancar los schedulers.

| Entrypoint | Rule Engine |
|---|---|
| `cmd/platform` | Módulo `rule_engine` de `AGATHA_MODULES`. **Requiere `processing`**. No está en el default |
| `cmd/rule_engine` | `-backfill [-point] [-from] [-to] [-rebuild]`; sale al terminar. No suscribe al bus |

Variables: `AGATHA_ENV` (`production` bloquea `-rebuild`), `RULE_ENGINE_BACKFILL_BATCH_SIZE` (500).

## Tests

- `domain/services/level_assessor_test.go`: `RawLevel` en cada borde; escenario completo (con `OPENED`/`CLEARED`); persistencia AMARILLO→NARANJA (literal a las 12 h) y NARANJA→ROJO; salto directo a ROJO; nunca dos niveles; escalamiento NULL; histéresis; racha de salida consecutiva; salida con `raw_level` guardado tras un cambio de umbral; `computed_level` sin HOLD ni racha de entrada; `reasons` de umbral y de persistencia.
- `domain/services/timeline_test.go`, `domain/entities/threshold_version_test.go`: recorte de la línea de tiempo; validación, redondeo, parche, K, vigencia y cortes/salidas por nivel (0,16 / 0,23 / 0,33).
- `application/services/threshold_service_test.go`: crear cierra la anterior en la misma TX; campos omitidos se copian; fecha pasada / orden / rangos / autoría ⇒ validación sin escribir nada; fecha futura; bulk atómico y "todos los puntos"; timeline con varias versiones; primer uso.
- `application/services/evaluation_service_test.go`: ignora 6 h; `NO_CALCULABLE` reenviada sin fila (contrato) y no reenviada si ya hay fila, si es tardía o en backfill; una `level_evaluated` por cada hora calculada (contrato completo, HOLD con computed NORMAL, cierre); motivo de persistencia; duplicado sin fila ni republicación; llegada tardía sin publicar; hueco que pausa; versión vigente en cada `evaluated_at`; primer uso; fallos de publicación y de guardado; el modo sin publicación produce las mismas filas.
- `infrastructure/persistence/mappers/mappers_test.go`: `reasons` ida y vuelta en JSON (`[]` sin motivos).
- `application/services/backfill_service_test.go`: orden y rachas por lotes, idempotencia, varias versiones, todos los puntos y rango, no publica, `-rebuild` bloqueado en production y recálculo fuera de ella.
- `infrastructure/events/indicator_evaluated_subscriber_test.go`: delega al caso de uso y registra sus errores.
- `internal/bootstrap/rule_engine_test.go`: traducción del contrato y **de punta a punta con el bus real** (secuencia que cruza el umbral con ventanas de 6 h y un `NO_CALCULABLE` ⇒ filas correctas y una `level_evaluated` por hora de 24 h, en orden, con `OPENED`/`CLEARED`).
- `infrastructure/persistence/repositories/repositories_integration_test.go` (`-tags integration`, `DATABASE_URL`): EXCLUDE y CHECK (incluido `episode_event`), cierre + creación en la misma TX y rollback, timeline, idempotencia de `Save`, `computed_level`/`episode_event`/`reasons` ida y vuelta, posición, borrado.
