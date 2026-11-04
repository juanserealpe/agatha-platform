# Prueba local de notificaciones (web y push)

Guía para levantar el módulo `notifications` en tu PC y probar el tiempo real de la web y el push a un
celular, **sin tocar la base del piloto**. Todos los comandos son de PowerShell y se corren desde la raíz
del repositorio, salvo que se indique otra carpeta.

Qué se puede probar así:

| Qué | Se ve | Cómo |
|---|---|---|
| Banner, sonido, notificación del navegador, indicador "En vivo" | Sí | Eventos de prueba (paso 4) |
| Mapa y panel de alertas | **No** | Las alertas de prueba no existen en ninguna base: el frontend solo actualiza las que ya trae de `/active` y `/pending`. Hacen falta alertas reales (VPS) |
| Push al celular | Sí | Comando `-test-send` (paso 5) |

## 1. Base de datos local (Docker)

Abrir **Docker Desktop** y esperar a que diga "Engine running".

La primera vez se crea el contenedor (PostgreSQL vacío en el puerto **5433**, para no chocar con otro
Postgres en el 5432):

```powershell
docker run -d --name agatha-pg-local -e POSTGRES_USER=agatha -e POSTGRES_PASSWORD=agatha_local -e POSTGRES_DB=agatha -p 127.0.0.1:5433:5432 postgres:16-alpine
```

Las veces siguientes basta con encenderlo:

```powershell
docker start agatha-pg-local
```

El backend crea sus tablas al arrancar. No hace falta cargar datos.

## 2. Backend

**`backend/.env`** (ignorado por git). Sin secretos: apunta a la base local.

```
DATABASE_URL=postgres://agatha:agatha_local@localhost:5433/agatha?sslmode=disable
AGATHA_ENV=development
AGATHA_MODULES=notifications,dashboard
AGATHA_CORS_ALLOWED_ORIGINS=http://localhost:5173
FCM_PROJECT_ID=agatha-5fe71
FCM_SERVICE_ACCOUNT_FILE=secrets/fcm-service-account.json
FCM_VALIDATE_ONLY=true
NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST=
NOTIFICATIONS_TEST_ENDPOINT=true
```

- `FCM_SERVICE_ACCOUNT_FILE`: la credencial de Firebase va en `backend/secrets/fcm-service-account.json`
  (ignorada por git y por Docker). Nunca se pega en un chat ni en un correo. Sin ella el backend arranca
  igual, con el push deshabilitado.
- **Nunca** apuntar `DATABASE_URL` a la base del piloto para estas pruebas.

Arrancar (en una terminal que se deja abierta):

```powershell
cd backend
go run ./cmd/platform
```

Listo cuando el log dice `servidor HTTP iniciado [addr :8080]`.

## 3. Frontend

**`frontend/.env.development.local`** (ignorado por git):

```
LOCAL_NOTIFICATIONS_URL=http://localhost:8080
```

Con eso Vite manda `/api/notifications` al backend local y el resto de `/api` al VPS. Sin la variable,
todo va al VPS.

> Ojo: crear el archivo con el editor o con `Set-Content -Encoding ascii`. Con `-Encoding utf8`,
> PowerShell 5.1 agrega un BOM y Vite no lee la primera línea.

Arrancar (en otra terminal):

```powershell
cd frontend
npm run dev
```

Abrir http://localhost:5173. El indicador de conexión debe decir **"En vivo"**. Aceptar el permiso de
notificaciones del navegador si lo pide.

## 4. Eventos de prueba (web)

Cada evento devuelve un `alert_id` **negativo**. Para seguir sobre la misma alerta, usar ese id en los
siguientes. Los ejemplos suponen `-1`.

```powershell
# Alerta nueva amarilla en el PK 34+470: banner amarillo y sonido
Invoke-RestMethod -Method Post http://localhost:5173/api/notifications/test-event -Body '{"measuring_point_id":11556,"level":"AMARILLO"}'

# Sube a naranja: el banner se reemplaza
Invoke-RestMethod -Method Post http://localhost:5173/api/notifications/test-event -Body '{"measuring_point_id":11556,"alert_id":-1,"level":"NARANJA","code":"UP_NARANJA_FAST"}'

# Sube a rojo: el banner titila
Invoke-RestMethod -Method Post http://localhost:5173/api/notifications/test-event -Body '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","code":"UP_ROJO"}'

# Se clasifica: el banner deja de titilar y no aparece otro
Invoke-RestMethod -Method Post http://localhost:5173/api/notifications/test-event -Body '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","change":"CLASSIFIED","classification":"CONFIRMADA"}'

# Aviso atrasado: lleva la etiqueta "Atrasado" (no va al push)
Invoke-RestMethod -Method Post http://localhost:5173/api/notifications/test-event -Body '{"measuring_point_id":11556,"level":"NARANJA","late":true}'
```

Más pruebas:

- **Varios avisos**: lanzar el primer comando varias veces (sin `alert_id`, cada uno es una alerta
  nueva). Se ve un banner por alerta, los más graves arriba, como máximo 3, y el resto resumido como
  "+N avisos más". Los banners quedan por encima de la leyenda y del minimapa.
- **Notificación del navegador**: pasar a otra pestaña y lanzar un evento.
- **Reconexión**: detener el backend (Ctrl+C) y volver a arrancarlo. El indicador pasa a "Reconectando"
  y vuelve a "En vivo".
- **Ver el stream crudo**: `curl.exe -N http://localhost:8080/api/notifications/stream`.

Todos los campos del evento de prueba están en
[NOTIFICATIONS_API.MD](NOTIFICATIONS_API.MD) §3.

## 5. Push a un celular

Hace falta el **token FCM** del celular (lo da el equipo móvil) o su **`deviceId`** si la app ya lo
registró en este backend. Desde `backend/`:

```powershell
# 1) Validar: FCM revisa el token y el mensaje, pero NO lo entrega
go run ./cmd/notifications -test-send -token "<token FCM>"

# 2) Entregar de verdad: solo a ese celular
$env:FCM_VALIDATE_ONLY = "false"; go run ./cmd/notifications -test-send -token "<token FCM>" -level ORANGE -kind LEVEL_UP; Remove-Item Env:FCM_VALIDATE_ONLY
```

- `-level`: `YELLOW`, `ORANGE` o `RED`. `-kind`: `NEW` o `LEVEL_UP`.
- `-device CEL-XXXXXX` en lugar de `-token` busca el token registrado en la base local.
- Probar con la app abierta, en segundo plano y cerrada.
- Si dice `UNREGISTERED`, el token ya no sirve: la app tiene que obtener uno nuevo.

Formato del mensaje y reglas: [CONTRATO_PUSH_FCM.md](CONTRATO_PUSH_FCM.md) §3 y §7.

## 6. Apagar

- Backend y frontend: Ctrl+C en cada terminal.
- Si no hay terminal donde presionar Ctrl+C (por ejemplo, porque se arrancaron en segundo plano), cerrar
  lo que esté escuchando en los puertos 8080 y 5173:

  ```powershell
  Get-NetTCPConnection -State Listen -LocalPort 8080,5173 | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force }
  ```

  Para comprobar que quedaron apagados, correr `Get-NetTCPConnection -State Listen -LocalPort 8080,5173`.
  Si responde con el error "no encontró objetos MSFT_NetTCPConnection", es que ya no hay nada corriendo.
- Base: `docker stop agatha-pg-local` (los datos se conservan para la próxima).
- Borrar la base por completo: `docker rm -f agatha-pg-local`.
