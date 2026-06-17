#!/usr/bin/env python3
"""Demo entrypoint: slide the incident-investigation sample timeline so it is always relative
to the current time, then start the UModel server.

The sample data carries real, valid ISO timestamps anchored to REF below (so the pack still
loads correctly outside this demo — e.g. test-integration.sh). Here we shift every timestamp by
(now - REF) before the server reads them, so the deployment / config-change / incident / promotion
times track wall-clock now (T-N) and the demo never "expires":

    config change ~ now-24h, deploy ~ now-12h, promotion active ~ now-4h, incident ~ now.

This matches the now-relative Prometheus/Elasticsearch telemetry seeded by the rest of the stack.
The shift is idempotent: it always works from a pristine copy (.orig), so restarts re-anchor to
the current time instead of compounding.
"""
import datetime as dt
import os
import re
import shutil
import sys

UTC = dt.timezone.utc
# Anchor baked into sample-data/entities.json (the INC-0042 P99 breach). If you re-date the
# sample, update this to the latest "present" timestamp in the file.
REF = dt.datetime(2026, 6, 17, 18, 10, 0, tzinfo=UTC)
TARGETS = ["/app/examples/incident-investigation/sample-data/entities.json"]
ISO = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z")


def main():
    now = dt.datetime.now(tz=UTC)
    delta = now - REF
    for path in TARGETS:
        if not os.path.exists(path):
            continue
        pristine = path + ".orig"
        if not os.path.exists(pristine):
            shutil.copyfile(path, pristine)        # keep the original anchored copy
        src = open(pristine).read()

        def shift(m):
            t = dt.datetime.strptime(m.group(0), "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=UTC)
            return (t + delta).strftime("%Y-%m-%dT%H:%M:%SZ")

        open(path, "w").write(ISO.sub(shift, src))
        print(f"[entrypoint] re-anchored {os.path.basename(path)} by {delta} "
              f"(REF {REF:%Y-%m-%d} -> now {now:%Y-%m-%d %H:%M}Z)", flush=True)

    os.execvp("umodel-server", ["umodel-server"] + sys.argv[1:])


if __name__ == "__main__":
    main()
