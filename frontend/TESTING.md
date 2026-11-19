# Pruebas del frontend de Agatha

Estado de las pruebas del frontend (React + TypeScript + Vite) al 18 de noviembre de 2026, y plan de las que siguen.

## Resumen

| Indicador | Valor |
|---|---|
| Tipo de pruebas actual | Unitarias (Vitest) |
| Archivos de prueba | 42 |
| Casos de prueba | 387 |
| Resultado | 387 de 387 pasan |
| Tiempo de ejecución | ~3 segundos |
| Comando | `npm test` |

## Lo que ya tenemos: pruebas unitarias

Cada prueba vive junto al archivo que prueba (`archivo.test.ts`). Se prueba la **lógica** de la aplicación: las reglas,
cálculos y transformaciones de datos, separadas de los componentes visuales.

### 1. Reglas de negocio (`features/*/lib`)

- **Alertas**: prioridad y orden, sonido, línea de tiempo, nuevas alertas, indicadores por sensor, notificación del
  navegador, formularios de clasificación, etiquetas y filtros, filtros guardados en la URL, caché del historial y del
  tiempo real, exportación de archivos, acciones por fila y modos de gestión.
- **Sensores**: nivel de batería, lectura del PK (punto kilométrico), búsqueda y estado del sensor.
- **Telemetría**: datos para gráficas, formato de valores y niveles del índice de ruido.
- **Mapa**: estado visual de cada nodo.

### 2. Utilidades compartidas (`shared/lib`)

- Formato de fechas (relativo, 24 h, `es-CO`).
- Rango de paginación.

### 3. Acceso a datos (`services`)

- Cliente HTTP: manejo de respuestas y errores.
- Adaptador HTTP de gestión de alertas: que cada llamada respete el contrato del backend.
- Cliente de alertas en tiempo real (stream).
- Lectura del nombre de archivo en descargas (`Content-Disposition`).

### 4. Modo demo (`demo`)

- Escenario simulado, series de datos, store de alertas, servicios demo, acciones y fuente de eventos simulada.
- Garantizan que la demo se comporte igual que la app real sin depender del backend.

### 5. Configuración de la app (`app`)

- Selección del modo de la aplicación (real o demo).

## Lo que nos falta

| Tipo de prueba | Estado | Qué deja sin cubrir |
|---|---|---|
| Unitarias de lógica | Hecho | — |
| Componentes y hooks | No hay | Lo que el usuario ve y cómo responde la interfaz |
| De punta a punta (E2E) | No hay | Flujos completos en un navegador real |
| Accesibilidad automática | No hay | Etiquetas, foco y contraste |
| Cobertura medida | No hay | Saber qué porcentaje del código está probado |
| Integración continua (CI) | No hay | Hoy las pruebas se corren a mano |

## Lo que podríamos hacer, en orden de prioridad

### 1. Integración continua (CI)

Un flujo de GitHub Actions que en cada pull request a `dev` ejecute lint, pruebas y build. Si algo falla, el cambio no
se integra.

- **Por qué primero**: cuesta poco y protege las 387 pruebas que ya existen.

### 2. Pruebas de componentes y hooks

Con **Vitest + Testing Library + jsdom**. Se renderiza el componente con servicios simulados y se verifica lo que
muestra.

Primeros candidatos:

- Panel de alertas y sus pestañas: estados de carga, error y vacío.
- Modal de detalle de alerta abierto desde un enlace.
- Banner de notificación y banner de "volvió a la normalidad".
- Filtros del historial sincronizados con la URL.
- Hooks de datos (TanStack Query) con un cliente de prueba.

### 3. Pruebas de punta a punta (E2E)

Con **Playwright** sobre el modo demo, sin depender del backend. Pocos flujos, los críticos:

- Abrir el mapa, ubicar el sensor del PK 34+470 y ver su detalle.
- Llega una alerta: aparece el banner, se agrega al panel y se abre su modal.
- Filtrar el historial de alertas y exportarlo.

### 4. Accesibilidad automática

Con **axe-core** dentro de las pruebas E2E: revisa etiquetas de iconos, foco visible y contraste sobre el mapa.

### 5. Complementarias (opcionales)

- **Cobertura mínima** exigida en `lib/` y `services/`.
- **Regresión visual**: capturas del mapa y la leyenda comparadas entre versiones.
- **Pruebas de contrato**: verificar que los tipos del frontend coincidan con los del backend.

## Pirámide de pruebas propuesta

```
            E2E (Playwright)         pocos flujos críticos
        Componentes y hooks          vistas principales
   Unitarias de lógica (Vitest)      base amplia — ya existe
```

La base ya está construida. Los siguientes pasos agregan las capas de arriba: menos pruebas, pero cada una cubre más de
la aplicación.
