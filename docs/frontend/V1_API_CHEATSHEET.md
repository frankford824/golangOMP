> Revision: V1.2 authority purge + OpenAPI GC (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2 path-closure GC)
> V1 SoT: docs/V1_BACKEND_SOURCE_OF_TRUTH.md

# V1 API 速查表(203 path · 一行一条)

> Revision: V1.1-A2 contract drift purge (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.1-A2)
> 与 v1.21 生产实际响应对齐


> 本表一行对应一个 `/v1` path；同一路径多 method 合并到 `Methods` 列。
> 重要: V1.1-A2 已校准,如有第三方文档与此处不一致,以本表为准。
> WebSocket 当前 OpenAPI 真实 path 为 `/ws/v1`，详见 `V1_API_WS.md`，不计入 203 个 `/v1` path。
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
| POST | `/v1/me/change-password` | Change my password | POST:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| GET | `/v1/me/org` | Get my org profile | GET:已登录 / scope-aware | [V1_API_ME.md](V1_API_ME.md) |
| GET | `/v1/roles` | List role catalog | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/access-rules` | List protected route access rules | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET, POST | `/v1/users` | List workflow users；Create workflow user | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/users/designers` | List designers | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET, PATCH, DELETE | `/v1/users/{id}` | Get workflow user；Update workflow user；Delete workflow user | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| PUT | `/v1/users/{id}/password` | Reset workflow user password | PUT:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST, PUT | `/v1/users/{id}/roles` | Add workflow user roles；Replace workflow user roles | POST:已登录 / scope-aware; PUT:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| DELETE | `/v1/users/{id}/roles/{role}` | Remove one workflow user role | DELETE:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/permission-logs` | List permission access logs | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/operation-logs` | List aggregated operation logs | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/audit-logs` | List audit records (cross-task) | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/server-logs` | List server logs | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/server-logs/clean` | Clean old server logs | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/admin/jst-users` | List JST users (Admin, via Bridge) | GET:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/admin/jst-users/import-preview` | Preview JST user import (Admin) | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/admin/jst-users/import` | Execute JST user import (Admin) | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/users/{id}/activate` | Activate a workflow user | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| POST | `/v1/users/{id}/deactivate` | Deactivate a workflow user | POST:已登录 / scope-aware | [V1_API_USERS.md](V1_API_USERS.md) |
| GET | `/v1/org/options` | Get organization options | GET:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/departments` | Create organization department | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| PUT | `/v1/org/departments/{id}` | Update organization department | PUT:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org/teams` | Create organization team | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| PUT | `/v1/org/teams/{id}` | Update organization team | PUT:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/departments/{id}/org-move-requests` | Create an org move request | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| GET | `/v1/org-move-requests` | List org move requests | GET:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org-move-requests/{id}/approve` | Approve an org move request | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/org-move-requests/{id}/reject` | Reject an org move request | POST:已登录 / scope-aware | [V1_API_ORG.md](V1_API_ORG.md) |
| POST | `/v1/tasks/prepare-product-codes` | Prepare task product codes | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/tasks` | List tasks；Create task | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}` | Get task read model | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/tasks/{id}/product-info` | Get per-task product information；Patch per-task product information | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/tasks/{id}/cost-info` | Get per-task cost information；Patch per-task cost information | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-quote/preview` | Preview cost quote for one task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/business-info` | Update task business-info and generic cost fields | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/filing-status` | Get task filing status view | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/filing/retry` | Retry task filing | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| PATCH | `/v1/tasks/{id}/procurement` | Update purchase-task procurement draft data | PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/procurement/advance` | Advance purchase-task procurement lifecycle | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/detail` | Get task aggregate detail (V1.1-A1 fast-path 5-section) | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/cost-overrides` | Get task cost-override governance audit timeline | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-overrides/{event_id}/review` | Upsert cost-override review placeholder boundary | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cost-overrides/{event_id}/finance-mark` | Upsert cost-override finance placeholder boundary | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assign` | Assign task to designer | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/batch/assign` | Batch assign tasks to designer | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/batch/remind` | Batch remind task handlers | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/submit-design` | Submit task design asset | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets` | List task-linked design assets | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/timeline` | List legacy task asset timeline | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/{asset_id}/versions` | List versions under one design asset | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/{asset_id}/download` | Get latest version download info for one design asset | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download` | Get specific version download info for one design asset | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload-sessions` | Create upload session | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/assets/upload-sessions/{session_id}` | Get upload session status | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload-sessions/{session_id}/complete` | Complete upload session and record asset version | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload-sessions/{session_id}/abort` | Abort upload session | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/upload` | Create small-file direct upload handoff (legacy path) | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/prepare` | Prepare task for warehouse handoff | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/assets/mock-upload` | Mock upload task asset | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/close` | Close task explicitly | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/claim` | Claim task for audit | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/approve` | Approve audit and move task to next status | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/reject` | Reject audit | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/transfer` | Transfer audit responsibility | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/handover` | Create audit handover | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/audit/handovers` | List audit handovers for task | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/audit/takeover` | Take over pending handover | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/outsource` | Create outsource order for task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/outsource-orders` | List outsource orders | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/warehouse/receipts` | List warehouse receipts | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-board/summary` | Get task-board queue summary | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/task-board/queues` | Get task-board queue tasks | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, PATCH | `/v1/workbench/preferences` | Get saved workbench preferences；Save workbench preferences | GET:已登录 / scope-aware; PATCH:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-templates` | List export templates | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/integration/connectors` | List integration connectors | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/integration/call-logs` | List integration call logs；Create integration call log | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/integration/call-logs/{id}` | Get integration call log | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/integration/call-logs/{id}/executions` | List integration executions；Create integration execution | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/retry` | Retry integration call log | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/replay` | Replay integration call log | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/executions/{execution_id}/advance` | Advance integration execution | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/integration/call-logs/{id}/advance` | Advance integration call log | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/export-jobs` | List export jobs；Create export job | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}` | Get export job | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET, POST | `/v1/export-jobs/{id}/dispatches` | List export job dispatches；Submit export job dispatch | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/dispatches/{dispatch_id}/advance` | Advance export job dispatch | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}/attempts` | List export job attempts | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}/events` | List export job events | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/claim-download` | Claim export job download handoff | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/export-jobs/{id}/download` | Read export job download handoff | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/refresh-download` | Refresh export job download handoff | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/start` | Start export job placeholder runner | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/export-jobs/{id}/advance` | Advance export job lifecycle | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/receive` | Mark warehouse receipt as received | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/reject` | Reject warehouse receipt and return task to a truthful rework state | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/warehouse/complete` | Complete warehouse flow and move task to pending close | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/customization/review` | Submit customization review for task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/customization-jobs` | List customization jobs | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/customization-jobs/{id}` | Get customization job detail | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/customization-jobs/{id}/effect-preview` | Submit customization effect preview | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/customization-jobs/{id}/effect-review` | Review customization effect | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/customization-jobs/{id}/production-transfer` | Transfer customization production to warehouse QC | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/{id}/events` | List task events | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/code-rules` | List code rules | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/code-rules/{id}/preview` | Preview generated code | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/code-rules/generate-sku` | Generate SKU code | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
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
| GET | `/v1/policies` | [V6] List policies | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| GET | `/v1/tasks/pool` | List task pool entries | GET:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/claim` | Claim a task module | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/actions/{action}` | Trigger a task module action | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/reassign` | Reassign a task module within team scope | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/modules/{module_key}/pool-reassign` | Reassign a task module between pools | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/tasks/{id}/cancel` | Cancel or close a task | POST:已登录 / scope-aware | [V1_API_TASKS.md](V1_API_TASKS.md) |
| POST | `/v1/task-create/asset-center/upload-sessions` | Create task-create reference upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/task-create/asset-center/upload-sessions/{session_id}` | Get task-create reference upload session | GET:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/task-create/asset-center/upload-sessions/{session_id}/complete` | Complete task-create reference upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/task-create/asset-center/upload-sessions/{session_id}/abort` | Abort task-create reference upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/reference-upload` | Upload task-create reference file | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets` | List design assets in task asset center | GET:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions` | List versions under one design asset | GET:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/download` | Get latest version download info for one design asset | GET:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download` | Get specific version download info for one design asset | GET:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions` | Create upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/small` | Create small-file upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/multipart` | Create multipart upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}` | Get upload session status | GET:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete` | Complete upload session and record asset version | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel` | Cancel upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort` | Abort upload session | POST:已登录 / scope-aware | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) |
| GET | `/v1/assets` | List assets | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET, DELETE | `/v1/assets/{id}` | Get asset；Delete asset | GET:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{id}/download` | Get asset download info | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{id}/preview` | Get asset preview info | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions` | Create asset upload session | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/upload-sessions/{session_id}` | Get asset upload session | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions/{session_id}/complete` | Complete asset upload session | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-sessions/{session_id}/cancel` | Cancel asset upload session | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/files/{path}` | Proxy OSS-backed business file | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET, POST | `/v1/assets/upload-requests` | List asset upload requests；Create asset upload request | GET:已登录 / scope-aware; POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/upload-requests/{id}` | Get asset upload request | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/upload-requests/{id}/advance` | Advance asset upload request | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| GET | `/v1/assets/{asset_id}/versions/{version_id}/download` | Download a specific asset version | GET:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/{asset_id}/archive` | Archive an asset | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/assets/{asset_id}/restore` | Restore an archived asset | POST:已登录 / scope-aware | [V1_API_ASSETS.md](V1_API_ASSETS.md) |
| POST | `/v1/task-drafts` | Create or update a task draft | POST:已登录 / scope-aware | [V1_API_DRAFTS.md](V1_API_DRAFTS.md) |
| GET, DELETE | `/v1/task-drafts/{draft_id}` | Get a task draft；Delete a task draft | GET:已登录 / scope-aware; DELETE:已登录 / scope-aware | [V1_API_DRAFTS.md](V1_API_DRAFTS.md) |
| GET | `/v1/me/notifications` | List my notifications | GET:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| POST | `/v1/me/notifications/{id}/read` | Mark one notification as read | POST:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| POST | `/v1/me/notifications/read-all` | Mark all notifications as read | POST:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| GET | `/v1/me/notifications/unread-count` | Get unread notification count | GET:已登录 / scope-aware | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) |
| GET | `/v1/tasks/batch-create/template.xlsx` | Download batch create Excel template | GET:已登录 / scope-aware | [V1_API_BATCH.md](V1_API_BATCH.md) |
| POST | `/v1/tasks/batch-create/parse-excel` | Parse a batch create Excel file | POST:已登录 / scope-aware | [V1_API_BATCH.md](V1_API_BATCH.md) |
| GET | `/v1/erp/products` | Search ERP Bridge products | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
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
| GET | `/v1/products/search` | Search local cached ERP products | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/products/sync/status` | Get ERP sync placeholder status | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| POST | `/v1/products/sync/run` | Run ERP sync placeholder manually | POST:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
| GET | `/v1/products/{id}` | Get product by ID | GET:已登录 / scope-aware | [V1_API_ERP.md](V1_API_ERP.md) |
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
| GET | `/v1/assets/search` | Search assets across tasks | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/design-sources/search` | Search design source entries | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/search` | Perform a global search | GET:已登录 / scope-aware | [V1_API_SEARCH.md](V1_API_SEARCH.md) |
| GET | `/v1/reports/l1/cards` | Get L1 report cards | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/l1/throughput` | Get L1 throughput report | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
| GET | `/v1/reports/l1/module-dwell` | Get L1 module dwell report | GET:super_admin | [V1_API_REPORTS.md](V1_API_REPORTS.md) |
