import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router'
// Estilos base de Leaflet antes que los de la app, para que los ajustes de index.css prevalezcan.
import 'leaflet/dist/leaflet.css'
import './index.css'
import { createAppRouter } from './app/router'
import { isClientError } from '@/services/http'

const MAX_QUERY_RETRIES = 3

function bootstrap() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        // Un 4xx (filtro inválido, alerta inexistente) no cambia al reintentar: se muestra el mensaje de inmediato.
        retry: (failureCount, error) => !isClientError(error) && failureCount < MAX_QUERY_RETRIES,
      },
    },
  })
  const router = createAppRouter()

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </StrictMode>,
  )
}

bootstrap()
