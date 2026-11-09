#!/usr/bin/env python3
"""Execute an agent-format UModel Loki plan through Loki's read-only HTTP API."""

import argparse
import base64
import json
import logging
import os
import ssl
import sys
from urllib import error, parse, request


LOG = logging.getLogger(__name__)


def _auth_header(auth):
    if not auth:
        return None
    kind = auth.get("type")
    if kind == "bearer":
        token = os.environ.get(auth.get("token_env", ""), "")
        if not token:
            raise ValueError("Loki bearer token environment variable is missing or empty")
        return "Bearer " + token
    if kind == "basic":
        username = os.environ.get(auth.get("username_env", ""), "")
        password = os.environ.get(auth.get("password_env", ""), "")
        if not username or not password:
            raise ValueError("Loki basic authentication environment variables are missing or empty")
        encoded = base64.b64encode((username + ":" + password).encode("utf-8"))
        return "Basic " + encoded.decode("ascii")
    raise ValueError("unsupported Loki authentication type")


def execute(plan, opener=None):
    if plan.get("mode") != "plan" or plan.get("query", {}).get("dialect") != "loki_logql":
        raise ValueError("expected a UModel Loki query plan")
    query = plan["query"]
    operation = plan.get("operation")
    if operation not in ("get_logs", "get_metrics"):
        raise ValueError("unsupported Loki plan operation")
    time_range = plan.get("time_range") or {}
    end = time_range.get("to")
    start = time_range.get("from")
    if not end or (operation == "get_logs" or query.get("query_type") == "range") and not start:
        raise ValueError("Loki plan is missing the required time range")

    endpoint = query["endpoint"].rstrip("/")
    prefix = "/" + query.get("api_prefix", "/loki/api/v1").strip("/")
    query_type = query.get("query_type")
    path = "query" if operation == "get_metrics" and query_type == "instant" else "query_range"
    headers = dict(query.get("headers") or {})
    tenant = query.get("tenant")
    if tenant:
        headers[query.get("tenant_header") or "X-Scope-OrgID"] = tenant
    auth_header = _auth_header(query.get("auth"))
    if auth_header:
        headers["Authorization"] = auth_header
    context = ssl.create_default_context() if query.get("tls_verify", True) else ssl._create_unverified_context()
    open_url = opener or (lambda req: request.urlopen(req, timeout=30, context=context))

    results = []
    for item in query.get("queries") or []:
        params = {"query": item["logql"]}
        if path == "query":
            params["time"] = end
        else:
            params.update({"start": start, "end": end})
            if operation == "get_logs":
                params["limit"] = query["size"]
                params["direction"] = query.get("direction", "backward")
            elif query.get("step"):
                params["step"] = query["step"]
        url = endpoint + prefix + "/" + path + "?" + parse.urlencode(params)
        req = request.Request(url, headers=headers, method="GET")
        try:
            with open_url(req) as response:
                payload = json.load(response)
        except error.HTTPError as exc:
            raise RuntimeError("Loki HTTP request failed with status " + str(exc.code)) from exc
        except (error.URLError, TimeoutError, OSError, json.JSONDecodeError) as exc:
            raise RuntimeError("Loki HTTP request or response failed: " + str(exc)) from exc
        if payload.get("status") != "success":
            raise RuntimeError("Loki query failed: " + str(payload.get("error", "unknown error")))
        data = payload.get("data") or {}
        expected = "streams" if operation == "get_logs" else ("vector" if path == "query" else "matrix")
        if data.get("resultType") != expected:
            raise ValueError("unexpected Loki result type: " + str(data.get("resultType")))
        results.append({"name": item.get("name"), "logql": item["logql"], "result": data.get("result") or []})

    if operation == "get_metrics":
        return {"result_type": "vector" if path == "query" else "matrix", "queries": results}

    entries = []
    seen = set()
    for result in results:
        for stream in result["result"]:
            labels = stream.get("stream") or {}
            for timestamp, line in stream.get("values") or []:
                key = (timestamp, json.dumps(labels, sort_keys=True), line)
                if key in seen:
                    continue
                seen.add(key)
                entries.append({"timestamp": timestamp, "labels": labels, "line": line})
    entries.sort(key=lambda row: int(row["timestamp"]), reverse=query.get("direction") != "forward")
    return {"result_type": "streams", "entries": entries[: query["size"]]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("plan", nargs="?", help="Agent-format plan JSON file; stdin when omitted")
    args = parser.parse_args()
    logging.basicConfig(format="%(asctime)s %(levelname)s %(pathname)s:%(lineno)d %(exception_type)s %(message)s")
    try:
        if args.plan:
            with open(args.plan, encoding="utf-8") as source:
                plan = json.load(source)
        else:
            plan = json.load(sys.stdin)
        json.dump(execute(plan), sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, json.JSONDecodeError) as exc:
        LOG.error("Loki plan execution failed: %s", exc, extra={"exception_type": type(exc).__name__})
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
