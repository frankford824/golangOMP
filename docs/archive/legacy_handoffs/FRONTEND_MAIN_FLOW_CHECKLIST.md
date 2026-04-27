# FRONTEND_MAIN_FLOW_CHECKLIST

Last purified: 2026-04-11

Current authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `docs/ASSET_UPLOAD_INTEGRATION.md`

## Frontend Mainline Checklist

### Authentication and org master

- Use `/v1/auth/*`, `/v1/users*`, `/v1/org/options`, `/v1/roles`.
- Keep `frontend_access` as the permission source for menus/pages/actions/modules.
- Preserve canonical ownership fields:
  - `owner_department`
  - `owner_org_team`
  - `owner_team` only as compatibility output

### Task create

- Use `POST /v1/tasks/reference-upload` for pre-task reference files.
- Persist returned `reference_file_ref` objects into `POST /v1/tasks.reference_file_refs`.
- Do not send `reference_images`.
- Keep actor/source fields from `/v1/tasks` and `/v1/tasks/{id}` unchanged.

### Asset upload

- Use `POST /v1/assets/upload-sessions`.
- Always send `task_id` on top-level upload-session creation.
- Use `asset_kind` as the canonical upload intent field.
- Do not send `upload_mode` for new integration (`upload_mode` is compatibility-only input).
- Always provide file name (`file_name`; compatibility alias `filename` remains accepted).
- Use canonical `asset_kind` values: `reference`, `source`, `delivery`, `preview`, `design_thumb`.
- Let the backend choose `single_part` or `multipart` and follow the returned upload plan.
- Read returned `upload_strategy` and execute the matching OSS upload plan.
- Upload bytes using the returned OSS upload plan.
- Call `/complete` after browser upload succeeds.
- For batch multi-SKU non-reference uploads, always send `target_sku_code`.
- For backend-generated preview/thumb derivatives, send `source_asset_id`.
- Keep `scope_sku_code` handling intact in UI state and task detail rendering.

### Asset download

- Read `download_mode` and `download_url`.
- Use `download_url` as the only supported business download entry.
- Prefer browser-direct signed OSS `download_url`; `/v1/assets/files/{path}` is compatibility fallback only.
- Use `/v1/assets/{id}/preview` to fetch backend-owned preview metadata.
- Source preview is two-tier:
  - direct OSS IMG preview for `jpg/png/bmp/gif/webp/tiff/heic/avif`
  - async backend-derived preview for non-direct source formats (such as `psd/psb`)
- Handle `/v1/assets/{id}/preview` non-success states:
  - `404`: asset not found
  - `409`: preview metadata not available yet (`INVALID_STATE_TRANSITION`)
- Do not implement raw PSD/PSB parsing in frontend.
- Do not expect `lan_url`, `tailscale_url`, `public_url`, or private-network download contracts.

### Deletions Required

Delete all old NAS frontend logic:

- NAS browser probe requests
- NAS upload attestation handling
- NAS allowlist / reachability checks
- NAS multipart gating
- NAS private-network download gating
- NAS URL selection logic
- NAS fallback from OSS
- NAS path-based file resolution

### Regression Checks

- task create still works
- batch SKU create still works
- `reference_file_refs` contract still works
- unified asset upload session create/complete/cancel still works
- `GET /v1/assets` and `GET /v1/assets/{id}` still return expected resource metadata
- `GET /v1/tasks/{id}/assets` still returns canonical task-scoped resource list
- `/v1/tasks/{id}/asset-center/assets*` remains compatibility-only and must not become new mainline usage
- `GET /v1/assets/{id}/download` and `GET /v1/assets/{id}/preview` return consumable runtime metadata (signed OSS direct when available; compatibility fallback otherwise)
- source/preview/thumb linkage (`source_asset_id`) remains queryable where provided
- multi-SKU delivery gating still works
- `/v1/tasks` actor/source fields still render correctly
- `/v1/org/options` and `frontend_access` still drive the same UI permissions
