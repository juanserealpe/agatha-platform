# Resumen del backend — estado al 2026-10-05

Monolito modular en Go con arquitectura hexagonal por módulo. Una sola base de datos PostgreSQL, un bus de eventos en memoria y un *composition root* (`internal/bootstrap`) que es el ÚNICO lugar que conoce a todos los módulos.

---

## 1. Mapa de módulos

| Módulo | Estado | Archivos Go (prod / test) | Qué hace |
|---|---|---|---|
| `shared/` | ✅ Implementado | — | Logger, wrappers HTTP, `AppError`, event bus en memoria y **kernel de cálculo `shared/features`** (fórmulas que comparten Baseline y Processing) |
| `ingestion` | ✅ Implementado (con pendientes) | 53 / 3 | Trae mediciones de DataOS cada 15 min, las guarda en `raw_measurements` y publica `ingestion.measurements_ingested` |
| `dashboard` | ✅ Implementado | 19 / 0 | API de solo lectura (`GET /api/dashboard/*`): dispositivos y mediciones para el frontend |
| `baseline` | ✅ Implementado, falta validación golden (T8) | 34 / 14 | Calcula y versiona la "línea base normal" de cada dispositivo |
| `processing` | ✅ Implementado, falta validación con el analista (P8) | 24 / 10 | Cada hora evalúa ventanas de 24 h y 6 h contra la línea base y guarda el índice I en `feature_evaluations` |
| `rule_engine` | ⬜ Vacío (solo README vacío) | 0 | **Siguiente**: leerá `feature_evaluations` y aplicará rachas/cortes de I |
| `alert_decision`, `alert_management`, `notifications`, `realtime`, `inspection_feedback`, `device_context`, `ai_client` | ⬜ Vacíos (placeholders) | 0 | Aún sin diseño ni código |

> Ojo: `device_context` existe como carpeta vacía, pero el contexto del dispositivo lo resolvió **Baseline** ("Device Context & Baseline").

---

## 2. Flujo completo de datos

```
DataOS ──(cada 15 min)──► INGESTION
                            │ guarda raw_measurements + ingestion_logs (1 TX)
                            │ post-commit, si Inserted > 0:
                            ▼
              evento ingestion.measurements_ingested  (bus en memoria)
                            │
                            ▼  internal/bootstrap traduce el evento
                        PROCESSING ◄──────── BASELINE (consulta síncrona)
                            │                    ▲
                            │                    │ lee raw_measurements (90 días)
                            │                    │ escribe baseline_versions
                            ▼                    │ recalcula por scheduler propio
                   feature_evaluations
                            │
                            ▼
                   RULE ENGINE (no implementado)

DASHBOARD ── lee devices + raw_measurements ──► frontend
```

Puntos clave:

- **Ingestion es el único que ESCRIBE telemetría.** Baseline, Processing y Dashboard solo LEEN `raw_measurements` y `devices`, cada uno con su propio modelo de lectura.
- **Baseline no conoce eventos ni a Processing.** Trabaja con su propio scheduler.
- **Processing no importa a Baseline ni a Ingestion** (lo impone `depguard`). Todo pasa por `internal/bootstrap`.
- Las métricas que importan para el índice son **B1 = 12** (fuga 500-1200 Hz), **B2 = 13** (fuga 1200-2000 Hz) y **Planitud = 18**.

---

## 3. Baseline — qué es la "línea base"

Es la foto estadística de cómo se comporta un dispositivo **cuando todo está normal**. Sin eso, no se puede decir si un valor es raro.

**Qué calcula** (sobre los últimos 90 días):

- **Señal**: `mediana_B2`, `θ_D = P90 de B2`, `P90_Planitud`, `min_elevado_B2`, `Δmin`.
- **Componentes**: mediana y MAD de **D, P, Ex** (ventanas de 6 h y 24 h) y **C** (solo 24 h).

**Cómo lo calcula** (estimación iterativa, máx. 5 pasadas):

