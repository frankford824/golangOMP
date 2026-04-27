# V1.3-A1 Issue 1 Evidence - draft reference URL 404

## Finding

Root cause is dual-sided, with backend evidence supporting the frontend symptom:

- `POST /v1/task-drafts` stores the request body as raw JSON. `transport/handler/task_draft.go` reads the raw body and passes it through; `service/task_draft/service.go` saves `cloneRaw(raw)`; `repo/mysql/task_draft_repo.go` writes the blob into `task_drafts.payload`.
- There is no backend scrub or refresh step for short-lived `download_url` fields inside draft payloads.
- OpenAPI defines `TaskDraftPayload` as `additionalProperties: true` and says the payload mirrors `POST /v1/tasks`, so persisted draft payload may include whatever the frontend sends.
- OpenAPI `ReferenceFileRef.download_url` explicitly says the signed URL is short-lived, default 15 minutes, and consumers should refresh after expiry/403/404.

## Download refresh path

`GET /v1/assets/{asset_id}/download` is a fresh lookup/sign path:

- `transport/handler/task_asset_center.go` routes global asset download to `globalSvc.DownloadLatest`.
- `service/asset_center/download.go` loads the current asset row, derives lifecycle state, checks `storage_key`, and calls `s.presigner.PresignDownloadURL(key)` on each request when OSS presigning is enabled.
- The response includes `expires_at` when the presigner returns a signed URL.

Therefore, if `GET /v1/assets/{asset_id}/download` succeeds while the draft-stored URL 404s, the asset is valid and the draft carried an expired signed URL. If the download endpoint also returns 404, the asset row or mapping is invalid/deleted.

## Recommendation

Frontend should treat draft `download_url` / `url` as display cache only, not authority. On draft restore or create-from-draft, use `asset_id`/`ref_id` and call `GET /v1/assets/{asset_id}/download` when it needs bytes or preview display, especially after 403/404 or `download_url_expires_at`.

Backend hardening option for V1.3-A1.1: scrub `download_url`, `url`, and `download_url_expires_at` from `reference_file_refs` before persisting draft payload, or add a read-time draft normalization endpoint. This is defensive but not required if frontend refreshes correctly.
