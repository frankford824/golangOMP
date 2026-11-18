#!/usr/bin/env python3
"""Read-only OA acceptance. Supply token through OA_SELFTEST_TOKEN."""
import argparse
import collections
from contextlib import closing
import concurrent.futures
import datetime as dt
import http.client
import json
import math
import os
import time
import urllib.parse


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default="https://oa.yongbo.cloud")
    parser.add_argument("--requests", type=int, default=100)
    parser.add_argument("--from", dest="first", type=dt.date.fromisoformat, required=True)
    parser.add_argument("--to", dest="last", type=dt.date.fromisoformat, required=True)
    parser.add_argument("--task-id", type=int, required=True)
    parser.add_argument("--sku", required=True)
    args = parser.parse_args()
    token = os.environ.get("OA_SELFTEST_TOKEN")
    root = urllib.parse.urlsplit(args.base)
    if not token or root.scheme not in ("https", "http") or root.username or root.password or root.path not in ("", "/") or root.query or root.fragment:
        parser.error("provide an OA origin and OA_SELFTEST_TOKEN")
    if root.scheme == "http" and root.hostname not in ("127.0.0.1", "localhost"):
        parser.error("plaintext is allowed only on loopback")
    if args.requests < 100 or args.last < args.first:
        parser.error("need at least 100 requests and an ordered date range")
    client_type = http.client.HTTPSConnection if root.scheme == "https" else http.client.HTTPConnection

    def request(connection, path, required):
        start = time.perf_counter()
        cache = "unreported"
        try:
            connection.request("GET", path, headers={"Authorization": "Bearer " + token})
            response = connection.getresponse()
            cache = response.getheader("X-OA-Order-Cache", "unreported")
            payload = response.read()
            status = str(response.status)
            value = None
            if response.status == 200:
                document = json.loads(payload)
                value = document.get("data") if isinstance(document, dict) else None
                if not isinstance(value, dict) or any(name not in value for name in required):
                    status = "bad_envelope"
                elif value.get("stale"):
                    status = "stale"
            return time.perf_counter() - start, status, cache, value
        except (OSError, ValueError, http.client.HTTPException) as exc:
            connection.close()
            return time.perf_counter() - start, type(exc).__name__, cache, None

    prefix = "/oa-api/orders/"
    def with_query(name, first, last, shop=""):
        return prefix + name + "?" + urllib.parse.urlencode({"from": first, "to": last, "shop_id": shop})

    with closing(client_type(root.netloc, timeout=15)) as discovery:
        shops_path = prefix + "shops"
        cost_path = prefix + f"tasks/{args.task_id}/skus/" + urllib.parse.quote(args.sku, safe="") + "/cost"
        values = {}
        for label, target, required in (
            ("shops", shops_path, ("rows",)),
            ("cost", cost_path, ("cost_price", "version", "erp_status")),
        ):
            elapsed, status, _, value = request(discovery, target, required)
            print(f"{label}: {status} {elapsed * 1000:.1f}ms", flush=True)
            if status != "200":
                return 1
            values[label] = value
    shops = sorted({row["shop_id"] for row in values["shops"]["rows"] if row.get("shop_id")})
    combinations = [(args.first + dt.timedelta(days=i), shop)
                    for i in range((args.last - args.first).days + 1) for shop in shops]
    if len(combinations) < args.requests:
        parser.error("select a wider period or a token with more authorized shops for distinct queries")

    def worker(targets):
        results = []
        with closing(client_type(root.netloc, timeout=15)) as connection:
            for target in targets:
                results.append(request(connection, target, ("rows", "freshness"))[:3])
        return results

    failures = 0
    for endpoint in ("summary", "products"):
        for mode in ("repeated", "different"):
            targets = [with_query(endpoint, args.first, args.last)] * args.requests
            if mode == "different":
                targets = [with_query(endpoint, day, day, shop) for day, shop in combinations[:args.requests]]
            with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
                entries = [item for batch in pool.map(worker, (targets[n::8] for n in range(8))) for item in batch]
            statuses = collections.Counter(status for _, status, _ in entries)
            caches = collections.Counter(cache for _, _, cache in entries)
            latencies = sorted(duration for duration, status, _ in entries if status == "200")
            p95 = latencies[math.ceil(.95 * len(latencies)) - 1] * 1000 if latencies else None
            cache_p95 = {}
            for kind in caches:
                bucket = sorted(duration for duration, status, cache in entries if status == "200" and cache == kind)
                if bucket:
                    cache_p95[kind] = bucket[math.ceil(.95 * len(bucket)) - 1] * 1000
            errors = len(entries) - statuses["200"]
            failures += errors + (p95 is None or p95 >= 1000)
            print(json.dumps(dict(endpoint=endpoint, mode=mode, requests=len(entries), p95_ms=p95,
                                  errors=errors, statuses=statuses, oa_cache=caches, oa_cache_p95_ms=cache_p95)), flush=True)
    return int(failures > 0)


if __name__ == "__main__":
    raise SystemExit(main())
