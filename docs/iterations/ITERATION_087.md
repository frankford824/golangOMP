# ITERATION_087

## Date
2026-03-19

## Goal
Implement the backend-merged upgrade for task filing and ERP auto-sync strategy:
- keep frontend main flow unchanged
- replace legacy single trigger boundary with backend state machine + auto triggers
- add idempotent payload sync control and retry path
- preserve backward compatibility (`filed_at`, legacy business-info trigger semantics)

## Scope
- Task types:
  - `original_product_development`
  - `new_product_development`
  - `purchase_task`
- Runtime surfaces:
  - create flow
  - business-info patch flow
  - procurement update/advance flow
  - audit final approve flow
  - warehouse complete precheck flow

## Backend Design Landed
- Filing states:
  - `not_filed`
  - `pending_filing`
  - `filing`
  - `filed`
  - `filing_failed`
- Trigger sources:
  - `create`
  - `business_info_patch`
  - `procurement_update`
  - `procurement_advance`
  - `audit_final_approved`
  - `warehouse_complete_precheck`
  - `manual_retry`
  - `legacy_filed_at`
- Idempotency:
  - `last_filing_payload_hash` for same-payload skip
  - `erp_sync_version` for payload evolution
  - `last_filing_payload_json` snapshot for diagnosis
- Retry:
  - `POST /v1/tasks/{id}/filing/retry`
- Read-model projection:
  - `filing_status`
  - `filing_error_message`
  - `filing_trigger_source`
  - `last_filing_attempt_at`
  - `last_filed_at`
  - `erp_sync_required`
  - `erp_sync_version`
  - `missing_fields`
  - `missing_fields_summary_cn`
  - legacy `filed_at` kept

## Code Changes
- Added migration:
  - [db/migrations/045_v7_task_filing_policy_upgrade.sql](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/db/migrations/045_v7_task_filing_policy_upgrade.sql)
- Added policy + trigger logic:
  - [service/task_filing_policy.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_filing_policy.go)
  - [service/task_filing_trigger_service.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_filing_trigger_service.go)
- Integrated service trigger points:
  - [service/task_service.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_service.go)
  - [service/audit_v7_service.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/audit_v7_service.go)
  - [service/warehouse_service.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/warehouse_service.go)
  - [service/task_detail_service.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_detail_service.go)
- API/handler/router:
  - [transport/handler/task.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/transport/handler/task.go)
  - [transport/http.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/transport/http.go)
- Main wiring:
  - [cmd/server/main.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/cmd/server/main.go)
  - [cmd/api/main.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/cmd/api/main.go)
- Domain/repo/read-model:
  - [domain/task.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/domain/task.go)
  - [domain/query_views.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/domain/query_views.go)
  - [domain/audit.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/domain/audit.go)
  - [repo/mysql/task.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/repo/mysql/task.go)
  - [service/task_workflow.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_workflow.go)

## OpenAPI and Docs
- Updated:
  - [docs/api/openapi.yaml](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/docs/api/openapi.yaml)
  - [CURRENT_STATE.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/CURRENT_STATE.md)
  - [MODEL_HANDOVER.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/MODEL_HANDOVER.md)
  - [ITERATION_INDEX.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/ITERATION_INDEX.md)
  - [设计流转自动化管理系统_V7.0_重构版_技术实施规格.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/设计流转自动化管理系统_V7.0_重构版_技术实施规格.md)

## Tests
- Added/updated tests:
  - [service/task_filing_flow_test.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_filing_flow_test.go)
  - [service/task_erp_bridge_test.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_erp_bridge_test.go)
  - [service/task_prd_service_test.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_prd_service_test.go)
  - [repo/mysql/task_test.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/repo/mysql/task_test.go)
- Verification executed:
  - `go test -c ./service` (pass)
  - `go test -c ./transport/handler` (pass)
  - `go test -c ./repo/mysql` (pass)
  - `go test ./cmd/server ./cmd/api` (pass)
- Environment limit:
  - full runtime execution of generated `*.test.exe` is blocked by host application-control policy in this environment.

## Boundary Statement
- Design Target: completed.
- Code Implemented: completed.
- Server Verified: local compile-level and package-level verification completed.
- Live Effective: not declared in this iteration.
