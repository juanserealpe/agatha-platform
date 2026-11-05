import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig(({ mode }) => {
  // Backend local para probar el tiempo real (p. ej. http://localhost:8080): solo /api/notifications va
  // ahí; el resto de /api sigue yendo al VPS. Vacía = todo al VPS.
  const localNotificationsUrl = loadEnv(mode, process.cwd(), '').LOCAL_NOTIFICATIONS_URL

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    build: {
      rolldownOptions: {
        output: {
          // Librerías separadas del código propio: cambian poco, así que tras cada despliegue el navegador
          // las reutiliza de caché y solo descarga lo que cambió. Recharts y el calendario no se agrupan:
          // siguen en el chunk que se carga al abrir una sección de telemetría.
          codeSplitting: {
            groups: [
              { name: 'vendor-react', test: /node_modules[\\/](react|react-dom|react-router|scheduler|@tanstack)[\\/]/ },
              { name: 'vendor-map', test: /node_modules[\\/](leaflet|react-leaflet|@react-leaflet)[\\/]/ },
            ],
          },
        },
      },
    },
    server: {
      proxy: {
        ...(localNotificationsUrl && {
          // Va antes que "/api": Vite usa la primera regla que coincide.
          "/api/notifications": { target: localNotificationsUrl, changeOrigin: true },
        }),
        "/api": {
          target: "https://129.213.118.230",
          changeOrigin: true,
          secure: false,
        },
      },
    },
  }
})
