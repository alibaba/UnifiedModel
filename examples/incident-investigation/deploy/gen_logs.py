#!/usr/bin/env python3
"""Generate ~72h of service logs for the incident-investigation demo and bulk-load them
into the demo Elasticsearch, so the pack's get_logs plans return rows that span the whole
incident (not a single burst). Timestamps are relative to wall-clock now, so the demo
always shows "the last three days". Standard library only.

The log volume and severity follow the modeled timeline (README "Timeline"):

    P0  healthy        [now-72h, now-24h)   sparse INFO across the platform
    T-48h                                   flash-sale promotion scheduled (business event)
    T-24h                                   checkout config change: max_retries 2 -> 5
    P1  retries-up     [now-24h, now-4h)    rising WARN: client retries, slow upstreams
    T-12h                                   payment-gw v3.2.1 rollout (logging change)
    P2  promo breach   [now-4h,  now]       ERROR flood on the payment path (503/504,
                                            circuit-breaker, retry-exhausted)

Fields match the log_set's Elasticsearch storage mapping (timestamp / svc_id / env /
severity / log_message / trace_id / span_id / http_status / upstream_service /
latency_ms / error_code / pod).
"""
import json
import os
import random
import time
import urllib.request

ES = os.environ.get("ES_URL", "http://elasticsearch:9200")
INDEX = "platform-service-logs-demo"   # matches the plan's index pattern platform-service-logs-*
HOUR = 3600
RNG = random.Random(0xC0FFEE)          # deterministic output run to run

# service_id -> short name, mirroring exporter.py / the entity ids the plans substitute.
SVC = {
    "63718b78868895d2590551b27ec6f51c": "payment-gateway",
    "149632df43354373835df2717cb8fb19": "checkout-service",
    "08409c1c4464bf2bc6346c777a2fabc7": "payment-router",
    "6b3718c2b64fd131c1d7c0f1362c6d41": "channel-alipay",
    "3640dfa237be9bc61ced0fd7af8c7dad": "channel-wechatpay",
    "7d6930095ab2e664ae5b49a4f6272539": "channel-unionpay",
    "c6ace8f158e5a0db93a8659ef446ddc1": "risk-control",
    "f25ae2923f5df058b6119ea79e434459": "order-service",
    "9f71098638361b9aa76a350a16f25626": "cart-service",
    "0e8aec70874b99241082f307b4b3b9c5": "inventory-service",
    "60c58dadd2d201fed7dcc1c0b2268139": "user-profile",
    "403ec4bd3891dc52bf5161b9dd65d16e": "search-service",
    "394ba4c250378ded9a5a2c4d5bec47c9": "auth-service",
    "b0bba8793e479be1f44025bb1a544d2a": "api-gateway",
}
ID = {v: k for k, v in SVC.items()}
HEALTHY = ["risk-control", "order-service", "cart-service", "inventory-service",
           "user-profile", "search-service", "auth-service", "api-gateway"]


def pod(name):
    return f"{name.replace('-service', '').replace('-', '')[:10]}-{RNG.randint(1000, 9999):04x}-{RNG.choice('abcdefghjkmnp')}{RNG.randint(10, 99)}"


def doc(ts, name, severity, msg, http="200", upstream="", latency=0, code="", env="prod", tid=None):
    # tid lets a caller correlate several docs into one distributed trace; otherwise random.
    t = tid or ("%016x" % RNG.getrandbits(64))
    sp = "%012x" % RNG.getrandbits(48)
    return {
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(ts)),
        "svc_id": ID[name], "env": env, "severity": severity, "log_message": msg,
        "trace_id": t, "span_id": sp, "http_status": http,
        "upstream_service": upstream, "latency_ms": latency, "error_code": code,
        "pod": pod(name),
    }


PROVIDERS = {"channel-alipay": "alipay-openapi", "channel-wechatpay": "wechatpay-api",
             "channel-unionpay": "unionpay-gw"}


