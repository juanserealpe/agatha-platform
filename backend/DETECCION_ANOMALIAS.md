# AGATHA: cómo detectamos anomalías de sonido y movimiento

Estado al 2026-11-21. Piloto con un solo punto de medición (MP-11556). Este documento resume cómo calculamos, cómo lo comunicamos, qué funciona, qué falta y qué riesgos hay. Para el detalle del movimiento ver `MOVIMIENTO_ALERTAS.md`.

---

## 1. Lo primero: detectamos ANOMALÍAS, no fugas ni deslizamientos

Este es el principio que ordena todo lo demás.

El sistema detecta que un sensor se comporta **distinto de lo normal para ese punto**: más ruido del habitual o un cambio de posición. Eso es una **anomalía**. Que detrás haya una fuga o un movimiento de terreno solo lo puede confirmar una verificación en campo.

**El techo de validación.** Para afirmar "esto es una fuga" necesitaríamos muchos eventos confirmados en campo, con fecha y hora, para comparar con lo que vio el sistema. Hoy eso no existe: hay eventos en los que el sistema detecta una anomalía y TGI no confirma qué pasó (ni que sí, ni que no). Sin esa confirmación no podemos medir cuántas veces acertamos ni cuántas falsas alarmas damos. Ese es el techo: **la calidad del sistema no puede demostrarse por encima de la calidad de las confirmaciones que recibe.**

Consecuencias que ya están en el producto:

- Los mensajes nunca dicen "hay una fuga" ni "no es una fuga". Dicen qué cambió y muestran el dato.
- Para el sonido mostramos el índice y lo comparamos con lo normal. Es evidencia, no un veredicto.
- Para la temperatura decimos "pasó lo mismo antes en este punto, pero no está confirmado".
- La clasificación del operador (CONFIRMADA / FALSA_ALERTA + etiqueta) es el único camino para salir de ese techo. Por eso es tan importante que se use.

---

## 2. Qué mide el sensor y qué usamos

El equipo envía 21 variables cada 15 minutos. Usamos siete:

| Variable | Métrica | Uso |
|---|---|---|
| Banda acústica de fuga 1 (`pspec_b1_leak`) | 12 | Sonido: coherencia |
| Banda acústica de fuga 2 (`pspec_b2_leak`) | 13 | Sonido: señal principal |
| Planitud espectral (`pspec_flatness`) | 18 | Sonido: coherencia (dato de análisis) |
| Inclinación pitch (`accel_pitch`) | 1 | Movimiento |
| Inclinación roll (`accel_roll`) | 2 | Movimiento |
| Deriva (`accel_drift_deg_cloud`) | 19 | Movimiento: salto |
| Temperatura (`env_temperature_c`) | 20 | Temperatura de la tubería |

Las otras 14 (batería, humedad, banda de viento, banda de guarda, RMS piezoeléctrico, etc.) se guardan pero no participan en la detección.

---

## 3. Por qué sonido y movimiento usan variables y lógicas distintas

Son fenómenos físicamente distintos, y por eso se miden distinto.

| | Sonido | Movimiento |
|---|---|---|
| **Qué buscamos** | Un ruido sostenido en las frecuencias típicas de un escape | Un cambio de la posición del sensor |
| **Variables** | Bandas del espectro acústico y su planitud | Ángulos pitch y roll en grados, y deriva |
| **Contra qué comparamos** | Contra el **propio historial del punto** (90 días): el ruido de fondo depende del lugar (tráfico, caudal, viento), así que un valor absoluto no significa nada | Contra **valores absolutos en grados**: un grado es un grado en cualquier lugar, y la resolución del equipo es 0,01° |
| **Forma del resultado** | Un índice continuo de 0 a 1 (índice de ruido, I) | Eventos discretos: salto, desplazamiento, retorno, cambio brusco, quieto |
| **Niveles** | Amarillo, Naranja, Rojo | Solo Amarillo |
| **Por qué esos niveles** | La intensidad y la duración del ruido sí permiten graduar | Un movimiento es un hecho (pasó o no pasó); graduarlo exigiría saber cuánto movimiento es peligroso, y eso no está validado |

