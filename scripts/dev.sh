#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_ADDR="${API_ADDR:-${UMODEL_API_ADDR:-:8080}}"
API_URL="${API_URL:-${UMODEL_API_URL:-http://localhost:8080}}"
WEB_PORT="${WEB_PORT:-${UMODEL_WEB_PORT:-5173}}"
DATA_ROOT="${DATA_ROOT:-${UMODEL_DATA:-data}}"
GRAPHSTORE="${GRAPHSTORE:-${UMODEL_GRAPHSTORE:-file.memory}}"
GO_TAGS="${GO_TAGS:-}"
if [[ -z "${GO_TAGS}" && "${GRAPHSTORE}" == "local.ladybug" ]]; then
  GO_TAGS="ladybug"
fi
QUICKSTART="${QUICKSTART:-0}"
QUICKSTART_WORKSPACE="${QUICKSTART_WORKSPACE:-demo}"
QUICKSTART_SAMPLE="${QUICKSTART_SAMPLE:-multi-domain-quickstart}"
PNPM="${PNPM:-pnpm}"
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

API_PID_FILE="${PID_DIR}/openumodel-dev-api.pid"
WEB_PID_FILE="${PID_DIR}/openumodel-dev-web.pid"
API_LOG="${LOG_DIR}/dev-api.log"
WEB_LOG="${LOG_DIR}/dev-web.log"
API_BIN="${PID_DIR}/bin/umodel-server"
WEB_PM=()
PNPM_VERSION="${PNPM_VERSION:-}"

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

if ! node -e 'const major = Number(process.versions.node.split(".")[0]); process.exit(major >= 22 ? 0 : 1)' >/dev/null 2>&1; then
  echo "Node.js 22 or newer is required. Install or activate a compatible Node.js runtime." >&2
  exit 1
fi

if [[ -z "${PNPM_VERSION}" ]]; then
  PNPM_VERSION="$(node -e 'const pkg = require(process.argv[1]); const pm = String(pkg.packageManager || "pnpm@9"); const match = pm.match(/^pnpm@(.+)$/); console.log(match ? match[1] : "9");' "${ROOT_DIR}/web/package.json")"
fi

resolve_web_tooling() {
  if command -v "${PNPM}" >/dev/null 2>&1; then
    WEB_PM=("${PNPM}")
    return
  fi

  if [[ "${PNPM}" != "pnpm" ]]; then
    echo "Configured PNPM=${PNPM} was not found. Set PNPM to a valid pnpm binary." >&2
    exit 1
  fi

  if command -v corepack >/dev/null 2>&1; then
    WEB_PM=(corepack "pnpm@${PNPM_VERSION}")
    return
  fi

  if command -v npm >/dev/null 2>&1; then
    WEB_PM=(npm exec --yes --package "pnpm@${PNPM_VERSION}" -- pnpm)
    echo "pnpm was not found; using npm exec with pnpm@${PNPM_VERSION}."
    return
  fi

  echo "pnpm 9 or newer is required to install Web UI dependencies." >&2
  echo "Install pnpm, enable corepack, or provide npm so make dev can run npm exec pnpm@${PNPM_VERSION}." >&2
  exit 1
}

install_web_dependencies() {
  echo "Installing Web UI dependencies with: ${WEB_PM[*]} install --frozen-lockfile"
  (cd "${ROOT_DIR}/web" && "${WEB_PM[@]}" install --frozen-lockfile >> "${WEB_LOG}" 2>&1)
}

