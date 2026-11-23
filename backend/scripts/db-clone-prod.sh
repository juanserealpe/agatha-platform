#!/usr/bin/env bash
# Reemplaza la BD LOCAL (docker-compose.local.yml) por una copia del esquema
# public de PRODUCCIÓN (Supabase). Producción solo se LEE (pg_dump).
#
#   ./scripts/db-clone-prod.sh [-y] [--keep-push-devices]
#
#   -y                    no pide confirmación
#   --keep-push-devices   conserva los tokens FCM reales en la copia (por
#                         defecto se borran: un notifications local con
#                         FCM_VALIDATE_ONLY=false mandaría push a celulares reales)
#
# Lee DATABASE_URL de .env.production. Usa la imagen postgres:17 para pg_dump
# (misma versión mayor que Supabase), así no hace falta instalar el cliente.
set -euo pipefail

cd "$(dirname "$0")/.."

PG_IMAGE="postgres:17"
COMPOSE_FILE="docker-compose.local.yml"
LOCAL_CONTAINER="agatha-postgres-local"
LOCAL_USER="agatha"
LOCAL_DB="agatha"
PROD_ENV_FILE=".env.production"

assume_yes=false
keep_push_devices=false
for arg in "$@"; do
	case "$arg" in
	-y) assume_yes=true ;;
	--keep-push-devices) keep_push_devices=true ;;
	*)
		echo "argumento desconocido: $arg" >&2
		exit 2
		;;
	esac
done

# read_env_var ARCHIVO CLAVE: imprime el valor sin comillas (sin ejecutar el archivo).
read_env_var() {
	local file=$1 key=$2 line value
	while IFS= read -r line || [[ -n $line ]]; do
		line=${line#export }
		if [[ $line == "$key="* ]]; then
			value=${line#*=}
			value=${value%\"}
			value=${value#\"}
			value=${value%\'}
			value=${value#\'}
			printf '%s' "$value"
			return 0
		fi
	done <"$file"
	return 1
}

if [[ ! -f $PROD_ENV_FILE ]]; then
	echo "falta $PROD_ENV_FILE con DATABASE_URL de producción" >&2
	exit 1
fi
PROD_URL=$(read_env_var "$PROD_ENV_FILE" DATABASE_URL) || {
	echo "$PROD_ENV_FILE no define DATABASE_URL" >&2
	exit 1
}

# El pooler en modo transacción (6543) no sirve para pg_dump; el de sesión
# (5432, mismo host y usuario) sí.
if [[ $PROD_URL == *":6543/"* ]]; then
	echo "aviso: el DSN usa el pooler de transacción (6543); pg_dump va por el de sesión (5432)"
	PROD_URL=${PROD_URL/:6543\//:5432/}
fi
prod_host=${PROD_URL#*@}
prod_host=${prod_host%%\?*}

echo "origen  (solo lectura): $prod_host"
echo "destino (se BORRA):     $LOCAL_CONTAINER/$LOCAL_DB"
if [[ $assume_yes != true ]]; then
	read -r -p "Escribí 'clonar' para reemplazar la BD local: " answer
	[[ $answer == "clonar" ]] || {
		echo "cancelado"
		exit 1
	}
fi

docker compose -f "$COMPOSE_FILE" up -d --wait

dump_file=$(mktemp --suffix=.agatha.dump)
trap 'rm -f "$dump_file"' EXIT

echo "1/4 pg_dump de producción…"
# El DSN viaja por variable de entorno, no como argumento visible en ps.
# --network host: usa el DNS del host (el bridge de Docker no resuelve acá).
docker run --rm --network host -e PROD_URL="$PROD_URL" -e PGCONNECT_TIMEOUT=15 "$PG_IMAGE" \
	sh -c 'pg_dump --dbname="$PROD_URL" --schema=public --no-owner --no-privileges --format=custom' \
	>"$dump_file"
echo "    dump: $(du -h "$dump_file" | cut -f1)"

local_psql() {
	docker exec -i "$LOCAL_CONTAINER" psql -v ON_ERROR_STOP=1 -q -U "$LOCAL_USER" -d "$LOCAL_DB" "$@"
}
local_restore() {
	docker exec -i "$LOCAL_CONTAINER" pg_restore -U "$LOCAL_USER" -d "$LOCAL_DB" \
		--no-owner --no-privileges --exit-on-error --single-transaction "$@" <"$dump_file"
}

echo "2/4 vaciando la BD local…"
local_psql -c "DROP SCHEMA IF EXISTS public CASCADE;"

echo "3/4 restaurando…"
# btree_gist vive en public en Supabase; se crea entre el esquema y las
# restricciones por si el dump no la trae (pg_dump -n omite extensiones).
local_restore --section=pre-data
local_psql -c "CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;"
local_restore --section=data --section=post-data

if [[ $keep_push_devices != true ]]; then
	local_psql -c "DELETE FROM push_devices;" 2>/dev/null &&
		echo "    push_devices vaciada (tokens reales fuera de la copia local)"
fi

echo "4/4 verificación:"
local_psql -c "SELECT 'raw_measurements' AS tabla, count(*) AS filas, max(timestamp) AS ultimo FROM raw_measurements
UNION ALL SELECT 'feature_evaluations', count(*), NULL FROM feature_evaluations
UNION ALL SELECT 'alerts', count(*), NULL FROM alerts;"
echo "listo: AGATHA_DB=local go run ./cmd/..."
