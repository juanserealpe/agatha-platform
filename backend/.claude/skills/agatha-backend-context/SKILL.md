---
name: agatha-backend-context
description: Carga automática de contexto y arquitectura para el backend agatha-platform (Go Monolito Modular). Usar SIEMPRE al iniciar cualquier tarea de desarrollo, refactorización o consulta en el backend.
---

# Contexto Arquitectónico y Estrategia de Documentación (`agatha-platform/backend`)

## Regla de Oro: Rutas de Documentación Viva
Al realizar cualquier cambio o análisis en el backend, DEBES consultar la documentación existente usando las siguientes referencias:

1. **Arquitectura Global y Guardrails**: Consulte `@backend/ARQUITECTURA_LITE.md`.
   - Contiene los guardrails obligatorios (uso de `shared/`, reglas de BD compartida PostgreSQL, aislamiento modular).
   - Contiene el modelo de base de datos relacional y el catálogo fijo de las 21 métricas (IDs 1 a 21).

2. **Contexto Vivo por Módulo**: Consulta `@backend/internal/modules/[modulo]/README.md` (ejemplo: `@backend/internal/modules/ingestion/README.md` o `@backend/internal/modules/dashboard/README.md`).
   - Contiene la responsabilidad del módulo, endpoints activos y DTOs actuales.

## Regla de Exclusión de Tokens (Historial)
- **IGNORAR POR COMPLETO** la carpeta `backend/docs/` y las subcarpetas `backend/internal/modules/[modulo]/docs/` que contienen versiones antiguas (`README_v1.0.X.md`).
- SOLO leer carpetas `docs/` si el usuario pide explícitamente auditar o generar notas de versión de un release histórico.

## Flujo de Cierre de Tarea (Trazabilidad)
Cada vez que finalices la implementación de una característica en Go:
1. Actualiza el `README.md` principal del módulo afectado con los nuevos endpoints, DTOs o reglas de negocio asi mismo con '@backend/ARQUITECTURA_LITE.md'.
2. Si y solo si el cambio modifica patrones o contratos globales, actualziza `@backend/ARQUITECTURA_LITE.md`.