start_web_server() {
  local vite_bin="${ROOT_DIR}/web/node_modules/.bin/vite"
  if [[ ! -x "${vite_bin}" ]]; then
    echo "Vite binary was not found at ${vite_bin}. Run make install-env or check the Web UI install log." >&2
    exit 1
  fi

  cd "${ROOT_DIR}/web"
  exec nohup env UMODEL_API_TARGET="${API_URL}" "${vite_bin}" --host 0.0.0.0 --port "${WEB_PORT}" --strictPort >> "${WEB_LOG}" 2>&1 < /dev/null
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

stop_port_listener() {
  local name="$1"
  local port="$2"

  if [[ -z "${port}" ]] || ! command -v lsof >/dev/null 2>&1; then
    return
  fi

  local pids
  pids="$(lsof -nP -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null | sort -u || true)"
  if [[ -z "${pids}" ]]; then
    return
  fi

  local pid
  while IFS= read -r pid; do
    [[ -z "${pid}" ]] && continue
    echo "Stopping ${name} listener pid ${pid} on port ${port}."
    kill "${pid}" >/dev/null 2>&1 || true
  done <<< "${pids}"
}

cleanup_after_failure() {
  if [[ -n "${WEB_PID:-}" ]]; then
    kill "${WEB_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${API_PID:-}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
  fi
  stop_port_listener "API" "${API_PORT:-}"
  stop_port_listener "Web" "${WEB_PORT:-}"
  rm -f "${WEB_PID_FILE}" "${API_PID_FILE}"
}

assert_pid_file_stale_or_absent() {
  local name="$1"
  local pid_file="$2"

  if [[ ! -f "${pid_file}" ]]; then
    return
  fi

  local pid
  pid="$(cat "${pid_file}" 2>/dev/null || true)"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
    echo "${name} already appears to be running with pid ${pid}." >&2
    echo "Use make status to inspect it or make stop-all before starting again." >&2
    exit 1
  fi

  rm -f "${pid_file}"
}

wait_for_api() {
  local attempts=60
  local api_status=0

  for ((attempt = 1; attempt <= attempts; attempt += 1)); do
    if ! kill -0 "${API_PID}" >/dev/null 2>&1; then
      wait "${API_PID}" || api_status="$?"
      echo "UModel API exited before becoming healthy (status ${api_status})." >&2
      exit "${api_status}"
    fi

    if curl -fsS "${API_URL}/healthz" >/dev/null 2>&1; then
      echo "UModel API is healthy."
      return
    fi

    sleep 0.5
  done

  echo "UModel API did not become healthy at ${API_URL}/healthz." >&2
  exit 1
}

wait_for_web() {
  local attempts=60
  local web_url="http://localhost:${WEB_PORT}"

  for ((attempt = 1; attempt <= attempts; attempt += 1)); do
    if ! kill -0 "${WEB_PID}" >/dev/null 2>&1; then
      echo "UModel Web exited before becoming reachable." >&2
      tail -n 40 "${WEB_LOG}" >&2 || true
      return 1
    fi

    if curl -fsS "${web_url}" >/dev/null 2>&1; then
      echo "UModel Web is reachable."
      return 0
    fi

    sleep 0.5
  done

  echo "UModel Web did not become reachable at ${web_url}." >&2
  tail -n 40 "${WEB_LOG}" >&2 || true
  return 1
}

API_PORT="${API_PORT:-$(port_from_endpoint "${API_ADDR}")}"
if [[ -z "${API_PORT}" ]]; then
  API_PORT="$(port_from_endpoint "${API_URL}")"
fi
resolve_web_tooling
ensure_port_free "API" "${API_PORT}"
ensure_port_free "Web" "${WEB_PORT}"
mkdir -p "${PID_DIR}" "${LOG_DIR}"
assert_pid_file_stale_or_absent "UModel API" "${API_PID_FILE}"
assert_pid_file_stale_or_absent "UModel Web" "${WEB_PID_FILE}"

echo "Installing UModel Web dependencies..."
if ! (
  install_web_dependencies
); then
  echo "UModel Web dependency install failed. See ${WEB_LOG}." >&2
  tail -n 40 "${WEB_LOG}" >&2 || true
  exit 1
fi

if is_enabled "${QUICKSTART}"; then
  echo "Starting UModel API at ${API_URL} (graphstore=${GRAPHSTORE}, data=${DATA_ROOT}, quickstart=${QUICKSTART_WORKSPACE}/${QUICKSTART_SAMPLE})"
else
  echo "Starting UModel API at ${API_URL} (graphstore=${GRAPHSTORE}, data=${DATA_ROOT})"
fi
echo "Building UModel API binary at ${API_BIN}"
mkdir -p "$(dirname "${API_BIN}")"
(
  cd "${ROOT_DIR}"
  go_build=(go build)
  if [[ -n "${GO_TAGS}" ]]; then
    go_build+=(-tags "${GO_TAGS}")
  fi
  go_build+=(-o "${API_BIN}" ./cmd/umodel-server)
  "${go_build[@]}"
)
(
  cd "${ROOT_DIR}"
  server_args=(--addr "${API_ADDR}" --data "${DATA_ROOT}" --graphstore "${GRAPHSTORE}")
  if is_enabled "${QUICKSTART}"; then
    server_args+=(--quickstart --quickstart-workspace "${QUICKSTART_WORKSPACE}" --quickstart-sample "${QUICKSTART_SAMPLE}")
  fi
  exec nohup "${API_BIN}" "${server_args[@]}" >> "${API_LOG}" 2>&1 < /dev/null
) &
API_PID="$!"
echo "${API_PID}" > "${API_PID_FILE}"

if ! wait_for_api; then
  cleanup_after_failure
  exit 1
fi

echo "Starting UModel Web at http://localhost:${WEB_PORT}"
(
  start_web_server
) &
WEB_PID="$!"
echo "${WEB_PID}" > "${WEB_PID_FILE}"

if ! wait_for_web; then
  cleanup_after_failure
  exit 1
fi

cat <<EOF
UModel dev is running in the background.
  API: ${API_URL}
  Web: http://localhost:${WEB_PORT}
  API log: ${API_LOG}
  Web log: ${WEB_LOG}
Use make status to monitor it and make stop-all to stop it.
EOF
