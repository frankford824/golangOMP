# Frontend Customization Handoff

Last purified: 2026-04-16

Current authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

This file is a frontend-facing handoff for the current customization lane only.

## What Is Implemented In This Round

- Customization is a first-class lane selected at task creation by `customization_required=true`.
- A customization task creates its primary `customization_job` immediately and enters customization review directly.
- Frontend role exposure now includes:
  - `CustomizationReviewer`
  - `CustomizationOperator`
- Department-role expectations in this round:
  - `CustomizationOperator` belongs to `定制美工部`
  - customization review belongs to `审核部` (`CustomizationReviewer`)
  - normal design remains under `设计研发部`
- Customization workbench flow is:
  1. task create
  2. `POST /v1/tasks/{id}/customization/review`
  3. `POST /v1/customization-jobs/{id}/effect-preview`
  4. optional `POST /v1/customization-jobs/{id}/effect-review`
  5. `POST /v1/customization-jobs/{id}/production-transfer`
  6. warehouse receive / reject / complete
- `/v1/customization-jobs` and `/v1/customization-jobs/{id}` are the primary list/detail entry points for customization management and workbench pages.
- Task detail, task events, `/v1/tasks/{id}/assets`, `/v1/assets*`, `/v1/task-board/*`, and `/v1/workbench/preferences` now accept customization reviewer/operator roles in runtime.

## Request Semantics

- `POST /v1/customization-jobs/{id}/effect-preview`
  - send `operator_id`
  - send `current_asset_id`
  - optionally send `order_no`
  - send `decision_type`
  - `decision_type=effect_preview` enters second review
  - `decision_type=final` skips second review and enters production transfer
- `POST /v1/customization-jobs/{id}/effect-review`
  - reviewer may send a replacement `current_asset_id` when directly修稿
  - `return_to_designer` routes to `pending_effect_revision`
  - `reviewer_fixed` routes to `pending_production_transfer`
- `POST /v1/customization-jobs/{id}/production-transfer`
  - send `operator_id`
  - send `current_asset_id`
  - optional trace-only fields: `transfer_channel`, `transfer_reference`
- `POST /v1/tasks/{id}/warehouse/reject`
  - may send `reject_category`
  - customization branch returns to the last customization operator instead of restarting the whole lane
- `GET /v1/warehouse/receipts`
  - use `workflow_lane` query param to split unified warehouse views by lane
  - each item includes `workflow_lane` and canonical upstream `source_department`

## Pricing Split

- reviewer stage writes business-entered review reference data on `customization_job`:
  - `customization_level_code`
  - `customization_level_name`
  - `review_reference_unit_price`
  - `review_reference_weight_factor`
  - `customization_review_decision`
- these are manual business-input fields from production templates/forms for request acceptance, persistence, and read-back.
- do not treat reviewer reference values as an automatic settlement derivation source.
- execution settlement freeze remains separate and happens later on first effect-preview submission:
  - `pricing_worker_type`
  - `unit_price`
  - `weight_factor`
- frontend must not treat reviewer reference pricing as the frozen settlement snapshot

## Asset Handling

- Keep using canonical `/v1/assets*` and upload sessions.
- Recommended asset-kind usage in customization pages:
  - source packages, fonts, editable originals: `source`
  - working稿, reviewer-fixed稿, effect稿, production稿: `delivery`
- Replacing an initial稿 means uploading a new asset or version and then submitting the new `current_asset_id`.
- reviewer-fixed replacement and effect-review replacement stay traceable by event payload (`previous_asset_id` -> `current_asset_id`) together with `workflow_lane` and canonical upstream `source_department`.
- Preview/download stays on canonical `GET /v1/assets/{id}/preview` and `GET /v1/assets/{id}/download`.

## Placeholder Boundaries

- `order_no` is stored and queryable through job detail, but this round does not add ERP order-detail verification.
- `transfer_channel` and `transfer_reference` are trace fields only; no external robot or ERP transfer callback contract is added in this round.
- Statistics pages are not delivered in this round. Backend only persists the base fields/events needed for later aggregation.
- “常规改定制” mid-flight lane conversion is not implemented in this round and remains a follow-up design item.
