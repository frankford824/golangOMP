# V1.3-A1 Issue 2 Evidence - reports overview 404

## Finding

`GET /v1/reports/l1/overview` is not a mounted backend route and is not an OpenAPI contract.

Evidence:

- `transport/http.go` mounts only:
  - `GET /v1/reports/l1/cards`
  - `GET /v1/reports/l1/throughput`
  - `GET /v1/reports/l1/module-dwell`
- `docs/api/openapi.yaml` defines only those same three report paths under `/v1/reports/l1`.
- No handler named overview exists in `transport/handler/report_l1.go`.

## Options

Option A - backend no change:

Frontend should stop calling `/v1/reports/l1/overview`. Build the overview view from existing canonical routes:

- `/v1/reports/l1/cards`
- `/v1/reports/l1/throughput?from=YYYY-MM-DD&to=YYYY-MM-DD`
- `/v1/reports/l1/module-dwell?from=YYYY-MM-DD&to=YYYY-MM-DD`

Option B - backend new feature:

Add `GET /v1/reports/l1/overview` as an aggregate endpoint combining cards, recent throughput, and module-dwell summaries. This is a new contract and implementation feature, not a V1.x bugfix, and should be handled in V2 or an explicitly approved V1.3 feature prompt.

## Recommendation

Choose Option A for V1.3-A1. Frontend should call the three existing canonical routes. Backend should not add `/overview` in this diagnostic/fix round.
