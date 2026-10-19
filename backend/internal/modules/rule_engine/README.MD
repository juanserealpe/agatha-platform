# Modulo: Rule Engine

**Responsabilidad**: interpretar cada hora calculada del índice I(24 h) de Processing con **umbrales versionados por punto** (Amarillo, Naranja, Rojo) y **reglas de persistencia e histéresis**, guardar el resultado en `rule_evaluations` y avisar a Alert Management **solo** cuando un punto está sobre el umbral (`rule_engine.leak_threshold_exceeded`) y una única vez cuando el episodio termina (`rule_engine.leak_threshold_cleared`). Processing **mide**; el Rule Engine **interpreta**; Alert Management (todavía no existe) **decide**.

Todo en **UTC** (`America/Bogota` solo en logs). Hoy hay un solo tipo de evento: `FUGA`.

## Estructura

```
rule_engine/
├── contract/                 # superficie pública (solo stdlib): tópicos + LeakThresholdExceeded/Cleared
├── domain/
│   ├── errors.go             # AppError del módulo
│   ├── entities/             # Level/Rule/Emission, ThresholdParams/Patch/Version, RuleEvaluation, ThresholdSegment
│   ├── repositories/         # ThresholdRepository (+ ThresholdWriter transaccional), RuleEvaluationRepository
│   └── services/             # lógica PURA: RawLevel, Assess (persistencia + histéresis), ClipTimeline
├── application/
│   ├── ports/                # Clock, LevelEventPublisher, IndicatorSource (feature_evaluations, solo lectura)
│   └── services/             # ThresholdService, EvaluationService (EvaluateIndicator), BackfillService
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
| `level` | Nivel interpretado (persistencia, histéresis, racha de entrada). |
| `rule` | `THRESHOLD` \| `PERSISTENCE` \| `HOLD` \| `NONE`: por qué tiene ese `level`. |
| `in_episode`, `hours_above`, `hours_in_level` | Estado para la hora siguiente (no hay estado en memoria) y contexto para Alert Management. |
| `threshold_version_id` FK | Versión usada (trazabilidad; la vigente en `evaluated_at`). |
| `low_confidence` | Copia de Processing (línea base de confianza baja). |
| `emitted` | `NONE` \| `EXCEEDED` \| `CLEARED`: qué evento correspondía (aunque fuera tardía y no se publicara). |
| `created_at` | Auditoría. |

`CHECK` en los enums. Índice `ix_rule_evaluations_point_event_recent (measuring_point_id, event_type, evaluated_at DESC)` para leer las últimas K horas (el único también lo cubriría; se dejó explícito según la especificación).

Las horas `NO_CALCULABLE` **no generan fila**. Los dos esquemas los crea `persistence.Migrate` (ver "Migración").

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
3. **Fuera de episodio**: raw ≥ AMARILLO y `runAmarillo ≥ min_hours` ⇒ abre (`level = nivelCalculado`, `EXCEEDED`, `hours_above = runAmarillo`); si no `NORMAL`/`NONE`/`NONE`.
4. **Dentro de episodio**: raw ≥ AMARILLO ⇒ `level = nivelCalculado` (sube o baja), `EXCEEDED`. raw NORMAL ⇒ `belowRun` = horas seguidas con I < `amarillo − exit_margin` **y raw guardado NORMAL**; si `belowRun ≥ exit_hours` cierra (`NORMAL`, `rule NONE`, `CLEARED`); si no `HOLD` (conserva el nivel, `NONE`).
5. `hours_above` = horas calculadas desde la primera hora de la racha de entrada, incluida la actual (la hora de cierre cuenta: es la última del episodio; 0 fuera de episodio). `hours_in_level` = horas seguidas con el mismo `level`.

**Huecos**: una hora `NO_CALCULABLE` no tiene fila, así que las rachas cuentan horas **calculadas**: un hueco **pausa** las rachas, no las reinicia.

**Cambio de umbral**: no recalcula horas pasadas. Las rachas usan el `raw_level` guardado; por eso `belowRun` exige raw guardado NORMAL (si se subiera `amarillo`, una hora que fue AMARILLO con su versión no se reinterpreta como "baja" para cerrar el episodio).

### Escenario de referencia (valores por defecto, `level_assessor_test.go`)

| I por hora | level | rule | emitted | Nota |
|---|---|---|---|---|
| 0,12 · 0,14 | NORMAL | NONE | NONE | |
| 0,19 | NORMAL | NONE | NONE | racha de entrada pendiente (1 < 2) |
| 0,20 | AMARILLO | THRESHOLD | EXCEEDED | abre, `hours_above` 2 |
| 0,21 | AMARILLO | THRESHOLD | EXCEEDED | |
| 0,27 | NARANJA | THRESHOLD | EXCEEDED | |
| 0,22 | AMARILLO | THRESHOLD | EXCEEDED | baja dentro del episodio |
| 12 h entre 0,18 y 0,24 | AMARILLO → NARANJA | THRESHOLD → PERSISTENCE | EXCEEDED | la racha ≥ AMARILLO viene desde la 0,19: llega a 12 en la 7.ª hora del bloque; la 12.ª es NARANJA/PERSISTENCE |
| hueco NO_CALCULABLE | — | — | — | sin fila; la racha sigue |
| 0,17 | NARANJA | HOLD | NONE | banda de histéresis |
| 0,15 · 0,14 | NARANJA · NORMAL | HOLD · NONE | NONE · **CLEARED** | la 2.ª cumple `exit_hours` |
| 0,20 · 0,21 | NORMAL · AMARILLO | NONE · THRESHOLD | NONE · EXCEEDED | episodio nuevo en la 2.ª hora |

Además: salto directo a ROJO (desde NORMAL respeta `min_hours`; dentro de un episodio sube de inmediato), persistencia NARANJA → ROJO a las 6 h, nunca dos niveles por persistencia, escalamiento NULL ⇒ nunca PERSISTENCE, 12 h literales desde una racha nueva ⇒ la 12.ª es la primera PERSISTENCE.

## Evaluación horaria (`EvaluationService.EvaluateIndicator`)

Entrada: `IndicatorEvaluatedNotice` (vista propia del evento de Processing, traducida en bootstrap). Solo `WindowHours == 24` y `Status == CALCULATED`; el resto se ignora (`Debug`).

1. Si ya existe la fila (punto, evento, hora) ⇒ nada (`DUPLICATE`; no se republica).
2. Versión vigente en `evaluated_at` (sin ninguna ⇒ primer uso, ver arriba) y las últimas K filas anteriores.
3. `Assess` + `INSERT … ON CONFLICT DO NOTHING` en una TX. Las horas de un punto se serializan con un lock en memoria (la siguiente necesita la fila de la anterior).
4. Post-commit, según `emitted`: publica `EXCEEDED` o `CLEARED`; `NONE` ⇒ nada. Un fallo al publicar **nunca** falla la evaluación.
5. Si hay filas posteriores del punto (llegada tardía o desordenada) ⇒ se guarda **sin publicar** + `Warn`. Las filas posteriores no se recalculan (para eso, `-rebuild`).
6. Log `Info`: punto, hora, I, raw, level, rule, versión y evento emitido.

Un error (BD) se registra en el subscriber y la hora queda sin interpretar: el evento es una pista; el backfill puede completarla.

## Eventos

### Consumidos

| Tópico | Publicador | Notice propio | Suscripción | Acción |
|---|---|---|---|---|
| `processing.indicator_evaluated` | Processing (`EvaluatePending`, post-commit) | `services.IndicatorEvaluatedNotice` | `rule_engine.indicator_evaluated` | `EvaluateIndicator` |

Best-effort: pueden perderse (overflow, reinicio) o duplicarse. `feature_evaluations` es la fuente de verdad; el backfill de Processing no publica.

### Publicados (`contract/`, `SchemaVersion = 1`)

**`rule_engine.leak_threshold_exceeded`** — `LeakThresholdExceeded`, cada hora calculada con `emitted = EXCEEDED` (incluida la que abre):

`rule_evaluation_id`, `measuring_point_id`, `event_type` (`FUGA`), `evaluated_at`, `level` (AMARILLO|NARANJA|ROJO), `previous_level` (nivel de la hora calculada anterior), `rule` (THRESHOLD|PERSISTENCE), `indicator_value`, `thresholds{amarillo, naranja, rojo}`, `threshold_version_id`, `hours_above`, `hours_in_level`, `low_confidence`, `occurred_at`, `schema_version`.

**`rule_engine.leak_threshold_cleared`** — `LeakThresholdCleared`, una sola vez al cerrar el episodio:

`rule_evaluation_id`, `measuring_point_id`, `event_type`, `evaluated_at`, `indicator_value`, `last_level` (nivel de la hora anterior al cierre), `hours_above` (duración del episodio, incluida la hora de cierre), `threshold_version_id`, `occurred_at`, `schema_version`.

No se emite nada en horas sin superación, en la racha de entrada pendiente ni en la banda de histéresis. **Clave de idempotencia para consumidores: `(measuring_point_id, event_type, evaluated_at)`.** Se publican por el puerto `LevelEventPublisher`; el adaptador al bus (`busLevelEventPublisher`) vive en `internal/bootstrap`. Hoy no hay suscriptores (Alert Management no existe): el bus los descarta.

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

- `domain/services/level_assessor_test.go`: `RawLevel` en cada borde; escenario completo; persistencia AMARILLO→NARANJA (literal a las 12 h) y NARANJA→ROJO; salto directo a ROJO; nunca dos niveles; escalamiento NULL; histéresis; racha de salida consecutiva; salida con `raw_level` guardado tras un cambio de umbral.
- `domain/services/timeline_test.go`, `domain/entities/threshold_version_test.go`: recorte de la línea de tiempo; validación, redondeo, parche, K y vigencia.
- `application/services/threshold_service_test.go`: crear cierra la anterior en la misma TX; campos omitidos se copian; fecha pasada / orden / rangos / autoría ⇒ validación sin escribir nada; fecha futura; bulk atómico y "todos los puntos"; timeline con varias versiones; primer uso.
- `application/services/evaluation_service_test.go`: ignora 6 h y `NO_CALCULABLE`; publica exceeded/cleared solo cuando corresponde (contrato completo); duplicado sin fila ni republicación; llegada tardía sin publicar; hueco que pausa; versión vigente en cada `evaluated_at`; primer uso; fallos de publicación y de guardado; el modo sin publicación produce las mismas filas.
- `application/services/backfill_service_test.go`: orden y rachas por lotes, idempotencia, varias versiones, todos los puntos y rango, no publica, `-rebuild` bloqueado en production y recálculo fuera de ella.
- `infrastructure/events/indicator_evaluated_subscriber_test.go`: delega al caso de uso y registra sus errores.
- `internal/bootstrap/rule_engine_test.go`: traducción del contrato y **de punta a punta con el bus real** (secuencia que cruza el umbral con ventanas de 6 h y un `NO_CALCULABLE` ⇒ filas correctas y eventos exceeded/cleared en el bus).
- `infrastructure/persistence/repositories/repositories_integration_test.go` (`-tags integration`, `DATABASE_URL`): EXCLUDE y CHECK, cierre + creación en la misma TX y rollback, timeline, idempotencia de `Save`, posición, borrado.
