# AGATHA — frontend

Aplicación web para monitorear en tiempo real el gasoducto Otero–Santana: mapa del piloto, alertas en vivo, gestión de alertas e historial. React 19 + Vite + TypeScript, desplegada en Vercel.

## Contexto

Sobre el ducto hay sensores instalados que miden el sonido de la tubería, el movimiento del terreno, las condiciones ambientales y la energía del equipo. El backend procesa esas mediciones cada hora y calcula un **índice de ruido**, que indica qué tan distinto suena el ducto frente a su comportamiento habitual en ese punto. Cuando el índice se mantiene alto, el backend abre una **alerta** con un nivel de criticidad: **Amarilla**, **Naranja** o **Roja**.

Una alerta acústica se llama **anomalía acústica** y no **fuga**, porque un sonido anómalo no siempre es una fuga: también lo producen la maquinaria, el tránsito pesado o el clima. Quien confirma si hubo una fuga es el operador, al clasificar la alerta.

El frontend no calcula niveles ni decide nada: muestra lo que entrega el backend y permite que el operador gestione cada alerta.

- **Usuarios**: operadores de la sala de control y analistas del piloto.
- **Alcance del piloto**: el trazado del ducto y la posición de los sensores son datos fijos del piloto (`src/features/*/data/`). El estado de las alertas viene del backend en tiempo real.
- **Backend**: la API de alertas está descrita en `backend/internal/modules/alert_management/ALERT_MANAGEMENT_API.MD`, y el stream de tiempo real en `backend/internal/modules/notifications/README.md`.

## Cómo usar la plataforma

La aplicación tiene dos secciones, en las pestañas bajo el encabezado: **Centro de Control** y **Historial de Alertas**.

### Encabezado: avisos en tiempo real

- **Indicador con campana**: muestra si los avisos de alertas llegan en tiempo real. Con **EN VIVO**, una alerta nueva aparece al instante. Con **Reconectando** o **Sin conexión**, la app sigue consultando el backend cada 30 s. Este indicador **no muestra el estado de los sensores**: eso se ve en el mapa y en el panel de sensores.
- **Activar avisos**: pide permiso para mostrar notificaciones del sistema cuando llega una alerta con la pestaña oculta. Se recomienda usar Chrome o Edge, con las notificaciones de Windows permitidas.
- **Altavoz**: activa o silencia el sonido que suena cuando llega una alerta nueva o una alerta sube de nivel.

### Centro de Control (`/monitoreo`)

1. **Mapa**: muestra el trazado del ducto y cada sensor con un color: verde (normal), amarillo, naranja o rojo (nivel de la alerta activa) y gris (sin comunicación o no operativo). La leyenda explica cada color. Al hacer clic en un sensor se abre su detalle, desde donde se puede ir a su telemetría.
2. **Aviso de alerta nueva**: aparece arriba a la derecha del mapa. Al hacer clic se abre la alerta.
3. **Panel de alertas**:
   - **Sin clasificar** (pestaña por defecto): alertas que esperan la gestión del operador. Una alerta Roja sin clasificar titila.
   - **Activas**: todas las alertas en curso, aunque ya estén clasificadas, porque todavía pueden subir de nivel.
4. **Panel de sensores**: lista los sensores, con un buscador por nombre o PK. Al seleccionar uno se cargan sus gráficas.
5. **Telemetría**: gráficas del sensor seleccionado por secciones (Anomalía acústica, Movimiento, Ambiental y Energía), con un rango de fechas personalizable. Cada gráfica explica qué mide y cómo leerla.

### Gestionar una alerta

Al abrir una alerta se ven sus datos, la explicación de por qué se generó y su línea de tiempo.

