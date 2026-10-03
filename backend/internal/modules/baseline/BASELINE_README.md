# Modulo: Baseline (Device Context & Baseline)

**Responsabilidad**: mantener, por dispositivo y **versionada en el tiempo**, la línea base estadística de su comportamiento normal, y exponerla con una consulta síncrona (`Facade.GetEffectiveBaseline`) que consumirá Processing.

- **Señal**: `mediana_B2`, `θ_D = P90_B2`, `P90_Planitud`, `min_elevado_B2`, `Δmin`.
- **Componentes**: mediana y MAD crudo de D, P, Ex (ventanas 6 h y 24 h) y C (solo 24 h).

Es **autosuficiente**: solo lee `raw_measurements` y `devices` (escritor: Ingestion). No publica ni consume eventos. Todo en **UTC** (`America/Bogota` solo en logs).

## Estructura

```
baseline/
├── facade.go / types.go              # Contrato público (solo stdlib). CONGELADO para Processing
├── domain/
│   ├── errors.go
│   ├── entities/                     # BaselineVersion, SignalBaseline, ComponentStat, ExcludedPeriod,
│   │                                 # ExclusionSet, EstimationSummary, PassReport, enums
│   ├── services/                     # Servicios puros (sin IO) + tests
│   └── repositories/                 # Ports: HistoryReader, BaselineRepository
├── application/services/             # UC-B1/UC-B2 RecalculationService, UC-B3 QueryService
└── infrastructure/
    ├── persistence/                  # models (GORM), mappers, repositories (+ tests integration)
    └── scheduling/recalc_scheduler.go
```

## Contrato público (Facade)

```go
GetEffectiveBaseline(ctx, measuringPointID, at) (EffectiveBaseline, error)
```

| Campo | Notas |
|---|---|
| `Status` | `AVAILABLE` \| `NONE`. **Ausencia no es error**: `NONE` + `DeviceFirstSeenAt` |
| `VersionID`, `Version`, `ValidFrom`, `Confidence` (`NORMAL`\|`LOW`), `AlgorithmVersion` | Versión con `valid_from <= at < valid_to` |
| `Signal *SignalStats` | `MedianB2`, `ThetaD`, `P90Planitud *`, `MinElevatedB2 *` (nil ⇒ `SIGNAL_DEGENERATE`), `DeltaMin` |
| `Components []ComponentStats` | `WindowHours` (6\|24), `Component` (`D`\|`P`\|`EX`\|`C`), `Median`, `MAD` (crudo), `NWindows` |
| `Flags []string` | Orden canónico, ver abajo |

## Casos de uso

| ID | Servicio | Invocado por |
|---|---|---|
| UC-B1 | `RecalculationService.Recalculate(RecalculateCommand{PointID, AsOf, Trigger, DryRun})` | UC-B2, `cmd/baseline` |
| UC-B2 | `RecalculationService.RecalculateDue(now)` — secuencial; un punto que falla no detiene al resto | `RecalcScheduler` |
| UC-B3 | `QueryService.GetEffective(pointID, at)` | `Facade` |

UC-B1: valida punto → `GetSamples([asOf − 90 d, asOf))` → `features.Normalize` → `RobustBaselineEstimator` → `INSUFFICIENT_HISTORY` no persiste (log `Info` solo si cambia el motivo) → `DryRun` no persiste → `SaveNewVersion` (conflicto concurrente = no-op).

## Algoritmo (resumen)

1. **Estimación iterativa** (máx. 5 pasadas): calcula la base con el conjunto excluido `E_{k−1}`, barre I(24 h) hora a hora con esa base, marca `I ≥ 0,18`, convierte marcas en `[t − 24 h, t)` y repite hasta que el **conjunto de horas marcadas** no cambie. Se persiste siempre el `E` con el que se calculó la base guardada.
2. **Señal** (sobre registros fuera de `E` con B2 válido; mínimo 96).
3. **Componentes**: ventanas 6 h (fin cada 3 h) y 24 h (fin cada 6 h) alineadas a UTC, dentro del rango; se descartan las que solapan `E` o con cobertura < 70 %. Combinación con < 20 ventanas no se guarda.
4. **Exclusión**: marcas por I descendente (empate: t ascendente) hasta 40 % del histórico.
5. **Barrido horario**: horas alineadas a UTC `t ∈ [history_from + 24 h, history_to]` (cerrado en ambos extremos: la ventana que termina en `history_from + 24 h` es `[history_from, history_from + 24 h)` y está completa; con 90 días alineados son 24 × 90 − 23 = 2 137 horas), ventana `[t − 24 h, t)` sobre todos los samples (dos punteros). El spec original decía `(history_from + 24 h, …]`: era un error.

**Decisiones de implementación no explícitas en el spec**:
- `Confidence = NORMAL` exige las **7 combinaciones** (6 h: D, P, EX; 24 h: D, P, EX, C) con ≥ 60 ventanas. Si falta alguna ⇒ `LOW` (sin ella Processing no puede calcular I).
- "Sin estadísticas 24 h" = falta cualquiera de 24 h D, P, EX o C.
- `Δmin` cuando B2 tiene un solo valor distinto ⇒ `1` (resolución práctica de B2).
- Piso de Ex con señal degenerada ⇒ `Δmin`.
- Una nueva versión con `valid_from <=` al de la vigente se rechaza (`PERSISTING_BASELINE`): el versionado no es retroactivo.

## Flags

