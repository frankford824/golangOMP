# V1.2-D-2 P5 · known_gap class report

- date: 2026-04-26 PT
- source: `docs/iterations/V1_2_D_2_FINAL_AUDIT.json`

## Class Distribution

| known_gap_class | count |
|---|---:|
| `dynamic_payload_documented` | 22 |
| `inline_or_middleware_route` | 11 |
| `reserved_route` | 10 |
| `delegated_handler_response` | 10 |
| `stream_response` | 1 |

## Gate

- `unknown`: 0
- `unclassified`: 0
- ABORT §3.7: not triggered

All known gaps now carry explicit class labels. The labels are operational evidence, not silent drift suppression: hard drift is still gated by `--fail-on-drift true` and final `summary.drift == 0`.
