# ASSET_UPLOAD_INTEGRATION

Last purified: 2026-04-14 (OSS direct cleanup)

Current authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

This file describes the current MAIN business asset integration only.
Frontend rollout must use only the canonical OSS routes listed here and in `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.

## Storage Boundary

- Business storage backend: OSS only (Alibaba Cloud OSS)
- Canonical byte path: browser ↔ OSS direct (presigned URL)
- NAS business role: removed
- Historical NAS assets: intentionally unsupported
- NAS operational access: see `docs/ops/NAS_SSH_ACCESS.md`

## OSS Direct Presign Architecture (v0.9 canonical)

MAIN is the **sole OSS presign issuer**. When `OSS_DIRECT_ENABLED=true`:

- **Upload**: MAIN initiates OSS multipart upload, generates presigned PUT URLs for each part, returns them in `oss_direct` response field. Browser uploads bytes directly to OSS. Browser calls MAIN `complete` with part ETags to finalize.
- **Download**: MAIN generates presigned GET URL with `response-content-disposition=attachment`. Browser fetches bytes directly from OSS.
- **Preview**: MAIN generates presigned GET URL with `response-content-disposition=inline`. Browser renders directly from OSS.

Environment variables for OSS direct:

- `OSS_DIRECT_ENABLED`: `true` to enable (default: `false`)
- `OSS_ENDPOINT`: internal OSS endpoint (e.g. `oss-cn-hangzhou-internal.aliyuncs.com`)
- `OSS_PUBLIC_ENDPOINT`: browser-facing OSS endpoint (e.g. `oss-cn-hangzhou.aliyuncs.com`)
- `OSS_BUCKET`: OSS bucket name
- `OSS_ACCESS_KEY_ID`: access key ID (short-lived or scoped recommended)
- `OSS_ACCESS_KEY_SECRET`: access key secret
- `OSS_PRESIGN_EXPIRY`: presigned URL lifetime (default: `15m`)
- `OSS_PART_SIZE`: multipart part size in bytes (default: `10485760` = 10MB)

OSS bucket CORS requirement: the bucket must allow `PUT` and `GET` from the frontend origin(s) with `ETag` exposed in `Access-Control-Expose-Headers`.

## Upload Contract (compatibility layer)

The upload-service proxy layer remains for backward compatibility only when `OSS_DIRECT_ENABLED=false`.

Runtime URL split (compatibility only, must not be used in new frontend):

- `UPLOAD_SERVICE_BASE_URL`: backend-to-upload-service internal/service URL (used by MAIN create/get/meta/complete/cancel).
- `UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL`: browser-facing upload URL rebasing target (optional).
- `UPLOAD_SERVICE_BROWSER_DOWNLOAD_BASE_URL`: browser-facing direct-download base (optional).

### 1. Pre-task reference upload

- Route: `POST /v1/tasks/reference-upload`
- Mode: backend-proxy upload to OSS business storage
  - prefer OSS direct backend write when `OSS_DIRECT_ENABLED=true`
  - fallback to upload-service proxy only for compatibility when OSS direct is unavailable
- Request shape: one `multipart/form-data` file field named `file`
- Result: one normalized `reference_file_ref`
- Frontend use:
  - upload the file to this endpoint
  - append the returned object into `POST /v1/tasks.reference_file_refs`

### 2. Asset upload session (OSS direct - canonical)

- Routes:
  - `POST /v1/assets/upload-sessions`
  - `POST /v1/assets/upload-sessions/{session_id}/complete`
  - `POST /v1/assets/upload-sessions/{session_id}/cancel`
- Mode: browser-direct upload to OSS via MAIN-issued presigned URLs
- Backend responsibility:
  - choose upload mode (currently all assets use `multipart`)
  - initiate OSS multipart upload and generate presigned PUT URLs per part
  - persist business session metadata
  - complete/cancel the business session (including OSS multipart finalization)
  - finalize asset/version metadata in MAIN
- **Create session response** includes:
  - `session`: business session metadata
  - `upload_strategy`: `"multipart"` (canonical)
  - `required_upload_content_type`: the exact `Content-Type` the browser must send on OSS PUT
  - `oss_direct` (when enabled): OSS direct upload plan with:
    - `mode`: `"multipart"` or `"single_part"`
    - `object_key`: the frozen OSS object key
    - `upload_id`: the OSS multipart upload ID
    - `required_upload_content_type`: exact signed upload content type
    - `parts[]`: array of `{part_number, upload_url, method, expires_at}` — presigned PUT URLs
    - `part_size`: recommended part size in bytes
    - `bucket`, `endpoint`: for reference
  - `remote`: compatibility proxy plan (deprecated, do not use for new frontend)
- **Complete request** must include (for OSS direct):
  - `upload_content_type`: echo the exact `required_upload_content_type` from create response when finalizing
  - `oss_parts[]`: array of `{part_number, etag}` — the ETags returned by OSS after each part upload
  - `oss_upload_id`: the multipart upload ID from create response
  - `oss_object_key`: the object key from create response
- **Object key policy**: new OSS direct upload sessions store an ASCII-only random filename component in `oss_direct.object_key`.
  MAIN derives only a safe extension from the user-provided filename and never embeds the original filename in the OSS path.
  The original user filename remains in business metadata (`session.filename`, `task_assets.original_filename`, and read DTO filename/original filename fields).
  Historical `storage_key` values are not migrated and remain readable through the existing download/preview signing path.
- Frontend responsibility:
  - for top-level `POST /v1/assets/upload-sessions`, always send `task_id`
  - send `asset_kind` as the canonical upload intent field
  - always provide file name (`file_name`)
  - when uploading backend-owned preview/thumb derivatives, send `source_asset_id`
  - **prefer `oss_direct` over `remote`** in the create session response
  - when `mime_type` is omitted, MAIN defaults the required upload content type to `application/octet-stream`
  - split the file into parts of `oss_direct.part_size` bytes
  - for each part, `PUT` the bytes to `oss_direct.parts[i].upload_url` with the exact `required_upload_content_type` value (no auth header needed)
  - collect the `ETag` response header from each PUT
  - call MAIN `complete` with the echoed `upload_content_type`, part ETags, upload_id, and object_key
  - do not inject internal service tokens into browser upload requests

### 3. Upload rules

- Current `v0.9` live runtime emits `upload_strategy=multipart` for task-scoped `reference`, `source`, `delivery`, `preview`, and `design_thumb` assets.
- This was an intentional incident fix after the upload-service `small` lane produced zero-byte reference objects on live OSS-backed uploads.
- `source`, `delivery`, `preview`, and `design_thumb` assets use the unified asset upload-session contract
- multi-SKU non-reference uploads must send `target_sku_code`
- upload-session responses keep the chosen SKU on `target_sku_code`
- MAIN persists `scope_sku_code` on the asset root and asset version read model
- MAIN persists optional `source_asset_id` linkage for preview/thumb derivatives when available
- `POST /v1/tasks/{id}/submit-design` supports batch mode via `assets[]`:
  - one submit action can complete multiple upload sessions (`source` and/or `delivery`)
  - each batch-delivery item must include `target_sku_code`
  - backend validates `assets[].target_sku_code` against task SKU scope and upload-session captured `target_sku_code`
  - response includes `submitted_assets[]` with completed session + persisted asset/version payload
- For task-scoped `source` / `delivery` upload sessions that were created while the task was actionable,
  MAIN allows `POST .../upload-sessions/{session_id}/complete` in `PendingAuditA` as a narrow post-transition window.
  This prevents batch submit races where later completes arrive after status transition; it does not reopen create-session permission in audit stages.
- MAIN no longer accepts or emits NAS upload probe evidence, NAS allowlist logic, or NAS private-network policy

### 4. PSD preview contract (backend-owned direction)

- Frontend must not parse or preview raw PSD/PSB directly.
- Backend owns preview/thumb derivative direction.
- Canonical resource shape:
  - source asset: `asset_kind=source` (raw PSD/PSB, downloadable by business rule)
  - preview asset: `asset_kind=preview` (backend-generated preview artifact)
  - thumb asset: `asset_kind=design_thumb` (backend-generated thumbnail artifact)
- Preview/thumb may link to source via `source_asset_id`.
- Canonical preview/read routes stay on `/v1/assets/{id}/preview` and `/v1/assets?source_asset_id={id}`.
- `/v1/assets/{id}/preview` runtime behavior:
  - `200` when preview metadata is available
  - `404` when asset does not exist
  - `409` (`INVALID_STATE_TRANSITION`) when asset exists but preview is not yet available

## Download Contract

- Metadata routes return backend-authorized file access contract:
  - `download_mode`: `"direct"` (canonical) or `"proxy"` (deprecated)
  - `download_url`: presigned OSS URL (when `download_mode=direct`, `access_hint=oss_presigned`)
  - `access_hint`: `"oss_presigned"` for OSS direct URLs
  - `expires_at`: when the presigned URL expires (frontend should refresh before expiry)
- Canonical behavior (OSS direct):
  - `GET /v1/assets/{id}/download` returns presigned GET URL with `Content-Disposition: attachment`
  - `GET /v1/assets/{id}/preview` returns presigned GET URL with `Content-Disposition: inline`
  - browser reads bytes directly from returned `download_url`
  - frontend must check `expires_at` and re-request if expired
- explicit rule: `/v1/assets/files/{path}` or `/files/{path}` must not be labeled as `download_mode=direct`
- Compatibility fallback:
  - `GET /v1/assets/files/{path}` remains as compatibility-only proxy route, not canonical
  - When `OSS_DIRECT_ENABLED=false`, old proxy-based download URLs are returned
- Missing-object hygiene:
  - do not add per-request OSS existence probes to the hot path
  - offline repair should probe historical `storage_key` values and mark `asset_storage_refs.status=archived` when the object is gone
  - once archived, MAIN stops emitting direct/proxy file access for that asset version and download/preview routes return `ASSET_MISSING`
- New frontend work must not depend on compatibility fields.
- No LAN URL
- No Tailscale URL
- No public-vs-private NAS split
- No NAS path resolution

## Metadata Model

Business asset metadata now centers on OSS semantics:

- `storage_provider=oss`
- `storage_key`
- `remote_upload_id`
- `remote_file_id`
- `mime_type`
- `file_size`
- `file_hash`
- `download_url`

## Frontend Deletions Required

Frontend must delete all legacy NAS business logic:

- NAS browser probe calls
- NAS allowlist admission logic
- NAS private-network upload/download gates
- NAS URL selection logic
- NAS fallback download logic
- NAS path-based asset resolution
- any OSS-first-then-NAS fallback

## Compatibility Notes

- `GET /v1/tasks/{id}/assets` is canonical task-scoped resource listing.
- `/v1/tasks/{id}/assets*` except the list route above remains compatibility-only and must not be used for new work.
- `/v1/task-create/asset-center/upload-sessions*` remains compatibility-only and must not be used for new work.
- `/v1/tasks/{id}/asset-center/assets*` is compatibility-only alias family; new task-linked reads must use `GET /v1/tasks/{id}/assets`.
- `/v1/tasks/{id}/asset-center/upload-sessions` remains available as a task-context helper route, but new generic integrations must start on `/v1/assets/upload-sessions`.
- `/v1/tasks/{id}/asset-center/upload-sessions/small` and `/multipart` are compatibility-only internal-dispatch routes and must not be used for new work. Their path suffix does not define canonical strategy selection.
- Compatibility routes do not preserve NAS semantics; they only alias the OSS-backed business mainline.
