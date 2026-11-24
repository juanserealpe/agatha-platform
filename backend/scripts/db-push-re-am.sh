#!/usr/bin/env bash
# Sube a PRODUCCIÓN (Supabase) las tablas de Rule Engine y Alert Management
# calculadas en la BD LOCAL, en vez de correr esos rebuilds contra Supabase
# (horas de idas y vueltas por red; en local son minutos).
#
#   ./scripts/db-push-re-am.sh [--check] [-y] [--target-url URL]
#   ./scripts/db-push-re-am.sh --restore ARCHIVO [-y] [--target-url URL]
#
#   --check            solo prechequeos (no escribe nada)
#   -y                 no pide confirmación
#   --target-url URL   destino explícito (pruebas local→local); por defecto
#                      DATABASE_URL de .env.production
#   --restore ARCHIVO  vuelve a cargar un backup que generó este script
#
# Procedimiento (ver backend/ARQUITECTURA_LITE.md, rebuild por copia):
#   1. backup de producción y VPS FRENADA (si escribe durante la copia, se mezcla)
#   2. ./scripts/db-clone-prod.sh -y        (misma entrada que producción)
#   3. en local SOLO: rule_engine -backfill [-motion] -rebuild
#                     alert_management -backfill -rebuild -drop-management
#      (NO baseline ni processing: cambiarían la entrada)
#   4. este script
#   5. levantar la VPS (los catch-ups rellenan las horas de la ventana)
#
# Garantías:
#   - aborta si la ENTRADA difiere entre local y destino (feature_evaluations,
#     baseline_versions, baseline_component_stats por md5; raw_measurements
#     por conteo y último timestamp): las alertas no corresponderían a los
#     índices del destino;
#   - aborta si las columnas de las 11 tablas difieren;
#   - guarda un backup de las 11 tablas del destino (restaurable con --restore);
#   - carga en UNA transacción (TRUNCATE + COPY + setval): todo o nada;
#   - verifica al final que las 11 tablas quedaron idénticas (md5).
set -euo pipefail

cd "$(dirname "$0")/.."

PG_IMAGE="postgres:17"
LOCAL_CONTAINER="agatha-postgres-local"
LOCAL_USER="agatha"
LOCAL_DB="agatha"
PROD_ENV_FILE=".env.production"
BACKUP_DIR="backups"

# Salida de Rule Engine y Alert Management. Las FK quedan dentro del conjunto
# (verificado): un solo TRUNCATE de las 11 no necesita CASCADE.
TABLES=(
	rule_threshold_versions rule_evaluations
	motion_threshold_versions motion_threshold_rules motion_evaluations
	alert_policy_versions alerts alert_level_segments
	alert_messages alert_observations alert_processing_state
)
# Entrada de Rule Engine (FUGA lee feature_evaluations; MOVIMIENTO, raw).
INPUT_TABLES=(feature_evaluations baseline_versions baseline_component_stats)

check_only=false
assume_yes=false
target_url=""
restore_file=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	--check) check_only=true ;;
	-y) assume_yes=true ;;
	--target-url)
		target_url=${2:?--target-url necesita un valor}
		shift
		;;
	--restore)
		restore_file=${2:?--restore necesita un archivo}
		shift
		;;
	*)
		echo "argumento desconocido: $1" >&2
		exit 2
		;;
	esac
	shift
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

if [[ -z $target_url ]]; then
	[[ -f $PROD_ENV_FILE ]] || {
		echo "falta $PROD_ENV_FILE con DATABASE_URL de producción" >&2
		exit 1
	}
	target_url=$(read_env_var "$PROD_ENV_FILE" DATABASE_URL) || {
		echo "$PROD_ENV_FILE no define DATABASE_URL" >&2
		exit 1
	}
