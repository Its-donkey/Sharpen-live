#!/usr/bin/env bash
set -euo pipefail

# Ports to clean up; override by passing port numbers as args.
ports=()
if (( $# > 0 )); then
  ports=("$@")
else
  ports=(8880 4173)
fi

if ! command -v lsof >/dev/null 2>&1; then
  echo "Error: lsof is required but not installed or not in PATH." >&2
  exit 1
fi

kill_for_port() {
  local port="$1"
  local pids
  pids=$(lsof -ti tcp:"$port" -sTCP:LISTEN 2>/dev/null || true)
  if [[ -z "$pids" ]]; then
    echo "No listener found on port $port"
    return
  fi
  echo "Killing PID(s) on port $port: $pids"
  if ! kill -TERM $pids 2>/dev/null; then
    echo "Non-terminating or permission issue, retrying with sudo…"
    sudo kill -TERM $pids
  fi
}

for port in "${ports[@]}"; do
  kill_for_port "$port"
done
