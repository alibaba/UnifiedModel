#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_ADDR="${API_ADDR:-${UMODEL_API_ADDR:-:8080}}"
API_URL="${API_URL:-${UMODEL_API_URL:-http://localhost:8080}}"
DATA_ROOT="${DATA_ROOT:-${UMODEL_DATA:-data}}"
GRAPHSTORE="${GRAPHSTORE:-${UMODEL_GRAPHSTORE:-file.memory}}"
GO_TAGS="${GO_TAGS:-}"
QUICKSTART="${QUICKSTART:-0}"
QUICKSTART_WORKSPACE="${QUICKSTART_WORKSPACE:-demo}"
QUICKSTART_SAMPLE="${QUICKSTART_SAMPLE:-multi-domain-quickstart}"
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

DEPLOY_PID_FILE="${PID_DIR}/openumodel-deploy.pid"
DEPLOY_LOG="${LOG_DIR}/deploy.log"

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

is_enabled() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

ensure_port_free() {
  local name="$1"
  local port="$2"

  if [[ -z "${port}" ]] || ! command -v lsof >/dev/null 2>&1; then
    return
  fi

  local pids
  pids="$(lsof -nP -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null | sort -u || true)"
  if [[ -n "${pids}" ]]; then
    echo "${name} port ${port} is already in use by pid(s): ${pids//$'\n'/, }" >&2
    echo "Stop it with make stop-all, or choose another port." >&2
    exit 1
  fi
}

assert_pid_file_stale_or_absent() {
  if [[ ! -f "${DEPLOY_PID_FILE}" ]]; then
    return
  fi

  local pid
  pid="$(cat "${DEPLOY_PID_FILE}" 2>/dev/null || true)"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
    echo "UModel deploy already appears to be running with pid ${pid}." >&2
    echo "Use make status to inspect it or make stop-all before starting again." >&2
    exit 1
  fi

  rm -f "${DEPLOY_PID_FILE}"
}

wait_for_api() {
  local attempts=60

  for ((attempt = 1; attempt <= attempts; attempt += 1)); do
    if ! kill -0 "${DEPLOY_PID}" >/dev/null 2>&1; then
      echo "UModel deploy server exited before becoming healthy." >&2
      tail -n 60 "${DEPLOY_LOG}" >&2 || true
      return 1
    fi

    if curl -fsS "${API_URL}/healthz" >/dev/null 2>&1; then
      echo "UModel deploy server is healthy."
      return 0
    fi

    sleep 0.5
  done

  echo "UModel deploy server did not become healthy at ${API_URL}/healthz." >&2
  tail -n 60 "${DEPLOY_LOG}" >&2 || true
  return 1
}

API_PORT="${API_PORT:-$(port_from_endpoint "${API_ADDR}")}"
if [[ -z "${API_PORT}" ]]; then
  API_PORT="$(port_from_endpoint "${API_URL}")"
fi

if [[ ! -d "${ROOT_DIR}/web/dist" ]]; then
  echo "web/dist does not exist. Run make build-ui before deploy." >&2
  exit 1
fi

ensure_port_free "API" "${API_PORT}"
mkdir -p "${PID_DIR}" "${LOG_DIR}"
assert_pid_file_stale_or_absent

if is_enabled "${QUICKSTART}"; then
  echo "Starting UModel production server at ${API_URL} (graphstore=${GRAPHSTORE}, data=${DATA_ROOT}, quickstart=${QUICKSTART_WORKSPACE}/${QUICKSTART_SAMPLE})"
else
  echo "Starting UModel production server at ${API_URL} (graphstore=${GRAPHSTORE}, data=${DATA_ROOT})"
fi
(
  cd "${ROOT_DIR}"
  go_run=(go run)
  if [[ -n "${GO_TAGS}" ]]; then
    go_run+=(-tags "${GO_TAGS}")
  fi
  go_run+=(./cmd/umodel-server --addr "${API_ADDR}" --data "${DATA_ROOT}" --graphstore "${GRAPHSTORE}" --ui-dir web/dist)
  if is_enabled "${QUICKSTART}"; then
    go_run+=(--quickstart --quickstart-workspace "${QUICKSTART_WORKSPACE}" --quickstart-sample "${QUICKSTART_SAMPLE}")
  fi
  exec nohup "${go_run[@]}" >> "${DEPLOY_LOG}" 2>&1 < /dev/null
) &
DEPLOY_PID="$!"
echo "${DEPLOY_PID}" > "${DEPLOY_PID_FILE}"

if ! wait_for_api; then
  kill "${DEPLOY_PID}" >/dev/null 2>&1 || true
  rm -f "${DEPLOY_PID_FILE}"
  exit 1
fi

cat <<EOF
UModel deploy is running in the background.
  App: ${API_URL}
  Log: ${DEPLOY_LOG}
Use make status to monitor it and make stop-all to stop it.
EOF
