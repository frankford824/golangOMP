# MODEL_HANDOVER

Last purified: 2026-04-13

> HANDOFF-INDEX ONLY
> NOT SOURCE OF TRUTH
> Start with `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`

## Current Handoff Baseline

- Authority set:
  - `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
  - `docs/api/openapi.yaml`
  - `transport/http.go`
- Current business storage boundary:
  - OSS only
  - no NAS upload/download/storage fallback
  - no NAS URL/path compatibility in runtime
- Current business upload surface:
  - `POST /v1/tasks/reference-upload`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`
- Current business download surface:
  - asset download metadata routes return `download_mode` + `download_url`
  - bytes are served through `GET /v1/assets/files/{path}`
- Compatibility-only frontend traps that remain mounted or visible:
  - `/v1/task-create/asset-center/upload-sessions*`
  - `/v1/tasks/{id}/assets*`
  - `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort`
  - `ReferenceFileRef.url`
  - `public_download_allowed`
  - `preview_public_allowed`
- Current task/business invariants that remain intact:
  - `reference_file_refs`
  - multi-SKU `scope_sku_code`
  - batch delivery audit gate
  - `owner_department` / `owner_org_team`
  - `owner_team` compatibility behavior
  - `/v1/tasks` actor/source projections
  - `/v1/org/options` and `frontend_access`
  - `POST /v1/tasks/prepare-product-codes`
- NAS remains only in ops/developer access docs and scripts:
  - `docs/ops/NAS_SSH_ACCESS.md`
- 2026-04-13 live execution snapshot:
  - explicit `v0.9` overwrite deploy executed (with backup-first workflow).
  - destructive clean reset executed for task/history/resource data while preserving users/roles/org access.
  - historical task-linked image/resource rows were present pre-reset and were removed by reset.
  - post-reset clean-state probes show old historical task-image paths no longer returned by task/resource APIs.
  - runtime required minimal post-deploy schema hotfix for missing live columns before clean-state probes were stable.

## 2026-04-13 OSS Upload/Release Handover Note

- Live release drift root cause was confirmed in deploy tooling, not release history corruption:
  - a remote deploy without explicit `--version` auto-created managed directory `v0.10`
  - remote deploys now require explicit `--version`, and live runtime was corrected back to `v0.9`
- Live browser upload/download contract after incident closure:
  - backend internal upload-service URL stays on `UPLOAD_SERVICE_BASE_URL`
  - browser-facing upload URLs are rebased to same-origin relative `/upload/*`
  - nginx now proxies `/upload/*` and prioritizes `^~ /v1/` so preview/reference image downloads do not fall into the frontend static-file regex
- Live task asset upload strategy after incident closure:
  - task-scoped `reference`, `source`, `delivery`, `preview`, and `design_thumb` uploads now return `upload_strategy=multipart`
  - this avoids the live upload-service `small` lane that produced zero-byte reference objects
- Live OSS verification evidence:
  - fresh task `395` uploads persisted OSS object keys under `tasks/RW-20260413-A-000390/assets/...`
  - fresh `reference`, `source`, and `preview` uploads completed successfully and downloaded back with matching SHA-256 hashes
  - source preview-unavailable semantics still return `409 INVALID_STATE_TRANSITION` until a preview asset exists
- Concurrency benchmark exposed one follow-up risk, not a current release blocker:
  - concurrent create-session calls can still drift business `asset_no` from remote object-key folder naming because remote asset identity is predicted before the final asset row is created
  - upload/download bytes still succeeded, but this naming consistency issue should be treated as the next data-integrity hardening item

## Required Reading Order

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

## Archive

- `docs/archive/*`
- `docs/iterations/*`
