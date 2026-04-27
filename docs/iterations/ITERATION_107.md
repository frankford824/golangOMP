# ITERATION_107

## Phase
MAIN batch task detail image return root-cause fix and single/batch read-model alignment

## Scope
- Investigate and fix missing image preview projection in batch task detail.
- Keep formal contract unchanged:
  - `reference_file_refs`
  - `design_assets`
  - `asset_versions`
- Add regression tests for single/batch consistency.

## Core Answers
1. Batch reference-image ownership:
   - Current formal ownership is mother-task level (`task_details.reference_file_refs_json`).
   - No formal item-level reference-image field exists in `task_sku_items`.
2. `GET /v1/tasks/{id}` detail read-model aggregation:
   - `reference_file_refs`: parsed from `task_details.reference_file_refs_json` (legacy fallback only when formal empty).
   - `design_assets`: from `design_assets` roots (primary).
   - `asset_versions`: from `task_assets` versions under each root.
3. Why single looked normal while batch was missing:
   - read-model exited early when `design_assets` roots were empty.
   - in affected lanes, task-level versions existed but roots were missing; projection returned empty arrays.
4. Mother-task projection gap:
   - yes, when roots were missing, detail read-model did not project available task-level version facts.
5. Item-level aggregation gap:
   - no item-level image ownership model is present; issue was root-empty projection, not SKU-item image storage.

## Root Cause
- `service/loadTaskDesignAssetReadModel` returned empty `design_assets` and `asset_versions` whenever `design_assets` roots were empty, even if task-level `task_assets` version records existed.
- This created observed single/batch divergence in real data.

## Runtime Fix
- Updated `service/task_design_asset_read_model.go`:
  - preserve existing formal root-first path.
  - add fallback projection path when roots are empty:
    - group task-level versions (`task_assets`) by `asset_id`
    - build response-level roots and hydrate derived fields/roles
    - return aligned `design_assets` + `asset_versions`
- No contract changes and no legacy-field promotion.

## Tests Added
- `service/task_design_asset_read_model_test.go`
  - `TestLoadTaskDesignAssetReadModelFallsBackWhenRootsMissing`
- `service/task_read_model_asset_versions_test.go`
  - `TestTaskReadModelBatchIncludesReferenceFileRefsAndFallbackAssetVersions`
  - includes formal `reference_file_refs_json` precedence over legacy `reference_images_json`
- `service/task_detail_asset_versions_test.go`
  - `TestTaskDetailAggregateBatchIncludesFallbackAssetVersions`

## Local Verification
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

## Publish
- Executed overwrite deploy to existing `v0.8`:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 batch task detail image fallback projection fix"`
- Release evidence (`deploy/release-history.log`):
  - deployed at `2026-04-02T04:44:38Z`
  - artifact sha256: `791de8fac0082de48ebdbb9e511586574f9e7c3feabdd0f288011c1031bbcfce`
- Runtime verification:
  - `GET /health` (8080) => `{"status":"ok"}`
  - `/proc/3774193/exe` => `/root/ecommerce_ai/releases/v0.8/ecommerce-api`

## Live Verification
- Executed remote single/batch image-detail closure with real upload-complete flow (LAN upload URL rewritten to returned tailscale host for reachability in this environment).
- Verification sample:
  - single `task_id=338`:
    - `reference_file_refs_count=1`
    - `design_assets_count=1`
    - `asset_versions_count=1`
    - `preview_public_url_ok=true`
  - batch `task_id=339`:
    - `reference_file_refs_count=1`
    - `design_assets_count=1`
    - `asset_versions_count=1`
    - `preview_public_url_ok=true`
- Result: single/batch detail image return behavior aligned on formal fields.

## Boundary
- Formal image contract remains unchanged:
  - references: `reference_file_refs`
  - roots: `design_assets`
  - versions: `asset_versions`
- Legacy compatibility fields remain non-canonical.