1. Calcula la base.
2. Recorre el histórico hora a hora calculando I(24 h) con esa base.
3. Marca como "anómalo" todo lo que tenga `I ≥ 0,18` (tope 40 % del histórico).
4. Recalcula la base **sin** esos periodos y repite hasta que el conjunto excluido no cambie.

Así la base no queda "contaminada" por las fugas que justamente queremos detectar.

**Versionado**: cada recálculo crea una fila nueva en `baseline_versions` con vigencia `[valid_from, valid_to)`. Solo una vigente por punto y **nunca retroactiva**.

**Cuándo recalcula** (`RecalcScheduler`, al arrancar + cada hora en el minuto 10):

| Situación | Trigger |
|---|---|
| No hay versión | `BOOTSTRAP` |
| Confianza `LOW` o sin historial de exclusión, y pasaron ≥ 24 h | `LOW_CONFIDENCE` |
| Pasaron ≥ 168 h (7 días) | `SCHEDULED` |

`Confidence = NORMAL` exige las 7 combinaciones de componentes con ≥ 60 ventanas; si falta alguna, `LOW`.

**Contrato público (congelado)**:

```go
baseline.Facade.GetEffectiveBaseline(ctx, pointID, at) → EffectiveBaseline
```

Devuelve la versión vigente en el instante `at`. Si no hay ninguna: `Status = NONE` (**no es un error**) y además `DeviceFirstSeenAt`.

---

## 4. Processing — cómo funciona (el corazón)

**Responsabilidad**: cada hora y por cada punto activo, evaluar las ventanas de **24 h** y **6 h** que terminan en esa hora y guardar **una fila por (punto, ventana, hora)** en `feature_evaluations`.

**Filosofía: MIDE, NO INTERPRETA.** Calcula Cobertura, D, P, Ex, C, D_Pl, z, s(z) e I. Las rachas, los cortes (0,18 / 0,35 / 0,70), las categorías y las alertas son trabajo del Rule Engine.

### 4.1 Tres disparadores, un solo caso de uso

| Disparador | Cuándo | Tope | `trigger` |
|---|---|---|---|
| Evento `measurements_ingested` | Cada ~15 min (cuando Ingestion inserta algo) | 24 horas | `EVENT` |
| `HourlyScheduler` (reconciliador) | Al arrancar + minuto 35 de cada hora | 72 horas por punto | `RECONCILER` |
| `cmd/replay` | Manual, para el analista | — | `REPLAY` (nunca se guarda en BD) |

Los tres terminan en `EvaluationService.EvaluatePending`.

### 4.2 `EvaluatePending` paso a paso

1. Valida `PointID > 0`.
2. **Lock en proceso por punto**: el evento y el reconciliador nunca evalúan el mismo punto a la vez.
3. **¿Hasta qué hora hay datos listos?** (`readyUntil`)
   - Si viene del evento: `floorHour(watermark)`. El watermark es el MAX(timestamp) del lote, así que hay datos al menos hasta ahí.
   - Si no: `floorHour(MAX(timestamp) de B2)` en la BD.
   - Nunca supera `floorHour(now)`.
4. **`SlotPlanner`** arma las horas a evaluar: `(última evaluada, readyUntil]`. Si el punto nunca se evaluó, toma las 72 horas anteriores. Si no hay nada nuevo, termina (pasa en 3 de cada 4 lotes, porque se ingesta cada 15 min y se evalúa por hora).
5. **Una sola lectura** de samples `[primera hora − 24 h, última hora)` y `features.Normalize`.
6. Por cada hora `t`, en orden:
   - Pide a Baseline la versión vigente en `t` (`BaselineProvider.GetEffective`).
   - Corta la ventana de 24 h `[t − 24h, t)` y la de 6 h (`SliceWindow`, sin copiar).
   - `WindowEvaluator` calcula con el kernel → **2 filas** (24 h y 6 h). La de 6 h reutiliza la **C de 24 h**.
