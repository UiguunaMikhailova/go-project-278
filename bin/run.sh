#!/usr/bin/env bash
set -euo pipefail

echo "[run.sh] Starting service"

if [ -n "${DATABASE_URL:-}" ]; then
  echo "[run.sh] Running DB migrations"
  goose -dir ./db/migrations postgres "${DATABASE_URL}" up
else
  echo "[run.sh] DATABASE_URL is not set, skipping migrations"
fi

echo "[run.sh] Starting Caddy"
caddy run --config /etc/caddy/Caddyfile &

echo "[run.sh] Starting Go app"
# на :80 слушает Caddy, приложение всегда занимает 8080 из Caddyfile
exec env PORT=8080 /app/bin/app
