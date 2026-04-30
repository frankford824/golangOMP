# v0.9 Backend Source Of Truth

Last purified: 2026-04-17

This document is the authoritative v1.0 backend policy and route-classification summary for MAIN. Use it together with `docs/api/openapi.yaml`.

Current round status:

- code complete
- docs complete
- local validation complete
- v1.0 RC validation passed
- release prep authorized
- v1.0 convergence closure completed on 2026-04-16
- auth_identity.json updated to v1.0 official department structure
- cleanup SQL updated to cover customization_jobs
- sensitive credentials removed from cleanup scripts
- 2026-04-16 v1.0 org-master-data convergence round (review-first): `config/auth_identity.json` pruned to official-only departments/teams; `db/migrations/057_v1_0_org_master_convergence.sql` migrates legacy user rows into the v1.0 baseline and disables legacy rows in `org_departments` / `org_teams`, so `/v1/org/options` no longer exposes legacy operations groups 1-7 or legacy compatibility departments
- 2026-04-17 Round H v1.7: `authorizeUserRoleChange` now mirrors the route-layer `/v1/users/:id/roles*` guard and accepts only `HRAdmin` and `SuperAdmin`

If documents disagree:

1. `transport/http.go` decides what is mounted now.
2. `docs/api/openapi.yaml` decides the current request/response contract.
3. This file decides route class, canonical usage, compatibility policy, and removal intent.
4. `docs/archive/*`, `docs/iterations/*`, `CURRENT_STATE.md`, and `MODEL_HANDOVER.md` are history or handoff index only unless restated here.

## Authority Set

### Primary authority

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

### Secondary current-use guides

