# ITERATION_091

Title: Account permission and department/team management minimum usable closure, responsibility mapping, unassigned pool, and `v0.8` overwrite publish

Date: 2026-03-23
Model: GPT-5 Codex

## 1. Background and goal
- This round was explicitly limited to the identity-management line:
  - account permission minimum closure
  - department/team management minimum closure
  - user/role management minimum closure
  - minimum `frontend_access` / scope expression
  - permission-log traceability
- This round was explicitly not allowed to break already-closed business mainlines:
  - reference-small fixed path
  - multipart `remote.headers.X-Internal-Token` fixed path
  - `GET /v1/tasks/{id}` design-asset aggregation fix
  - existing upload / complete / submit-design / task-detail reads
- This round was also explicitly not allowed to expand into:
  - full org-tree editing
  - full ABAC / row-level platform
  - login/register mainline rewrite

## 2. Baseline before this round
- Auth mainline already existed:
  - login
  - register
  - `/v1/auth/me`
  - user list / user detail
  - role list and role mutation
  - permission-log persistence
- But there were several minimum-closure gaps:
  - no stable fixed department/team model for the requested first version
  - no explicit unassigned-pool semantics
  - no management-range fields on users
  - no backend org-options read endpoint for admin use
  - `frontend_access` did not yet express the new management scope fields cleanly
  - JST user import did not consistently land unmatched users into an explicit pool
  - audit logs did not yet cleanly split org change and managed-scope change action types when both were changed together
- Existing historical users on live also still included old department/team values from earlier placeholder models. This round did not do a destructive bulk cleanup.

## 3. Design principles for this round
- Reuse the existing identity/auth mainline instead of rebuilding auth.
- Add only the minimum new fields and endpoints needed for management-side closure.
- Keep task business mainlines isolated from the new auth/org vocabulary.
- Do not reuse the new org teams to break historical task owner-team semantics.
- Make configuration and migrations idempotent or additive.
- Prefer explicit live evidence over document-only claims.

## 4. Department, team, and responsibility model
- Fixed departments shipped in backend config:
  - `人事部`
  - `设计部`
  - `运营部`
  - `采购部`
  - `仓储部`
  - `烘焙仓储部`
  - `未分配`
- Fixed first-version teams shipped in backend config:
  - `人事部 -> 人事管理组`
  - `设计部 -> 定制美工组, 设计审核组`
  - `运营部 -> 运营一组 ... 运营七组`
  - `采购部 -> 采购组`
  - `仓储部 -> 仓储组`
  - `烘焙仓储部 -> 烘焙仓储组`
  - `未分配 -> 未分配池`
- Backend validation now enforces:
  - `team` must belong to `department`
  - `managed_departments` must exist in the fixed department list
  - `managed_teams` must exist in the fixed team catalog
- First-version responsibility mappings are expressed by config plus user fields, not by hardcoded name-branch business logic:
  - `刘芸菲 -> HRAdmin + OrgAdmin`, managed `人事部`
  - `王亚琳 -> DepartmentAdmin + DesignDirector`, managed `设计部`
  - `马雨琪 -> DesignReviewer`
  - `章鹏鹏 -> TeamLead`, managed `定制美工组`
  - `方晓兵 -> DepartmentAdmin`, managed `采购部/仓储部/烘焙仓储部`

## 5. Unassigned-pool solution
- The unassigned pool is now explicit:
  - `department = 未分配`
  - `team = 未分配池`
- Register flow and JST import both support landing unmatched users here first.
- Pool users are intentionally minimal-scope users and do not inherit formal business department visibility.
- Formal dispatch from pool to a business org is now done through user patching and role assignment.

## 6. Roles and minimum data-scope expression
- Added/standardized the minimum management-role catalog:
  - `SuperAdmin`
  - `HRAdmin`
  - `OrgAdmin`
  - `RoleAdmin`
  - `DepartmentAdmin`
  - `TeamLead`
  - `DesignDirector`
  - `DesignReviewer`
  - `Member`
- Kept compatibility workflow roles active:
  - `Admin`
  - `Ops`
  - `Designer`
  - `Audit_A`
  - `Audit_B`
  - `Warehouse`
  - `Outsource`
  - `ERP`
