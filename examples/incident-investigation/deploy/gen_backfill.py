#!/usr/bin/env python3
"""Generate ~72h of historical metrics for the incident-investigation demo and write
them as an OpenMetrics dump that `promtool tsdb create-blocks-from openmetrics` loads
into Prometheus before it starts (see docker-compose.yml). The live exporter.py then
continues the series from "now", so instant queries stay fresh while range queries see
the full incident arc.

The history follows the modeled timeline (README "Timeline"), anchored to wall-clock now:

    P0  healthy           [now-72h, now-24h)   baseline; everything nominal
    P1  retries-up        [now-24h, now-4h)    T-24h config change -> checkout
                                               client_retry_rate steps 8% -> 55%;
                                               T-12h deploy lands with NO metric effect
    P2  promo breach      [now-4h,  now]        the flash sale goes active (3.5x) -> retry storm:
                                               latency / errors / upstream timeouts spike

So a range query shows retry_rate inflecting at the config change, everything else
inflecting at promo-active, and the T-12h deployment leaving no trace (the red herring).

Reuses exporter.py's SERVICES / PROFILES / BUCKETS as the P2 peak, so the history and the
live tail share one source of truth. Standard library only.
"""
import math
import os
import time

from exporter import BUCKETS, PROFILES, SERVICES

WINDOW_S = 72 * 3600     # history depth
STEP_S = 300             # one sample per 5 min (range queries use [15m]/[30m] windows)
END_OFFSET_S = 120       # stop just before now so the live exporter owns the present
OUT = os.environ.get("BACKFILL_OUT", "/backfill/openmetrics.txt")

# Timeline boundaries, as age (seconds before now).
T_CONFIG = 24 * 3600     # checkout retry config change
T_PROMO = 4 * 3600       # promotion goes active
RAMP_CONFIG = 1800       # config change lands over ~30 min
RAMP_PROMO = 3600        # traffic ramps over ~60 min

# The payment path that melts down once the promo amplifies the retry storm.
HOT = {
    "payment-gateway", "payment-router",
    "channel-alipay", "channel-wechatpay", "channel-unionpay",
}
CHECKOUT = "checkout-service"

PROFILE_MEAN = {"fast": 0.06, "mid": 0.18, "high": 0.30, "slow": 0.45, "very_slow": 0.55}
BASE_LOAD = 1.0 / 3.5    # pre-promo load is 1/multiplier of the peak


def step_up(age, boundary, width):
    """0 before the boundary (older than it), ramping to 1 after (more recent), linear
    across `width` centered on the boundary. `age` is seconds-before-now and decreases
    toward the present, so a smaller age means the event has taken effect."""
    hi, lo = boundary + width / 2.0, boundary - width / 2.0
    if age >= hi:
        return 0.0
    if age <= lo:
        return 1.0
    return (hi - age) / (hi - lo)


def blend(p_lo, p_hi, a):
    return {le: (1 - a) * PROFILES[p_lo][le] + a * PROFILES[p_hi][le] for le in BUCKETS}


def instant(sid, ts, now):
    """Instantaneous rates/gauges for a service at time ts."""
    name, env, rps, eps, prof, mean, c_rps, r_rps, u_rps, u_to, inflight, cpu, mem = SERVICES[sid]
    age = now - ts

    if name in HOT:
        a = step_up(age, T_PROMO, RAMP_PROMO)            # 0 in P0/P1 -> 1 in P2
        lf = BASE_LOAD + a * (1 - BASE_LOAD)
        retry_ratio = r_rps / c_rps if c_rps else 0.0
        return {
            "rps": rps * lf,
            "eps": (rps * lf * 0.005) * (1 - a) + eps * a,
            "c_rps": c_rps * lf,
            "r_rps": c_rps * lf * retry_ratio,
            "u_rps": u_rps * lf,
            "u_to": (u_rps * lf * 0.002) * (1 - a) + u_to * a,
            "frac": blend("fast", prof, a),
            "mean": PROFILE_MEAN["fast"] * (1 - a) + mean * a,
            "inflight": inflight * (0.55 + 0.45 * a),
            "cpu": cpu * (0.55 + 0.45 * a),
            "mem": mem * (0.55 + 0.45 * a),
        }

    if name == CHECKOUT:
        a = step_up(age, T_PROMO, RAMP_PROMO)            # load ramps at promo
        b = step_up(age, T_CONFIG, RAMP_CONFIG)          # retry rate steps at config change
        lf = BASE_LOAD + a * (1 - BASE_LOAD)
        retry_rate = 0.08 + b * (0.55 - 0.08)
        return {
            "rps": rps * lf,
            "eps": rps * lf * 0.009,                      # checkout itself errors little
            "c_rps": c_rps * lf,
            "r_rps": c_rps * lf * retry_rate,
            "u_rps": u_rps * lf,
            "u_to": (u_rps * lf * 0.002) + (u_to - u_rps * lf * 0.002) * a,
            "frac": blend("fast", prof, a),
            "mean": PROFILE_MEAN["fast"] * (1 - a) + mean * a,
            "inflight": inflight * (0.55 + 0.07 * b + 0.38 * a),
            "cpu": cpu * (0.55 + 0.07 * b + 0.38 * a),
            "mem": mem * (0.55 + 0.45 * a),
        }

    # Healthy platform: flat, with a gentle deterministic ripple so lines aren't dead-flat.
    s = 1 + 0.04 * math.sin(ts / 2400.0)
    return {
        "rps": rps * s, "eps": eps * s, "c_rps": c_rps * s, "r_rps": r_rps * s,
        "u_rps": u_rps * s, "u_to": u_to * s,
        "frac": PROFILES[prof], "mean": mean,
        "inflight": inflight, "cpu": cpu, "mem": mem,
    }


