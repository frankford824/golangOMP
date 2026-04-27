# ASSET_ACCESS_POLICY

Last purified: 2026-04-11

Current authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`

## Current Policy

- Business asset storage is OSS-only.
- `source_access_mode` is now `standard` for business responses.
- Business download metadata returns:
  - `download_mode`
  - `download_url`
  - `access_hint`
- Resource metadata may also expose:
  - `archive_status`
  - `archived_at`
  - `last_access_at`
- `download_url` is the only supported file URL field in current business contracts.
- `public_url`, `lan_url`, `tailscale_url`, and private-network-only semantics are removed from the mainline.
- Canonical resource lookup now starts from `/v1/assets*`; task routes expose task-linked views, not a separate storage model.

## Access Semantics

- `reference_direct`: task-create or reference-style direct access through the backend proxy route
- `source_controlled`: source file governed by task/business access rules, but still OSS-backed
- `delivery_flow`: delivery asset governed by audit/warehouse flow
- `preview_assist`: preview-only helper asset

## Removed Policy Elements

The following are intentionally not part of current business behavior:

- NAS private-network requirement
- NAS browser probe evidence
- NAS allowlist logic
- NAS LAN/Tailscale URL emission
- NAS download fallback