- `frontend_access` was expanded to stably surface:
  - `roles`
  - `scopes`
  - `menus`
  - `pages`
  - `actions`
  - `view_all`
  - `department_codes`
  - `team_codes`
  - `managed_departments`
  - `managed_teams`
- Admin safety remained in place:
  - last active `Admin` / `SuperAdmin` cannot be removed
  - last active `Admin` / `SuperAdmin` cannot be disabled

## 7. API and response-structure closure
- Unified the minimum user/org/role field shape across:
  - `GET /v1/auth/me`
  - `GET /v1/users`
  - `GET /v1/users/{id}`
  - `GET /v1/roles`
  - `GET /v1/permission-logs`
- Added minimum admin org-read endpoint:
  - `GET /v1/org/options`
- `GET /v1/org/options` now returns:
  - `departments`
  - `teams_by_department`
  - `role_catalog_summary`
  - `unassigned_pool_enabled`
- Extended `PATCH /v1/users/{id}` to support the minimum management closure:
  - `display_name`
  - `status`
  - `department`
  - `team`
  - `email`
  - `mobile`
  - `managed_departments`
  - `managed_teams`
- Kept role mutation on the existing routes:
  - `POST /v1/users/{id}/roles`
  - `PUT /v1/users/{id}/roles`
  - `DELETE /v1/users/{id}/roles/{role}`

## 8. Audit-log closure
- Permission logs now cover the required minimum mutation classes:
  - `role_assigned`
  - `role_removed`
  - `user_org_changed`
  - `user_scope_changed`
  - `user_status_changed`
  - `user_pool_assigned`
  - `user_updated`
  - `register`
- A final small follow-up patch in this same round split `user_org_changed` and `user_scope_changed` into separate log entries when both happen in the same patch request. That made live audit output match the intended boundary more clearly.

## 9. Release action
- Local verification before release:
  - `go test ./service -run "Identity|JST"` passed
  - `go test ./transport/... -run "Auth|User|Org|Permission"` passed
  - `go build ./cmd/server ./cmd/api` passed
- Note on local test environment:
  - `go test ./config` remained blocked by host Application Control when Windows tried to launch generated `config.test.exe`
  - this was an environment execution restriction, not a compile failure in the identity patch
- Release entrypoint used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "v0.8 overwrite: org audit log split for minimal account/org closure"`
- Deployment target:
  - `jst_ecs:/root/ecommerce_ai/releases/v0.8`
- Release result:
  - overwrite publish completed in place on `v0.8`
  - `8080/8081/8082` all reported healthy after publish
  - `OVERALL_OK=true`

## 10. Live verification result
- Live read validation:
  - `GET /v1/auth/me` returned `roles`, `department`, `team`, and `frontend_access`
  - `GET /v1/roles` returned the expanded 17-role catalog
  - `GET /v1/users` returned user rows with org/role/frontend fields
  - `GET /v1/users/{id}` returned matching detail fields
  - `GET /v1/org/options` returned 7 departments, team mappings, role catalog summary, and `unassigned_pool_enabled=true`
- Live safe-sample mutation validation:
  - registered a new user into `未分配 / 未分配池`
  - patched the user into `运营部 / 运营一组`
  - added `managed_teams = ["运营一组"]`
  - replaced roles with `Member + TeamLead`
  - toggled status `disabled -> active`
  - read back the final user detail successfully
- Live permission-log evidence for that sample user included:
  - `register`
  - `user_pool_assigned`
  - `user_org_changed`
  - `user_scope_changed`
  - `role_assigned`
  - `user_status_changed`

## 11. Current boundary
- This is now a minimum usable account/org management closure on live `v0.8`.
- It is not a full organization platform.
- Historical live users still exist with older org values such as earlier placeholder departments/teams. The backend now tolerates that state, but full normalization is deferred.
- Task owner-team compatibility was intentionally kept separate from the new auth/org team catalog so the current task mainlines do not regress.

## 12. Next TODO
- Historical task `500` investigation/cleanup remains the next practical priority.
- Dirty-data cleanup and historical user/org normalization remain next-priority cleanup work.
- Broader regression and data-fix coverage should follow after the current minimum closure.