¿Y la temperatura? No detecta nada por sí sola. Acompaña al movimiento: si el sensor se movió y la temperatura de la tubería dejó de variar como siempre, es un indicio que vale la pena mostrar (en este punto ya pasó antes), pero no está confirmado.

---

## 4. Sonido: cómo se calcula

La cadena tiene cuatro módulos:

```
raw_measurements
   |
   v
BASELINE        "qué es normal en este punto"         (semanal)
   |
   v
PROCESSING      "cuánto se aparta esta ventana"       (cada hora)
   |
   v
RULE ENGINE     "qué nivel es"                        (cada hora)
   |
   v
ALERT MANAGEMENT "qué alerta y qué mensaje"           (cada hora)
```

### 4.1 Baseline: qué es normal en este punto

Con los últimos **90 días** del punto se calcula:

- `median_B2`: la mediana de la banda de fuga 2. Es el ruido típico.
- `theta_D` (θD): el percentil 90 de B2. Por encima de θD, el ruido se considera "elevado".
- `P90_planitud`: el percentil 90 de la planitud. Se usa para calcular `d_pl` (dato de análisis).
- La mediana y la dispersión (MAD) de cada componente del índice (ver 4.2) en las ventanas históricas.

**Limpieza del historial.** La línea base no debe aprender como "normal" un período anómalo. Por eso:

- Se excluyen las horas con índice I ≥ 0,18, en hasta 5 pasadas.
- Se excluyen TODAS las alertas del punto (de cualquier clasificación), con un margen de 2 h antes y después.
- Nunca se excluye más del 40 % del historial.

**Recálculo:** semanal; diario si la confianza es baja.

### 4.2 Processing: el índice de ruido (I)

Cada hora se toma la ventana de las últimas 24 h (96 registros esperados). Si llegó menos del 70 %, la ventana es NO_CALCULABLE.

Se calculan cuatro componentes:

| Componente | Qué mide | Fórmula |
|---|---|---|
| **D**, duración | Cuánto tiempo hubo ruido elevado | % de registros con B2 > θD |
| **P**, persistencia | Si el ruido elevado fue seguido | La racha continua más larga con B2 > θD, en horas (un hueco de más de 20 min corta la racha) |
| **Ex**, exceso | Cuánto ruido de más hubo | Suma de lo que B2 supera a `median_B2`, corregida por cobertura |
| **C**, coherencia | Si las señales acústicas se mueven juntas | Promedio de la correlación de Spearman (en valor absoluto) entre B1, B2 y planitud (mínimo 40 registros) |

Cada componente se compara con su propio historial y se convierte en un valor entre 0 y 1:

```
z = (valor − mediana histórica) / (máx(MAD, piso) × 1,4826)
s(z) = 1 / (1 + e^(−0,6 × (z − 2)))

I = s(P) × [ s(D) + s(Ex) + s(C) ] / 3
```

¿Por qué P multiplica y los demás promedian? Porque una fuga es un ruido **sostenido**. Si el ruido no persiste (s(P) bajo), el índice cae aunque haya picos fuertes. Eso filtra ruidos cortos como un camión que pasa.

Los pisos del MAD evitan que un punto muy estable dispare con variaciones mínimas.

**d_pl (antes "firma de fuga"):** el % de registros de las últimas 24 h con planitud por encima de su percentil 90 histórico. Un escape suele producir un ruido "plano" (de banda ancha, tipo siseo). Es información adicional que Processing sigue calculando y guardando: no entra en I ni en el nivel. (Desde nov 2026 ya no confirma niveles altos: ver 4.3.)

### 4.3 Rule Engine: el nivel de la hora

