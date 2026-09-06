#!/bin/sh
set -eu

/app/migrate.sh
exec /app/masterfabric
