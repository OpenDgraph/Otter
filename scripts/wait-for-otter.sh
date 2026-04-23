#!/usr/bin/env bash
# Wait until Otter's HTTP proxy responds on /health, and (optionally) until a
# DQL query round-trip against /query succeeds. Used by `make e2e` between
# `docker compose up` and the test run so the E2E suite starts against a
# ready stack.
#
# Environment:
#   OTTER_HTTP   base URL for the proxy  (default http://localhost:8084)
#   OTTER_WS     base URL for websocket  (default http://localhost:8089)
#   WAIT_SECS    total timeout in seconds (default 90)
#
# Exit codes:
#   0   stack is ready
#   1   timed out

set -euo pipefail

OTTER_HTTP="${OTTER_HTTP:-http://localhost:8084}"
OTTER_WS="${OTTER_WS:-http://localhost:8089}"
WAIT_SECS="${WAIT_SECS:-90}"

start_ts=$(date +%s)
deadline=$(( start_ts + WAIT_SECS ))

log() { printf '[wait-for-otter] %s\n' "$*" >&2; }

wait_endpoint_http() {
  local url="$1"
  local label="$2"
  while true; do
    if curl -sSf --max-time 2 "$url" >/dev/null 2>&1; then
      log "ready: $label ($url)"
      return 0
    fi
    now=$(date +%s)
    if (( now >= deadline )); then
      log "timeout waiting for $label after ${WAIT_SECS}s: $url"
      return 1
    fi
    sleep 1
  done
}

wait_tcp() {
  local host="$1"
  local port="$2"
  local label="$3"
  while true; do
    if (exec 3<>/dev/tcp/"$host"/"$port") 2>/dev/null; then
      exec 3<&- 3>&-
      log "ready: $label ($host:$port)"
      return 0
    fi
    now=$(date +%s)
    if (( now >= deadline )); then
      log "timeout waiting for $label after ${WAIT_SECS}s: $host:$port"
      return 1
    fi
    sleep 1
  done
}

parse_host() { echo "$1" | sed -E 's#^https?://##; s#/.*$##' | cut -d: -f1; }
parse_port() {
  local hp
  hp=$(echo "$1" | sed -E 's#^https?://##; s#/.*$##')
  case "$hp" in
    *:*) echo "${hp##*:}" ;;
    *)   echo "80" ;;
  esac
}

log "waiting for Otter stack (timeout=${WAIT_SECS}s)"
wait_endpoint_http "${OTTER_HTTP}/health" "otter /health"
# The WebSocket handler does not answer a plain HTTP GET with 2xx, so probe
# the raw TCP port instead of relying on curl.
wait_tcp "$(parse_host "$OTTER_WS")" "$(parse_port "$OTTER_WS")" "otter websocket port"

# Round-trip a trivial DQL query through Otter to make sure a backend is
# actually wired. Failing here means compose came up but Otter cannot reach
# Dgraph yet.
probe_query='{ q(func: has(name), first: 1) { uid } }'
while true; do
  if resp=$(curl -sSf --max-time 5 -X POST \
      -H 'Content-Type: application/dql' \
      --data "$probe_query" \
      "${OTTER_HTTP}/query" 2>/dev/null); then
    log "ready: /query round-trip ok"
    log "stack up in $(( $(date +%s) - start_ts ))s"
    exit 0
  fi
  now=$(date +%s)
  if (( now >= deadline )); then
    log "timeout waiting for /query round-trip after ${WAIT_SECS}s"
    exit 1
  fi
  sleep 1
done
