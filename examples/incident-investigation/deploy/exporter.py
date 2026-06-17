#!/usr/bin/env python3
"""Prometheus exporter for the UModel incident-investigation demo.

Emits exactly the series the `incident-investigation` metric_set queries —
`platform_service_request_total` / `_error_total` / `_request_duration_seconds_bucket`,
`platform_service_client_request_total` / `_client_retry_total`,
`platform_service_upstream_request_total` / `_upstream_timeout_total`,
`platform_service_inflight_requests`, `_cpu_utilization_ratio`, `_memory_utilization_ratio`
— labelled by `service_id` (the entity id the plan substitutes). Counters grow with
wall-clock time so `rate(...[1m])` returns realistic values after a couple of scrapes.
Standard library only.

The per-service profile reproduces the modeled incident: checkout-service drives a
retry storm (client_retry_rate ~55%, the 2→5 config change), payment-gateway and the
payment-router → Alipay/WeChat/UnionPay channels show breaching p99 + high error and
upstream-timeout rates; the rest of the platform is healthy.
"""
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

START = time.time()
PORT = 9100

BUCKETS = [0.05, 0.1, 0.25, 0.5, 1.0, 1.5, 2.0, 3.0, 5.0]

# Cumulative fraction of requests with duration <= le, per latency profile.
PROFILES = {
    "fast":      {0.05: 0.70, 0.1: 0.99, 0.25: 0.997, 0.5: 0.999, 1.0: 1.0, 1.5: 1.0, 2.0: 1.0, 3.0: 1.0, 5.0: 1.0},
    "mid":       {0.05: 0.40, 0.1: 0.65, 0.25: 0.88, 0.5: 0.965, 1.0: 0.992, 1.5: 0.998, 2.0: 1.0, 3.0: 1.0, 5.0: 1.0},
    "high":      {0.05: 0.20, 0.1: 0.40, 0.25: 0.62, 0.5: 0.80, 1.0: 0.94, 1.5: 0.985, 2.0: 0.997, 3.0: 1.0, 5.0: 1.0},
    "slow":      {0.05: 0.12, 0.1: 0.25, 0.25: 0.45, 0.5: 0.65, 1.0: 0.88, 1.5: 0.965, 2.0: 0.99, 3.0: 0.998, 5.0: 1.0},
    "very_slow": {0.05: 0.08, 0.1: 0.16, 0.25: 0.34, 0.5: 0.55, 1.0: 0.78, 1.5: 0.90, 2.0: 0.965, 3.0: 0.992, 5.0: 1.0},
}

# service_id -> name, env, rps, eps, lat profile, mean s, client_rps, retry_rps,
#               upstream_rps, upstream_timeout_rps, inflight, cpu ratio, mem ratio
SERVICES = {
    # --- payment path: the incident blast radius ---
    "63718b78868895d2590551b27ec6f51c": ("payment-gateway", "prod", 4200, 621, "slow", 0.45, 8200, 1640, 8200, 985, 420, 0.88, 0.77),
    "149632df43354373835df2717cb8fb19": ("checkout-service", "prod", 9800, 88, "mid", 0.18, 29000, 16000, 29000, 1450, 300, 0.72, 0.61),
    "08409c1c4464bf2bc6346c777a2fabc7": ("payment-router", "prod", 11800, 1086, "slow", 0.40, 12200, 3700, 12200, 1700, 520, 0.81, 0.70),
    "6b3718c2b64fd131c1d7c0f1362c6d41": ("channel-alipay", "prod", 5200, 598, "very_slow", 0.50, 5200, 260, 5200, 780, 300, 0.74, 0.66),
    "3640dfa237be9bc61ced0fd7af8c7dad": ("channel-wechatpay", "prod", 4100, 279, "slow", 0.38, 4100, 160, 4100, 287, 180, 0.68, 0.60),
    "7d6930095ab2e664ae5b49a4f6272539": ("channel-unionpay", "prod", 2600, 112, "high", 0.30, 2600, 80, 2600, 104, 110, 0.59, 0.55),
    "c6ace8f158e5a0db93a8659ef446ddc1": ("risk-control", "prod", 9900, 139, "mid", 0.16, 4000, 120, 4000, 80, 190, 0.64, 0.58),
    "f25ae2923f5df058b6119ea79e434459": ("order-service", "prod", 3100, 37, "mid", 0.20, 6000, 240, 6000, 120, 95, 0.58, 0.55),
    "9f71098638361b9aa76a350a16f25626": ("cart-service", "prod", 7400, 155, "mid", 0.16, 3000, 90, 3000, 60, 140, 0.69, 0.64),
    "c04f73b02ebb6c7727e3e431876338db": ("ledger-service", "prod", 7300, 29, "fast", 0.08, 2000, 10, 2000, 8, 70, 0.52, 0.49),
    # --- healthy platform ---
    "0e8aec70874b99241082f307b4b3b9c5": ("inventory-service", "prod", 5200, 21, "fast", 0.05, 1500, 5, 1500, 3, 45, 0.51, 0.48),
    "44dd315b3186506721ac64c0b670f41b": ("notification-service", "prod", 1800, 5, "fast", 0.05, 800, 2, 800, 1, 20, 0.33, 0.40),
    "60c58dadd2d201fed7dcc1c0b2268139": ("user-profile", "prod", 6100, 12, "fast", 0.03, 1000, 3, 1000, 2, 30, 0.44, 0.52),
    "403ec4bd3891dc52bf5161b9dd65d16e": ("search-service", "prod", 8800, 44, "fast", 0.06, 2000, 8, 2000, 5, 60, 0.60, 0.58),
    "61dbd77bb5e09f4567478ca5fb8c655b": ("recommendation-engine", "prod", 4300, 26, "mid", 0.11, 3000, 10, 3000, 8, 55, 0.55, 0.63),
    "7dc05862c25ff7db21ab830e782d8e5a": ("pricing-service", "prod", 6700, 20, "fast", 0.04, 1200, 4, 1200, 2, 40, 0.47, 0.44),
    "394ba4c250378ded9a5a2c4d5bec47c9": ("auth-service", "prod", 12400, 12, "fast", 0.02, 500, 2, 500, 1, 50, 0.49, 0.51),
    "b0bba8793e479be1f44025bb1a544d2a": ("api-gateway", "prod", 41000, 451, "mid", 0.06, 41000, 200, 41000, 410, 250, 0.63, 0.57),
}