def chain(ts):
    """One failed checkout as a correlated trace flowing down the payment path, so an RCA can
    follow a single request: checkout -> payment-gateway -> payment-router -> channel -> provider.
    Every hop shares the trace_id and orderId; the timeout originates downstream and surfaces up
    as retry exhaustion."""
    ch = RNG.choice(list(PROVIDERS))
    tid = "%016x" % RNG.getrandbits(64)
    oid = RNG.randint(10 ** 9, 10 ** 10)
    lat = RNG.randint(2000, 2600)
    return [
        doc(ts, "checkout-service", "ERROR",
            f"POST /api/checkout/confirm orderId={oid}: payment confirm failed after 5 retries; order abandoned",
            http="504", upstream="payment-gateway", latency=lat + 60, code="PAYMENT_FAILED", tid=tid),
        doc(ts, "payment-gateway", "ERROR",
            f"charge orderId={oid} failed: upstream payment-router timeout after 2000ms; retry 5/5 exhausted",
            http="504", upstream="payment-router", latency=lat, code="UPSTREAM_TIMEOUT", tid=tid),
        doc(ts, "payment-router", "ERROR",
            f"route orderId={oid} -> {ch}: timeout after {lat - 120}ms; circuit half-open, no capacity to retry",
            http="504", upstream=ch, latency=lat - 120, code="UPSTREAM_TIMEOUT", tid=tid),
        doc(ts, ch, "ERROR",
            f"provider gateway timeout ({PROVIDERS[ch]}) for orderId={oid}",
            http="504", upstream=PROVIDERS[ch], latency=lat - 180, code="PROVIDER_TIMEOUT", tid=tid),
    ]


def main():
    now = int(time.time())
    docs = []

    # --- P0 healthy: sparse INFO across the platform, one every ~25 min ---
    t = now - 72 * HOUR
    while t < now - 24 * HOUR:
        name = RNG.choice(HEALTHY + ["checkout-service", "payment-gateway"])
        if name in ("checkout-service", "payment-gateway"):
            docs.append(doc(t, name, "INFO", "charge completed ok (latency within SLO)",
                            http="200", upstream="payment-router", latency=RNG.randint(40, 160)))
        else:
            docs.append(doc(t, name, "INFO", f"request completed {RNG.choice(['GET','POST'])} /api 200",
                            latency=RNG.randint(8, 90)))
        t += RNG.randint(18 * 60, 32 * 60)

    # --- T-48h: promotion scheduled (the business trigger) ---
    docs.append(doc(now - 48 * HOUR, "order-service", "INFO",
                    "campaign 'Flash Sale' scheduled: multiplier=3.5, planned_qps=12000, status=scheduled"))

    # --- T-24h: checkout config change (the latent root cause) ---
    docs.append(doc(now - 24 * HOUR, "checkout-service", "AUDIT",
                    "applied config cfg-checkout-retry: max_retries 2->5, timeout 500ms->2000ms (change_id=CHG-7741)"))

    # --- P1 retries-up: rising WARN, ramping toward the breach ---
    warns = [
        ("checkout-service", "payment-gateway", "retry {n}/5 calling payment-gateway (upstream slow)", "RETRY"),
        ("payment-gateway", "payment-router", "upstream payment-router p99 elevated ({lat}ms)", "UPSTREAM_SLOW"),
        ("checkout-service", "payment-gateway", "client retry budget {p}% consumed", "RETRY_BUDGET"),
        ("payment-router", "channel-alipay", "channel-alipay latency degraded ({lat}ms)", "UPSTREAM_SLOW"),
    ]
    t = now - 24 * HOUR
    while t < now - 4 * HOUR:
        age_h = (now - t) / HOUR
        # cadence tightens from ~40 min early in P1 to ~8 min just before the breach
        gap = int(8 * 60 + (age_h - 4) / 20.0 * (40 - 8) * 60)
        src, up, tmpl, code = RNG.choice(warns)
        msg = tmpl.format(n=RNG.randint(2, 5), lat=RNG.randint(900, 1900), p=RNG.randint(55, 90))
        docs.append(doc(t, src, "WARN", msg, http="200", upstream=up,
                        latency=RNG.randint(800, 1900), code=code))
        t += max(gap, 5 * 60)

    # --- T-12h: the red-herring deployment ---
    docs.append(doc(now - 12 * HOUR, "payment-gateway", "INFO",
                    "rollout payment-gw v3.2.1 complete: logging format change + correlation-id header propagation",
                    code="DEPLOY"))

    # --- P2 promo breach: ERROR flood on the payment path, as correlated request traces
    # (follow one trace_id down the stack) interleaved with standalone saturation signals ---
    standalone = [
        ("payment-gateway", "payment-router", "500", "circuit breaker OPEN for payment-router (downstream error rate 14.8%)", 5, "CIRCUIT_OPEN"),
        ("payment-router", "", "503", "all payment channels saturated; charge queue depth {q}", 14, "QUEUE_SATURATED"),
        ("channel-wechatpay", "wechatpay-api", "503", "throttled by provider: WECHATPAY_RATE_LIMITED", 30, "PROVIDER_THROTTLED"),
        ("payment-gateway", "payment-router", "503", "payment-router returned 503 Service Unavailable, shedding charge request", 1985, "UPSTREAM_UNAVAILABLE"),
        ("checkout-service", "payment-gateway", "499", "client closed request while awaiting payment confirm (5/5 retries pending)", 3000, "CLIENT_CLOSED"),
    ]
    t = now - 4 * HOUR
    k = 0
    while t < now:
        age_min = (now - t) / 60
        # density rises toward now: ~1 event every 4 min early, ~1 every 45 s near the peak
        gap = int(45 + age_min / 240.0 * (240 - 45))
        if k % 2 == 0:
            docs.extend(chain(t))                      # one full follow-the-trace failure
        else:
            src, up, http, msg, lat, code = RNG.choice(standalone)
            sev = "WARN" if http == "499" else "ERROR"
            docs.append(doc(t, src, sev, msg.format(q=RNG.randint(1500, 2200)),
                            http=http, upstream=up, latency=lat, code=code))
        k += 1
        t += max(gap, 30)

    docs.sort(key=lambda d: d["timestamp"])
    load(docs)


