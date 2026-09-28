# Capa Shared (`internal/shared`)

Utilidades reutilizables de consumo **OBLIGATORIO**. No reinventar — usar lo que ya existe.

## 1. Errores de Dominio (`shared/errors`)

### Códigos (`ErrorCode`)

| Constante | Valor | HTTP Status |
|---|---|---|
| `CodeNotFound` | `NOT_FOUND` | 404 |
| `CodeInvalidInput` | `INVALID_INPUT` | 400 |
| `CodeFutureDate` | `FUTURE_DATE` | 400 |
| `CodeAlreadyIngested` | `ALREADY_INGESTED` | 409 |
| `CodeBeforeInitialDate` | `BEFORE_INITIAL_DATE` | 400 |
| `CodeInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | 400 |
| `CodeConsultingLastState` | `CONSULTING_LAST_STATE` | 500 |
| `CodeGettingIngestedData` | `GETTING_INGESTED_DATA` | 502 |

### Errores pre-construidos

`ErrNotFound`, `ErrDeviceNotFound`, `ErrFutureDate`, `ErrInvalidDate`

### Uso

```go
// Retornar error existente
return sharedErr.ErrNotFound

// Crear error de dominio nuevo
return &sharedErr.AppError{Code: sharedErr.CodeInvalidInput, Message: "campo X requerido"}
```

## 2. Helpers HTTP (`shared/http`)

| Función | Firma |
|---|---|
| `WriteJSON` | `WriteJSON(w http.ResponseWriter, status int, data any)` |
| `HandleError` | `HandleError(w http.ResponseWriter, err error)` |

**PROHIBIDO** usar `json.NewEncoder` o escribir headers manualmente. Usar siempre `WriteJSON`.

```go
// Respuesta exitosa
sharedHTTP.WriteJSON(w, http.StatusOK, map[string]any{"items": results})

// Respuesta de error (mapea AppError → HTTP status automáticamente)
sharedHTTP.HandleError(w, err)
```

## 3. Logger (`shared/logger`)

Métodos: `Info(msg, kv...)`, `Warn(msg, kv...)`, `Error(msg, kv...)`. Timezone fijo: `America/Bogota`.

```go
log := logger.New("ingestion", "INFO")
log.Error("falló extracción", "deviceID", id, "err", err)
```
