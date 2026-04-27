# ITERATION_037 - Task Asset Storage / Upload Adapter Boundary Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUTO_037.md`

## 1. Goals
- Execute exactly one new phase after Step 36:
  - task asset storage / upload adapter boundary hardening
- Keep existing task-asset, export `result_ref`, `product_selection`, category, and cost contracts additive and non-regressive.
- Avoid real NAS/object storage/upload infrastructure while making the future seam explicit.

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_036.md`
- `docs/phases/PHASE_AUTO_037.md`
- latest PRD / V7 implementation spec

## 3. Files Changed
- `docs/phases/PHASE_AUTO_037.md`
- `db/migrations/020_v7_asset_storage_upload_boundary.sql`
- `domain/asset_storage.go`
- `domain/task_asset.go`
- `repo/interfaces.go`
- `repo/mysql/task_asset.go`
- `repo/mysql/upload_request.go`
- `repo/mysql/asset_storage_ref.go`
- `service/asset_upload_service.go`
- `service/task_asset_service.go`
- `service/task_step04_service_test.go`
- `transport/handler/asset_upload.go`
- `transport/handler/task_asset.go`
- `transport/handler/design_submission.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_037.md`
- `MODEL_HANDOVER.md`

## 4. DB / Migration Changes
- Added migration `020_v7_asset_storage_upload_boundary.sql`.
- Added `upload_requests` for placeholder upload-intent records.
- Added `asset_storage_refs` for placeholder storage-reference metadata.
- Extended `task_assets` additively with:
  - `upload_request_id`
  - `storage_ref_id`
  - `mime_type`
  - `file_size`

## 5. API Changes
- Added internal placeholder APIs:
  - `POST /v1/assets/upload-requests`
  - `GET /v1/assets/upload-requests/{id}`
- Extended existing task-asset write payloads additively:
  - `POST /v1/tasks/{id}/submit-design`
  - `POST /v1/tasks/{id}/assets/mock-upload`
  - new optional fields:
    - `upload_request_id`
    - `mime_type`
    - `file_size`
- Extended task-asset read model additively:
  - `upload_request_id`
  - `storage_ref_id`
  - `mime_type`
  - `file_size`
  - nested `storage_ref`

## 6. Design Decisions
- Kept `task_assets` as the business asset timeline and did not convert it into a real file-service table.
- Introduced two separate placeholder concepts:
  - `upload_requests` = upload intent before asset binding
  - `asset_storage_refs` = storage/reference metadata after asset binding
- Preserved legacy `file_path` / `whole_hash`:
  - still accepted
  - now treated as migration-era metadata
  - not treated as the preferred future boundary
- Task-asset writes now always create a placeholder `storage_ref`, with or without a prior upload request.
- `submit-design` and `mock-upload` may optionally consume `upload_request_id`, binding that request to the created task asset and `storage_ref`.

## 7. Export Relation
- This round does not merge task-asset `storage_ref` with export `result_ref`.
- The intended shared boundary is now explicit:
  - adapter
  - ref type
  - ref key
  - file metadata
  - placeholder flag
  - status
- `result_ref` remains export-center-specific handoff metadata, while `storage_ref` is the task-asset-side placeholder reference model.
- Future NAS/object-storage/file-service work should attach beneath those metadata boundaries rather than adding more path/hash semantics directly into business tables.

## 8. Correction Notes
- `CURRENT_STATE.md` and `MODEL_HANDOVER.md` previously still described task-asset file flow mainly as Step-04 `mock-upload` plus legacy `file_path` / `whole_hash`.
- This iteration corrects that drift by documenting the new Step 37 placeholder boundary and advancing OpenAPI from `0.32.1` to `0.33.0`.

## 9. Verification
- `gofmt -w` on all changed Go files
- `go test ./...`
  - partial success; `workflow/service` test binary execution is blocked locally by Application Control policy
  - other packages completed
- `go test -c ./service`
  - compile-only verification for `workflow/service`
- OpenAPI YAML parse validation via local Python `yaml.safe_load`

## 10. Risks / Known Gaps
- No real file upload transport exists yet.
- No NAS/object storage/CDN/download service exists yet.
- `whole_hash` remains checksum-hint metadata only and is not strictly verified.
- Upload requests are internal placeholder APIs, not frontend-ready upload sessions.
- Export `result_ref` and task-asset `storage_ref` are only semantically aligned for now; they are not yet unified into one shared persistence model.

## 11. Next Batch Recommended Roadmap
1. Step 38: integration center execution boundary hardening
2. Step 39: export runner / storage boundary planning
3. Step 40: cost-rule governance / versioning / override hardening
4. Keep real file infrastructure deferred until after those placeholder boundaries stay stable