def load(docs):
    print(f"waiting for Elasticsearch at {ES} ...", flush=True)
    for _ in range(150):
        try:
            urllib.request.urlopen(f"{ES}/_cluster/health", timeout=3).read()
            break
        except Exception:
            time.sleep(2)
    print("Elasticsearch is up.", flush=True)

    mapping = {"mappings": {"properties": {
        "timestamp": {"type": "date"}, "svc_id": {"type": "keyword"}, "env": {"type": "keyword"},
        "severity": {"type": "keyword"}, "log_message": {"type": "text"}, "trace_id": {"type": "keyword"},
        "span_id": {"type": "keyword"}, "http_status": {"type": "keyword"},
        "upstream_service": {"type": "keyword"}, "latency_ms": {"type": "long"},
        "error_code": {"type": "keyword"}, "pod": {"type": "keyword"},
    }}}
    req = urllib.request.Request(f"{ES}/{INDEX}", data=json.dumps(mapping).encode(),
                                 headers={"Content-Type": "application/json"}, method="PUT")
    try:
        urllib.request.urlopen(req, timeout=10).read()
        print(f"created index {INDEX}", flush=True)
    except Exception as e:
        print(f"  (index PUT: {e}; continuing)", flush=True)

    body = "".join('{"index":{}}\n' + json.dumps(d) + "\n" for d in docs)
    req = urllib.request.Request(f"{ES}/{INDEX}/_bulk", data=body.encode(),
                                 headers={"Content-Type": "application/x-ndjson"}, method="POST")
    urllib.request.urlopen(req, timeout=60).read()
    urllib.request.urlopen(urllib.request.Request(f"{ES}/{INDEX}/_refresh", method="POST"), timeout=10).read()
    cnt = urllib.request.urlopen(f"{ES}/{INDEX}/_count", timeout=10).read().decode()
    print(f"seed complete: {len(docs)} docs across 72h -> {cnt}", flush=True)


if __name__ == "__main__":
    main()