Umbrales vigentes:

| Nivel | Índice I |
|---|---|
| Amarillo | ≥ 0,16 |
| Naranja | ≥ 0,25 |
| Rojo | ≥ 0,35 |

Reglas:

1. **Apertura del episodio:** 2 horas SEGUIDAS con I ≥ 0,16.
2. **Escalamiento por persistencia:**
   - 12 h en Amarillo o más sube a Naranja.
   - 6 h en Naranja o más sube a Rojo.
   - Sube como máximo un nivel por regla.
3. **Sin confirmación por planitud.** Se eliminó en noviembre de 2026: antes, en el MP-11556, un Naranja o Rojo sin `d_pl` suficiente quedaba en Amarillo. Ahora el nivel de la hora nunca queda por debajo del que marca el índice (la persistencia solo puede subirlo). Histórico en `docs/CAMBIO_CONFIRMACION_PLANITUD.md`.
4. **Cierre del episodio:** 2 horas seguidas con I < 0,14 (Amarillo menos un margen de 0,02). Ese margen evita que la alerta se abra y se cierre cada hora cuando el índice ronda el umbral.
5. **Histéresis:** si el índice baja de Amarillo pero no cumple el cierre, se mantiene el nivel anterior.

Los umbrales están versionados por punto, con vigencia, y nunca se reinterpreta el pasado: cada hora queda con la versión vigente en ese momento.

### 4.4 Alert Management: la alerta de FUGA

Política vigente:

| Regla | Valor |
|---|---|
| Bajar de nivel | 6 h por debajo del corte del nivel actual |
| Rojo sin clasificar | No baja de nivel (sigue en Rojo hasta que el operador lo clasifique) |
| Reactivación | Si vuelve antes de 24 h y la alerta anterior sigue sin clasificar, se reactiva la misma alerta |
| Resumen | Cada 6 h sin cambios |
| Aviso previo de subida | 3 h antes de cambiar de nivel |
| Tendencia | Cambio de 0,02 en 3 h |
| Oscilación | 4 cruces del límite en 6 h: se avisa una vez y no se informa cada cambio |
| Aviso tardío | Una hora que llega con más de 2 h de atraso se marca como tardía |

---

## 5. Movimiento: cómo se calcula (resumen)

Cada hora se evalúa la hora anterior con una semana de datos crudos. Todo nivel horario es la **mediana** de pitch y roll de esa hora.

| Regla | Condición (valores del MP-11556) |
|---|---|
| **Salto** | Deriva ≥ 0,03°, o pitch y roll cambian juntos ≥ 0,03° en 1 h y en el mismo sentido |
| **Desplazamiento sostenido** | 2 h seguidas con los dos ejes a 0,04° o más de la posición base (mediana de hace 24 h a hace 6 h), en el mismo sentido, y con un salto en las últimas 6 h |
| **Desplazamiento por hueco** | Después de 1 h o más sin datos, el sensor aparece en otra posición (0,04° o más en los dos ejes) |
| **Retorno** | Recupera al menos el 50 % del desplazamiento en los dos ejes |
| **Cambio brusco de un eje** | Un solo eje cambia ≥ 0,04° en hasta 3 h, sin salto ni desplazamiento |
| **Quieto** | 24 h con un rango de 0,02° o menos en los dos ejes y sin saltos |

¿Por qué "los dos ejes juntos"? Porque un movimiento real del soporte mueve pitch y roll a la vez. Un solo eje que se corre lento suele ser deriva térmica o mecánica del propio sensor.

**Episodio y alerta:**

- **Abre** con un salto, un desplazamiento nuevo o un cambio brusco.
- **Cierra** cuando el sensor estuvo quieto 24 h y pasaron al menos 24 h desde la apertura.
- Si cierra estando desplazado, la posición nueva pasa a ser la normal.
- Siempre es Amarillo.