`SIGNAL_DEGENERATE`, `SIGNAL_LOW_VARIABILITY`, `NO_EXCLUSION_HISTORY`, `EXCLUSION_CAP_REACHED`, `BASELINE_REVIEW` (orden canónico). Triggers: `BOOTSTRAP`, `SCHEDULED`, `LOW_CONFIDENCE`, `MANUAL`.

## Política de recálculo (`RecalculationPolicy.IsDue`)

| Condición | Trigger |
|---|---|
| Sin versión | `BOOTSTRAP` |
| `LOW` o `NO_EXCLUSION_HISTORY` y `now − created_at ≥ 24 h` | `LOW_CONFIDENCE` |
| `now − created_at ≥ 168 h` | `SCHEDULED` |

`RecalcScheduler`: una ejecución al arrancar + tick horario en el minuto `BASELINE_TICK_MINUTE` (UTC), timeout 10 min por ejecución.

## Persistencia

Tablas creadas por **`AutoMigrate`** en `NewBaselineModule` (los tags del modelo generan también el índice parcial, el de vigencia, la FK y los CHECKs):

- `baseline_versions` — una fila por versión. `UNIQUE (measuring_point_id, version)`; `ux_baseline_versions_current` = índice único parcial `WHERE valid_to IS NULL` (**una sola vigente por punto**); `ix_baseline_versions_effective (measuring_point_id, valid_from DESC)`. `excluded_periods` JSONB `[{from, to, peak_i}]`, `flags` TEXT[]. Sin FK a `devices`.
- `baseline_component_stats` — PK `(baseline_version_id, window_hours, component)`, FK a `baseline_versions(id)`, MAD crudo (sin 1,4826 ni piso).

`SaveNewVersion` (una TX): `SELECT vigente FOR UPDATE` → `valid_to = ValidFrom` → `INSERT` versión `MAX(version)+1` + stats. SQLSTATE `23505` ⇒ `ErrBaselineConcurrentUpdate` (no-op).

Lecturas: `raw_measurements` (métricas 12/13/18, una consulta por punto, pivote en el mapper; NaN/±Inf ⇒ ausente) y `devices WHERE active = true` (la tabla de puntos de medición en código se llama `devices`).

## Errores (`domain/errors.go`)

| Error | Código | Cuándo |
|---|---|---|
| `ErrInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | `pointID ≤ 0` |
| `ErrConsultingHistory` | `CONSULTING_MEASUREMENTS` | Falla `GetSamples` / `GetFirstSeenAt` / `ListActive…` |
| `ErrConsultingBaseline` | `CONSULTING_BASELINE` | Falla `GetEffective` / `GetLatest` |
| `ErrPersistingBaseline` | `PERSISTING_BASELINE` | Falla `SaveNewVersion` |
| `ErrBaselineConcurrentUpdate` | `CONSULTING_BASELINE` | Otro proceso creó la vigente (no-op) |

No son errores: `INSUFFICIENT_HISTORY`, `Status = NONE`, flags. El error crudo de GORM se loguea en el servicio y nunca sale del módulo.

## Configuración (entorno)

| Variable | Default |
|---|---|
| `BASELINE_METRIC_B1` / `_B2` / `_PLANITUD` | 12 / 13 / 18 |
| `BASELINE_HISTORY_DAYS` | 90 |
| `BASELINE_MIN_SIGNAL_RECORDS` | 96 |
| `BASELINE_MIN_COMPONENT_WINDOWS` | 20 (60 para `NORMAL` es fijo) |
| `BASELINE_EXCLUSION_I_CUTOFF` | 0.18 (tope 40 % fijo) |
| `BASELINE_MAX_PASSES` | 5 |
| `BASELINE_RECALC_INTERVAL` / `BASELINE_LOW_RECALC_INTERVAL` | `168h` / `24h` |
| `BASELINE_TICK_MINUTE` | 10 |

Un valor inválido se ignora con `Warn` y se usa el default.

## Aislamiento

`depguard` (`baseline-isolation`): no importa `modules/ingestion`, `modules/processing` ni `shared/eventbus`. Solo `shared/{logger, errors, features}`. Processing consumirá la fachada a través de **su propio port** y un adapter en `internal/bootstrap`.

## Cableado y entrypoints

`bootstrap.NewBaselineModule(platform)` → `BaselineModule{Facade, Recalculator, NewScheduler}`.

| Entrypoint | Baseline |
|---|---|
| `cmd/platform` | Se compone después de `NewPlatform` y antes de Ingestion; su scheduler arranca antes que el de Ingestion y se espera antes de `Platform.Close` |
| `cmd/scheduler` | Vía `NewIngestionApplication()`; arranca su scheduler |
| `cmd/ingestion` | Se compone (migra) pero no arranca el scheduler |
| `cmd/baseline` | `baseline -point 11556 -asof 2026-07-01T00:00:00Z [-dry-run] [-json]` — validación con el analista (pasadas, θ_D, % excluido, ventanas, periodos excluidos) |

## Tests

- `go test ./internal/modules/baseline/...` — dominio, aplicación (fakes), mappers, scheduler, fachada; incluye rendimiento (90 días < 5 s; se omite con `-short`).
- `go test -tags integration ./internal/modules/baseline/infrastructure/persistence/repositories/` con `DATABASE_URL` — migración/índice parcial, versionado, concurrencia, pivote.
- **Golden del estudio (15.5, bloqueante para T8)**: `application/services/golden_study_test.go`. Se activa al poner `testdata/study_11556.csv` (timestamp RFC3339, B1, B2, Planitud) y `BASELINE_STUDY_ASOF`. Espera θ_D = 67, mediana 54, ≈ 21,8 % ± 0,5 pp, 4 pasadas, sin tope.
