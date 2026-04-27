# FRONTEND_ALIGNMENT_v0.5

Status: obsolete historical note only.

Do not use this document for current frontend rollout, OSS upload/download integration, or API onboarding.

Reason:

- it belongs to the old v0.5 alignment phase
- it may contain pre-OSS or pre-v0.9 wording
- it is not allowed to define canonical routes or fields

Current frontend starting points:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
4. `docs/API_USAGE_GUIDE.md`
5. `docs/ASSET_UPLOAD_INTEGRATION.md`

Canonical OSS asset contract for new frontend work:

- `POST /v1/tasks/reference-upload`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`
- `GET /v1/assets/files/{path}`

Compatibility-only and not for new frontend work:

- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/assets*`
- `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort`
- `ReferenceFileRef.url`
- `public_download_allowed`
- `preview_public_allowed`
