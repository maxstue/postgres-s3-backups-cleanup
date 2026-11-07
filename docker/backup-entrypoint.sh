#!/bin/sh
set -eu
pg_isready --dbname="$BACKUP_DATABASE_URL"
pg_dump --version
exec /usr/local/bin/backup
