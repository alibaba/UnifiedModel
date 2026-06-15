#!/usr/bin/env python3
"""Tiny Prometheus exporter for the UModel quickstart demo.

Emits exactly the series the `multi-domain-quickstart` metric_set queries —
`devops_service_request_total`, `devops_service_error_total`, and the histogram
`devops_service_request_duration_seconds_bucket` — labelled by `service_id`, for
the four `devops.service` entity ids. Counters grow with wall-clock time, so once
Prometheus has scraped a couple of times `rate(...[1m])` returns realistic,
non-zero values. Standard library only; no third-party dependencies.

The per-service profile tells the demo story: checkout-service is "degraded"
(high error rate + high p95 latency); the others are healthy.
"""
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

START = time.time()
PORT = 9100

# Histogram bucket upper bounds in seconds (a "+Inf" bucket is added too).
BUCKETS = [0.05, 0.1, 0.25, 0.5, 1.0, 2.0, 5.0]

# Cumulative fraction of requests with duration <= le, per latency profile.
# Chosen so histogram_quantile(0.95, ...) lands near the target p95.
PROFILES = {
    # p95 ~0.1s
    "fast": {0.05: 0.55, 0.1: 0.96, 0.25: 0.99, 0.5: 0.999, 1.0: 1.0, 2.0: 1.0, 5.0: 1.0},
    # p95 ~0.6s
    "mid": {0.05: 0.30, 0.1: 0.55, 0.25: 0.85, 0.5: 0.94, 1.0: 0.985, 2.0: 0.999, 5.0: 1.0},
    # p95 ~1.9s
    "slow": {0.05: 0.10, 0.1: 0.20, 0.25: 0.40, 0.5: 0.60, 1.0: 0.80, 2.0: 0.96, 5.0: 0.99},
}

# service_id -> (display_name, environment, requests/s, errors/s, latency profile, mean latency s)
SERVICES = {
    "10000000000000000000000000000101": ("checkout-service", "prod", 120.0, 18.0, "slow", 0.9),
    "10000000000000000000000000000102": ("catalog-api", "prod", 200.0, 1.0, "fast", 0.06),
    "10000000000000000000000000000103": ("delivery-service", "prod", 60.0, 4.0, "mid", 0.35),
    "10000000000000000000000000000104": ("telemetry-collector", "prod", 40.0, 0.2, "fast", 0.05),
}


def render():
    elapsed = max(time.time() - START, 1.0)
    out = []
    out.append("# HELP devops_service_request_total Total requests handled by a service.")
    out.append("# TYPE devops_service_request_total counter")
    for sid, (_n, env, rps, _e, _p, _m) in SERVICES.items():
        out.append(f'devops_service_request_total{{service_id="{sid}",environment="{env}"}} {rps * elapsed:.1f}')
    out.append("# HELP devops_service_error_total Total errored requests.")
    out.append("# TYPE devops_service_error_total counter")
    for sid, (_n, env, _r, eps, _p, _m) in SERVICES.items():
        out.append(f'devops_service_error_total{{service_id="{sid}",environment="{env}"}} {eps * elapsed:.1f}')
    out.append("# HELP devops_service_request_duration_seconds Request duration in seconds.")
    out.append("# TYPE devops_service_request_duration_seconds histogram")
    for sid, (_n, env, rps, _e, prof, mean) in SERVICES.items():
        total = rps * elapsed
        frac = PROFILES[prof]
        for le in BUCKETS:
            out.append(
                f'devops_service_request_duration_seconds_bucket'
                f'{{service_id="{sid}",environment="{env}",le="{le}"}} {total * frac[le]:.1f}'
            )
        out.append(
            f'devops_service_request_duration_seconds_bucket'
            f'{{service_id="{sid}",environment="{env}",le="+Inf"}} {total:.1f}'
        )
        out.append(f'devops_service_request_duration_seconds_count{{service_id="{sid}",environment="{env}"}} {total:.1f}')
        out.append(f'devops_service_request_duration_seconds_sum{{service_id="{sid}",environment="{env}"}} {total * mean:.1f}')
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

    def log_message(self, *args):  # quiet
        pass


if __name__ == "__main__":
    print(f"demo metrics exporter listening on :{PORT}/metrics", flush=True)
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
