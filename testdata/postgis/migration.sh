#!/bin/bash
set -e

# indent command output by 4 spaces
run() {
  "$@" 2>&1 | sed 's/^/    /'
}

export PGHOST="postgis"
export PGPORT="5432"
export PGUSER="postgres"
export PGPASSWORD="postgres"
export PGDATABASE="postgres"

echo "Dropping existing 'tegola' database (if any)..."
run psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -c "DROP DATABASE IF EXISTS tegola;"

echo "Restoring database from dump..."
run pg_restore -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -C /testdata/postgis/tegola.dump

echo "Dropping and creating role 'tegola_no_access'..."
run psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -c "DROP ROLE IF EXISTS tegola_no_access; CREATE ROLE tegola_no_access LOGIN PASSWORD 'postgres';"

echo "Applying SQL files from /testdata with prefix 'postgis-'..."
for sqlfile in /testdata/postgis/postgis-*.sql; do
  echo "Applying $sqlfile..."
  run psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "tegola" -f "$sqlfile"
done

echo "Migration completed successfully."

