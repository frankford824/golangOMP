# ASSET_STORAGE_AND_FLOW_RULES

Last purified: 2026-04-15

## Current Rules

- MAIN business asset storage is OSS-only.
- Runtime code must not depend on NAS path semantics.
- `storage_provider=oss` is the only valid business storage provider.
- `storage_key` is the canonical persisted object identifier for business download/proxy flows.
- Asset download metadata must use `download_url`, not NAS-specific URL families.

## Flow Rules

- Pre-task reference upload uses `POST /v1/tasks/reference-upload`.
- Browser-direct asset upload uses `POST /v1/assets/upload-sessions`.
- Canonical task-linked asset lookup is `GET /v1/tasks/{id}/assets`.
- `/v1/tasks/{id}/asset-center/assets*` remains compatibility-only alias family for migration safety.
- Browser-direct asset uploads must be completed through `/complete`.
- `scope_sku_code` remains required for multi-SKU non-reference uploads.
- Batch audit submission waits for all required SKU-scoped delivery assets.
- Customization lane must reuse the same canonical asset and upload-session system; do not create a separate customization asset service.
- Customization asset intent mapping for this round is:
  - `asset_kind=source`: large-art source files, font files, other editable source packages
  - `asset_kind=delivery`: reviewer-fixed稿, customization operator稿, effect-review replacement稿, production download稿
  - `asset_kind=preview` / `design_thumb`: backend-derived preview artifacts linked through `source_asset_id`
- Replacing the first customization稿 is modeled by uploading a new asset or version and moving the customization job `current_asset_id` to the latest effective asset.
- Source-to-preview and source-to-derived relationships continue to use `source_asset_id`; this remains the only supported linkage field.
- Warehouse and customization roles now consume the same `/v1/assets*` download and preview routes as other lanes.

## Explicit Non-Rules

These are no longer allowed in the business mainline:

- NAS upload sessions
- NAS multipart orchestration
- NAS browser-direct upload contracts
- NAS download URLs
- NAS fallback asset resolution
- OSS-first-then-NAS fallback
