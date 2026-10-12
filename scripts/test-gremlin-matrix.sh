#!/usr/bin/env bash
# Disposable real-server acceptance matrix. No existing graph is cleared.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
runtime="${UMODEL_CONTAINER_RUNTIME:-docker}"
image="${GREMLIN_TEST_IMAGE:-docker.io/tinkerpop/gremlin-server:3.7.3}"
state="$(mktemp -d)"
state="$(cd "$state" && pwd -P)"
prefix="umodel-gremlin-matrix-$$"
containers=("$prefix-plain" "$prefix-secure" "$prefix-persistent" "$prefix-transaction")
port_allocator="$prefix-port"
cleanup() {
  "$runtime" rm -f "${containers[@]}" "$port_allocator" >/dev/null 2>&1 || true
  rm -rf "$state"
}
trap cleanup EXIT

fixtures="$repo_root/tests/fixtures/gremlin"
sed 's@/test/graph.properties@/test/persistent.properties@' "$fixtures/server.yaml" > "$state/persistent.yaml"
sed 's@/test/graph.properties@/test/transaction.properties@' "$fixtures/server.yaml" > "$state/transaction.yaml"
openssl req -newkey rsa:2048 -nodes -x509 -days 2 -subj '/CN=localhost' \
  -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' \
  -keyout "$state/server.key" -out "$state/ca.pem" >/dev/null 2>&1
openssl pkcs12 -export -inkey "$state/server.key" -in "$state/ca.pem" \
  -out "$state/server.p12" -passout pass:umodel-test-password
"$runtime" run --rm --user 0 --memory=512m -v "$fixtures:/test:ro" -v "$state:/state" \
  --entrypoint java "$image" -cp 'lib/*:ext/tinkergraph-gremlin/lib/*' \
  groovy.ui.GroovyMain /test/create-credentials.groovy

configs=(/test/server.yaml /test/secure.yaml /state/persistent.yaml /state/transaction.yaml)
ports=()
for i in 0 1 2 3; do
  publish='127.0.0.1::8182'
  if [ "$i" = 2 ]; then
    # Docker can assign a different ephemeral host port after stop/start.
    # Allocate an available port, then pin it for the persistent server so
    # the restart test and its cleanup retain the same GREMLIN_URI.
    "$runtime" run -d --name "$port_allocator" -p "$publish" \
      --entrypoint /bin/sleep "$image" 60 >/dev/null
    binding="$("$runtime" port "$port_allocator" 8182/tcp)"
    "$runtime" rm -f "$port_allocator" >/dev/null
    publish="127.0.0.1:${binding##*:}:8182"
  fi
  "$runtime" run -d --name "${containers[$i]}" --user 0 --memory=768m \
    -p "$publish" -v "$fixtures:/test:ro" -v "$state:/state" \
    --entrypoint /opt/gremlin-server/bin/gremlin-server.sh "$image" "${configs[$i]}" >/dev/null
  binding="$("$runtime" port "${containers[$i]}" 8182/tcp)"
  ports+=("${binding##*:}")
done

for i in 0 1 2 3; do
  ready=0
  for attempt in $(seq 1 60); do
    if [ "$i" = 1 ]; then
      # The secure WsAndHttp endpoint authenticates GraphSON messages with
      # SASL, not plain HTTP requests. A verified WebSocket upgrade proves
      # readiness; the matrix then checks SASL credentials on real requests.
      code="$(curl --silent --max-time 1 --cacert "$state/ca.pem" -o /dev/null -w '%{http_code}' \
        -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
        -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' -H 'Sec-WebSocket-Version: 13' \
        "https://127.0.0.1:${ports[$i]}/gremlin" || true)"
      if [ "$code" = 101 ]; then
        ready=1
        break
      fi
    else
      args=("http://127.0.0.1:${ports[$i]}/gremlin")
      if curl --fail --silent --max-time 2 -H 'Content-Type: application/json' \
        -d '{"gremlin":"g.V().limit(0).count()"}' "${args[@]}" >/dev/null; then
        ready=1
        break
      fi
    fi
    sleep 1
  done
  if [ "$ready" != 1 ]; then
    "$runtime" logs "${containers[$i]}"
    exit 1
  fi
done

export GREMLIN_TIMEOUT=30s UMODEL_TEST_GREMLIN=1 UMODEL_TEST_GRAPHSTORE=gremlin
export UMODEL_TEST_NEO4J=0 UMODEL_TEST_LADYBUG=0
export GREMLIN_SECURE_URI="wss://127.0.0.1:${ports[1]}/gremlin"
export GREMLIN_SECURE_CA="$state/ca.pem"
export GREMLIN_TX_URI="ws://127.0.0.1:${ports[3]}/gremlin"

echo 'Running the full plain WebSocket matrix, including TLS rejection and server rollback.'
GREMLIN_URI="ws://127.0.0.1:${ports[0]}/gremlin" GREMLIN_CA_FILE='' GREMLIN_USERNAME='' GREMLIN_PASSWORD='' make test-gremlin
echo 'Running the full TLS and password-authenticated matrix.'
GREMLIN_URI="$GREMLIN_SECURE_URI" GREMLIN_CA_FILE="$state/ca.pem" \
  GREMLIN_USERNAME=umodel-test-user GREMLIN_PASSWORD=umodel-test-password make test-gremlin
echo 'Checking database restart, graph persistence, disconnected writes, and index recovery.'
GREMLIN_URI="ws://127.0.0.1:${ports[2]}/gremlin" GREMLIN_CA_FILE='' GREMLIN_USERNAME='' GREMLIN_PASSWORD='' \
  UMODEL_CONTAINER_RUNTIME="$runtime" UMODEL_GREMLIN_CONTAINER="${containers[2]}" \
  go test -race -count=1 -run TestGremlinDatabaseRestart ./tests/e2e