def counter(out, name, help_, idx, scale=None):
    out.append(f"# HELP {name} {help_}")
    out.append(f"# TYPE {name} counter")
    el = max(time.time() - START, 1.0)
    for sid, row in SERVICES.items():
        v = row[idx] * el
        out.append(f'{name}{{service_id="{sid}",environment="{row[1]}"}} {v:.1f}')


def gauge(out, name, help_, idx):
    out.append(f"# HELP {name} {help_}")
    out.append(f"# TYPE {name} gauge")
    for sid, row in SERVICES.items():
        out.append(f'{name}{{service_id="{sid}",environment="{row[1]}"}} {row[idx]}')


def render():
    el = max(time.time() - START, 1.0)
    out = []
    counter(out, "platform_service_request_total", "Total requests handled.", 2)
    counter(out, "platform_service_error_total", "Total errored requests.", 3)
    counter(out, "platform_service_client_request_total", "Outbound client requests.", 6)
    counter(out, "platform_service_client_retry_total", "Outbound client retries.", 7)
    counter(out, "platform_service_upstream_request_total", "Upstream requests.", 8)
    counter(out, "platform_service_upstream_timeout_total", "Upstream timeouts.", 9)
    # histogram
    out.append("# HELP platform_service_request_duration_seconds Request duration in seconds.")
    out.append("# TYPE platform_service_request_duration_seconds histogram")
    for sid, (n, env, rps, eps, prof, mean, *_rest) in SERVICES.items():
        total = rps * el
        frac = PROFILES[prof]
        for le in BUCKETS:
            out.append(f'platform_service_request_duration_seconds_bucket{{service_id="{sid}",environment="{env}",le="{le}"}} {total * frac[le]:.1f}')
        out.append(f'platform_service_request_duration_seconds_bucket{{service_id="{sid}",environment="{env}",le="+Inf"}} {total:.1f}')
        out.append(f'platform_service_request_duration_seconds_count{{service_id="{sid}",environment="{env}"}} {total:.1f}')
        out.append(f'platform_service_request_duration_seconds_sum{{service_id="{sid}",environment="{env}"}} {total * mean:.1f}')
    gauge(out, "platform_service_inflight_requests", "In-flight requests.", 10)
    gauge(out, "platform_service_cpu_utilization_ratio", "CPU utilization (0-1).", 11)
    gauge(out, "platform_service_memory_utilization_ratio", "Memory utilization (0-1).", 12)
    return ("\n".join(out) + "\n").encode()


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path in ("/metrics", "/"):
            body = render()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; version=0.0.4")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    print(f"incident demo metrics exporter listening on :{PORT}/metrics", flush=True)
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
