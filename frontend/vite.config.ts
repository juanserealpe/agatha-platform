import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
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
      "/api": {
        target: "https://129.213.118.230",
        changeOrigin: true,
        secure: false,
      },
    },
  },
})