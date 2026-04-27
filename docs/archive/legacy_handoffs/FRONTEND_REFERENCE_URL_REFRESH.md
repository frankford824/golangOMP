# Reference URL Refresh (Round T)
Effective: v1.18

## What changed
`reference_file_refs[].download_url` returned by GET /v1/tasks/{id} (and by the task
creation response) is now a short-lived OSS presigned URL, valid by default for 15
minutes. Each ref carries a new `download_url_expires_at` (ISO8601 timestamp).

The legacy form `/v1/assets/files/<key>` is no longer returned for live references.

## Frontend action required
1. Treat `download_url` as ephemeral. Do not cache it across sessions.
2. If `<img>` fails to load OR a 403/404 is observed when fetching the URL, refetch
   GET /v1/tasks/{id} and use the refreshed `download_url`.
3. If `download_url_expires_at` is present and within 60s of expiry, proactively
   refetch before the image render.
4. Task creation still accepts `reference_file_refs` as before; no request-side change.

## Contract unchanged for task-level assets
Task-level design assets (`design_assets[].current_version.download_url`,
`asset_versions[].download_url`) continue to use asset-center presigned URLs as
before; no change for those.