**Temperatura** (máquina de estados):

```
VARIABLE --(desplazamiento)--> VERIFICANDO --(temperatura plana)--> ESTABLE
```

Y desde ahí:

- Si el sensor vuelve con la temperatura aún plana, pasa a `RETORNO_TEMP_ESTABLE`.
- Si en 36 h la temperatura no se aplana, pasa a `SIN_ESTABILIZAR`.
- Si la temperatura vuelve a fluir, regresa a `VARIABLE`.

Qué cuenta como plana y qué como fluye:

- **Plana:** al menos 5 cambios horarios en las últimas 6 h, todos menores a 0,03 °C/h.
- **Fluye:** 2 horas seguidas con un cambio de 0,08 °C/h o más.

Los puntos nuevos arrancan en **modo sombra**: se evalúan y se guardan, pero no generan alertas.

---

## 6. FUGA y MOVIMIENTO conviven, no se mezclan

- Son **dos alertas separadas**. Puede haber una de cada una activa a la vez en el mismo punto, pero nunca dos del mismo tipo.
- Cada una tiene sus niveles, sus mensajes y su gestión.
- Lo único que comparten son los mensajes de **temperatura** (`TEMP_*`): salen en la alerta de MOVIMIENTO y se copian a la de FUGA activa, sin cambiarle el nivel ni el estado.

¿Por qué separadas? Porque mezclarlas obliga a decidir cuál "manda" y a explicar colores combinados. Separadas, el operador ve dos hechos independientes y los relaciona él.

---

## 7. Cómo le decimos al usuario lo que pasa

### 7.1 Reglas de redacción

1. **Decimos el estado, nunca qué hacer.** Nada de "enviar cuadrilla" ni "no hace falta salir a campo": la decisión es del operador.
2. **Siempre con el dato.** Cada mensaje cita el valor y lo compara con lo normal. Ejemplo: "Dato: 0,27 (Naranja desde 0,25; lo normal es menos de 0,16)."
3. **Sin veredictos.** No decimos "es una fuga" ni "no es una fuga". Mostramos el dato y lo comparamos con lo normal.
4. **Lenguaje de campo.** No usamos "umbral", "persistencia" ni "I". Decimos "índice de ruido", "límite" y "lo normal".
5. **El título dice qué pasó y el nivel.** El cuerpo agrega el detalle sin repetir el título.
6. **Los avisos se leen de un vistazo.** Banner y push empiezan con una frase simple y terminan con "Dato: …".
7. **Sensor en ajuste.** Si la línea base todavía tiene poca confianza, se agrega "Sensor en ajuste: resultado preliminar."

Cada mensaje se guarda tal como se mostró, con la versión del catálogo (hoy la 7). Un cambio de texto no reescribe el historial.

### 7.2 Tipos de mensaje

| Tipo | Uso | Push |
|---|---|---|
| NOTIFICACIÓN | Algo cambió y requiere atención: abre, sube de nivel, se movió el sensor, la temperatura se estabilizó | Sí |
| ACTUALIZACIÓN | Seguimiento: bajó, sigue igual, sin datos, volvió | No |
| RESUMEN | Cada 6 h sin cambios | No |
| FIN | La alerta terminó | No |
| GESTIÓN | Clasificación y etiqueta del operador | No |

### 7.3 Ciclo de gestión

```
SIN_CLASIFICAR --> CLASIFICADA (CONFIRMADA | FALSA_ALERTA) --> CON ETIQUETA
```

Etiquetas posibles:

- **Si se clasificó como CONFIRMADA:** fuga confirmada en campo, movimiento de tierra, maquinaria ajena u otro.
- **Si se clasificó como FALSA_ALERTA:** clima extremo, tránsito pesado, mantenimiento programado u otro.

Lo que escribe el operador **no se puede modificar después** (lo protege un trigger en la base de datos).

---

## 8. Escenarios

