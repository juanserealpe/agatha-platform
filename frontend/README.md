# AGATHA — frontend

Monitoreo en tiempo real del gasoducto Otero-Santana: mapa del piloto, alertas de fuga en vivo, gestión de alertas e historial. React 19 + Vite + TypeScript, desplegado en Vercel.

## Comandos

```bash
npm install
npm run dev       # http://localhost:5173 (proxy de /api al backend, ver vite.config.ts)
npm test          # vitest
npm run lint
npm run build     # tsc + vite build en dist/
npm run preview   # sirve dist/ en http://localhost:4173
```

## Variables de entorno

Ver [.env.example](.env.example): `VITE_API_BASE_URL`, `VITE_ENABLE_DEMO`, `VITE_FEATURE_ALERT_EVIDENCE` y `LOCAL_NOTIFICATIONS_URL` (solo en desarrollo).

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