7. `SaveAll` en **una sola transacción** con `ON CONFLICT DO NOTHING`.
8. **Si falla la lectura o Baseline, aborta sin guardar nada.** Las filas son inmutables: un error transitorio no debe quedar grabado como `NO_CALCULABLE`. El reconciliador reintenta en el siguiente tick.

### 4.3 Estados de cada fila (gana la primera condición que se cumple)

| # | Condición | Resultado |
|---|---|---|
| 1 | Sin línea base vigente | `NO_CALCULABLE` / `NO_SIGNAL_BASELINE` |
| 2 | Señal degenerada (sin `MinElevatedB2`) | `NO_CALCULABLE` / `SIGNAL_DEGENERATE` |
| 3 | Cobertura < 70 % | `NO_CALCULABLE` / `LOW_COVERAGE` |
| 4 | Falta estadística de algún componente | `NO_CALCULABLE` / `NO_COMPONENT_BASELINE` |
| 5 | < 40 registros con las 3 señales en 24 h (no hay C) | `NO_CALCULABLE` / `INSUFFICIENT_COHERENCE_DATA` |
| 6 | Todo bien | `CALCULATED` (z, s(z), I) |

`NO_CALCULABLE` **no es un error**: se guarda siempre, para que el Rule Engine sepa que esa hora existió.

Marcas que no cambian el estado: `low_confidence` (base `LOW`), `silent` (primeros 7 días del dispositivo), `MAD_FLOOR_APPLIED_*`, `CONSTANT_SERIES_*`.

### 4.4 Garantías

- **Idempotente**: índice único `(punto, ventana, hora)` + `ON CONFLICT DO NOTHING`.
- **Todo o nada** por lote.
- **Sin huecos**: tras cada tick del reconciliador, toda hora `≤ floorHour(now − 30 min)` tiene fila (salvo una caída de más de 72 h, que se recupera en los ticks siguientes).
- **Reproducible**: cada fila guarda `baseline_version_id`, `algorithm_version` y `config_version`.
- Un dato que llega tarde **nunca** modifica una evaluación ya hecha.

---

## 5. Cómo se conectan Processing y Baseline

Esta es la parte más interesante de la arquitectura. Processing **necesita** la línea base, pero **no puede importar** el módulo `baseline`. ¿Cómo se resuelve? Con un **puerto + adaptador** (patrón hexagonal de libro):

```
processing/application/ports            internal/bootstrap/processing.go            baseline/
┌──────────────────────────┐          ┌──────────────────────────────────┐        ┌───────────────────────┐
│ BaselineProvider         │◄─implem.─│ baselineProviderAdapter          │─usa───►│ Facade                │
│  GetEffective(ctx,id,at) │          │  toBaselineSnapshot(eb)          │        │  GetEffectiveBaseline │
│  → BaselineSnapshot      │          │  (traducción 1:1, sin caché)     │        │  → EffectiveBaseline  │
└──────────────────────────┘          └──────────────────────────────────┘        └───────────────────────┘
```

- **Processing define lo que necesita** (`BaselineProvider` y su propia entidad `BaselineSnapshot`, armada con tipos del kernel `features.Signal` y `features.ComponentStats`).
- **Baseline expone lo que ofrece** (`Facade` + `EffectiveBaseline`, solo stdlib, contrato congelado).
- **Bootstrap traduce** uno al otro (`toBaselineSnapshot`):
  - `Status == AVAILABLE` → `Available = true`. `NONE` → `Available = false`, y Processing lo convierte en `NO_SIGNAL_BASELINE`.
  - Los punteros `P90Planitud` / `MinElevatedB2` → `HasP90Planitud` / `HasMinElevated`.
  - La lista de componentes → `map[windowHours]ComponentStats`.
  - `DeviceFirstSeenAt` → `FirstSeenAt` (con eso se calcula `silent`).
- **Sin caché**: la consulta está indexada y la base cambia como mucho una vez por día.

