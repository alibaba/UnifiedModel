#!/bin/sh
# Seed the demo Elasticsearch with logs matching the incident-investigation log_set's
# storage mapping (timestamp / svc_id / env / severity / log_message / trace_id / span_id /
# http_status / upstream_service / latency_ms / error_code / pod) so get_logs plans return
# rows. Runs once (the es-seed compose service).
set -eu

ES="${ES_URL:-http://elasticsearch:9200}"
INDEX="platform-service-logs-demo"   # matches the plan's index pattern platform-service-logs-*

echo "waiting for Elasticsearch at $ES ..."
until curl -fs "$ES/_cluster/health" >/dev/null 2>&1; do sleep 2; done
echo "Elasticsearch is up."

echo "creating index $INDEX ..."
curl -fs -X PUT "$ES/$INDEX" -H 'Content-Type: application/json' -d '{
  "mappings": { "properties": {
    "timestamp":        { "type": "date" },
    "svc_id":           { "type": "keyword" },
    "env":              { "type": "keyword" },
    "severity":         { "type": "keyword" },
    "log_message":      { "type": "text" },
    "trace_id":         { "type": "keyword" },
    "span_id":          { "type": "keyword" },
    "http_status":      { "type": "keyword" },
    "upstream_service": { "type": "keyword" },
    "latency_ms":       { "type": "long" },
    "error_code":       { "type": "keyword" },
    "pod":              { "type": "keyword" }
  } }
}' >/dev/null 2>&1 || echo "  (index may already exist; continuing)"

echo "bulk loading logs ..."
curl -fs -X POST "$ES/$INDEX/_bulk" \
  -H 'Content-Type: application/x-ndjson' \
  --data-binary @/logs.ndjson >/dev/null

curl -fs -X POST "$ES/$INDEX/_refresh" >/dev/null 2>&1 || true
echo "seed complete: $(curl -fs "$ES/$INDEX/_count")"
