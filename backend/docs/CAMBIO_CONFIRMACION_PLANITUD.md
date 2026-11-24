> Retirado (2026-11-24): la confirmación por planitud se eliminó del código; este documento queda como histórico.

# Cambio: confirmación por planitud

Noviembre 2026 · Rule Engine y Alert Management · solo backend

## Resumen

Antes, el Rojo por valor no pasaba por la planitud: el índice I ≥ 0,35 llevaba la alerta a Rojo aunque el ruido no tuviera firma de fuga.

Ahora, en los puntos que activan la opción nueva, la alerta **solo pasa de Amarillo si el ruido tiene firma de fuga**. Esto vale para cualquier nivel desde Naranja, sea por valor o por duración.

Hay tres cambios más:

- Los mensajes muestran el número de la firma de fuga y no dicen si el evento es o no una fuga.
- La web y la app reciben `leak_signature` para mostrar ese dato.
- Hay un comando para aplicar la regla al histórico de un punto.

**Firma de fuga:** porcentaje de registros de las últimas 24 h con planitud alta (`d_pl`). Se compara con el mínimo de la versión vigente (`d_pl_min`, 10 % para el MP-11556).

## 1. Antes y ahora

| Tema | Antes | Ahora |
| --- | --- | --- |
| Rojo por valor (I ≥ 0,35) | Siempre Rojo, aunque la firma de fuga estuviera por debajo del mínimo | Con `confirm_rojo_with_planitud = true`, queda en Amarillo si la firma no llega al mínimo |
| Naranja y Rojo por duración | Con `confirm_naranja_with_planitud = true`, quedaban en Amarillo sin firma | Igual que antes |
| Motivo de la limitación | `NARANJA_NOT_CONFIRMED`, con `from` = `NARANJA` | Mismo código; `from` ahora también puede ser `ROJO` |
| Evento `rule_engine.level_evaluated` | v3: `d_pl` en cada hora; `d_pl_min` solo dentro del motivo | v4: `d_pl` y `d_pl_min` en cada hora calculada |
| Mensajes al operario | Catálogo v5: no mencionaban la firma de fuga | Catálogo v6: citan la firma (por ejemplo, 4,6 %) frente al mínimo (10 %), junto al índice frente a su corte |
| Veredictos en los mensajes | Sin una regla que los prohibiera | Una prueba (`TestCatalog_NeverGivesALeakVerdict`) prohíbe "es una fuga", "no es una fuga", "parece" y "forma de fuga" |
| `/api/alerts` y `alert_changed` | Sin el dato de la firma | Campo `leak_signature` con `d_pl`, `d_pl_min` y `confirmed` |
| Aplicar una versión al pasado | Imposible: las versiones solo rigen desde su creación | Con `-reset-versions` se aplica al histórico de un punto, fuera de production |

## 2. Rule Engine

### 2.1 Opción `confirm_rojo_with_planitud`

- Es una columna nueva en `rule_threshold_versions` (`BOOLEAN NOT NULL DEFAULT false`).
- Solo puede estar en true si `confirm_naranja_with_planitud` también lo está. La base lo exige con un `CHECK`, y la API rechaza la combinación inválida con 400.
- Se acepta en el `patch` de `POST /thresholds`, `/thresholds/bulk` y `/thresholds/simulate`, y aparece en la respuesta de cada versión.
- Las versiones existentes quedan en false y se comportan igual que antes.

### 2.2 Regla en `Assess`

```text
si confirm_naranja_with_planitud y nivelCalculado >= NARANJA
   y (d_pl es NULL o d_pl < d_pl_min)
   y (raw != ROJO o confirm_rojo_with_planitud):
      nivelCalculado = AMARILLO
      motivo NARANJA_NOT_CONFIRMED {from, d_pl, d_pl_min}
```

- La persistencia se sigue contando sobre el `raw_level` guardado. Si en una hora posterior la firma llega al mínimo, esa hora recupera su nivel, incluido el Rojo por duración acumulada.
- Un `d_pl` NULL cuenta como firma no confirmada.

### 2.3 Comando `-reset-versions`

```bash
go run ./cmd/rule_engine -backfill -rebuild -reset-versions -point <id> -confirm <id>
```

- Toma la versión vigente del punto y la convierte en la única versión 1, con `valid_from` en la primera medición. Después recalcula todo el histórico del punto con esa versión.
- El borrado de evaluaciones y de versiones, la creación de la versión 1 y el recálculo corren en una sola transacción. Si algo falla, no se aplica nada.
- Se niega a correr en estos casos:
  - falta `-rebuild` o falta `-point`;
  - `-confirm` es distinto de `-point`;
  - se pasa `-from` o `-to`;
  - el punto tiene versiones programadas a futuro;
  - `AGATHA_ENV=production`.
- Es la segunda excepción a la regla de que las versiones no son retroactivas. La primera es la versión 1 sembrada al inicio.

## 3. Alert Management (catálogo v6)

### 3.1 Mensajes nuevos