- `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
- `docs/TASK_CREATE_RULES.md`
- `docs/API_USAGE_GUIDE.md`
- `docs/ASSET_UPLOAD_INTEGRATION.md`
- `docs/ASSET_ACCESS_POLICY.md`
- `docs/ASSET_STORAGE_AND_FLOW_RULES.md`
- `docs/FRONTEND_CUSTOMIZATION_HANDOFF.md`
- `docs/ops/NAS_SSH_ACCESS.md`

### Archive only

- `docs/archive/*`
- `docs/iterations/*`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/archive/V7_MODEL_HANDOVER_APPENDIX.md`

## Official v0.9 Mainline

New frontend work and new backend work must start on these route families:

- Auth and access:
  - `/v1/auth/*`
  - `/v1/access-rules`
  - `/v1/org/options`
  - `/v1/org/departments*`
  - `/v1/org/teams*`
  - `/v1/roles`
  - `/v1/users*`
- ERP and product selection:
  - `/v1/erp/products*`
  - `/v1/erp/categories`
  - `/v1/erp/warehouses`
  - `/v1/erp/sync-logs*`
- Task create and lifecycle:
  - `/v1/tasks/reference-upload`
  - `/v1/tasks/prepare-product-codes`
  - `/v1/tasks*`
  - `/v1/tasks/{id}/product-info`
  - `/v1/tasks/{id}/cost-info`
  - `/v1/tasks/{id}/business-info`
  - `/v1/tasks/{id}/filing-status`
  - `/v1/tasks/{id}/filing/retry`
  - `/v1/tasks/{id}/procurement*`
  - `/v1/tasks/{id}/detail`
  - `/v1/tasks/{id}/cost-overrides`
  - `/v1/tasks/{id}/events`
  - `/v1/tasks/{id}/customization/review`
  - `/v1/customization-jobs*`
- Canonical asset resource layer:
  - `/v1/assets`
  - `/v1/assets/{id}`
  - `/v1/assets/{id}/download`
  - `/v1/assets/{id}/preview`
  - `/v1/assets/upload-sessions*`
  - `/v1/assets/files/{path}`
- Canonical task-linked asset lookup:
  - `/v1/tasks/{id}/assets`
- Task-linked compatibility aliases (migration safety only, no new integration):
  - `/v1/tasks/{id}/asset-center/assets*`
  - `/v1/tasks/{id}/asset-center/upload-sessions`
  - `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}*`
- Workflow actions:
  - `/v1/tasks/{id}/assign`
  - `/v1/tasks/batch/*`
  - `/v1/tasks/{id}/submit-design`
  - `/v1/tasks/{id}/audit/*`
  - `/v1/tasks/{id}/warehouse/*`
  - `/v1/tasks/{id}/customization/review`
  - `/v1/customization-jobs/{id}/effect-preview`
  - `/v1/customization-jobs/{id}/effect-review`
  - `/v1/customization-jobs/{id}/production-transfer`
  - `/v1/warehouse/receipts`

Frontend rollout must treat the following OSS routes as the only canonical asset contract:

- `POST /v1/tasks/reference-upload`
- `POST /v1/assets/upload-sessions`
- `POST /v1/assets/upload-sessions/{session_id}/complete`
- `POST /v1/assets/upload-sessions/{session_id}/cancel`
- `GET /v1/assets/{id}/download`
- `GET /v1/assets/{id}/preview`

## OSS-Only Asset Runtime

### Storage backend policy

- OSS is the only official business storage backend.
- NAS is not part of the business upload path.
- NAS is not part of the business download path.
- NAS is not part of the business storage metadata model.
- Historical NAS paths, NAS URLs, NAS browser probes, NAS allowlists, and NAS fallback logic are intentionally unsupported.
- Historical NAS assets are intentionally abandoned in this stage.

### Official upload modes

- `POST /v1/tasks/reference-upload`
  - backend-proxy upload to OSS object storage (prefer OSS direct write when enabled; fallback to upload-service proxy only when OSS direct is unavailable)
  - returns one normalized `reference_file_ref`
- `POST /v1/assets/upload-sessions`
  - browser-direct upload session creation with backend-selected strategy
  - current `v0.9` live runtime emits `multipart` for all task-scoped asset kinds, including `reference`
- `POST /v1/assets/upload-sessions/{session_id}/complete`
  - backend verifies/finalizes remote OSS result and persists asset metadata
  - for multipart sessions, MAIN complete is allowed to perform backend-side remote complete fallback when browser-side remote complete confirmation is still pending
- `POST /v1/assets/upload-sessions/{session_id}/cancel`
  - backend cancels MAIN business session and aborts the remote OSS upload session when needed

### Official download mode

- Asset download metadata routes now return direct OSS/browser-consumable download metadata by default:
  - `download_mode=direct`
  - `download_url` points to browser-direct OSS URL only when a real browser-direct URL is available
- `download_mode=direct` must not be used for compatibility proxy paths like `/v1/assets/files/{path}` or `/files/{path}`.
- `UPLOAD_SERVICE_BROWSER_DOWNLOAD_BASE_URL` is the runtime switch for building browser-direct object URLs from `storage_key` when upload-service direct URL fields are absent.
- `GET /v1/assets/files/{path}` remains mounted as compatibility-only proxy/fallback byte route.
- public nginx must prioritize `^~ /v1/` over static-extension regex rules so preview/reference image downloads are not misrouted to the frontend static root.
- No NAS LAN URL, no Tailscale URL, no private-network-only route, and no NAS file-path resolution remain in the business mainline.

### Runtime metadata truth

Business runtime metadata is centered on OSS-style fields and identifiers:

- `storage_provider=oss`
- `storage_key` / object key
- `remote_upload_id`
- `remote_file_id`
- `mime_type`
- `file_size`
- `file_hash`
- `download_url`

Runtime code must not depend on NAS path semantics or legacy NAS columns to resolve files.

### PSD preview ownership

- Frontend must not parse or preview raw PSD/PSB directly.
- Backend owns PSD preview direction and model shape.
- Source preview in `v0.9` is two-tier:
  - Tier 1 (direct): for OSS IMG-supported source formats (`jpg/png/bmp/gif/webp/tiff/heic/avif`), `GET /v1/assets/{id}/preview` returns a signed OSS direct URL with `x-oss-process`.
  - Tier 2 (derived): for non-direct formats (notably `psd/psb`), backend asynchronously derives and stores `preview/design_thumb` assets after source upload completion.
- Canonical model distinguishes:
  - source asset (`asset_kind=source`) for raw editable file
  - preview asset (`asset_kind=preview`) for backend-generated preview artifact
  - thumb asset (`asset_kind=design_thumb`) for backend-generated thumbnail artifact
- Preview/thumb assets can be linked to source via `source_asset_id`.
- `GET /v1/assets/{id}/preview` for non-direct source assets must return `409 INVALID_STATE_TRANSITION` (`asset preview is not available`) until a linked derived preview is ready.
- Raw source remains downloadable through canonical `download_url` when business policy allows.

## Canonical Field Truth

- Task-create references:
  - canonical create field: `reference_file_refs`
  - canonical pre-task upload route: `POST /v1/tasks/reference-upload`
  - rejected legacy field: `reference_images`
- Task ownership:
  - canonical fields: `owner_department`, `owner_org_team`
  - compatibility field: `owner_team`
  - compatibility department names remain accepted for migration safety, but runtime create/read paths normalize them back to the canonical current departments
- Task lane projection:
  - canonical lane split source field: `customization_required`
  - canonical read projection field: `workflow_lane` (`normal` / `customization`)
  - `/v1/tasks` and workbench `query_template` now accept `workflow_lane` as the canonical list/workbench split filter
  - warehouse unified intake read model now carries `workflow_lane` and canonical upstream `source_department`
- Task actor/source projections:
  - canonical requester fields: `requester_id`, `requester_name`
  - canonical active-responsibility fields: `designer_id`, `designer_name`, `current_handler_id`, `current_handler_name`
  - canonical creation-source fields: `creator_id`, `creator_name`
- Design assets:
  - canonical namespace: `/v1/assets*`
  - canonical task-scoped list path: `/v1/tasks/{id}/assets`
  - task-linked compatibility namespace: `/v1/tasks/{id}/asset-center/*`
  - compatibility namespace: `/v1/tasks/{id}/assets*` except `GET /v1/tasks/{id}/assets`
  - compatibility-only input field: `upload_mode` (new frontend integrations must use backend-returned `upload_strategy`)
  - canonical fields: `design_assets`, `asset_versions`, `scope_sku_code`, `source_asset_id`
  - canonical asset_kind values in active business flow: `reference`, `source`, `delivery`, `preview`, `design_thumb`
  - minimal lifecycle fields: `upload_status`, `archive_status`, `archived_at`, `last_access_at`
  - canonical file access fields: `download_mode`, `download_url`, `storage_provider`, `storage_key`
  - compatibility-only file fields still present in some payloads: `ReferenceFileRef.url`, `public_download_allowed`, `preview_public_allowed`
  - new frontend work must not consume those compatibility-only file fields
- Product-code generation:
  - canonical preview path: `POST /v1/tasks/prepare-product-codes`
  - task create remains backend-owned for default SKU allocation
- Frontend access:
  - current naming target is `frontend_access.roles/scopes/menus/pages/actions/modules`
  - frontend-ready menu/page visibility must include `resource_management` and `customization_management`
  - current frontend pages for this area include `assets_index`, `task_assets`, `asset_detail`, `customization_jobs`, `customization_job_detail`

## Customization Flow Truth (MVP)

- ERP order-detail linkage is explicitly out of scope for customization flow.
- Runtime must not call ERP order-info/order-detail APIs for customization routing, pricing, remark sync, or blocking checks.
- Customization level must remain business-configurable and non-hardcoded; do not assume fixed `A/B/C` buckets.
- Full-time vs part-time distinction is a pricing identity (`employment_type`), not a primary permission split.
- Do not introduce role explosion for pricing identity; permissions still answer who can operate workflow actions.
- Minimal pricing source for customization piece-rate is keyed by:
  - `customization_level_code`
  - `employment_type` (`full_time`, `part_time`)
- Canonical review output fields are:
  - `customization_level_code`
  - `customization_level_name`
  - `customization_price`
  - `customization_weight_factor`
  - `customization_note`
  - `customization_review_decision` (`approved`, `return_to_designer`, `reviewer_fixed`)
- Customization-job reviewer reference fields persist separately from execution freeze:
  - `review_reference_unit_price`
  - `review_reference_weight_factor`
- In this phase, customization review annotations are business-entered reference fields from production forms/templates.
- Review stage only requires request acceptance, persistence, and read-back for those reference fields.
- Do not derive execution settlement automatically from review reference values at review time.
- Do not introduce a new pricing-engine or auto-grading platform in this phase.
- Canonical split fields on task are:
  - `customization_required` as the canonical creation-time lane selector
  - `customization_source_type` (`new_product`, `existing_product`)
- `need_outsource` is compatibility-only for new integrations and is now a derived or legacy intent marker; mainline workflow entry must not depend on it.
- Task-create normalization must fold legacy `need_outsource` / `is_outsource` inputs into `customization_required=true`.
- A task created with `customization_required=true` enters `PendingCustomizationReview` immediately and must not enter the normal design workbench first.
- Each customization-lane task must have one primary `customization_job` record created during task creation so customization list/workbench visibility exists immediately.
- Task workflow sub-status filtering must treat `customization` as the canonical lane scope keyword; `outsource` remains compatibility-only alias for legacy callers.
- Pricing snapshot on each customization job must persist:
  - `pricing_worker_type`
  - `unit_price`
  - `weight_factor`
- Reviewer-stage reference pricing must not be collapsed into the frozen execution snapshot:
  - review sets level metadata plus `review_reference_unit_price` / `review_reference_weight_factor`
  - freeze later resolves `pricing_worker_type` + `unit_price` + `weight_factor`
- Customization-job runtime tracking fields must persist:
  - `order_no`
  - `source_asset_id`
  - `current_asset_id`
  - `assigned_operator_id`
  - `last_operator_id`
- Replacement/audit trace events for customization and audit handoff must remain searchable with:
  - `previous_asset_id`
  - `current_asset_id`
  - `replacement_actor_id`
  - `workflow_lane`
  - canonical upstream `source_department`
- Pricing snapshot freeze point for MVP is operator assignment entry (first effect-preview submission).
- Pricing snapshot resolution is mandatory at freeze point and uses `(employment_type + customization_level_code)` only.
- If the pricing rule is missing at freeze point, effect-preview submission must fail with a clear 4xx error and must not partially advance task/job status.
- `POST /v1/customization-jobs/{id}/effect-preview` is the customization-operator work entry:
  - it must capture `operator_id`
  - it may capture `order_no`
  - it must point at the effective working稿 through `current_asset_id`
  - `decision_type=effect_preview` advances to review
  - `decision_type=final` skips effect-review and advances directly to production transfer
- `POST /v1/customization-jobs/{id}/effect-review` is the customization-reviewer second gate:
  - `return_to_designer` routes only to `PendingEffectRevision`
  - `reviewer_fixed` may replace the effective稿 via `current_asset_id`
  - reviewer-fixed replacement must be traceable in task events with `previous_asset_id` + `current_asset_id`; it is not a silent overwrite
- `POST /v1/customization-jobs/{id}/production-transfer` is the bounded production handoff:
  - runtime keeps this as a system-traceable action/event
  - `transfer_channel` / `transfer_reference` are placeholder trace fields for robot transfer or later integration
  - this round does not introduce a new external transfer platform
- Customization workflow status chain is:
  - `PendingCustomizationReview`
  - `PendingCustomizationProduction`
  - `PendingEffectReview`
  - `PendingEffectRevision`
  - `PendingProductionTransfer`
  - `PendingWarehouseQC`
  - `RejectedByWarehouse`
  - `Completed`
- Customization job status chain is:
  - `pending_customization_review`
  - `pending_customization_production`
  - `pending_effect_review`
  - `pending_effect_revision`
  - `pending_production_transfer`
  - `pending_warehouse_qc`
  - `rejected_by_warehouse`
  - `completed`
- Warehouse reject in customization branch must route back to `last_customization_operator_id` and keep rejection reason/category on task/job.
- `last_customization_operator_id` tracks the most recent production-side customization operator and is updated on:
  - effect-preview submission
  - production-transfer submission
- Customization branch asset modeling must stay on canonical `/v1/assets*` + upload-session infrastructure:
  - large-art source files and fonts use `asset_kind=source`
  - reviewer-fixed稿, customization operator稿, effect-review replacement稿, and production download稿 use `asset_kind=delivery`
  - preview/thumb derivations stay on `asset_kind=preview` / `design_thumb`
  - replacing an initial稿 means creating a new asset/version timeline entry and updating `current_asset_id`; do not create a parallel customization asset system
- Frontend-facing role exposure for this lane is still based on the unified role/department model:
  - reviewer action role: `CustomizationReviewer`
  - operator action role: `CustomizationOperator`
  - canonical ownership remains `owner_department` + `owner_org_team`
  - `owner_team` remains compatibility-only
- Statistical outputs such as作图量、等级完成量、合格率、错误原因分类 are deferred in this round; only the base fields/events needed for later aggregation are persisted now.
- Excel note about “常规改定制” remains an explicit follow-up design concern; this round does not add mid-flight lane-conversion behavior.

## Organization And Permission Boundary Truth

- Current business target departments (v1.0 official, exposed via `/v1/org/options` and accepted by registration / user admin / task ownership inputs) are:
  - `人事部` → `人事管理组`
  - `运营部` → `淘系一组`, `淘系二组`, `天猫一组`, `天猫二组`, `拼多多南京组`, `拼多多池州组`
  - `设计研发部` → `默认组`
  - `定制美工部` → `默认组`
  - `审核部` → `普通审核组`, `定制审核组`
  - `云仓部` → `默认组`
- `未分配` / `未分配池` remains preserved as the system unassigned bucket and is the only non-business department that may appear in org selectors.
- Legacy department names (`设计部`, `采购部`, `仓储部`, `烘焙仓储部`) and legacy team names (`运营一组` .. `运营七组`, `研发默认组`, `定制默认组`, `云仓默认组`, `定制美工审核组`, `常规审核组`, `设计组`, `定制美工组`, `设计审核组`, `采购组`, `仓储组`, `烘焙仓储组`, 人事部`默认组`) are kept only as disabled compatibility rows in `org_departments` / `org_teams` for historical FK and audit integrity. They must not appear in `/v1/org/options` and must not be accepted by registration, user admin, or task create inputs.
- v1.0 Legacy-to-official user-row migration mapping (applied by `db/migrations/057_v1_0_org_master_convergence.sql`):
  - `设计部 / 设计组` → `设计研发部 / 默认组`
  - `设计部 / 定制美工组` → `定制美工部 / 默认组`
  - `设计部 / 设计审核组` → `审核部 / 普通审核组`
  - `设计部 / *其他*` → `设计研发部 / 默认组`
  - `采购部 / *任意*` → `运营部 / 淘系一组`
  - `仓储部`, `烘焙仓储部` / *任意* → `云仓部 / 默认组`
  - `人事部 / 默认组` → `人事部 / 人事管理组`
  - `设计研发部 / 研发默认组` → `设计研发部 / 默认组`
  - `定制美工部 / 定制默认组` → `定制美工部 / 默认组`
  - `云仓部 / 云仓默认组` → `云仓部 / 默认组`
  - `运营部 / 运营一组..运营七组` → `淘系一组` / `淘系二组` / `天猫一组` / `天猫二组` / `拼多多南京组` / `拼多多池州组` (运营六组/七组 both fold into `拼多多池州组`)
  - `审核部 / 定制美工审核组` → `审核部 / 定制审核组`
  - `审核部 / 常规审核组` → `审核部 / 普通审核组`
- Formal product-facing roles are:
  - `Ops`
  - `DepartmentAdmin`
  - `TeamLead`
  - `Designer`
  - `CustomizationOperator`
  - `Warehouse`
  - `HRAdmin`
  - `SuperAdmin`
- Legacy `OrgAdmin` / `Admin` / `RoleAdmin` remain compatibility-only and must not be expanded as new product roles.
- `CustomizationOperator` is a dedicated role and maps to the dedicated customization-art lane.
- Audit remains the unified quality gate for both design and customization upstream files.
- Product-facing audit grouping is:
  - `普通审核`
  - `定制审核`
  - backend may still keep `Audit_A` / `Audit_B` / `CustomizationReviewer` as technical roles
- `DepartmentAdmin` is department-scoped and owns the full department-management boundary:
  - allowed: view all members in own department
  - allowed: move members across teams in own department
  - allowed: assign unassigned users into own department
  - allowed: create user accounts in own department
  - allowed: disable user accounts in own department
  - allowed: reset passwords in own department
  - allowed: reassign tasks across teams inside own department
- As of v1.7, service-layer `authorizeUserRoleChange` mirrors the route-layer guard and accepts only `HRAdmin` and `SuperAdmin`.
- `TeamLead` is scoped to own team:
  - allowed: manage own team members only
  - allowed: view all tasks in own department
  - allowed: reassign only own-team tasks
  - not allowed: create/disable/reset accounts
  - not allowed: move users across departments
- `HRAdmin` and `SuperAdmin` remain company-wide management roles above `DepartmentAdmin`.
- Request actor runtime truth:
  - `RequestActor.frontend_access` must be hydrated to a minimal reliable view when session-token auth is used
  - task write/read authorization must not rely on a bare `frontend_access.view_all` flag without role-backed scope
  - `RequestActor.roles` and `RequestActor.frontend_access.roles` must be built from the same canonicalized role slice: the post-normalization slice that `prepareUserForResponse` persists on `user.Roles`, including the `[Member]` default-fallback. Server-side authorization (`authorizeUserRead`, `authorizeUserListFilter`, and all peer role-scope checks) reads that single source of truth.
- Log visibility policy:
  - `GET /v1/operation-logs`: `HRAdmin` / `SuperAdmin`, with legacy `Admin` kept only as compatibility access
  - `GET /v1/audit-logs`: audit roles plus management roles; department/team managers are row-filtered to own scope
- Operations reassignment policy:
  - visibility stays shared at the operations task center
  - reassignment is allowed only for requester/initiator, current owning-group `TeamLead`, or scoped management roles
  - ordinary Ops members without those conditions must be denied

## DataScope Stage Visibility (Round J)

Task read visibility is no longer limited to org-owner fields plus self matches. The canonical `DataScope` model now carries an explicit stage-visibility dimension for mid-lane business roles. This stage dimension is authoritative for task list reads, task-board candidate reads, and task detail read authorization. It does not widen any write action scope and does not imply `ViewAll`.

- `Audit_A` -> `PendingAuditA`, `RejectedByAuditA`, lane=`normal`
- `Audit_B` -> `PendingAuditB`, `RejectedByAuditB`, lane=`normal`
- `Warehouse` -> `PendingWarehouseQC`, `PendingWarehouseReceive`, `RejectedByWarehouse`, `PendingProductionTransfer`, lane unrestricted
- `Outsource` -> `PendingOutsource`, `Outsourcing`, `PendingOutsourceReview`, lane unrestricted
- `CustomizationOperator` -> `InProgress`, `PendingCustomizationProduction`, `RejectedByAuditA`, `RejectedByAuditB`, lane=`customization`
- `CustomizationReviewer` -> `PendingCustomizationReview`, `PendingEffectReview`, `PendingEffectRevision`, lane=`customization`
- `Designer` gets no stage visibility grant and continues to rely on `designer_id`
- `Ops` gets no stage visibility grant and continues to rely on canonical owner-org visibility (`owner_department` / `owner_org_team`)

Department-admin union rule:

- `DepartmentAdmin` in `审核部` gets `Audit_A ∪ Audit_B ∪ CustomizationReviewer`
- `DepartmentAdmin` in `云仓部` gets the `Warehouse` stage set
- `DepartmentAdmin` in `定制美工部` gets `CustomizationOperator ∪ CustomizationReviewer`, all with lane=`customization`
- `DepartmentAdmin` in `运营部`, `设计研发部`, `人事部`, or `未分配` gets no extra stage grant
- `DesignDirector` and `TeamLead` get no extra stage grant

Lane scoping is mandatory where specified above. In particular, `CustomizationOperator` and `CustomizationReviewer` must match `workflow_lane=customization`, and `Audit_A` / `Audit_B` stage visibility must match `workflow_lane=normal`. Repo SQL may implement this through the canonical lane projection source (`customization_required`) so long as the externally visible rule remains lane-equivalent.

## Compatibility And Removal Policy

Compatibility families still mounted for migration safety are route aliases only. They must not preserve NAS semantics.
They are obsolete for frontend rollout and must not be used for new integration.

| Route family | Class | Successor | New usage allowed? | Policy |
|---|---|---|---|---|
| `/v1/task-create/asset-center/upload-sessions*` | compatibility only | `POST /v1/tasks/reference-upload` | no | keep only until frontend migration finishes |
| `GET /v1/tasks/{id}/assets` | canonical task-scoped resource list | `GET /v1/assets?task_id={id}` | yes | keep as canonical task-context read path |
| `/v1/tasks/{id}/assets*` (except `GET /v1/tasks/{id}/assets`) | compatibility only | `/v1/tasks/{id}/asset-center/*` for resource reads; `/v1/assets/upload-sessions*` for upload-session subpaths | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/assets` | compatibility only | `GET /v1/tasks/{id}/assets` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions` | compatibility only | `GET /v1/tasks/{id}/assets/{asset_id}/versions` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/assets/{asset_id}/download` | compatibility only | `GET /v1/tasks/{id}/assets/{asset_id}/download` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download` | compatibility only | `GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/upload-sessions` | compatibility only | `/v1/assets/upload-sessions` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}` | compatibility only | `/v1/assets/upload-sessions/{session_id}` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/upload-sessions/small` | compatibility only | `/v1/assets/upload-sessions` | no | keep only until frontend migration finishes |
| `/v1/tasks/{id}/asset-center/upload-sessions/multipart` | compatibility only | `/v1/assets/upload-sessions` | no | keep only until frontend migration finishes |
| `POST /v1/tasks/{id}/assets/upload` | deprecated | `/v1/assets/upload-sessions` | no | keep explicit deprecation response only |
| `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort` | compatibility only | `/v1/assets/upload-sessions/{session_id}/cancel` | no | keep alias only until frontend migration finishes |
| `POST /v1/tasks/{id}/outsource` | compatibility only | create the task with `customization_required=true` and operate on `/v1/tasks/{id}/customization/review` + `/v1/customization-jobs*` | no | retain only for historical late-branch tasks |
| `GET /v1/outsource-orders` | compatibility only | `/v1/customization-jobs` | no | retain only for historical late-branch visibility |
| `/v1/products/*` and `/v1/erp/products*` | active compatibility surface | keep both until real caller usage is fully verified | no | do not remove or tighten blindly in this round |

## Final closure truths

1. Task creation supports multiple reference-image uploads through `POST /v1/tasks/reference-upload` + `reference_file_refs`.
2. Task flow supports large design-file upload/download through OSS-backed upload sessions and canonical download metadata.
3. Frontend must not choose small-vs-multipart endpoint families; backend decides strategy and returns `upload_strategy`.
4. Backend owns PSD preview direction; frontend consumes preview/thumb assets and metadata, not raw PSD parsing.
5. Asset/resource is first-class (`/v1/assets*` + explicit metadata model), not hidden task attachments.
6. Task remains workflow mainline but is not the sole resource anchor; task-to-asset linkage is explicit and queryable.
7. Backend business APIs own session creation/validation/completion and metadata persistence.
8. OSS stores objects only; canonical business-facing file access is `download_url` with `download_mode=direct` when direct URL can be issued, while `/v1/assets/files/{path}` is compatibility-only proxy fallback.

### Asset upload authorization

- Asset upload sessions (`/v1/assets/upload-sessions` create/complete/cancel) honor stage
  visibility in addition to organisational scope: a role that is granted stage visibility
  for a given (workflow_lane, task_status) pair is allowed to upload for tasks in that
  stage, subject to the action's AllowedStatuses and role set.

## Current Batch Delivery Submission Policy

- For multi-SKU delivery uploads, each non-reference upload must carry `target_sku_code`.
- Upload-session responses expose the captured batch SKU on `target_sku_code`.
- Persisted design-asset read models expose batch SKU scope on `scope_sku_code` for the asset root and asset version.
- Whole-task audit advance still waits for all required SKU-scoped delivery assets.
- `POST /v1/tasks/{id}/submit-design` now supports batch mode via `assets[]`, so one submit action can complete multiple delivery/source upload sessions.
- In batch mode, each delivery item must include `target_sku_code`, and it must match the upload session's captured `target_sku_code`.
- For pre-created task-scoped `source` / `delivery` upload sessions, `POST /v1/assets/upload-sessions/{session_id}/complete` remains allowed in `PendingAuditA` as a narrow post-transition completion window.
  This window exists to prevent batch completion races after state transition and does not allow creating new upload sessions in audit stages.
- This refactor does not change the batch audit gate; it only changes storage from NAS semantics to OSS semantics.

## NAS Boundary

- NAS remains available only as ops/developer infrastructure reachable by SSH/SCP/RSYNC.
- NAS operational access lives under `docs/ops/NAS_SSH_ACCESS.md`.
- NAS operational access must not be described as part of the business upload/download/storage design.

## Current Round Release Rule

For this round:

- release prep requires explicit authorization
- deployment is allowed only when release gates pass
- required DB migrations must be checked and handled before claiming full production readiness
- post-release runtime and business-flow verification remains mandatory before frontend rollout claims

## Reading Order

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `docs/ASSET_UPLOAD_INTEGRATION.md` when working on upload/download/frontend migration
5. `docs/ops/NAS_SSH_ACCESS.md` only when doing ops or developer NAS access work
