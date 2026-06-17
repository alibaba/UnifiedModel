#!/usr/bin/env sh
# Stop the incident-investigation demo stack and clean up its containers, network, and
# volumes. Pass --all to also remove the built image. Bring it back with start.sh.
set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_FILE="$DIR/docker-compose.yml"

RMI=""
case "${1:-}" in
  --all|--images) RMI="--rmi local" ;;
  "") ;;
  *) echo "usage: stop.sh [--all]   (--all also removes the built demo image)" >&2; exit 2 ;;
esac

if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif podman compose version >/dev/null 2>&1; then
  COMPOSE="podman compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  echo "error: need 'docker compose' or 'podman compose' (or docker-compose) on PATH" >&2
  exit 1
fi

echo "==> Stopping the demo stack with: $COMPOSE"
# shellcheck disable=SC2086
$COMPOSE -f "$COMPOSE_FILE" down -v --remove-orphans $RMI

echo "==> Stopped. Removed containers, network, and volumes${RMI:+ and the built image}."
[ -z "$RMI" ] && echo "    (run 'stop.sh --all' to also remove the built image)"
echo "    Bring it back: sh \"$DIR/start.sh\""