Los valores son ilustrativos y siguen las reglas vigentes del MP-11556.

### Escenario 1. Ruido que crece

| Hora | I | Qué pasa | Mensaje |
|---|---|---|---|
| 08:00 | 0,17 | Primera hora sobre Amarillo, todavía no abre (hacen falta 2) | — |
| 09:00 | 0,19 | Segunda hora seguida: abre | `OPEN/AMARILLO`: "Alerta nueva en {pk}: nivel Amarillo" |
| 10:00 a 17:00 | 0,18 a 0,22 | Sigue en Amarillo | `SUMMARY` a las 6 h; `COUNTDOWN` 3 h antes de subir: "Puede subir a Naranja" |
| 20:00 | 0,21 | 12 h en Amarillo | `UP_NARANJA_PERSIST`: "La alerta ALR-… subió a Naranja" |
| Más tarde | 0,36 | Supera Rojo | `UP_ROJO` |
| ... | < 0,35 | Lleva horas por debajo de Rojo, pero sin clasificar | `ROJO_HELD`: "Sigue en Rojo" |
| El operador clasifica | | | `CLASSIFIED` |
| Después | < 0,14 dos horas | Cierra | `END_NORMAL`: "La alerta ALR-… terminó" |

### Escenario 2. Ruido alto de golpe

El índice llega a 0,28 de una hora a otra. Es Naranja por valor: la alerta abre (o sube) directo a Naranja, sin esperar nada más. Desde nov 2026 no hay confirmación por planitud, así que un ruido fuerte y sostenido (maquinaria, por ejemplo) también escala; la clasificación del operador es lo que lo distingue de una fuga.

### Escenario 3. Índice que ronda el límite

El índice va 0,15, 0,17, 0,15, 0,17… Gracias al margen de salida (0,14) la alerta no se cierra y se vuelve a abrir cada hora. Si cruza el límite 4 veces en 6 h, sale `OSCILLATING` ("El ruido oscila … No se informará cada cambio") y se dejan de mandar avisos por cada cruce.

### Escenario 4. Se cae la transmisión

Con una alerta activa, la ventana queda con menos del 70 % de los datos:

- Al perder datos: `NO_CALC_START`: "Sin datos suficientes".
- Al volver: `NO_CALC_END`: "Vuelve a haber datos".
- La alerta conserva su nivel mientras no hay datos.

### Escenario 5. Vuelve el ruido al día siguiente

La alerta terminó a las 10:00 y el ruido vuelve a las 22:00:

- **Sin clasificar:** se reactiva la misma alerta (`REACTIVATED`).
- **Ya clasificada:** se abre una alerta nueva que cita a la anterior (`NEW_AFTER_CLASSIFIED`: "… la alerta anterior (ALR-…) se clasificó como …").

### Escenario 6. El sensor se mueve de golpe

| Momento | Qué detecta | Mensaje |
|---|---|---|
| 14:00 | Pitch +0,12° y roll +0,10° en 1 h | `MOV_OPEN/SALTO`: "Movimiento brusco del sensor". Alerta de MOVIMIENTO en Amarillo |
| 16:00 | Sigue desplazado, la temperatura entra en verificación | — |
| Día siguiente | La temperatura casi no cambia (máximo 0,015 °C/h) | `TEMP_STABILIZED`: "La temperatura de la tubería se estabilizó … no está confirmado". Si hay una FUGA activa, el mismo mensaje sale también en ella |
| Después | El sensor recupera la posición | `MOV_RETURN`: "El sensor volvió a su posición" |
| 6 h después | La temperatura sigue plana | `TEMP_STILL_STABLE` |
| Más tarde | La temperatura vuelve a fluir | `TEMP_VARIES_AGAIN`: "Estuvo estable 31 h (…)" |
| 24 h quieto | Cierra | `END_NORMAL/MOVIMIENTO`: "Volvió a la normalidad" |

