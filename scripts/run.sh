#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"


DB_HOST="${POSTGRES_HOST:-localhost}"
DB_PORT="${POSTGRES_PORT:-5432}"
DB_NAME="${POSTGRES_DB:-project-sem-1}"
DB_USER="${POSTGRES_USER:-validator}"
DB_PASSWORD="${POSTGRES_PASSWORD:-val1dat0r}"

HTTP_ADDR="${HTTP_ADDR:-:8080}"
PID_FILE="${PID_FILE:-$ROOT_DIR/.api.pid}"
LOG_FILE="${LOG_FILE:-$ROOT_DIR/app.log}"

echo "[run] waiting for postgres at ${DB_HOST}:${DB_PORT} ..."
for i in $(seq 1 60); do
  if PGPASSWORD="$DB_PASSWORD" pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done


PGPASSWORD="$DB_PASSWORD" pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1 || {
  echo "[run] postgres is not ready" >&2
  exit 1
}


if [ -f "$PID_FILE" ]; then
  old_pid="$(cat "$PID_FILE" || true)"
  if [ -n "${old_pid:-}" ] && kill -0 "$old_pid" >/dev/null 2>&1; then
    echo "[run] stopping previous api pid=$old_pid"
    kill "$old_pid" || true
    sleep 1
  fi
fi

echo "[run] starting app on ${HTTP_ADDR} ..."
export HTTP_ADDR
export DB_HOST="$DB_HOST"
export DB_PORT="$DB_PORT"
export DB_NAME="$DB_NAME"
export DB_USER="$DB_USER"
export DB_PASSWORD="$DB_PASSWORD"


export POSTGRES_HOST="$DB_HOST"
export POSTGRES_PORT="$DB_PORT"
export POSTGRES_DB="$DB_NAME"
export POSTGRES_USER="$DB_USER"
export POSTGRES_PASSWORD="$DB_PASSWORD"

nohup go run ./cmd/api >"$LOG_FILE" 2>&1 &
echo $! > "$PID_FILE"

echo "[run] waiting for http://localhost:8080/health ..."
for i in $(seq 1 60); do
  if curl -fsS "http://localhost:8080/health" >/dev/null 2>&1; then
    echo "[run] OK"
    exit 0
  fi
  sleep 1
done

echo "[run] ERROR: app didn't become ready. Last logs:" >&2
tail -n 200 "$LOG_FILE" >&2 || true
exit 1
