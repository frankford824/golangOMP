# CURRENT_STATE

Last purified: 2026-04-13

> ARCHIVE-INDEX ONLY
> NOT SOURCE OF TRUTH
> Start with `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`

## Current Executable Facts

- MAIN business asset runtime is OSS-only.
- `POST /v1/tasks/reference-upload` remains the canonical pre-task reference upload entry.
- `/v1/tasks/{id}/asset-center/*` remains the canonical task asset namespace.
- `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort` is compatibility-only and not a frontend rollout entrypoint.
- `/v1/assets/files/{path}` is the official byte-serving route for business downloads.
- Returned business file metadata now uses `download_url`; NAS URL families are gone from the mainline.
- `ReferenceFileRef.url`, `public_download_allowed`, and `preview_public_allowed` are compatibility-only and must not drive new frontend work.
- `storage_provider=oss` is the only valid business storage provider.
- Historical NAS asset compatibility is intentionally not supported.
- NAS remains only as ops/developer SSH-accessible infrastructure; see `docs/ops/NAS_SSH_ACCESS.md`.
- Batch/multi-SKU delivery gating remains intact: whole-task audit advance waits for all required SKU-scoped delivery assets.
- Canonical task actor/source fields remain `requester_*`, `creator_*`, `designer_*`, and `current_handler_*`.
- Canonical ownership remains `owner_department` and `owner_org_team` with `owner_team` as compatibility output only.
- 2026-04-13 explicit release authorized flow executed:
  - `v0.9` overwritten in place on live host.
  - backup-first destructive reset executed for task/history/resource business data with users preserved.
  - pre-reset historical task/image references were confirmed and then cleared.
  - clean-state re-test shows old historical task image visibility disappeared; no leftover historical task image surfaced after reset.
  - follow-up runtime hotfix applied for missing live schema columns (`users.employment_type`, task customization columns) to restore `/v1/tasks*` clean-state behavior.

## 2026-04-13 OSS Incident Closure

- Root cause of broken external multipart/browser upload was stale live config: `UPLOAD_STORAGE_PROVIDER=nas`, browser-facing multipart base leaked `http://192.168.0.125:8089`, and nginx did not proxy `/upload/*`.
- Root cause of unexpected `v0.10` was deploy tooling drift: `deploy/deploy.sh` auto-selected `next_managed_release_version` on remote deploys when `--version` was omitted. Remote deploys now hard-fail unless `--version` is explicit.
- Final live runtime target is confirmed back on `v0.9`:
  - `/root/ecommerce_ai/current -> /root/ecommerce_ai/releases/v0.9`
  - `deploy-state.env` reports `CURRENT_VERSION=v0.9`
  - `8080/8081/8082` health checks all return `200`
- Browser-direct task asset uploads now use same-origin relative `/upload/*` targets for both public dist and local-Vite-compatible backend responses.
- Task-scoped `reference` uploads were switched to the working multipart lane after the upload-service `small` lane produced zero-byte objects on live OSS-backed uploads.
- Public preview/reference image download now works after nginx was corrected to prioritize `^~ /v1/` over the static-extension regex location.
- Live OSS-backed verification on task `395` succeeded for fresh `reference`, `source`, and `preview` assets:
  - storage keys persisted as OSS object keys under `tasks/RW-20260413-A-000390/assets/...`
  - `GET /v1/tasks/395/assets`, `GET /v1/assets/{id}/download`, `GET /v1/assets/{id}/preview`, and `GET /v1/assets/files/{path}` all returned the expected post-fix behavior
  - downloaded byte hashes matched uploaded byte hashes for the fresh multipart-backed reference/source/preview assets

## Read Order

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

## Archive

- Historical snapshots remain under `docs/archive/*`.
- Iteration evidence remains under `docs/iterations/*`.
