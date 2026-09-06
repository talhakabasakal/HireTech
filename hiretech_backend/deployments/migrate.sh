#!/bin/sh
set -eu

: "${DATABASE_URL:?DATABASE_URL is required for migrations}"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q <<'SQL'
CREATE TABLE IF NOT EXISTS hiretech_schema_migrations (
    version BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

for migration in /app/migrations/0*.sql; do
    [ -f "$migration" ] || continue
    filename=$(basename "$migration")
    version=${filename%%_*}

    applied=$(psql "$DATABASE_URL" -Atq \
        -c "SELECT 1 FROM hiretech_schema_migrations WHERE version = ${version} LIMIT 1")
    [ "$applied" = "1" ] && continue

    sql=$(awk '
        /^-- \+goose Up$/ { in_up=1; next }
        /^-- \+goose Down$/ { in_up=0 }
        in_up { print }
    ' "$migration")

    {
        printf '%s\n' "$sql"
        printf 'INSERT INTO hiretech_schema_migrations(version) VALUES (%s);\n' "$version"
    } | psql "$DATABASE_URL" -v ON_ERROR_STOP=1 --single-transaction -q

    echo "applied $filename"
done