- **¿Por qué se generó esta alerta?**: explica el tipo de alerta, cuánto se aleja la medición de lo normal y. El **nivel de anomalía** es un puntaje, no una probabilidad.
- **Línea de tiempo**: los tramos de cada nivel y sus mensajes. Una **Notificación** marca un cambio importante que se avisó al operador (la alerta abrió, subió de nivel o la temperatura de la tubería cambió de comportamiento). Una **Actualización** registra un cambio de estado sin aviso, por ejemplo una bajada de nivel o una hora sin datos. Al pasar el cursor sobre el tipo de mensaje aparece su significado.
- **Paso 1. Clasificación (obligatoria)**: **Confirmada**, si el evento ocurrió o se verificó en campo, o **Falsa alerta**. La app pide confirmar antes de guardar.
- **Paso 2. Causa y etiquetado (opcional)**: la causa del evento, entre las opciones que corresponden a la clasificación elegida. "Otro" permite escribirla.
- **Observaciones**: notas libres sobre la alerta.

El enlace de la página abre la misma alerta (parámetro `?alerta=`), así que se puede compartir.

### Historial de Alertas (`/historial`)

- **Filtros**: dispositivo, fecha de inicio y de fin, nivel de criticidad, evento y estado de gestión. Los filtros quedan en la URL: al recargar o compartir el enlace se conservan.
- **Tabla**: cada fila muestra la alerta, el dispositivo, el inicio y el fin, el nivel actual y el máximo, el evento, el estado y la etiqueta. Los botones de **Acción** dependen del estado: clasificar, etiquetar o ver.
- **Exportar Excel**: descarga las alertas que cumplen los filtros aplicados.

## Desarrollo

### Comandos

```bash
npm install
npm run dev       # http://localhost:5173 (proxy de /api al backend, ver vite.config.ts)
npm test          # vitest
npm run lint
npm run build     # tsc + vite build en dist/
npm run preview   # sirve dist/ en http://localhost:4173
```

Antes de cada commit deben pasar `npm run lint`, `npm test` y `npm run build`.

### Variables de entorno

Ver [.env.example](.env.example): `VITE_API_BASE_URL`, `VITE_ENABLE_DEMO`, `VITE_FEATURE_ALERT_EVIDENCE` y `LOCAL_NOTIFICATIONS_URL` (solo en desarrollo).

### Estructura

```
src/
  app/         router, providers y layout raíz
  pages/       una carpeta por ruta (monitoring, history)
  features/    módulos por dominio: alerts, map, sensors, telemetry
  services/    acceso a datos: tipos del contrato y adaptadores HTTP
  shared/      piezas reutilizables sin conocimiento del dominio
  components/  ui/ (shadcn) y layout/
  demo/        datos simulados de /demo
```

Las reglas de capas, estilo y datos están en [CLAUDE.md](CLAUDE.md). El estado de las pruebas y el plan de las siguientes están en [TESTING.md](TESTING.md).

## Demo en `/demo`

El mismo despliegue sirve una demo con datos simulados en `/demo` (por ejemplo `/demo/monitoreo`). Nunca llama al backend: alertas, stream de tiempo real, dispositivos y series salen de un store en memoria (`src/demo/`) que responde con la misma forma que la API. Su código va en un chunk aparte que solo se descarga en `/demo`.

- Una franja indica "DEMO — datos simulados"; las notificaciones del navegador llevan el prefijo `[SIMULACIÓN]`.
- El panel **Simulación** (abajo a la izquierda, plegable) tiene:
  - **Escenario guiado** sobre el Punto 1 (PK 34+470): Amarilla → Naranja → Roja (titila) → clasificar → Normal. "Siguiente paso" lo aplica de inmediato; "Disparar en 5 s" da tiempo de cambiar de pestaña para ver la notificación del navegador.
  - **Acciones sueltas**: alerta nueva (en un punto libre), subir nivel y clasificar (sobre la alerta activa más reciente), desconectar/reconectar el stream. Al reconectar, el cliente reintenta con su backoff normal (puede tardar hasta 30 s) y recibe lo que se perdió.
  - **Reiniciar la demo**: recarga la página y todo vuelve al inicio.
- "Salir de la demo" vuelve a la app real en `/`.
- La exportación a Excel no está disponible en la demo.
- `VITE_ENABLE_DEMO=false` apaga la ruta: `/demo` redirige a la app real.
