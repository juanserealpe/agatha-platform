import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router'
// Estilos base de Leaflet antes que los de la app, para que los ajustes de index.css prevalezcan.
import 'leaflet/dist/leaflet.css'
import './index.css'
import { DEMO_BASENAME, DEMO_ENABLED, isDemoPath } from './app/appMode'
import { createAppRouter } from './app/router'
import { isClientError } from '@/services/http'

const MAX_QUERY_RETRIES = 3

// Función async en vez de top-level await: el módulo de entrada termina de evaluarse sin esperar a la demo.
async function bootstrap() {
  const demoMode = DEMO_ENABLED && isDemoPath(window.location.pathname)
  if (demoMode) {
    // Chunk aparte: la app real nunca descarga el código de la demo.
    const { installDemo } = await import('./demo/installDemo')
    installDemo()
  }

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        // Un 4xx (filtro inválido, alerta inexistente) no cambia al reintentar: se muestra el mensaje de inmediato.
        retry: (failureCount, error) => !isClientError(error) && failureCount < MAX_QUERY_RETRIES,
      },
    },
  })
  const router = createAppRouter(demoMode ? { basename: DEMO_BASENAME } : {})

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </StrictMode>,
  )
}

void bootstrap()
