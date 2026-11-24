#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_BIN="${PYTHON:-python3}"
PNPM_BIN="${PNPM:-pnpm}"
PNPM_VERSION="${PNPM_VERSION:-}"
API_URL="${API_URL:-${UMODEL_API_URL:-http://localhost:8080}}"
WEB_PORT="${WEB_PORT:-${UMODEL_WEB_PORT:-5173}}"
CHECK_OPTIONAL="${CHECK_OPTIONAL:-0}"
WEB_PM=()
USE_NPM_SCRIPTS=0
USE_EXISTING_NODE_MODULES=0

version_at_least() {
  local version="$1"
  local minimum="$2"
  awk -v version="${version}" -v minimum="${minimum}" '
    BEGIN {
      split(version, v, ".")
      split(minimum, m, ".")
      for (i = 1; i <= 3; i++) {
        vi = (v[i] == "" ? 0 : v[i]) + 0
        mi = (m[i] == "" ? 0 : m[i]) + 0
        if (vi > mi) exit 0
        if (vi < mi) exit 1
      }
      exit 0
    }
  '
}

normalize_version() {
  echo "$1" | sed -E 's/^[^0-9]*//; s/[^0-9.].*$//'
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

detect_pnpm_version() {
  if [[ -n "${PNPM_VERSION}" ]]; then
    echo "${PNPM_VERSION}"
    return
  fi

  if [[ ! -f "${ROOT_DIR}/web/package.json" ]]; then
    echo "9"
    return
  fi

  if command_exists node; then
    node -e 'const pkg = require(process.argv[1]); const pm = String(pkg.packageManager || "pnpm@9"); const match = pm.match(/^pnpm@(.+)$/); console.log(match ? match[1] : "9");' "${ROOT_DIR}/web/package.json"
    return
  fi

  echo "9"
}

check_go() {
  if ! command_exists go; then
    echo "ERROR: Go 1.26+ is required." >&2
    return 1
  fi

  local raw version
  raw="$(go env GOVERSION 2>/dev/null || go version | awk '{print $3}')"
  version="$(normalize_version "${raw}")"
  if ! version_at_least "${version}" "1.26"; then
    echo "ERROR: Go 1.26+ is required; found ${raw}." >&2
    return 1
  fi
  echo "OK: Go ${raw}"
}

check_python() {
  if command_exists "${PYTHON_BIN}" && \
     "${PYTHON_BIN}" -c 'import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)' >/dev/null 2>&1; then
    echo "OK: $(${PYTHON_BIN} --version 2>&1) ($(command -v ${PYTHON_BIN}))"
    return 0
  fi

  local detail
  if command_exists "${PYTHON_BIN}"; then
    detail="found $(${PYTHON_BIN} --version 2>&1) at $(command -v ${PYTHON_BIN})"
  else
    detail="${PYTHON_BIN} not found"
  fi

  local candidate
  for candidate in \
    "${CONDA_PREFIX:+${CONDA_PREFIX}/bin/python}" \
    "${VIRTUAL_ENV:+${VIRTUAL_ENV}/bin/python}" \
    "python"; do
    [[ -z "${candidate}" ]] && continue
    if command_exists "${candidate}" && \
       "${candidate}" -c 'import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)' >/dev/null 2>&1; then
      echo "ERROR: Python 3.10+ is required; ${detail}." >&2
      echo "       Compatible Python detected at $(command -v ${candidate}) ($(${candidate} --version 2>&1))." >&2
      echo "       Retry with: make <target> PYTHON=${candidate}" >&2
      echo "       Or adjust PATH so '${PYTHON_BIN}' resolves to it (an activated env may be behind /usr/bin in PATH)." >&2
      return 1
    fi
  done

  echo "ERROR: Python 3.10+ is required; ${detail}." >&2
  return 1
}

check_node() {
  if ! command_exists node; then
    echo "ERROR: Node.js 22+ is required for the Web UI." >&2
    return 1
  fi

  if ! node -e 'const major = Number(process.versions.node.split(".")[0]); process.exit(major >= 22 ? 0 : 1)' >/dev/null 2>&1; then
    echo "ERROR: Node.js 22+ is required; found $(node -v)." >&2
    return 1
  fi
  echo "OK: Node $(node -v)"
}

check_web_package_manager() {
  if [[ ! -f "${ROOT_DIR}/web/package.json" ]]; then
    echo "WARN: web/package.json is not present; skipping Web UI package manager check." >&2
    return
  fi

  local expected
  expected="$(detect_pnpm_version)"

  if command_exists "${PNPM_BIN}"; then
    local version
    version="$(${PNPM_BIN} --version 2>/dev/null || echo 0)"
    if ! version_at_least "${version}" "9"; then
      echo "ERROR: pnpm 9+ is required; found ${version}." >&2
      return 1
    fi
    echo "OK: pnpm ${version}"
    return
  fi

  if [[ "${PNPM_BIN}" != "pnpm" ]]; then
    echo "ERROR: configured PNPM=${PNPM_BIN} was not found." >&2
    return 1
  fi

  if command_exists corepack; then
    echo "OK: corepack is available for pnpm@${expected}"
    return
  fi

  if command_exists npm; then
    echo "OK: npm $(npm -v) is available to run pnpm@${expected} with npm exec"
    return
  fi

  if [[ -d "${ROOT_DIR}/web/node_modules" ]]; then
    echo "WARN: no pnpm/corepack/npm found, but web/node_modules exists; build/dev can use existing local packages." >&2
    return
  fi

  echo "ERROR: pnpm 9+, corepack, or npm is required to install Web UI dependencies." >&2
  return 1
}

web_node_modules_satisfied() {
  [[ -d "${ROOT_DIR}/web/node_modules" ]] || return 1
  command_exists node || return 1

  node - "${ROOT_DIR}/web/package.json" <<'NODE'
const { createRequire } = require('node:module')
const { readFileSync } = require('node:fs')
const { resolve } = require('node:path')

const packagePath = resolve(process.argv[2])
const pkg = JSON.parse(readFileSync(packagePath, 'utf8'))
const req = createRequire(packagePath)
const dependencies = {
  ...(pkg.dependencies || {}),
  ...(pkg.devDependencies || {}),
}
const missing = []

for (const name of Object.keys(dependencies)) {
  try {
    req.resolve(name)
  } catch {
    missing.push(name)
  }
}

if (missing.length > 0) {
  console.error(`Missing Web UI dependencies in web/node_modules: ${missing.join(', ')}`)
  process.exit(1)
}
NODE
}

check_optional_java() {
  local failed=0
  if command_exists java; then
    echo "OK: $(java -version 2>&1 | head -n 1)"
  else
    echo "WARN: Java is not installed; verify-java will be unavailable." >&2
    failed=1
  fi

  if command_exists mvn; then
    echo "OK: Maven $(mvn -v 2>/dev/null | head -n 1)"
  else
    echo "WARN: Maven is not installed; verify-java dependency resolution will be skipped." >&2
    failed=1
  fi

  if [[ "${CHECK_OPTIONAL}" == "1" && "${failed}" != "0" ]]; then
    return 1
  fi
}

check_env() {
  local failed=0
  check_go || failed=1
  check_python || failed=1
  check_node || failed=1
  check_web_package_manager || failed=1
  check_optional_java || failed=1

  if [[ "${failed}" != "0" ]]; then
    echo "Environment check failed." >&2
    return 1
  fi
  echo "Environment check passed."
}

resolve_web_tooling() {
  local purpose="${1:-install}"
  local expected
  expected="$(detect_pnpm_version)"

  if command_exists "${PNPM_BIN}"; then
    WEB_PM=("${PNPM_BIN}")
    return
  fi

  if [[ "${PNPM_BIN}" != "pnpm" ]]; then
    echo "Configured PNPM=${PNPM_BIN} was not found." >&2
    return 1
  fi

  if command_exists corepack; then
    WEB_PM=(corepack pnpm)
    return
  fi

  if [[ "${purpose}" != "install" && -d "${ROOT_DIR}/web/node_modules" ]] && command_exists npm && web_node_modules_satisfied; then
    USE_NPM_SCRIPTS=1
    return
  fi

  if command_exists npm; then
    WEB_PM=(npm exec --yes --package "pnpm@${expected}" -- pnpm)
    return
  fi

  if [[ -d "${ROOT_DIR}/web/node_modules" ]] && web_node_modules_satisfied; then
    USE_EXISTING_NODE_MODULES=1
    return
  fi

  echo "pnpm 9+, corepack, or npm is required to install missing Web UI dependencies." >&2
  return 1
}

web_install() {
  USE_NPM_SCRIPTS=0
  USE_EXISTING_NODE_MODULES=0
  resolve_web_tooling install

  if [[ "${USE_EXISTING_NODE_MODULES}" == "1" ]]; then
    echo "Skipping Web UI dependency install because web/node_modules already exists and no package manager is available."
    return
  fi

  echo "Installing Web UI dependencies with: ${WEB_PM[*]} install --frozen-lockfile"
  (cd "${ROOT_DIR}/web" && "${WEB_PM[@]}" install --frozen-lockfile)
}

web_build() {
  USE_NPM_SCRIPTS=0
  USE_EXISTING_NODE_MODULES=0
  resolve_web_tooling build

  if [[ "${USE_NPM_SCRIPTS}" == "1" ]]; then
    echo "pnpm was not found; using existing web/node_modules with npm run build."
    (cd "${ROOT_DIR}/web" && npm run build)
    return
  fi

  if [[ "${USE_EXISTING_NODE_MODULES}" == "1" ]]; then
    echo "pnpm and npm were not found; using existing web/node_modules binaries."
    (cd "${ROOT_DIR}/web" && ./node_modules/.bin/tsc --noEmit && ./node_modules/.bin/vite build)
    return
  fi

  (cd "${ROOT_DIR}/web" && "${WEB_PM[@]}" install --frozen-lockfile && "${WEB_PM[@]}" build)
}

web_dev() {
  USE_NPM_SCRIPTS=0
  USE_EXISTING_NODE_MODULES=0
  resolve_web_tooling dev

  if [[ "${USE_NPM_SCRIPTS}" == "1" ]]; then
    echo "pnpm was not found; using existing web/node_modules with npm run dev."
    (cd "${ROOT_DIR}/web" && UMODEL_API_TARGET="${API_URL}" npm run dev -- --port "${WEB_PORT}" --strictPort)
    return
  fi

  if [[ "${USE_EXISTING_NODE_MODULES}" == "1" ]]; then
    echo "pnpm and npm were not found; using existing web/node_modules/.bin/vite."
    (cd "${ROOT_DIR}/web" && UMODEL_API_TARGET="${API_URL}" ./node_modules/.bin/vite --host 0.0.0.0 --port "${WEB_PORT}" --strictPort)
    return
  fi

  (cd "${ROOT_DIR}/web" && "${WEB_PM[@]}" install --frozen-lockfile && UMODEL_API_TARGET="${API_URL}" "${WEB_PM[@]}" dev --port "${WEB_PORT}" --strictPort)
}

install_env() {
  check_go
  check_python
  check_node
  check_web_package_manager

  if [[ ! -d "${ROOT_DIR}/.venv" ]]; then
    echo "Creating Python virtual environment at .venv"
    "${PYTHON_BIN}" -m venv "${ROOT_DIR}/.venv"
  fi
  echo "Installing Python dependencies from tools/requirements.txt"
  "${ROOT_DIR}/.venv/bin/python" -m pip install -r "${ROOT_DIR}/tools/requirements.txt" -q

  echo "Downloading Go module dependencies"
  (cd "${ROOT_DIR}" && go mod download)
  (cd "${ROOT_DIR}/sdk/go" && go mod download)

  web_install

  if command_exists mvn; then
    echo "Resolving Java SDK dependencies"
    (cd "${ROOT_DIR}/generated/java" && mvn dependency:resolve -q -B 2>/dev/null || true)
  else
    echo "Skipping Java SDK dependency resolution because Maven is not installed."
  fi
}

case "${1:-check}" in
  check)
    check_env
    ;;
  install)
    install_env
    ;;
  web-install)
    web_install
    ;;
  web-build)
    web_build
    ;;
  web-dev)
    web_dev
    ;;
  *)
    echo "Usage: $0 {check|install|web-install|web-build|web-dev}" >&2
    exit 2
    ;;
esac
