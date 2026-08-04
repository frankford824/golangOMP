# AI retrieval read-path rehearsal baseline

Measured on 2026-07-20 against the local `jst_erp_prodclone` production snapshot after the V8 read-model
changes. This is reproducible rehearsal evidence, not a live-production latency claim. No trustworthy
pre-change timing capture exists in this branch, so this report does **not** claim a percentage improvement
over the old implementation.

Command:

```bash
go run ./cmd/tools/read-performance-baseline --dsn "$REHEARSAL_MYSQL_DSN" --iterations 30 \
  > /tmp/yongbo-read-performance-baseline.json
```

`REHEARSAL_MYSQL_DSN` must use the same multi-statement capability as the production application DSN because
the task-detail bundle intentionally returns several read-only result sets in one database round trip (for
example, `?parseTime=true&multiStatements=true`).

Snapshot scale at the time of the rehearsal:

- tasks: 2,048
- task resource groups: 2,808
- task assets: 19,278
- external assets: 218,422

| Surface | DB round trips/call | p50 | p95 | p99 | allocated bytes/op | GC during 30 calls |
|---|---:|---:|---:|---:|---:|---:|
| Task detail | 1 | 1.591 ms | 1.764 ms | 1.776 ms | 49,904 | 0 |
| Task list | 3 | 33.040 ms | 36.645 ms | 37.060 ms | 432,402 | 5 |
| Resource groups | 6 | 10.536 ms | 11.693 ms | 11.705 ms | 291,567 | 3 |
| Exact asset search | 2 | 116.725 ms | 120.192 ms | 120.710 ms | 416,625 | 4 |

A second isolated 30-call confirmation run on the same snapshot produced p95 values of 1.647 ms, 33.740 ms,
10.517 ms and 123.833 ms respectively. The exact-search variation was 3.0%, inside the 10% release guard;
the earlier 100-call run was retained only as diagnostic evidence because it overlapped other local Docker and
browser-gate load and is not comparable to the isolated baseline.

`EXPLAIN ANALYZE` observations:

- task detail was a single-row primary-key lookup and the application made one database round trip;
- task list used the reverse covering `idx_tasks_experience_observer_updated` index for the sampled page;
- resource-group hydration stayed at six round trips, but the page query scanned 2,808 groups and sorted the
  1,245 finalized rows; this is acceptable at the rehearsal size and must be rechecked as groups grow;
- the sampled task-asset page used `idx_task_assets_experience_observer_created` but exact asset search remains
  the slowest deterministic surface.

Release comparison rules:

1. Repeat the same tool on a production-equivalent snapshot and keep the JSON in the release evidence bundle.
2. Exact-path p95 must not exceed this rehearsal baseline by more than 10% without an approved explanation.
3. Run the separate retrieval golden gate and concurrent SSE load test; this SQL baseline does not prove
   semantic recall, Qdrant latency, permissions, or provider time-to-first-token.
4. Do not compare runs with materially different row counts, MySQL cache warmth, hardware, or connection-pool
   settings without recording those differences.