| Código | Tipo | Cuándo | Título | Cuerpo |
| --- | --- | --- | --- | --- |
| `OPEN`, variante `SIN_FIRMA` | Aviso | La alerta abre en Amarillo porque la firma no llegó al mínimo | Alerta nueva en {pk}: nivel Amarillo | Por el índice de ruido la alerta estaría en {from} desde las {hora_inicio}, pero solo el {d_pl} de los registros de las últimas 24 h tiene firma de fuga (se necesita {d_pl_min}). Dato: {valor} ({from} desde {corte}; lo normal es menos de {normal}). |
| `NO_LEAK_SHAPE` | Actualización en la línea de tiempo (no es push) | Una hora de una alerta abierta queda limitada a Amarillo; sale una vez por racha | Se mantiene en Amarillo | Por el índice de ruido la alerta estaría en {from}, pero solo el {d_pl} de los registros de las últimas 24 h tiene firma de fuga (se necesita {d_pl_min}). Dato: {valor} ({from} desde {corte}). |
| `UP_*`, variante `CONFIRMADA` | Aviso | El nivel sube y la hora anterior estaba limitada | Igual que en v5 | Empieza con "El {d_pl} de los registros de las últimas 24 h tiene firma de fuga (mínimo {d_pl_min})." y sigue con el texto de v5 |

Ejemplo con la alerta del 13 nov, a las 12:00 del 14 nov:

> **Se mantiene en Amarillo**
> Por el índice de ruido la alerta estaría en Rojo, pero solo el 4,6 % de los registros de las últimas 24 h tiene firma de fuga (se necesita 10 %). Dato: 0,36 (Rojo desde 0,35).

Antes, esa misma hora mostraba la alerta en Rojo y el mensaje no decía nada de la firma.

Reglas del catálogo:

- Los mensajes no dicen qué hacer (salir a campo, revisar, verificar, enviar).
- No dicen si el evento es o no una fuga.
- El cuerpo no repite el título.
- Cada número va junto al límite que lo explica.

### 3.2 Cómo se evita que `NO_LEAK_SHAPE` se repita

- Una racha es una serie de horas calculadas seguidas que quedaron limitadas. `NO_LEAK_SHAPE` sale una sola vez por racha.
- Una hora calculada sin limitación termina la racha. Una hora sin cálculo no la termina.
- Si la alerta abrió limitada, el `OPEN` de la variante `SIN_FIRMA` cuenta como el mensaje de esa racha.

### 3.3 Campo `leak_signature`

```json
"leak_signature": { "d_pl": 4.6, "d_pl_min": 10, "confirmed": false }
```

- Describe la última hora calculada de la alerta. `confirmed` es true si esa hora no se limitó y `d_pl ≥ d_pl_min`.
- Vale `null` si la versión del punto no tiene `d_pl_min`, es decir, si la regla no está activa.
- Se guarda en tres columnas nuevas de `alerts`: `leak_d_pl`, `leak_d_pl_min` y `leak_confirmed`. Las alertas existentes quedan en NULL.
- Se expone en la fila y el detalle de `/api/alerts` y en `alert_management.alert_changed`. El esquema de `alert_changed` sigue en la versión 1, porque el cambio es aditivo.
- Si cambia `confirmed` pero no cambia el nivel, no se publica un `alert_changed` aparte.

### 3.4 Reconstrucción

`alert_management -backfill -rebuild` vuelve a generar los mensajes con el catálogo v6 y **no publica avisos**. Antes de correrlo hay que saber que borra la gestión del operador: clasificaciones, observaciones y estado.

## 4. Lo que no cambió

- La ecuación del índice I y el módulo Processing.
- Los cortes (0,16 / 0,25 / 0,35), la persistencia (12 h y 6 h) y la salida (0,14 durante 2 h).
- Cuándo se abre y se cierra un episodio: la planitud solo limita el color de la alerta.
- Las reglas de Alert Management: bajar tras 6 h, retener el Rojo sin clasificar, la reactivación y la gestión.
- Los puntos sin la opción nueva activa se comportan exactamente igual que antes.
- Baseline e Ingestion.

## 5. Aplicación al MP-11556

1. Desplegar el código. La migración agrega las columnas nuevas.
2. Crear la versión del punto por la API con este patch:

   ```json
   { "confirm_naranja_with_planitud": true, "d_pl_min": 10, "confirm_rojo_with_planitud": true }
   ```

3. Validar con `simulate`: la fuga de agosto debe seguir en Naranja a las 16:00 del 24 ago y en Rojo a las 22:00, y la alerta del 13 nov debe quedar como máximo en Amarillo.
4. Hacer un respaldo de la base (`pg_dump`).
5. Exportar la gestión del punto con `GET /api/alerts/export.xlsx`, en especial la fuga confirmada de agosto.
6. Correr `rule_engine -backfill -rebuild -reset-versions -point <id> -confirm <id>`.
7. Correr `alert_management -backfill -rebuild -point <id>`.
8. Comprobar los episodios contra la simulación del requerimiento.
9. Volver a cargar la gestión exportada.

**Reversa:** restaurar el respaldo del paso 4. Crear otra versión ya no alcanza, porque el histórico quedó reescrito.

## 6. Estado y pendientes

- Las pruebas unitarias pasan en Rule Engine, Alert Management, bootstrap y notifications.
- **El SQL real no se probó**: ni la migración de las columnas nuevas ni el reset de versiones. Hay que probarlo primero en una copia restaurada de la base y después correrlo contra el piloto.
- La web y la app todavía no muestran `leak_signature`. Para mostrarlo en tiempo real hace falta un nuevo tipo de `alert_changed` (`LEAK_SIGNATURE_CHANGED`).
- El mínimo de 10 % está calibrado con una sola fuga confirmada. Hay que revisarlo cuando haya más eventos reales.
