# V1 Contract Diff

Current frontend integration targets the canonical V1 paths from `frontend/INDEX.md`.

## Cleared

- Task pool uses `GET /v1/tasks/pool`.
- Task detail uses `GET /v1/tasks/{id}/detail`.
- Task drafts update through `POST /v1/task-drafts` with `draft_id`.
- Task asset uploads use `/v1/tasks/{id}/asset-center/upload-sessions/*`.
- Create-before-task assets use `/v1/task-create/asset-center/upload-sessions`.
- Reports use `/v1/reports/l1/*`.
- Organization option reads use `GET /v1/org/options`.

## Remaining Backend Dependency

- `docs/api/openapi.yaml` is not present in this checkout, so generated OpenAPI types are placeholders until the backend OpenAPI artifact is added.
