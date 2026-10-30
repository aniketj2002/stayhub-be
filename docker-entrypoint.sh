#!/bin/sh
set -e

if [ "${AUTO_MIGRATE:-true}" = "true" ]; then
  echo "[StayHub] Running database migrations..."
  /app/stayhub-migrate up
fi

if [ "${AUTO_SEED:-true}" = "true" ]; then
  echo "[StayHub] Running idempotent database seed..."
  /app/stayhub-seed
fi

echo "[StayHub] Starting API server..."
exec /app/stayhub-api
