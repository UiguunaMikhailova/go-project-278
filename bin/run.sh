#!/usr/bin/env bash
set -euo pipefail

echo "[run.sh] Starting service"

if [ -n "${DATABASE_URL:-}" ]; then
  echo "[run.sh] Running DB migrations"
  goose -dir ./db/migrations postgres "${DATABASE_URL}" up
else
  echo "[run.sh] DATABASE_URL is not set, skipping migrations"
fi

echo "[run.sh] Starting Go app"
exec /app/bin/app