### Escenario 7. El sensor dejó de transmitir y volvió en otra posición

Después de 5 h sin datos, el sensor aparece 0,06° más inclinado en los dos ejes. Sale `MOV_OPEN/HUECO`: "El sensor cambió de posición sin datos … Estuvo 5 h sin transmitir."

### Escenario 8. Un solo eje se mueve

El roll cambia 0,05° en 2 h y el pitch no. Sale `MOV_OPEN/BRUSCO`: "El sensor se movió de golpe hacia un solo lado …".

### Escenario 9. Movimiento lento de los dos ejes

Pitch y roll se corren 0,05° a lo largo de un día, sin ningún salto. **No abre nada.** Un corrimiento lento es normal (dilatación, asentamiento) y queda registrado solo como dato informativo.

### Escenario 10. Ruido y movimiento a la vez

Hay una FUGA en Naranja y el sensor se mueve. Se abre una segunda alerta, de MOVIMIENTO. El operador ve las dos y gestiona cada una por separado.

---

## 9. Lo que tenemos: puntos buenos

- **Cada punto se compara consigo mismo.** El sonido no usa un valor universal: aprende el ruido normal de cada lugar.
- **Todo es reproducible y trazable.** Cada hora queda guardada con sus variables, la versión de umbrales, la versión del algoritmo y su origen (en vivo, puesta al día o histórico). Se puede reconstruir el historial completo y explicar cada mensaje.
- **Lógica pura y probada.** Las reglas están en funciones sin base de datos ni reloj, con pruebas golden contra los notebooks del analista (`motion_ref.py` para el movimiento).
- **Versiones sin reinterpretar el pasado.** Un cambio de umbral rige desde su fecha. Además se puede simular antes de aplicarlo (endpoint `simulate`).
- **Filtros de falsas alarmas:** 2 horas seguidas para abrir, margen de salida, los dos ejes juntos para el movimiento, y el aviso de oscilación.
- **Mensajes honestos.** Muestran evidencia y no dan veredictos ni órdenes.
- **Gestión inmutable.** Lo que clasifica el operador no se puede reescribir.
- **Resiliencia.** Las horas perdidas se recuperan solas (puesta al día cada hora), los eventos son idempotentes y los avisos tardíos se marcan.
- **Modo sombra.** Permite instalar un punto nuevo sin generar alertas mientras se observa.
- **Módulos separados.** Cada etapa tiene una sola responsabilidad. Alert Management no envía notificaciones: publica un evento y otro módulo lo entrega.

---

## 10. Puntos críticos

| Punto | Por qué importa |
|---|---|
| **Techo de validación (TGI)** | Sin confirmaciones de campo no se puede medir la precisión ni la tasa de falsas alarmas. Todo umbral es una hipótesis hasta que haya casos confirmados |
| **Un solo dispositivo** | Los umbrales (0,16 / 0,25 / 0,35, los 0,03° y 0,04°) se ajustaron con un punto y pocos eventos. Pueden no servir en otro lugar |
| **Pocos eventos de referencia** | Hay un puñado de eventos conocidos en el MP-11556. Es poco para afirmar que la temperatura estable significa algo |
| **Línea base contaminada** | Si las anomalías cubren más de alrededor del 10 % del historial, θD sube y la exclusión deja de funcionar. Una anomalía con pérdida de datos tampoco se excluye |
| **Cobertura de datos** | Con menos del 70 % de datos no se calcula. Una pérdida nocturna larga y recurrente deja la línea base en confianza baja para siempre |
| **Hora del proveedor** | Dataflow Labs envía la hora local de Colombia marcada como UTC. Está corregido al recibir, pero los datos guardados antes de la corrección tienen 5 h de diferencia |
| **Reconstrucciones** | Un rebuild con el backend en vivo corriendo arruina el resultado. Hay que detener el backend antes |
| **Seguridad** | Las APIs de umbrales no tienen autenticación: `created_by` viaja en el cuerpo de la petición |
| **Pruebas contra PostgreSQL real** | Las pruebas de integración nunca corrieron contra una base real (la única es la de Supabase del piloto) |
| **Movimiento solo Amarillo** | No hay forma de distinguir un movimiento grave de uno leve. Es correcto mientras no haya datos para graduar |

