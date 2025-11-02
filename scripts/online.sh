#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="${ROOT_DIR}/frontend"
BACKEND_DIR="${ROOT_DIR}/backend"

# Ensure frontend dependencies are present before starting servers.
if [ ! -d "${FRONTEND_DIR}/node_modules" ]; then
  echo "Installing frontend dependencies..."
  npm --prefix "${FRONTEND_DIR}" install
fi

API_PORT="${API_PORT:-8880}"
API_ORIGIN="${API_ORIGIN:-http://localhost:${API_PORT}}"

export VITE_API_BASE_URL="${VITE_API_BASE_URL:-${API_ORIGIN}}"
export SHARPEN_DATA_DIR="${SHARPEN_DATA_DIR:-${BACKEND_DIR}/data}"
export SHARPEN_STATIC_DIR="${SHARPEN_STATIC_DIR:-${FRONTEND_DIR}/dist}"
export SHARPEN_STREAMERS_FILE="${SHARPEN_STREAMERS_FILE:-${BACKEND_DIR}/data/streamers.json}"
export SHARPEN_SUBMISSIONS_FILE="${SHARPEN_SUBMISSIONS_FILE:-${BACKEND_DIR}/data/submissions.json}"
export LISTEN_ADDR="${LISTEN_ADDR:-:${API_PORT}}"
export ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sharpen.live}"
export ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme123}"

cleanup() {
  local status=$1
  trap - EXIT INT TERM
  if [ -n "${VITE_PID-}" ] && kill -0 "${VITE_PID}" 2>/dev/null; then
    kill "${VITE_PID}" 2>/dev/null || true
  fi
  if [ -n "${API_PID-}" ] && kill -0 "${API_PID}" 2>/dev/null; then
    kill "${API_PID}" 2>/dev/null || true
  fi
  wait 2>/dev/null || true
  exit "${status}"
}

trap 'cleanup $?' EXIT
trap 'cleanup 130' INT
trap 'cleanup 143' TERM

echo "Starting frontend (Vite) dev server..."
(
  cd "${FRONTEND_DIR}"
  VITE_API_BASE_URL="${VITE_API_BASE_URL}" npm run dev -- --host
) &
VITE_PID=$!

echo "Starting Sharpen Live API server on ${LISTEN_ADDR}..."
(
  cd "${BACKEND_DIR}"
  go run ./cmd/server
) &
API_PID=$!

echo "All services running. Frontend: http://localhost:5173  API: ${API_ORIGIN}"

wait -n "${VITE_PID}" "${API_PID}"
