#!/usr/bin/env bash
# Runs a command against a freshly migrated, throwaway PostgreSQL database, then drops it.
#   scripts/db-throwaway.sh <command> [args...]
# The command sees TEST_DB_NAME and TEST_DATABASE_URL (superuser URL of the throwaway database)
# and TEST_TEMPLATE_DB (a migrated template database, cloned per test by api/internal/dbtest).
# Environment (set by the Makefile):
#   PSQL          psql command that connects as a superuser (local: via docker compose; CI: plain psql)
#   PG_SUPER_URL  superuser URL without database, for example postgres://postgres:postgres@localhost:5432
#   GOOSE         command that runs goose from api/ with the tools module
set -euo pipefail
: "${PSQL:?}" "${PG_SUPER_URL:?}" "${GOOSE:?}"
[ $# -gt 0 ] || { echo "usage: $0 <command> [args...]" >&2; exit 2; }

name="bappelitbangda_tmp_$$_$(date +%s)"
cleanup() {
  # Drops the throwaway database, the template and every clone (names share the ${name} prefix).
  # shellcheck disable=SC2086  # PSQL is a command line with arguments
  $PSQL -d postgres >/dev/null 2>&1 <<SQL || true
SELECT format('DROP DATABASE IF EXISTS %I WITH (FORCE)', datname) FROM pg_database WHERE datname LIKE '${name}%' ORDER BY datname DESC \gexec
SQL
}
trap cleanup EXIT

# shellcheck disable=SC2086
$PSQL -d postgres -c "CREATE DATABASE ${name}" >/dev/null
export TEST_DB_NAME="$name"
export TEST_DATABASE_URL="${PG_SUPER_URL}/${name}?sslmode=disable"
# shellcheck disable=SC2086
$GOOSE -dir db/migrations postgres "$TEST_DATABASE_URL" up >/dev/null
# Goose has disconnected, so the database can serve as a template.
# shellcheck disable=SC2086
$PSQL -d postgres -c "CREATE DATABASE ${name}_tpl TEMPLATE ${name}" >/dev/null
export TEST_TEMPLATE_DB="${name}_tpl"
"$@"