---

## 11. Qué mejorar ahora (corto plazo, sin cambiar el modelo)

1. **Que se clasifiquen las alertas.** Es la mejora con más impacto, porque es lo único que levanta el techo de validación. Conviene pedirle a TGI un registro mínimo por intervención en campo: fecha, hora, punto y qué encontraron.
2. **Medir la tasa de falsas alarmas** con las clasificaciones que ya existen, por tipo de alerta y por nivel.
3. **Excluir de la línea base las horas sin cobertura** que caen dentro de anomalías, para que no la contaminen.
4. **Corregir el desfase del recálculo** de la línea base: hoy se corre 1 h en cada recálculo.
5. **Corregir los datos guardados antes del arreglo de la hora** (+5 h) si se van a usar para análisis.
6. **Autenticación** en las APIs de escritura.
7. **Correr las pruebas de integración** contra una base de pruebas separada del piloto.

---

## 12. Qué escalar después (cuando haya más dispositivos)

| Tema | Propuesta |
|---|---|
| **Terrenos distintos** | Perfiles de terreno (estable, moderado, activo) con sus umbrales. Un punto nuevo hereda un perfil y solo se ajusta lo necesario |
| **Calibración** | Usar el modo sombra: después de unas 2 semanas, el sistema sugiere el perfil según cómo se comportó el punto. Una persona revisa con `simulate` y aprueba |
| **Techo de seguridad** | Un movimiento grande alerta siempre, sea cual sea el perfil. Un terreno inquieto puede ser justamente el riesgo |
| **Umbrales relativos para movimiento** | Comparar cada punto con su propia variabilidad, como ya hace el sonido. Requiere una versión nueva del algoritmo; solo si los perfiles no alcanzan |
| **Variables no usadas** | Evaluar si la banda de viento (b0) o la de guarda (b3) ayudan a descartar falsas alarmas de sonido (viento, lluvia) |
| **Contexto externo** | Lluvia, mantenimiento programado y obras cercanas, para explicar anomalías antes de alertar |
| **Correlación entre puntos** | Si varios puntos cercanos se mueven a la vez, es más probable que sea un evento del terreno que un sensor aislado |
| **Rendimiento** | Hoy un rebuild del historial de un punto tarda más de 2 h. Con decenas de puntos hará falta paralelizar por punto |
| **Mensajes con límites por punto** | Si cada punto tiene sus umbrales, los textos que citan "lo normal" deberían tomar el valor del punto en vez de un número fijo |

---

## 13. Glosario

| Término | Significado |
|---|---|
| Anomalía | Comportamiento distinto de lo normal para ese punto. No implica una causa |
| Índice de ruido (I) | Número entre 0 y 1 que resume cuánto se aparta el sonido de lo normal en 24 h |
| d_pl (antes "firma de fuga") | % de registros de 24 h con la planitud del sonido alta. Se calcula y se guarda como dato de análisis, pero desde nov 2026 no influye en el nivel ni en los mensajes |
| Línea base | Lo que es normal en un punto, calculado con 90 días |
| Episodio | Período en que el Rule Engine considera la señal anómala. Se traduce en una alerta |
| Pitch / roll | Inclinación del sensor en sus dos ejes |
| Desplazamiento | El sensor cambió de posición y se quedó ahí |
| Retorno | El sensor volvió a su posición anterior |
| Modo sombra | El punto se evalúa pero no genera alertas |
| Reactivación | Una alerta terminada vuelve a activarse si la anomalía reaparece antes de 24 h y no se clasificó |