fi
# El pooler de transacción (6543) no sirve para COPY/pg_dump; el de sesión sí.
if [[ $target_url == *":6543/"* ]]; then
	echo "aviso: el DSN usa el pooler de transacción (6543); se usa el de sesión (5432)"
	target_url=${target_url/:6543\//:5432/}
fi
target_host=${target_url#*@}
target_host=${target_host%%\?*}

# Cliente contra el destino: contenedor descartable, DSN por variable de
# entorno (no visible en ps), sesión en UTC para que las huellas coincidan.
target_run() {
	docker run --rm -i --network host -e TARGET_URL="$target_url" -e PGTZ=UTC \
		-e PGCONNECT_TIMEOUT=15 "$PG_IMAGE" sh -c "$1"
}
target_psql() {
	target_run 'psql --dbname="$TARGET_URL" -X -q -At -v ON_ERROR_STOP=1'
}
local_psql() {
	docker exec -i -e PGTZ=UTC "$LOCAL_CONTAINER" psql -X -q -At -v ON_ERROR_STOP=1 -U "$LOCAL_USER" -d "$LOCAL_DB"
}

in_list() {
	local out="" t
	for t in "$@"; do out+="${out:+,}'$t'"; done
	printf '%s' "$out"
}
# fingerprint_sql TABLA...: una fila "tabla|filas|md5" por tabla (orden de
# filas por su texto: no depende de ids ni del orden físico). COLLATE "C":
# Supabase y el Postgres local dicen en_US pero ordenan distinto (otra versión
# de las reglas de la libc); el orden binario es el mismo en los dos.
fingerprint_sql() {
	local t sep=""
	for t in "$@"; do
		printf "%sSELECT '%s', count(*), md5(coalesce(string_agg(t::text, '|' ORDER BY t::text COLLATE \"C\"), '')) FROM public.%s t\n" "$sep" "$t" "$t"
		sep="UNION ALL "
	done
	printf ";\n"
}
raw_sql="SELECT measuring_point_id, count(*), max(\"timestamp\") FROM public.raw_measurements GROUP BY 1 ORDER BY 1;"
schema_sql="SELECT table_name || '|' || md5(string_agg(column_name || ':' || data_type, ',' ORDER BY column_name COLLATE \"C\"))
FROM information_schema.columns WHERE table_schema = 'public' AND table_name IN ($(in_list "${TABLES[@]}"))
GROUP BY table_name ORDER BY table_name;"

# compare ETIQUETA SQL: corre SQL en los dos lados y aborta si difieren.
# La sesión se fija igual en ambos: Supabase usa extra_float_digits = 0 (un
# float se imprime redondeado: 0.1346 en vez de 0.13460000000000003) y el
# Postgres local 1; sin esto, el mismo dato da huellas distintas. Con 1 el
# texto es exacto (dos floats distintos nunca se imprimen igual).
compare() {
	local label=$1 sql="SET extra_float_digits = 1; SET TIME ZONE 'UTC'; $2" src dst
	src=$(local_psql <<<"$sql")
	dst=$(target_psql <<<"$sql")
	if [[ $src != "$dst" ]]; then
		echo "ABORTADO: $label difiere entre local y destino." >&2
		diff <(printf '%s\n' "$src") <(printf '%s\n' "$dst") | sed 's/^</  local  /; s/^>/  destino/' >&2 || true
		return 1
	fi
	echo "    ok: $label"
}

confirm() {
	[[ $assume_yes == true ]] && return 0
	read -r -p "$1" answer
	[[ $answer == "$2" ]] || {
		echo "cancelado"
		exit 1
	}
}

# load_header: lo que va antes de las filas, dentro de la misma transacción.
load_header() {
	printf "SET LOCAL agatha.alerts_rebuild = 'on';\n"
	printf "TRUNCATE %s;\n" "$(IFS=,; echo "${TABLES[*]/#/public.}")"
}
dump_args=""
for t in "${TABLES[@]}"; do dump_args+=" -t public.$t"; done

echo "destino (se REEMPLAZAN 11 tablas): $target_host"

if [[ -n $restore_file ]]; then
	[[ -s $restore_file ]] || {
		echo "no existe o está vacío: $restore_file" >&2
		exit 1
	}
	confirm "Escribí 'restaurar' para cargar $restore_file en el destino: " restaurar
	target_run 'psql --dbname="$TARGET_URL" -X -q -o /dev/null -v ON_ERROR_STOP=1 --single-transaction' <"$restore_file"
	echo "restaurado: $restore_file"
	exit 0
fi

docker exec "$LOCAL_CONTAINER" pg_isready -q -U "$LOCAL_USER" || {
	echo "la BD local no responde: docker start $LOCAL_CONTAINER" >&2
	exit 1
}

echo "1/5 prechequeo de la entrada (local = destino)…"
compare "raw_measurements (conteo y último dato por punto)" "$raw_sql"
compare "feature_evaluations y baseline (md5)" "$(fingerprint_sql "${INPUT_TABLES[@]}")"

echo "2/5 prechequeo del esquema de las 11 tablas…"
compare "columnas" "$schema_sql"

echo "    filas a subir:"
local_psql <<<"$(fingerprint_sql "${TABLES[@]}")" | while IFS='|' read -r t n _; do printf '      %-26s %s\n' "$t" "$n"; done

if [[ $check_only == true ]]; then
	echo "--check: todo coincide, no se escribió nada"
	exit 0
fi

echo "La VPS tiene que estar FRENADA: si escribe durante la carga, se mezcla."
confirm "Escribí 'subir' para reemplazar las 11 tablas en $target_host: " subir

mkdir -p "$BACKUP_DIR"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_file="$BACKUP_DIR/re_am_before_${stamp}.sql"
load_file=$(mktemp --suffix=.re_am.sql)
trap 'rm -f "$load_file"' EXIT

echo "3/5 backup del destino → $backup_file"
{
	load_header
	target_run "pg_dump --dbname=\"\$TARGET_URL\" --data-only --no-owner --no-privileges$dump_args"
} >"$backup_file"
grep -q '^COPY public.alerts ' "$backup_file" || {
	echo "ABORTADO: el backup no trae alerts; no se toca el destino" >&2
	exit 1
}

echo "4/5 carga (una transacción)…"
{
	load_header
	docker exec "$LOCAL_CONTAINER" sh -c "pg_dump -U $LOCAL_USER -d $LOCAL_DB --data-only --no-owner --no-privileges$dump_args"
} >"$load_file"
target_run 'psql --dbname="$TARGET_URL" -X -q -o /dev/null -v ON_ERROR_STOP=1 --single-transaction' <"$load_file"

echo "5/5 verificación (md5 de las 11 tablas)…"
compare "las 11 tablas" "$(fingerprint_sql "${TABLES[@]}")" || {
	echo "Restaurar: ./scripts/db-push-re-am.sh --restore $backup_file" >&2
	exit 1
}
echo "listo. Backup previo: $backup_file  (restaurar con --restore)"
