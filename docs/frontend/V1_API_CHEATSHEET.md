# V1 API 速查表(353 path · 一行一条)

> Revision: V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.3-A2)

> 本表一行对应一个 `/v1` path；同一路径多 method 合并到 `Methods` 列。
> WebSocket 当前 OpenAPI 真实 path 为 `/ws/v1`，详见 `V1_API_WS.md`，不计入 353 个 `/v1` path。
> 新前端只接 canonical 路径；compatibility/deprecated 路径仅作迁移兜底。

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
| GET | `/v1/roles` | List role catalog | GET:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, OrgAdmin, RoleAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/access-rules` | List protected route access rules | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET, POST | `/v1/users` | List workflow users；Create workflow user | GET:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, TeamLead, OrgAdmin, RoleAdmin; POST:HRAdmin, SuperAdmin, DepartmentAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/users/designers` | List designers | GET:Ops, Designer, CustomizationOperator, Audit_A, Audit_B, Admin, HRAdmin, SuperAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_USERS.md](V1_API_USERS.md) |
| GET, PATCH, DELETE | `/v1/users/{id}` | Get workflow user；Update workflow user；Deprecated compatibility delete workflow user | GET:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, TeamLead, OrgAdmin, RoleAdmin; PATCH:HRAdmin, SuperAdmin, DepartmentAdmin; DELETE:SuperAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| PUT | `/v1/users/{id}/password` | Reset workflow user password | PUT:HRAdmin, SuperAdmin, DepartmentAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| POST, PUT | `/v1/users/{id}/roles` | Add workflow user roles；Replace workflow user roles | POST:HRAdmin, SuperAdmin; PUT:HRAdmin, SuperAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| DELETE | `/v1/users/{id}/roles/{role}` | Remove one workflow user role | DELETE:HRAdmin, SuperAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/permission-logs` | List permission access logs | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/operation-logs` | List aggregated operation logs | GET:Admin, SuperAdmin, HRAdmin | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/audit-logs` | List audit records (cross-task) | GET:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, TeamLead, OrgAdmin, RoleAdmin, Audit_A, Audit_B, CustomizationReviewer | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/server-logs` | List server logs | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/server-logs/clean` | Clean old server logs | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/admin/jst-users` | List JST users (Admin, via Bridge) | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/admin/jst-users/import-preview` | Preview JST user import (Admin) | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/admin/jst-users/import` | Import JST users (Admin) | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/users/{id}/activate` | Activate a workflow user | POST:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, TeamLead | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/users/{id}/deactivate` | Deactivate a workflow user | POST:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, TeamLead | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/org/options` | Get organization options | GET:Admin, SuperAdmin, HRAdmin, DepartmentAdmin, OrgAdmin, RoleAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/departments` | Create organization department | POST:HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| PUT, DELETE | `/v1/org/departments/{id}` | Update organization department；Hard-delete organization department | PUT:HRAdmin, SuperAdmin; DELETE:HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/departments/{id}/merge` | Merge organization department into another department | POST:HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/teams` | Create organization team | POST:HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| PUT, DELETE | `/v1/org/teams/{id}` | Update organization team；Hard-delete organization team | PUT:HRAdmin, SuperAdmin; DELETE:HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/teams/{id}/merge` | Merge organization team into another team | POST:HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/departments/{id}/org-move-requests` | Create an org move request | POST:DepartmentAdmin, HRAdmin, SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| GET | `/v1/org-move-requests` | List org move requests | GET:SuperAdmin, HRAdmin, DepartmentAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org-move-requests/{id}/approve` | Approve an org move request | POST:SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org-move-requests/{id}/reject` | Reject an org move request | POST:SuperAdmin | [V1_API_ORG.md](V1_API_ORG.md) |
| GET, POST | `/v1/trace-events` | List business trace events；Record frontend business trace event | GET:Admin, SuperAdmin, HRAdmin; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/product-management` | List product management records | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/product-management/combo-tree` | List product management records grouped by ERP combo SKU | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/product-management/cost-dashboard` | Get product cost issue dashboard | GET:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/product-management/cost-recalculation-runs` | List product cost recalculation runs；Create a product cost recalculation run | GET:Ops, ERP, Admin, SuperAdmin; POST:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/product-management/cost-recalculation-runs/{run_id}` | Get a product cost recalculation run with preview items | GET:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/cost-recalculation-runs/{run_id}/apply` | Apply a previewed product cost recalculation run | POST:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/cost-recalculation-runs/{run_id}/sync-erp` | Queue applied recalculation items for ERP base-data sync | POST:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/cost-recalculation-runs/{run_id}/cancel` | Cancel an open product cost recalculation run | POST:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/{id}/reparse-image` | Reparse the managed image for one product-center record | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/{id}/image` | Set a manual image for one product-center record | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/{id}/sync-request` | Queue full ERP sync for one product-center record | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/{id}/base-sync-request` | Queue ERP base-data sync for one product-center record | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/product-management/{id}/image-sync-request` | Queue ERP image sync for one product-center record | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/cost-rule-bindings` | List cost rule i_id bindings；Create cost rule i_id binding | GET:Ops, ERP, Admin, SuperAdmin; POST:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/cost-rule-bindings/unbound-candidates` | List unbound i_id candidates from legacy pricing fallback | GET:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/cost-rule-bindings/{id}` | Patch cost rule i_id binding | PATCH:Ops, ERP, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload-sessions` | Create task reference upload session | POST:Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/reference-upload-sessions/{session_id}` | Get task reference upload session | GET:Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload-sessions/{session_id}/complete` | Complete task reference upload session | POST:Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/reference-upload-sessions/{session_id}/abort` | Abort task reference upload session | POST:Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/prepare-product-codes` | Prepare task product codes | POST:Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/tasks` | List tasks；Create task | GET:已登录 / 主流程读全量可见; POST:Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/filter-options` | Get task center filter options | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}` | Get task read model | GET:已登录 / 主流程读全量可见 | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/predictions` | Get task next-action prediction suggestions | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/tasks/{id}/product-info` | Get per-task product information；Patch per-task product information | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin; PATCH:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/tasks/{id}/cost-info` | Get per-task cost information；Patch per-task cost information | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin; PATCH:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/sku-items/{sku_item_id}` | Patch one batch SKU item | PATCH:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/sku-items/{sku_item_id}/cost-info` | Patch per-SKU cost information for a batch task item | PATCH:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-quote/preview` | Preview cost quote for one task | POST:Ops, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/business-info` | Update task business-info and generic cost fields | PATCH:Ops, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/filing-status` | Get task filing status view | GET:Ops, Warehouse, Admin, Designer, Audit_A, Audit_B | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/filing/retry` | Retry task filing | POST:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/procurement` | Update purchase-task procurement draft data | PATCH:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/procurement/advance` | Advance purchase-task procurement lifecycle | POST:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/detail` | Get task aggregate detail (V1.1-A1 fast-path) | GET:已登录 / 主流程读全量可见 | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/cost-overrides` | Get task cost-override governance audit timeline | GET:Ops, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-overrides/{event_id}/review` | Upsert cost-override review placeholder boundary | POST:Ops, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-overrides/{event_id}/finance-mark` | Upsert cost-override finance placeholder boundary | POST:ERP, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assign` | Assign task to designer | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/batch/assign` | Batch assign tasks to designer | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/batch/remind` | Batch remind task handlers | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/submit-design` | Submit task design asset | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/audit-supplements` | List audit post-close supplement uploads | GET:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit-supplements/upload-sessions` | Create audit supplement upload session | POST:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit-supplements/upload-sessions/{session_id}/complete` | Complete audit supplement upload session | POST:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets` | List task-linked design assets | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/reference-assets/batch-download` | Batch download task reference direct URL manifest | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/timeline` | List legacy task asset timeline | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/{asset_id}/versions` | List versions under one design asset | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/{asset_id}/download` | Get latest version download info for one design asset | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download` | Get specific version download info for one design asset | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload-sessions` | Create upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/upload-sessions/{session_id}` | Get upload session status | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload-sessions/{session_id}/complete` | Complete upload session and record asset version | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload-sessions/{session_id}/abort` | Abort upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload` | Create small-file direct upload handoff (legacy path) | POST:Designer, Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/prepare` | Prepare task for warehouse handoff | POST:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/mock-upload` | Mock upload task asset | POST:Designer, Ops | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/close` | Close task explicitly | POST:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/claim` | Claim task for audit | POST:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/approve` | Approve audit and move task to next status | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/reject` | Reject audit | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/transfer` | Transfer audit responsibility | POST:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/audit/handover-candidates` | List audit handover candidates for current actor | GET:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/audit/handover-batch` | Create batch audit handovers | POST:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/handover` | Create audit handover | POST:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/audit/handovers` | List audit handovers for task | GET:Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/takeover` | Take over pending handover | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/outsource` | Create outsource order for task | POST:Outsource, Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/outsource-orders` | List outsource orders | GET:Outsource, Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/warehouse/receipts` | List warehouse receipts | GET:Warehouse, Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-board/overview` | Get the main operations dashboard overview | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-board/summary` | Get task-board queue summary | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-board/queues` | Get task-board queue tasks | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/workbench/preferences` | Get saved workbench preferences；Save workbench preferences | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin; PATCH:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-templates` | List export templates | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/external-assets/events` | Ingest NAS filesystem events | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/integration/connectors` | List integration connectors | GET:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/integration/call-logs` | List integration call logs；Create integration call log | GET:Admin, ERP; POST:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/integration/call-logs/{id}` | Get integration call log | GET:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/integration/call-logs/{id}/executions` | List integration executions；Create integration execution | GET:Admin, ERP; POST:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/retry` | Retry integration call log | POST:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/replay` | Replay integration call log | POST:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/executions/{execution_id}/advance` | Advance integration execution | POST:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/advance` | Advance integration call log | POST:Admin, ERP | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/export-jobs` | List export jobs；Create export job | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin; POST:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}` | Get export job | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/export-jobs/{id}/dispatches` | List export job dispatches；Submit export job dispatch | GET:Admin; POST:Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/dispatches/{dispatch_id}/advance` | Advance export job dispatch | POST:Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}/attempts` | List export job attempts | GET:Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}/events` | List export job events | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/claim-download` | Claim export job download handoff | POST:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}/download` | Read export job download handoff | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/refresh-download` | Refresh export job download handoff | POST:Ops, Designer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/start` | Start export job placeholder runner | POST:Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/advance` | Advance export job lifecycle | POST:Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/receive` | Mark warehouse receipt as received | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/reject` | Reject warehouse receipt | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/complete` | Complete warehouse flow and move task to pending close | POST:Warehouse, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/customization/review` | Submit customization review for task | POST:CustomizationReviewer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/customization-jobs` | List customization jobs | GET:CustomizationReviewer, CustomizationOperator, Ops, Designer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/customization-jobs/{id}` | Get customization job detail | GET:CustomizationReviewer, CustomizationOperator, Ops, Designer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/customization-jobs/{id}/effect-preview` | Submit customization effect preview | POST:CustomizationOperator, Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/customization-jobs/{id}/effect-review` | Review customization effect | POST:CustomizationReviewer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/customization-jobs/{id}/production-transfer` | Transfer customization production to warehouse QC | POST:CustomizationOperator, Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/events` | List task events | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/code-rules` | List code rules | GET:Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/code-rules/{id}/preview` | Preview generated code | GET:Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/code-rules/generate-sku` | [ARCHIVED] Legacy CodeRule SKU generation | POST:Ops, Admin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/sku/preview_code` | [V6] Preview SKU code | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/sku/list` | [V6] List SKUs | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/sku` | [V6] Create SKU | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/sku/{id}` | [V6] Get SKU by ID | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/sku/{id}/sync_status` | [V6] Frontend sequence-gap recovery | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/audit` | [V6] Submit audit decision | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/agent/sync` | [V6] NAS agent sync | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/agent/pull_job` | [V6] Agent pull job | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/agent/heartbeat` | [V6] Agent heartbeat | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/agent/ack_job` | [V6] Agent ack job | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/incidents` | [V6] List incidents | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/incidents/{id}/assign` | [V6] Assign incident | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/incidents/{id}/resolve` | [V6] Resolve incident | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/policies` | [V6] List policies | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PUT | `/v1/policies/{id}` | [V6] Update policy | PUT:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/rule-templates` | [V6] List rule templates | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PUT | `/v1/rule-templates/{type}` | [V6] Get rule template by type；[V6] Upsert rule template by type | GET:已登录 / scope-aware; PUT:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/pool` | List task pool entries | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/claim` | Claim a task module | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/actions/{action}` | Trigger a task module action | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/reassign` | Reassign a task module within team scope | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/pool-reassign` | Reassign a task module between pools | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cancel` | Cancel or close a task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/excel-assist/template.xlsx` | Download single-task Excel assist template | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/excel-assist/parse-excel` | Parse a single-task Excel assist file | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/predictions/search` | Get global-search prediction suggestions | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/predictions/task-create` | Get task-create form prediction suggestions | GET:Ops, Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/predictions/assets` | Get asset-center prediction suggestions | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/predictions/management` | Get management prediction suggestions | GET:Admin, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/experience/config` | Get experience learning runtime flags | GET:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/experience/client-config` | Get client-safe experience learning flags | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/experience/reason-tags` | List client-safe experience reason tags | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/experience/behavior-events:batch` | Record client experience behavior events | POST:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/experience/micro-question-eligibility` | Check whether a micro-question can be shown | GET:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/experience/micro-question-answers` | Record a micro-question answer | POST:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/ai-suggestions/{suggestion_event_id}/feedback` | Record AI suggestion feedback | POST:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/events` | List asset-workbench operation records | GET:AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/notifications` | List my asset-workbench notifications | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/notifications/{id}/read` | Mark one asset-workbench notification as read | POST:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/notifications/read-all` | Mark all my asset-workbench notifications as read | POST:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/notifications/unread-count` | Get my asset-workbench unread notification count | GET:AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/notifications/broadcast` | Broadcast a notification to one, many, or all users | POST:Admin, SuperAdmin, HRAdmin, DepartmentAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/entry` | Resolve asset workbench entry state | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/batch-jobs` | List asset workbench batch jobs | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/asset-workbench/batch-jobs/{job_id}` | Get one asset workbench batch job | GET:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/access/request` | Request asset workbench access | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/access/open` | Open or restore asset workbench access | POST:AssetManager, SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/asset-workbench/access/disable` | Disable asset workbench access | POST:SuperAdmin | [V1_API_TASKS.md](V1_API_TASKS.md) |
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
| POST | `/v1/task-create/asset-center/upload-sessions` | Create task-create reference upload session | POST:Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/task-create/asset-center/upload-sessions/{session_id}` | Get task-create reference upload session | GET:Ops | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/task-create/asset-center/upload-sessions/{session_id}/complete` | Complete task-create reference upload session | POST:Ops | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/task-create/asset-center/upload-sessions/{session_id}/abort` | Abort task-create reference upload session | POST:Ops | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/reference-upload` | Upload task-create reference file through backend compatibility proxy | POST:Ops | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets` | List design assets in task asset center | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions` | List versions under one design asset | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/download` | Get latest version download info for one design asset | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download` | Get specific version download info for one design asset | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions` | Create upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/small` | Create small-file upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/multipart` | Create multipart upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}` | Get upload session status | GET:Designer, Ops, Audit_A, Audit_B | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete` | Complete upload session and record asset version | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel` | Cancel upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort` | Abort upload session | POST:Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/assets` | List assets | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/batch-download` | Batch download asset direct URL manifest | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/excel-package/preview` | Preview Excel image package manifest | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/excel-package/preview-file` | Preview Excel image package manifest from uploaded file | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET, DELETE | `/v1/assets/{asset_id}` | Get asset；Delete asset | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin; DELETE:SuperAdmin, CustomizationReviewer, Audit_A, Audit_B, AssetManager | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/download` | Get asset download info | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/content` | Stream external netdisk asset content | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/preview` | Get asset preview info | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions` | Create asset upload session | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, AssetManager, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/upload-sessions/{session_id}` | Get asset upload session | GET:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, AssetManager, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions/{session_id}/complete` | Complete asset upload session | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, AssetManager, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions/{session_id}/cancel` | Cancel asset upload session | POST:Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, AssetManager, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/files/{path}` | Authorize and redirect OSS-backed business file | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET, POST | `/v1/assets/upload-requests` | List asset upload requests；Create asset upload request | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin; POST:Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/upload-requests/{id}` | Get asset upload request | GET:Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-requests/{id}/advance` | Advance asset upload request | POST:Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/search/batch` | Batch search assets by SKU or task number | POST:Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/versions/{version_id}/download` | Download a specific asset version | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/{asset_id}/archive` | Archive an asset | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/{asset_id}/restore` | Restore an archived asset | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
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
| GET | `/v1/erp/users` | List JST company users (Bridge-side, pre-wiring) | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/upsert` | Upsert ERP Bridge product (Bridge-side write endpoint) | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/style/update` | Update ERP item style (Bridge-side write endpoint) | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/sync-logs` | List ERP Bridge sync logs | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/sync-logs/{id}` | Get ERP Bridge sync log detail | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/shelve/batch` | Shelve products in batch through Bridge | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/products/unshelve/batch` | Unshelve products in batch through Bridge | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/erp/inventory/virtual-qty` | Update virtual inventory qty through Bridge | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/products/search` | Search local cached ERP products | GET:Ops, Designer, Audit_A, Audit_B, Warehouse | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/products/sync/status` | Get ERP sync placeholder status | GET:ERP, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/products/sync/run` | Run ERP sync placeholder manually | POST:ERP, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/products/{id}` | Get product by ID | GET:Ops, Designer, Audit_A, Audit_B, Warehouse | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, POST | `/v1/categories` | List categories；Create category | GET:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector; POST:Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/categories/search` | Search categories | GET:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, PATCH | `/v1/categories/{id}` | Get category by ID；Patch category | GET:Ops, Warehouse, Admin; PATCH:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, POST | `/v1/category-mappings` | List category-to-ERP mappings；Create category-to-ERP mapping | GET:Ops, Warehouse, Admin; POST:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/category-mappings/search` | Search category-to-ERP mappings | GET:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, PATCH | `/v1/category-mappings/{id}` | Get category-to-ERP mapping by ID；Patch category-to-ERP mapping | GET:Ops, Warehouse, Admin; PATCH:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, POST | `/v1/cost-rules` | List cost rules；Create cost rule | GET:Ops, Warehouse, Admin; POST:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET, PATCH | `/v1/cost-rules/{id}` | Get cost rule by ID；Patch cost rule | GET:Ops, Warehouse, Admin; PATCH:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/cost-rules/{id}/history` | Get cost rule lineage history | GET:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/cost-rules/preview` | Preview cost rule estimate | POST:Ops, Warehouse, Admin | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/erp/products/by-code` | Lookup ERP product by code | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/assets/search` | Search assets across tasks | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/design-sources/search` | Search design source entries | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/search` | Perform a global search | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/reports/experience/stats` | Get experience learning observation metrics | GET:SuperAdmin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/experience/samples` | List experience event samples | GET:SuperAdmin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/experience/review-items` | List experience attribution review items | GET:SuperAdmin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| POST | `/v1/reports/experience/review-items/{item_key}/decision` | Record an experience attribution review decision | POST:SuperAdmin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/l1/cards` | Get L1 report cards | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/l1/throughput` | Get L1 throughput report | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/l1/module-dwell` | Get L1 module dwell report | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/l1/kpi-events` | Get enriched KPI task events | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
