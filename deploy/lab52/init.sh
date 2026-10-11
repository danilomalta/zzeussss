#!/bin/sh
set -eu
# Called once by PostgreSQL in a NEW private lab volume. Password is stdin SQL,
# never an argv parameter; generated hexadecimal input is validated first.
case "$TITAN_LAB_DB_PASSWORD" in ''|*[!0-9a-f]*) exit 1;; esac
[ "${#TITAN_LAB_DB_PASSWORD}" -eq 64 ]
printf "CREATE ROLE titan_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '%s'; ALTER DATABASE titan_lab52 OWNER TO titan_app; GRANT ALL ON SCHEMA public TO titan_app;\n" "$TITAN_LAB_DB_PASSWORD" | psql -v ON_ERROR_STOP=1 --username postgres --dbname titan_lab52 >/dev/null
PGPASSWORD="$TITAN_LAB_DB_PASSWORD" psql -v ON_ERROR_STOP=1 --username titan_app --dbname titan_lab52 -f /titan-base.sql >/dev/null
