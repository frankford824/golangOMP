# ITERATION_097

Title: MAIN task/org formal connection, canonical task ownership, minimum org-scoped visibility, overwrite publish to existing `v0.8`, and live verification

Date: 2026-03-31
Model: GPT-5 Codex

## 1. Goal
- Keep the already-live org/role minimum closure and the already-fixed task-create mainline intact.
- Add formal canonical task-side org ownership without deleting legacy task ownership.
- Wire minimum task list/detail visibility to canonical ownership.
- Overwrite publish the result onto the existing live release line `v0.8` and verify it online.

## 2. Canonical ownership model
- Kept legacy compatibility field:
  - `tasks.owner_team`
- Added canonical task-side ownership fields:
  - `tasks.owner_department`
  - `tasks.owner_org_team`
- Chosen boundary:
  - name-based canonical fields on `tasks`
  - no separate code/label table in this round
  - no historical full cleanup in this round
- Create-time behavior:
  - legacy `owner_team` still works directly
  - supported org-team input such as `运营三组` still normalizes into legacy `owner_team`
  - new tasks also persist canonical department/team
  - legacy-only create input only backfills canonical values when mapping is deterministic

## 3. Runtime/code changes
- Added migration:
  - `db/migrations/047_v7_task_canonical_org_ownership.sql`
- Added canonical ownership resolution/read helpers:
  - `service/task_org_ownership.go`
  - `resolveTaskCanonicalOrgOwnership(...)`
  - `buildTaskReadModelOrgOwnership(...)`
  - `applyTaskOrgVisibilityScope(...)`
- Extended task create/detail/list flow to read/write canonical ownership:
  - `service/task_service.go`
  - `service/task_query.go`
  - `service/task_detail_service.go`
  - `repo/mysql/task.go`
  - `transport/handler/task.go`
  - `transport/handler/task_filters.go`
- Connected minimum visibility to full user org data:
  - `service/task_data_scope_guard.go`
  - `service/data_scope.go`
  - `cmd/server/main.go`
  - `cmd/api/main.go`
- Widened task read route role allowance so management roles can actually use the new minimum visibility:
  - `transport/http.go`
- Final JSON contract fix:
  - removed `omitempty` from task/list ownership JSON fields in `domain/task.go` and `domain/query_views.go`

## 4. Tests and local verification
- Added/updated coverage for:
  - create with org-team compatibility writing canonical ownership
  - legacy owner-team compatibility fallback
  - list/detail canonical ownership hydration
  - minimum org visibility scope
  - repo SQL filter/scope support for canonical ownership
- Required local commands passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Additional repo-layer regression also passed:
  - `go test ./repo/mysql`

## 5. Publish
- First overwrite publish command:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task canonical org ownership and visibility"`
- Second overwrite publish command:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task canonical org ownership and visibility json contract fix"`
- Result:
  - existing `v0.8` overwritten in place
  - production entrypoint remained `./cmd/server`
  - live binaries remained:
    - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `/root/ecommerce_ai/erp_bridge_sync`

## 6. Live migration and failure record
- First live acceptance after the first overwrite exposed a real schema gap:
  - `GET /v1/tasks` returned `500`
  - live DB had not yet applied `047_v7_task_canonical_org_ownership.sql`
- Backup created before schema mutation:
  - `/root/ecommerce_ai/backups/20260331T033855Z_task_canonical_org_047`
- Live migration then applied from:
  - `/root/ecommerce_ai/releases/v0.8/db/migrations/047_v7_task_canonical_org_ownership.sql`
- After schema fix, one more runtime contract issue remained:
  - empty `owner_department` / `owner_org_team` fields were still omitted from some JSON responses because of `omitempty`
- That omission was fixed locally, then overwrite-published again.

## 7. Live verification
- Health:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
- `/proc/<pid>/exe`:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/root/ecommerce_ai/erp_bridge_sync`
- Task create with org-team compatibility:
  - `original_product_development` + `owner_team="运营三组"` -> task `157`, returned:
    - `owner_team=内贸运营组`
    - `owner_department=运营部`
    - `owner_org_team=运营三组`
  - `new_product_development` + `owner_team="运营三组"` -> task `158`, same ownership result
  - `purchase_task` + `owner_team="运营三组"` -> task `159`, same ownership result
- Additional verification tasks:
  - `owner_team="运营一组"` -> task `160`, returned `owner_department=运营部`, `owner_org_team=运营一组`
  - `owner_team="定制美工组"` -> task `161`, returned `owner_department=设计部`, `owner_org_team=定制美工组`
- List/detail contract:
  - `/v1/tasks` returned `owner_team`, `owner_department`, `owner_org_team`
  - `/v1/tasks/158` returned `owner_team=内贸运营组`, `owner_department=运营部`, `owner_org_team=运营三组`
- Canonical filters:
  - `/v1/tasks?owner_org_team=运营三组` included tasks `158` and `159`, excluded task `160`
  - `/v1/tasks?owner_department=运营部` included tasks `158` and `160`, excluded task `161`
- Visibility verification:
  - registered `dept_admin_1774928594` with roles `Member + DepartmentAdmin`
  - registered `team_lead_1774928594` with roles `Member + TeamLead`
  - admin view-all session saw all verification tasks
  - `DepartmentAdmin` session saw both ops-department tasks and not the design-department task
  - `TeamLead` session saw only the `运营一组` task and not the `运营三组` / design tasks

## 8. Boundaries and remaining risk
- This is not the final org model.
- This is not a full ABAC or row-level visibility engine.
- Legacy `tasks.owner_team` is still retained and still used for compatibility.
- Historical tasks are not fully backfilled:
  - deterministic department backfill only
  - ambiguous historical org-team ownership still remains empty