**El otro punto de unión es `shared/features`**: Baseline usa las MISMAS fórmulas (`Normalize`, `ComputeComponents`, `Score`, `MADFloor`…) para construir la base que Processing usa para evaluar contra ella. Si cada módulo tuviera su propia fórmula, la base y la evaluación podrían no coincidir. Por eso el kernel está versionado con `AlgorithmVersion`, y ese valor queda guardado en ambas tablas.

**Y el evento de Ingestion** llega igual por bootstrap: `toMeasurementsIngestedNotice` traduce `MeasurementsIngested` (contrato de Ingestion) al `MeasurementsIngestedNotice` propio de Processing (`watermark` → `Watermark`, `batch_seq` → `BatchSeq`, `from` → `From`). Es una capa anticorrupción.

**Orden de arranque obligatorio**: `Platform → Baseline → Processing → Ingestion → Dashboard`. Processing necesita la `Facade` de Baseline, y se tiene que suscribir **antes** de que Ingestion publique, porque un evento sin suscriptor se pierde. Además todo tiene que correr en **el mismo proceso** (el bus es en memoria).

### `cmd/replay`: Processing + Baseline simulados

Herramienta para calibrar con el analista. Es el mismo `EvaluatePending`, pero con otros adaptadores, y **no escribe en la BD** (conexión `read_only`, sin migraciones ni schedulers). La salida es un CSV.

| Modo | Línea base usada en cada hora |
|---|---|
| `asof` | La que **habría estado vigente** en ese momento (`Replayer.SimulateVersions` simula los recálculos con la política real) |
| `fixed` | Una sola versión calculada en `-fixed-asof` (para comparar con el backtest del analista) |

---

## 6. Entrypoints (`cmd/`)

| Binario | Qué levanta |
|---|---|
| `cmd/platform` | **Todo** (default de Docker): schedulers de Baseline, Processing e Ingestion + API Dashboard en `:8080` |
| `cmd/api` | Solo Dashboard |
| `cmd/scheduler` | Ingestion + Processing + scheduler de Baseline |
| `cmd/ingestion` | Una ingesta puntual (rango hardcodeado) + Processing por evento, y sale |
| `cmd/baseline` | Recálculo manual de un punto (`-point -asof [-dry-run] [-json]`) |
| `cmd/replay` | Reproducción histórica de Processing a CSV |

---

## 7. Pendientes reales (verificados en el código hoy)

**Validaciones con el analista (bloqueantes antes de confiar en producción)**

- **Baseline T8**: el golden test (`golden_study_test.go`) se salta solo hasta que exista `testdata/study_11556.csv` + `BASELINE_STUDY_ASOF`. Hoy **no existe** la carpeta `testdata`.
- **Processing P8**: correr `cmd/replay -mode fixed` contra la BD real y comparar con el backtest (E1: I máx ≈ 0,99 y 64 h ≥ 0,70; E5: < 0,70).

**Ingestion**

- `SaveBatch` hace un `Create` plano **sin `ON CONFLICT DO NOTHING`**: una sola muestra duplicada tumba todo el lote. El comentario en `measurement_repository.go:87` dice lo contrario y es engañoso.
- El scheduler pide una ventana fija y no usa `last_ingested_at` para continuar desde donde quedó.
- `INGESTION_README.md` está vacío (0 bytes). La documentación viva de Ingestion hoy está solo en `ARQUITECTURA_LITE.md`.
- `cmd/ingestion/main.go` tiene cambios sin stagear (solo cambió el rango de fechas hardcodeado).

**Comentarios desactualizados en Processing**

- `processing/application/services/commands.go:34` todavía dice que el evento de Ingestion "no transporta watermark". Ya lo transporta.

**Tests de integración**

- Nunca se corrieron los `-tags integration` (Baseline, Processing, Ingestion). El `DATABASE_URL` del `.env` apunta a un Supabase remoto.

**Siguiente módulo**: `rule_engine`. El contrato ya está definido del lado de Processing: rachas solo sobre I(24 h), niveles anidados 0,18 / 0,35 / 0,70, `NO_CALCULABLE` congela las rachas y más de 12 `NO_CALCULABLE` seguidas las reinicia.
