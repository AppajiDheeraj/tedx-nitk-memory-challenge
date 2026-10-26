#!/usr/bin/env bash
# Ping the Render Free service so it does not spin down after ~15 minutes of idle.
# Usage:
#   ./scripts/keep-awake.sh
#   KEEP_AWAKE_URL=https://example.onrender.com/health INTERVAL_SECONDS=600 ./scripts/keep-awake.sh
set -euo pipefail

URL="${KEEP_AWAKE_URL:-https://tedx-nitk-memory-challenge.onrender.com/health}"
INTERVAL_SECONDS="${INTERVAL_SECONDS:-600}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"

if ! command -v curl >/dev/null 2>&1; then
  echo "keep-awake: curl is required" >&2
  exit 1
fi

echo "keep-awake: pinging ${URL} every ${INTERVAL_SECONDS}s (Ctrl+C to stop)"

while true; do
  stamp="$(date '+%Y-%m-%d %H:%M:%S')"
  if http_code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "${TIMEOUT_SECONDS}" "${URL}")"; then
    echo "${stamp}  ${http_code}  ${URL}"
  else
    echo "${stamp}  FAIL  ${URL}" >&2
  fi
  sleep "${INTERVAL_SECONDS}"
done
