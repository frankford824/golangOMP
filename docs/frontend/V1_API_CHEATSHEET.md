# V1 API 速查表(242 path · 一行一条)

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 本表一行对应一个 `/v1` path；同一路径多 method 合并到 `Methods` 列。
> WebSocket 当前 OpenAPI 真实 path 为 `/ws/v1`，详见 `V1_API_WS.md`，不计入 242 个 `/v1` path。
> 新前端只接本表列出的当前 V8 路径。

| Methods | Path | Summary | RBAC | family doc |
|---|---|---|---|---|
| POST | `/v1/auth/register` | Register workflow user | POST:已登录 / scope-aware | [V1_API_AUTH.md](V1_API_AUTH.md) |
| GET | `/v1/auth/register-options` | Get registration department/team options | GET:已登录 / scope-aware | [V1_API_AUTH.md](V1_API_AUTH.md) |
| POST | `/v1/auth/login` | Login workflow user | POST:已登录 / scope-aware | [V1_API_AUTH.md](V1_API_AUTH.md) |
| GET | `/v1/auth/me` | Get current authenticated user | GET:已登录 / scope-aware | [V1_API_AUTH.md](V1_API_AUTH.md) |
| PUT | `/v1/auth/password` | Change current user password | PUT:已登录 / scope-aware | [V1_API_AUTH.md](V1_API_AUTH.md) |
| GET | `/v1/me/task-drafts` | List my task drafts | GET:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| GET, PATCH | `/v1/me` | Get my profile；Update my profile | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| POST, DELETE | `/v1/me/avatar` | Upload my avatar；Delete my avatar | POST:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| GET | `/v1/me/avatar-files/{filename}` | Read avatar file | GET:公开 | [V1_API_ME.md](V1_API_ME.md) |
| POST | `/v1/me/change-password` | Change my password | POST:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| GET | `/v1/me/org` | Get my org profile | GET:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| GET, POST | `/v1/users` | List workflow users；Create workflow user | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/users/designers` | List task assignment candidates | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET, PATCH | `/v1/users/{id}` | Get workflow user；Update workflow user | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| PUT | `/v1/users/{id}/password` | Reset workflow user password | PUT:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/users/{id}/activate` | Activate a workflow user | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/users/{id}/deactivate` | Deactivate a workflow user | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/org/options` | Get organization options | GET:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/departments` | Create organization department | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| PUT, DELETE | `/v1/org/departments/{id}` | Update organization department；Hard-delete organization department | PUT:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/departments/{id}/merge` | Merge organization department into another department | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/teams` | Create organization team | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| PUT, DELETE | `/v1/org/teams/{id}` | Update organization team；Hard-delete organization team | PUT:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/teams/{id}/merge` | Merge organization team into another team | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| GET | `/v1/access/permissions` | List the code-maintained capability catalog | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/access/roles` | List administrator-managed business roles；Create a business role | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/access/roles/{id}` | Update role display metadata | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/access/roles/{id}/archive` | Archive an in-use role without physical deletion | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PUT | `/v1/access/roles/{id}/permissions` | Atomically replace a role's capability set | PUT:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/access/users` | Search the minimal personnel selector for explicit assignments | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PUT | `/v1/access/users/{id}/assignments` | Atomically replace a user's roles and stable organization-ID scopes | PUT:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/access/users/{id}/effective` | Resolve capabilities, scopes, sources and policy revision | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PUT | `/v1/access/org-policies/{subject_type}/{subject_id}` | Read explicitly enabled defaults for an organization ID；Atomically replace policies for an organization ID | GET:已登录 / scope-aware; PUT:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/access/preview` | Preview the current effective-access projection for a user | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/access/events` | List access-policy audit events | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/submit-design` | Submit designer-selected mode and source files for unified audit | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/decision` | Approve and finalize, or return the task to design | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/audit/handover-candidates` | List PendingAudit tasks currently handled by the caller and eligible for handover | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/audit/handover-batch` | Hand over caller-owned PendingAudit tasks to an eligible auditor | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/handover` | Hand over a PendingAudit task currently handled by the caller | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/audit/handovers` | List audit handovers visible in the caller's task scope | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/takeover` | Accept a PendingAudit handover assigned to the caller | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/reopen` | Reopen a completed design or retouch task under optimistic concurrency | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/resource-bundle` | Read task, SKU and retouch resource groups | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/resource-groups` | Search the current working and finalized resource-group read model | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/resource-groups/{id}` | Read one resource group and its current revisions | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/resource-groups/{id}/revisions` | List every historical revision of one resource group | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/resource-groups/batch-download` | Expand finalized revision items into an ordered download manifest | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/sku-planning/image-upload-sessions` | Stage one planning-SKU product image before task creation | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/sku-planning/image-upload-sessions/{session_id}` | Read a planning-SKU image staging session | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/sku-planning/image-upload-sessions/{session_id}/complete` | Complete staging and return an image_upload_ref | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/sku-planning/image-upload-sessions/{session_id}/abort` | Abort an unbound planning-SKU image staging session | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/sku-planning/template.xlsx` | Download the standard or ERP planning-SKU import template | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/sku-planning/parse-excel` | Parse and validate an import workbook without creating a task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/planning-skus/{item_id}` | Create an immutable correction revision for a completed planning SKU | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/planning-skus` | Get the current planning-SKU rows and private product-image previews for one task | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/planning-skus/export.xlsx` | Export all planning SKUs for one task | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/planning-skus/export.xlsx` | Export up to 5000 selected planning-SKU rows | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/planning-skus/erp-retry` | Queue retry for failed planning-SKU ERP projections | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/planning-skus/erp-resync` | Explicitly queue ERP overwrite after a completed-SKU correction | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/trace-events` | Record frontend business trace event | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/cost-management/dashboard` | Get current SKU cost issue dashboard | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/cost-management/recalculation-runs` | List SKU cost recalculation runs；Create a SKU cost recalculation run | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/cost-management/recalculation-runs/{run_id}` | Get a SKU cost recalculation run with preview items | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/cost-management/recalculation-runs/{run_id}/apply` | Apply a previewed SKU cost recalculation run | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/cost-management/recalculation-runs/{run_id}/sync-erp` | Queue applied recalculation items for ERP base-data sync | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/cost-management/recalculation-runs/{run_id}/cancel` | Cancel an open SKU cost recalculation run | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/cost-rule-bindings` | List cost rule i_id bindings；Create cost rule i_id binding | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/cost-rule-bindings/unbound-candidates` | List unbound i_id candidates from legacy pricing fallback | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/cost-rule-bindings/{id}` | Patch cost rule i_id binding | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload-sessions` | Create task reference upload session | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/reference-upload-sessions/{session_id}` | Get task reference upload session | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload-sessions/{session_id}/complete` | Complete task reference upload session | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload-sessions/{session_id}/abort` | Abort task reference upload session | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-assets/{task_asset_id}/download` | Get controlled download metadata for one immutable task-asset version | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-assets/{task_asset_id}/preview` | Get controlled preview metadata for one immutable task-asset version | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/prepare-product-codes` | Prepare task product codes | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/tasks` | List tasks within explicit capability and data scope；Create task | GET:已登录 / 主流程读全量可见; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/filter-options` | Get task center filter options | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}` | Get task read model | GET:已登录 / 主流程读全量可见 | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/tasks/{id}/product-info` | Get per-task product information；Patch per-task product information | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/tasks/{id}/cost-info` | Get per-task cost information；Patch per-task cost information | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/sku-items/{sku_item_id}` | Patch one batch SKU item | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/sku-items/{sku_item_id}/cost-info` | Patch per-SKU cost information for a batch task item | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-quote/preview` | Preview cost quote for one task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/business-info` | Update task business-info and generic cost fields | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/filing-status` | Get task filing status view | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/filing/retry` | Retry task filing | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/detail` | Get task aggregate detail (V1.1-A1 fast-path) | GET:已登录 / 主流程读全量可见 | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/cost-overrides` | Get task cost-override governance audit timeline | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assign` | Assign task to designer | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/batch/assign` | Batch assign tasks to designer | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/batch/remind` | Batch remind task handlers | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets` | List task-linked design assets | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/reference-assets/batch-download` | Batch download task reference direct URL manifest | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-board/overview` | Get the main operations dashboard overview | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/external-assets/events` | Ingest NAS filesystem events | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/events` | List task events | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/code-rules` | List code rules | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/code-rules/{id}/preview` | Preview generated code | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/claim` | Claim a task module | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/actions/{action}` | Trigger a task module action | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cancel` | Cancel a task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/excel-assist/template.xlsx` | Download single-task Excel assist template | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/excel-assist/parse-excel` | Parse a single-task Excel assist file | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/ai/chat/config` | Get the current data-assistant capability contract | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/ai/chat/conversations` | List the caller's active conversations；Create an owner-scoped conversation retained for 90 days | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, DELETE | `/v1/ai/chat/conversations/{conversation_id}` | Read one owner-scoped conversation and its evidence citations；Hide a conversation immediately and hard-delete its body within 24 hours | GET:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/ai/chat/conversations/{conversation_id}/messages:stream` | Stream a read-only evidence-backed answer | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/ai/chat/admin/conversations` | List conversation metadata across users | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/ai/chat/admin/conversations/{conversation_id}` | Review one cross-user conversation and write a metadata-only audit event | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/events` | List asset-workbench operation records | GET:AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/notifications` | List my asset-workbench notifications | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/notifications/{id}/read` | Mark one asset-workbench notification as read | POST:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/notifications/read-all` | Mark all my asset-workbench notifications as read | POST:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/notifications/unread-count` | Get my asset-workbench unread notification count | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/notifications/broadcast` | Broadcast a notification to one, many, or all users | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/entry` | Resolve asset workbench entry state | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/batch-jobs` | List asset workbench batch jobs | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/batch-jobs/{job_id}` | Get one asset workbench batch job | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/register` | Register an asset workbench account | POST:公开 | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/access/request` | Request asset workbench access | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/access/open` | Open or restore asset workbench access | POST:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/access/disable` | Disable asset workbench access | POST:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/profile` | Update my asset workbench profile | PATCH:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/profiles` | List asset workbench profiles | GET:HRAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/asset-workbench/profiles/{user_id}` | Get one complete asset workbench profile；Update one asset workbench profile | GET:HRAdmin, AssetSettlement, SuperAdmin; PATCH:HRAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/members` | List asset workbench members | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/members/{user_id}/identity` | Deprecated asset workbench binary identity endpoint | PATCH:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/members/{user_id}/roles` | Update asset workbench member roles | PATCH:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/accounts/merge/preview` | Preview asset workbench account merge | POST:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/accounts/merge` | Confirm asset workbench account merge | POST:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/people-lookup` | Search people for workbench access | GET:AssetManager, AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/groups/{group_id}/members` | List asset workbench group members | GET:AssetManager, AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/difficulty-classes` | List enabled asset workbench difficulty classes；Create asset workbench difficulty class | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin; POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/difficulty-classes/admin` | List all asset workbench difficulty classes for administration | GET:AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/difficulty-classes/{difficulty_code}` | Update asset workbench difficulty class | PATCH:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/price-matrix` | List asset workbench price matrix rules；Create asset workbench price matrix rule | GET:AssetTemplateAdmin, AssetSettlement, SuperAdmin; POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/price-matrix/{rule_id}` | Enable or disable a price matrix rule | PATCH:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/price-matrix/{rule_id}/supersede` | Supersede a price matrix rule with a new revision | POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/deduction-rules` | List asset workbench deduction rules；Create asset workbench deduction rule | GET:AssetTemplateAdmin, AssetSettlement, SuperAdmin; POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/deduction-rules/{rule_id}` | Enable or disable a deduction rule | PATCH:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/deduction-rules/{rule_id}/supersede` | Supersede a deduction rule with a new revision | POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/welfare-rules` | List asset workbench welfare rules；Create asset workbench welfare rule | GET:AssetTemplateAdmin, AssetSettlement, SuperAdmin; POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/welfare-rules/{rule_id}` | Enable or disable a welfare rule | PATCH:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/welfare-rules/{rule_id}/supersede` | Supersede a welfare rule with a new row | POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/promo-coupons` | List asset workbench promo coupons；Create asset workbench promo coupon | GET:AssetTemplateAdmin, AssetSettlement, SuperAdmin; POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/promo-coupons/{rule_id}` | Enable or disable a promo coupon | PATCH:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/promo-coupons/{rule_id}/supersede` | Supersede a promo coupon with a new row | POST:AssetTemplateAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/overview-search` | Search asset workbench overview | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/drive/directories` | List asset workbench drive directories | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/drive/orders` | List asset workbench drive orders | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/drive/files` | List asset workbench drive files | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/drive/folder` | Browse one virtual drive folder | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/drive/search` | Search asset workbench drive files | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/drive/locate` | Locate one asset workbench drive file | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/submissions` | List asset workbench submissions；Create asset workbench submission | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin; POST:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/submissions/{submission_id}/void` | Void an asset workbench submission | POST:AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/items/{item_id}` | Update editable fields of a submission item | PATCH:AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/items/qc/excel` | Import submission item QC statuses from Excel | POST:AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/error-imports` | Import asset workbench quality error deductions | POST:AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/error-imports/excel` | Import asset workbench quality error deductions from Excel | POST:AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/files/{file_id}` | Update uploaded work file metadata | PATCH:AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/files/{file_id}/preview` | Get uploaded work file preview metadata | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/files/{file_id}/download` | Get uploaded work file download info | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/files/{file_id}/archive` | Browse uploaded archive as a virtual folder | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/files/{file_id}/archive/entry` | Stream one virtual file from an uploaded archive | GET:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/files/batch-move` | Batch move submission files to an upload directory | POST:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/files/batch-delete` | Batch delete submission files | POST:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/system-search` | Search publishable material assets from asset workbench | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/materials/groups` | Group operational and external materials | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/materials/group-files` | List files inside one material group | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/materials/browse` | Browse material assets as a virtual directory tree | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PUT | `/v1/asset-workbench/settlement/supplement-permissions` | List asset workbench supplement permissions；Open or close supplement permission for one person | GET:AssetSettlement, SuperAdmin; PUT:AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/settlement/supplement-eligible-months` | Get the natural month currently eligible for supplement entry | GET:AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/settlement/report` | Get asset workbench settlement report | GET:AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/settlement/supplements` | List asset workbench settlement supplements；Create asset workbench settlement supplement | GET:AssetSettlement, SuperAdmin; POST:AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/settlement/my` | Get my asset-workbench settlement and supplement upload state | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/settlement/supplements/excel` | Import monthly settlement supplements from Excel | POST:AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| DELETE | `/v1/asset-workbench/settlement/supplements/{supplement_id}` | Delete a draft or approved settlement supplement | DELETE:AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/upload-sessions` | Create asset workbench upload session | POST:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/upload-sessions/{session_id}/complete` | Complete asset workbench upload session | POST:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/upload-directories` | List enabled asset workbench upload directories；Create asset workbench upload directory | GET:AssetSubmitter, AssetManager, SuperAdmin; POST:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/upload-directories/admin` | List all asset workbench upload directories for administration | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/asset-workbench/upload-directories/{directory_id}` | Update asset workbench upload directory | PATCH:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/asset-workbench/client-materials` | List client-downloadable materials；Publish an asset to client materials | GET:AssetSubmitter, AssetManager, SuperAdmin; POST:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/client-materials/batch-update` | Batch publish or update client materials | POST:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/client-materials/search` | Search client-downloadable materials | GET:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH, DELETE | `/v1/asset-workbench/client-materials/{material_id}` | Update client material publication；Delete client material publication | PATCH:AssetManager, SuperAdmin; DELETE:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/client-materials/{material_id}/download` | Get client material download info | GET:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/client-materials/{material_id}/preview` | Get client material preview metadata | GET:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/client-materials/batch-download` | Batch download client materials | POST:AssetSubmitter, AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/system-assets/{asset_id}/preview` | Get asset workbench system asset preview | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload` | Upload task-create reference file through backend compatibility proxy | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/assets/batch-download` | Batch download asset direct URL manifest | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET, DELETE | `/v1/assets/{asset_id}` | Get asset；Delete asset | GET:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/download` | Get asset download info | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/content` | Stream external netdisk asset content | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/preview` | Get asset preview info | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions` | Create asset upload session | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/upload-sessions/{session_id}` | Get asset upload session | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions/{session_id}/complete` | Complete asset upload session | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions/{session_id}/cancel` | Cancel asset upload session | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/files/{path}` | Authorize and redirect OSS-backed business file | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/task-drafts` | Create or update a task draft | POST:已登录 / scope-aware | [V1_API_DRAFTS.md](V1_API_DRAFTS.md) |
| GET, DELETE | `/v1/task-drafts/{draft_id}` | Get a task draft；Delete a task draft | GET:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_DRAFTS.md](V1_API_DRAFTS.md) |
| GET | `/v1/me/notifications` | List my notifications | GET:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| POST | `/v1/me/notifications/{id}/read` | Mark one notification as read | POST:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| POST | `/v1/me/notifications/read-all` | Mark all notifications as read | POST:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| GET | `/v1/me/notifications/unread-count` | Get unread notification count | GET:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| GET | `/v1/me/notifications/web-push/config` | Get Web Push runtime config | GET:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| POST | `/v1/me/notifications/web-push/subscriptions` | Register current browser Web Push subscription | POST:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| DELETE | `/v1/me/notifications/web-push/subscriptions/current` | Disable the current browser Web Push subscription | DELETE:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| POST | `/v1/me/notifications/web-push/test` | Send a Web Push test notification | POST:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| GET, PATCH | `/v1/me/notifications/preferences` | Get my notification preferences；Update my notification preferences | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| GET | `/v1/tasks/batch-create/template.xlsx` | Download batch create Excel template | GET:已登录 / scope-aware | [V1_API_BATCH.md](V1_API_BATCH.md) |
| POST | `/v1/tasks/batch-create/parse-excel` | Parse a batch create Excel file | POST:已登录 / scope-aware | [V1_API_BATCH.md](V1_API_BATCH.md) |
| GET | `/v1/erp/products` | Search ERP Bridge products | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/iids` | List ERP product i_id options | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/products/{id}` | Get ERP Bridge product detail | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/categories` | List ERP Bridge categories | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/warehouses` | List ERP warehouses (wms_co_id) | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/upsert` | Upsert ERP Bridge product (Bridge-side write endpoint) | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/style/update` | Update ERP item style (Bridge-side write endpoint) | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/sync-logs` | List ERP Bridge sync logs | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/sync-logs/{id}` | Get ERP Bridge sync log detail | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/shelve/batch` | Shelve products in batch through Bridge | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/unshelve/batch` | Unshelve products in batch through Bridge | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/inventory/virtual-qty` | Update virtual inventory qty through Bridge | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, POST | `/v1/categories` | List categories；Create category | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/categories/search` | Search categories | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, PATCH | `/v1/categories/{id}` | Get category by ID；Patch category | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, POST | `/v1/category-mappings` | List category-to-ERP mappings；Create category-to-ERP mapping | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/category-mappings/search` | Search category-to-ERP mappings | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, PATCH | `/v1/category-mappings/{id}` | Get category-to-ERP mapping by ID；Patch category-to-ERP mapping | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, POST | `/v1/cost-rules` | List cost rules；Create cost rule | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, PATCH | `/v1/cost-rules/{id}` | Get cost rule by ID；Patch cost rule | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/cost-rules/{id}/history` | Get cost rule lineage history | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/cost-rules/preview` | Preview cost rule estimate | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/products/by-code` | Lookup ERP product by code | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/design-sources/search` | Search design source entries | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/search` | Perform a global search | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