def main():
    now = int(time.time())
    end = now - END_OFFSET_S
    start = end - WINDOW_S
    times = list(range(start, end + 1, STEP_S))
    sids = list(SERVICES.keys())

    # Precompute instantaneous values once per (service, timestep).
    inst = {sid: [instant(sid, ts, now) for ts in times] for sid in sids}

    def lbl(sid):
        return f'service_id="{sid}",environment="{SERVICES[sid][1]}"'

    lines = []

    def counter(family, key):
        lines.append(f"# TYPE {family} counter")
        for sid in sids:
            cum = 0.0
            for i, ts in enumerate(times):
                cum += inst[sid][i][key] * STEP_S
                lines.append(f'{family}_total{{{lbl(sid)}}} {cum:.0f} {ts}')

    def gauge(family, key):
        lines.append(f"# TYPE {family} gauge")
        for sid in sids:
            for i, ts in enumerate(times):
                lines.append(f'{family}{{{lbl(sid)}}} {inst[sid][i][key]:.4f} {ts}')

    counter("platform_service_request", "rps")
    counter("platform_service_error", "eps")
    counter("platform_service_client_request", "c_rps")
    counter("platform_service_client_retry", "r_rps")
    counter("platform_service_upstream_request", "u_rps")
    counter("platform_service_upstream_timeout", "u_to")

    # Histogram: cumulative per-bucket increments stay monotonic by construction.
    hist = "platform_service_request_duration_seconds"
    lines.append(f"# TYPE {hist} histogram")
    for sid in sids:
        cum_bucket = {le: 0.0 for le in BUCKETS}
        cum_inf = cum_sum = 0.0
        rows = []
        for i, ts in enumerate(times):
            d = inst[sid][i]
            inc = d["rps"] * STEP_S
            cum_inf += inc
            cum_sum += inc * d["mean"]
            for le in BUCKETS:
                cum_bucket[le] += inc * d["frac"][le]
                rows.append(f'{hist}_bucket{{{lbl(sid)},le="{le}"}} {cum_bucket[le]:.0f} {ts}')
            rows.append(f'{hist}_bucket{{{lbl(sid)},le="+Inf"}} {cum_inf:.0f} {ts}')
            rows.append(f'{hist}_count{{{lbl(sid)}}} {cum_inf:.0f} {ts}')
            rows.append(f'{hist}_sum{{{lbl(sid)}}} {cum_sum:.1f} {ts}')
        lines.extend(rows)

    gauge("platform_service_inflight_requests", "inflight")
    gauge("platform_service_cpu_utilization_ratio", "cpu")
    gauge("platform_service_memory_utilization_ratio", "mem")

    lines.append("# EOF")

    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"wrote {len(lines)} lines covering {len(times)} samples x {len(sids)} services "
          f"({WINDOW_S // 3600}h @ {STEP_S}s) to {OUT}", flush=True)

    done = os.path.join(os.path.dirname(OUT), "done")
    with open(done, "w") as f:
        f.write("ok\n")


if __name__ == "__main__":
    main()
