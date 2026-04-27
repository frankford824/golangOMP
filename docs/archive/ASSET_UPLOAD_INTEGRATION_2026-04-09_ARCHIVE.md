# ASSET_UPLOAD_INTEGRATION

Authoritative v0.9 contract: `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`

## Scope

- This document is the current truth source for MAIN <-> NAS upload/download integration.
- Canonical task asset API namespace is `/v1/tasks/{id}/asset-center/*`.
- Compatibility task asset API namespace is `/v1/tasks/{id}/assets*`.
- It supersedes older Step 68 wording that still described:
  - `delivery` as a small-upload path
  - task-create `reference` small uploads as a path that must call NAS `complete`
  - the `public_url` read proxy as the root cause of the 0-byte reference bug

## Responsibility Split

### MAIN owns

- upload-session business records in `upload_requests`
- task/create binding and `reference_file_refs`
- task asset metadata in `design_assets`, `task_assets`, and `asset_storage_refs`
- frontend-facing aggregation APIs
- audit and task-event traceability
- post-upload verification and binding rules

### NAS upload service owns

- `/upload/files`
- lightweight browser probe endpoint such as:
  - `GET /upload/ping`
- multipart session allocation and part upload orchestration
- `/complete` and `/abort` for multipart-oriented flows
- physical file persistence
- `/files/*` download source
- returned remote file metadata

## Runtime Config

- Server-to-server address:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
- Browser multipart address:
  - `UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL=http://192.168.0.125:8089`
- Browser probe contract:
  - `UPLOAD_SERVICE_BROWSER_PROBE_PATH=/upload/ping`
  - `UPLOAD_SERVICE_BROWSER_PROBE_METHOD=GET`
  - `UPLOAD_SERVICE_BROWSER_PROBE_MAX_AGE=2m`
  - `UPLOAD_SERVICE_BROWSER_PROBE_ATTESTATION_SECRET=<shared-secret>`
- Token:
  - `UPLOAD_SERVICE_INTERNAL_TOKEN=nas-upload-token-2026`
- Storage provider:
  - `UPLOAD_STORAGE_PROVIDER=nas`
- Rule:
  - `UPLOAD_SERVICE_BASE_URL` stays the service-to-service address.
  - Browser direct multipart upload must use the browser-facing multipart host.

## Current Effective Upload Modes

### Task-create reference upload

- Formal frontend entry:
  - `POST /v1/tasks/reference-upload`
- Handler:
  - `transport/handler/task_create_reference_upload.go`
- Service:
  - `service/task_create_reference_upload_service.go`
- Upload client:
  - `service/upload_service_client.go`
- Mode:
  - `reference` uses `small`
- Real chain:
  1. MAIN creates a pre-task small upload session.
  2. MAIN uploads bytes to NAS `/upload/files`.
  3. MAIN probes the stored object by `storage_key`.
  4. MAIN verifies stored `size` and `sha256`.
  5. MAIN binds the verified object into `asset_storage_refs` and returns `ReferenceFileRef`.
- Current rule:
  - small `reference` uploads must not call NAS `complete`.
  - The canonical result comes from `/upload/files` plus MAIN probe verification.

### Task asset-center upload

- Canonical routes:
  - `GET /v1/tasks/{id}/asset-center/assets`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`
- Compatibility aliases still retained:
  - `/v1/tasks/{id}/assets`
  - `/v1/tasks/{id}/assets/upload-sessions*`
  - `/v1/tasks/{id}/assets/upload`
- Handler:
  - `transport/handler/task_asset_center.go`
- Service:
  - `service/task_asset_center_service.go`
- Modes:
  - `reference` = `small`
  - `delivery` = `multipart`
  - `source` = `multipart`
  - `preview` = `multipart`
- Validation is enforced in code:
  - `reference` must use `small`
  - non-reference assets must use `multipart`
- Browser direct multipart upload now uses a probe-driven gate:
  1. frontend probes `GET http://192.168.0.125:8089/upload/ping`
  2. NAS returns lightweight probe data plus signed `attestation`
  3. frontend passes the probe payload as `network_probe` when calling multipart session create
  4. MAIN validates freshness + method + URL + status code and verifies the NAS-signed `attestation` before issuing private URLs
  5. if probe fails or attestation is missing/invalid, MAIN returns `403 UPLOAD_ENV_NOT_ALLOWED`
