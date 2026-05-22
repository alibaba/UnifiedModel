#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_ADDR="${API_ADDR:-${UMODEL_API_ADDR:-:8080}}"
API_URL="${API_URL:-${UMODEL_API_URL:-http://localhost:8080}}"
WEB_PORT="${WEB_PORT:-${UMODEL_WEB_PORT:-5173}}"
DATA_ROOT="${DATA_ROOT:-${UMODEL_DATA:-data}}"
GRAPHSTORE="${GRAPHSTORE:-${UMODEL_GRAPHSTORE:-file.memory}}"
PID_DIR="${PID_DIR:-${ROOT_DIR}/.run}"
LOG_DIR="${LOG_DIR:-${PID_DIR}/logs}"

case "${PID_DIR}" in
  /*) ;;
  *) PID_DIR="${ROOT_DIR}/${PID_DIR}" ;;
esac
case "${LOG_DIR}" in
  /*) ;;
  *) LOG_DIR="${ROOT_DIR}/${LOG_DIR}" ;;
esac

port_from_endpoint() {
  local endpoint="${1:-}"
  endpoint="${endpoint#http://}"
  endpoint="${endpoint#https://}"
  endpoint="${endpoint%%/*}"

  if [[ "${endpoint}" =~ :([0-9]+)$ ]]; then
    echo "${BASH_REMATCH[1]}"
    return
  fi
  if [[ "${endpoint}" =~ ^[0-9]+$ ]]; then
    echo "${endpoint}"
    return
  fi
}

port_listeners() {
  local port="$1"
  if [[ -z "${port}" ]] || ! command -v lsof >/dev/null 2>&1; then
    echo "unknown"
    return
  fi

  local pids
  pids="$(lsof -nP -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null | sort -u || true)"
  if [[ -z "${pids}" ]]; then
    echo "none"
  else
    echo "${pids//$'\n'/, }"
  fi
}

show_pid_file() {
  local label="$1"
  local pid_file="$2"
  local log_file="$3"

  if [[ ! -f "${pid_file}" ]]; then
    echo "  ${label}: not started by make"
    return
  fi

  local pid
  pid="$(cat "${pid_file}" 2>/dev/null || true)"
  if [[ -z "${pid}" ]]; then
    echo "  ${label}: invalid pid file ${pid_file}"
    return
  fi

  if kill -0 "${pid}" >/dev/null 2>&1; then
    echo "  ${label}: running pid ${pid}"
  else
    echo "  ${label}: stopped stale pid ${pid}"
  fi
  echo "    log: ${log_file}"
}

show_api_health() {
  local response
  response="$(curl -fsS "${API_URL}/healthz" 2>/dev/null || true)"
  if [[ -z "${response}" ]]; then
    echo "  API health: unavailable at ${API_URL}/healthz"
  else
    echo "  API health: ${response}"
  fi
}

show_web_health() {
  local web_url="http://localhost:${WEB_PORT}"
  if curl -fsS "${web_url}" >/dev/null 2>&1; then
    echo "  Web health: reachable at ${web_url}"
  else
    echo "  Web health: unavailable at ${web_url}"
  fi
}

API_PORT="${API_PORT:-$(port_from_endpoint "${API_ADDR}")}"
if [[ -z "${API_PORT}" ]]; then
  API_PORT="$(port_from_endpoint "${API_URL}")"
fi

cat <<EOF
UModel status
  graphstore: ${GRAPHSTORE}
  data root: ${DATA_ROOT}
  pid dir: ${PID_DIR}
  log dir: ${LOG_DIR}

Processes
EOF
show_pid_file "dev api" "${PID_DIR}/openumodel-dev-api.pid" "${LOG_DIR}/dev-api.log"
show_pid_file "dev web" "${PID_DIR}/openumodel-dev-web.pid" "${LOG_DIR}/dev-web.log"
show_pid_file "deploy" "${PID_DIR}/openumodel-deploy.pid" "${LOG_DIR}/deploy.log"

cat <<EOF

Ports
  API ${API_PORT:-unknown}: $(port_listeners "${API_PORT:-}")
  Web ${WEB_PORT}: $(port_listeners "${WEB_PORT}")

Health
EOF
show_api_health
show_web_health
