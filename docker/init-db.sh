#!/usr/bin/env bash
set -Eeuo pipefail

psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set=admin_user="$POSTGRES_USER" --set=app_user="$NEXAFLOW_DB_USER" --set=app_password="$NEXAFLOW_DB_PASSWORD" <<'EOSQL'
SELECT format(
  'CREATE ROLE %I LOGIN PASSWORD %L NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION',
  :'app_user',
  :'app_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_user')
\gexec

SELECT format(
  'ALTER ROLE %I WITH LOGIN PASSWORD %L NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION',
  :'app_user',
  :'app_password'
)
\gexec

CREATE EXTENSION IF NOT EXISTS vector;

SELECT format('REASSIGN OWNED BY %I TO %I', :'admin_user', :'app_user')
WHERE :'admin_user' <> :'app_user'
\gexec

SELECT format('ALTER DATABASE %I OWNER TO %I', current_database(), :'app_user')
\gexec
SELECT format('ALTER SCHEMA public OWNER TO %I', :'app_user')
\gexec
SELECT format('GRANT ALL ON SCHEMA public TO %I', :'app_user')
\gexec
EOSQL