- Browser direct upload uses:
  - `http://192.168.0.125:8089`
- MAIN service-to-service calls still use:
  - `UPLOAD_SERVICE_BASE_URL`

## Current Effective Gate Policy

- Large-file multipart upload:
  - primary admission logic is now probe-driven, not public-IP allowlist driven
  - backend requires frontend probe evidence (`network_probe`) for multipart session issuance
  - probe evidence must include a NAS-signed `attestation`; backend no longer trusts bare frontend-reported `reachable/method/url/status` alone
  - backend still records `source_ip`, RFC1918 match, and legacy public allowlist match as diagnostics only
- Large-file private-network download:
  - frontend must pass the same probe result through `X-Network-Probe-*` headers, including `X-Network-Probe-Attestation`
  - backend returns `403 UPLOAD_ENV_NOT_ALLOWED` instead of returning unusable NAS private URLs when probe is missing or failed
- External-safe lane that remains unchanged:
  - `POST /v1/tasks/reference-upload`

## Current Read Chain

- `public_url` returned by MAIN points to:
  - `/v1/assets/files/{storage_key}`
- That route is implemented by:
  - `transport/handler/asset_files.go`
- Behavior:
  - MAIN only proxies the read request to NAS `/files/{storage_key}`
  - MAIN is not the storage source
  - MAIN is not the file-content producer
- Conclusion:
  - `/v1/assets/files/*` is a read proxy only
  - it was not the root cause of the 0-byte reference issue

## Reference 0-Byte Incident Closure

### Symptom

- `POST /v1/tasks/reference-upload` returned `201`
- returned `ReferenceFileRef` looked structurally correct
- new `public_url` downloads returned:
  - `200`
  - `Content-Length=0`
  - empty body
- historical references were still normal

### Confirmed root cause

- MAIN proxy-body forwarding was not the cause
- NAS `/files/*` was not globally broken
- The single root cause was NAS small upload plus `complete` pseudo-success:
  - remote manual `create + /upload/files + /complete` could also land a 0-byte file
  - remote manual `create + /upload/files` landed the file correctly

### Implemented fix

- `task-create reference` small path no longer calls NAS `complete`
- MAIN now trusts `/upload/files` return fields:
  - `file_id`
  - `storage_key`
  - `file_size`
- MAIN adds a stored-file probe
- MAIN fails immediately if stored `size` or `sha256` does not match
- MAIN no longer wraps a 0-byte object as a successful `ReferenceFileRef`

### Mandatory rule

> `task-create reference` 的 small 上传链路以 `/upload/files` 返回结果为准，不再调用 NAS `complete`；MAIN 必须对落盘结果做 size/hash 校验，校验失败直接报错，不得生成成功 ref。

- Plain English: the task-create reference small-upload path must trust `/upload/files`, must not call NAS `complete`, and must fail immediately on stored size/hash mismatch instead of returning a successful ref.

## Deprecated or Historical-Only Statements

- `delivery small upload` is historical-only and must not be used as the current contract.
- `task-create reference small upload must call complete` is deprecated.
- `all small uploads share one identical completion rule` is false for the current task-create reference path.
- `public_url` proxy is the cause of the 0-byte incident is false.
- `/v1/tasks/{id}/assets*` is the primary asset-center namespace is false for v0.9.

## Effective Code Entrypoints

- Runtime entry:
  - `cmd/server/main.go`
- Route registration:
  - `transport/http.go`
- Task-create reference upload:
  - `transport/handler/task_create_reference_upload.go`
  - `service/task_create_reference_upload_service.go`
- Asset-center:
  - `transport/handler/task_asset_center.go`
  - `service/task_asset_center_service.go`
- Upload client:
  - `service/upload_service_client.go`
- Read proxy:
  - `transport/handler/asset_files.go`
