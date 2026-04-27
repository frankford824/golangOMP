# V1.3-A1 Issue 4 Evidence - asset list download/preview URL consistency

## Contract check

`GET /v1/assets`:

- Response item schema is `DesignAsset`.
- `DesignAsset` includes root metadata and nested `current_version`, `approved_version`, `warehouse_ready_version`.
- The nested `DesignAssetVersion` schema includes `download_url` and `preview_available`.
- The root `DesignAsset` schema does not include root-level `download_url`, `preview_url`, or `download_url_expires_at`.

`GET /v1/tasks/{id}/assets`:

- Canonical task-scoped list.
- Response item schema is also `DesignAsset`, so same behavior as `/v1/assets?task_id={id}`.

`GET /v1/tasks/{id}/asset-center/assets`:

- Compatibility task-scoped list.
- Response item schema is also `DesignAsset`, so same behavior as the canonical task list.

Single download/preview:

- `GET /v1/assets/{asset_id}/download` returns `AssetDownloadInfo` with `download_mode`, `download_url`, `preview_available`, `filename`, `file_size`, `mime_type`, and `expires_at`.
- `GET /v1/assets/{asset_id}/preview` returns preview metadata through the same one-asset access pattern.

## Implementation check

There are two list families:

- Canonical/global list (`service/asset_center.Search`) builds `asset_center.AssetDetail` from current asset rows and does not presign list items. This model exposes metadata such as `storage_key`, filename, lifecycle/archive state, but no `download_url`.
- Task asset-center list uses `domain.DesignAsset` / `DesignAssetVersion`. Version objects have `download_url` fields for compatibility, but there is no list-time `expires_at` field and no root-level `preview_url`.

## Trade-off

Adding fresh `download_url` / `preview_url` to every list item would make list calls perform many signing operations and would return many short-lived URLs that are hard to cache correctly. It also increases accidental persistence risk when list payloads are saved into drafts.

Keeping lists as metadata and using single asset access endpoints gives clearer semantics: list for discovery, `GET /v1/assets/{asset_id}/download` or `/preview` for fresh short-lived access.

## Recommendation

Do not add root-level list `download_url` / `preview_url` in V1.3-A1. Frontend should request fresh signed access only when rendering or opening a specific asset, and should refresh on expiry/403/404.
