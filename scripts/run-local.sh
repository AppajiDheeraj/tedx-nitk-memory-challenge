#!/usr/bin/env bash
# Start the memory game on http://localhost:8082
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env ]]; then
  echo "Missing .env. Copy .env.example to .env and set DATABASE_URL." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is not set in .env" >&2
  exit 1
fi

export COOKIE_SECURE=false
export PORT="${PORT:-8082}"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed. Install it from https://go.dev/dl/" >&2
  exit 1
fi

echo "Starting TEDx memory challenge at http://localhost:${PORT}"
exec go run .
