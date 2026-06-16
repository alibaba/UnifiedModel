#!/bin/sh
# Seed the demo Elasticsearch with logs matching the multi-domain-quickstart log_set's
# storage mapping (event_time / svc_id / env / severity / log_message / trace_id) so
# get_logs plans return rows. Runs once (the es-seed compose service).
set -eu

ES="${ES_URL:-http://elasticsearch:9200}"
INDEX="devops-service-logs-demo"   # matches the plan's index pattern devops-service-logs-*

echo "waiting for Elasticsearch at $ES ..."
until curl -fs "$ES/_cluster/health" >/dev/null 2>&1; do sleep 2; done
echo "Elasticsearch is up."

echo "creating index $INDEX ..."
curl -fs -X PUT "$ES/$INDEX" -H 'Content-Type: application/json' -d '{
  "mappings": { "properties": {
    "event_time":  { "type": "date" },
    "svc_id":      { "type": "keyword" },
    "env":         { "type": "keyword" },
    "severity":    { "type": "keyword" },
    "log_message": { "type": "text" },
    "trace_id":    { "type": "keyword" }
  } }
}' >/dev/null 2>&1 || echo "  (index may already exist; continuing)"

echo "bulk loading logs ..."
curl -fs -X POST "$ES/$INDEX/_bulk" \
  -H 'Content-Type: application/x-ndjson' \
  --data-binary @/logs.ndjson >/dev/null

curl -fs -X POST "$ES/$INDEX/_refresh" >/dev/null 2>&1 || true
echo "seed complete: $(curl -fs "$ES/$INDEX/_count")"
