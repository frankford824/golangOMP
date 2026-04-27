# v0.9 Model Handoff Manifest

Last updated: 2026-04-17

This file is the handoff entry index for future models or developers. It is not the API contract by itself.

## Read First

1. Read this manifest.
2. Read `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.
3. Read `docs/api/openapi.yaml`.
4. Read `transport/http.go` if you need mounted-runtime confirmation.
5. Read focused guides only if the task needs them.

## Layer 1: Authoritative Source Of Truth

Only these are authoritative:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

Use Layer 1 to decide:

- canonical routes
- compatibility-only routes
- current request/response contract
- upload/download/storage behavior
- removal policy
- which frontend docs are still valid

## Layer 2: Current Operational Guides

These are current-use guides. They do not override Layer 1.

- `docs/TASK_CREATE_RULES.md`
- `docs/API_USAGE_GUIDE.md`
- `docs/ASSET_UPLOAD_INTEGRATION.md`
- `docs/ASSET_ACCESS_POLICY.md`
- `docs/ASSET_STORAGE_AND_FLOW_RULES.md`
- `docs/FRONTEND_MAIN_FLOW_CHECKLIST.md`
- `docs/ops/NAS_SSH_ACCESS.md`
- root `CURRENT_STATE.md` and `MODEL_HANDOVER.md` only as orientation indexes

Use them for:

- task-create rules
- frontend migration alignment
- OSS upload/download flow
- ops-only NAS SSH access

Do not use `docs/archive/obsolete_alignment/FRONTEND_ALIGNMENT_v0.5.md` as a current frontend starting point. It is obsolete and retained only as a downgraded historical marker.

## Layer 3: Archive / History Only

These are history or evidence only. Do not use them as the current spec unless Layer 1 restates the same rule.

- `docs/archive/*`
- `docs/iterations/*`
- `docs/phases/*`
- `ITERATION_INDEX.md`
- old forensic notes and patch guides

## Current Mainline Summary

- Business asset runtime is OSS-only.
- Historical NAS compatibility is intentionally abandoned.
- Canonical pre-task reference upload is `POST /v1/tasks/reference-upload`.
- Canonical asset resource namespace is `/v1/assets*`.
- Canonical task-linked asset lookup path is `GET /v1/tasks/{id}/assets`; `/v1/tasks/{id}/asset-center/*` read/upload-session routes are compatibility-only aliases.
- Official file-serving route is `GET /v1/assets/files/{path}`.
- Compatibility-only routes and fields remain visible only for migration safety and must not be used for new frontend rollout.
- NAS remains only as ops/developer infrastructure and must be treated as out-of-band from business storage/runtime.

## Current Round Status

- code complete
- docs complete
- local validation complete
- v1.0 RC validation passed
- release prep authorized
- v1.0 convergence closure completed on 2026-04-16
- auth_identity.json aligned to v1.0 official departments
- 2026-04-17 Round J v1.9: DataScope now includes stage visibility for mid-lane departments. `service/data_scope.go` grants lane-aware task-status visibility to `Audit_A`, `Audit_B`, `Warehouse`, `Outsource`, `CustomizationOperator`, and `CustomizationReviewer`, plus the locked department-admin unions for `审核部`, `云仓部`, and `定制美工部`. Task list SQL (`repo/mysql/task.go`), task-board candidate scans, and task-detail read authorization now all honor the same scope model, and tests cover list scope, board-candidate scope, read-detail allow/deny, and SQL clause composition.
- 2026-04-17 Round I v1.8: post-UAT backend convergence completed for five items without route-guard broadening or schema migration: (1) `POST /v1/assets/upload-sessions/:id/complete` now defers source-derived preview generation behind a short grace window so serial source(PSD)->delivery(JPG) completes do not contend on `task_assets.version_no`; duplicate-key collisions on `uq_task_assets_task_version` are translated to structured `409 CONFLICT` with `deny_code=asset_version_race_retry` and conflict logs now include trace/request/task/version context; (2) `HRAdmin` frontend_access now exposes `org.manage`, `role.assign`, and `role.read` in both derived spec and JSON config, matching mounted route-layer permissions for org department/team maintenance, role assignment, and role catalog read; (3) `DepartmentAdmin` frontend_access retains backward-compatible umbrella keys and adds six fine-grained actions (`department.users.create`, `department.users.move_team`, `department.users.disable`, `department.users.reset_password`, `department.users.assign_from_unassigned`, `task.reassign.department`) aligned to existing route guards only; (4) `PATCH /v1/users/{id}` additive semantics were re-audited and confirmed unchanged because unspecified request fields already remain no-op through pointer-based handler/service update flow; (5) `/v1/org/options` is now documented as canonical `departments[].teams`, with `teams_by_department` retained only as a deprecated v1.8 compatibility mirror and responses that include it emit `Deprecation: version="v1.8"`.
- 2026-04-17 Round H v1.7: service-layer `authorizeUserRoleChange` now converges with route-layer `/v1/users/:id/roles*` guards and allows only `HRAdmin` / `SuperAdmin`
- T3 resolved: legacy `Admin` / `RoleAdmin` compatibility no longer bypasses service-layer role-change authorization
- cleanup scripts updated for customization tables and credential safety
- Round B backend convergence released as v1.4 on 2026-04-16: derivedFrontendSpec(RoleDeptAdmin) Menus/Pages reduced to user_admin/department_users; config/frontend_access.json aligned to target visibility matrix for org_admin/role_admin/admin_roles/org_options; frontend SPA changes tracked separately in the vue/ repository and not in this release.
- Round C backend convergence released as v1.5 on 2026-04-17: minimal post-UAT backend tightening — (1) `GET /v1/users/designers` route guard widened from `[Ops, Designer, Admin]` to `[Ops, Designer, Admin, HRAdmin, SuperAdmin]` so Ops-style task creators plus HR/SuperAdmin can look up designers cross-department; DepartmentAdmin intentionally stays out of this route and continues to be scoped by `authorizeUserListFilter` on the canonical `/v1/users` path; (2) `derivedFrontendSpec` grew explicit defense-in-depth branches for the 8 business roles (`Admin`, `Ops`, `Designer`, `Audit_A`, `Audit_B`, `Warehouse`, `Outsource`, `ERP`), mirroring `config/frontend_access.json` so role coverage survives a JSON load failure; (3) `derivedFrontendSpec(RoleTeamLead)` Menus/Pages/Actions restored to the SOT "可看本部门全部任务 / 管理本组成员 / 只能操作本组任务" capability set (`task_list`+`user_admin` menus, `team_users`+`task_list`+`my_tasks` pages, `team.users.read`+`team.manage`+`task.reassign.team`+`task.list` actions); (4) `docs/api/openapi.yaml` `/v1/auth/login` request schema aligned to runtime — `username` is the required field and `account` is now documented as the compatibility alias (runtime handler in `transport/handler/auth.go` is the source of truth and was not modified). Round C does NOT fix the UAT "only dashboard/profile" symptom; runtime diagnostic has proven that issue is frontend-side and is tracked in a separate frontend handoff.
- Round C open follow-up: `service/identity_service.go` `authorizeUserListFilter` currently denies any actor whose only management-relevant role is `Ops` with `management_access_required`. The canonical `GET /v1/users/designers` route now admits Ops at the route layer, but the service-level filter must also accept Ops (unscoped, no department restriction) for the end-to-end flow to work in UAT. This is an intentional Round C-future follow-up and was not modified in v1.5 per the Round C constraint scope.
- Round D: introduced `ListAssignableDesigners` dedicated service method at v1.6; `/v1/users/designers` now bypasses `authorizeUserListFilter` to enable Ops cross-department designer lookup without widening `ListUsers` management scope. `authorizeUserListFilter` and the canonical `GET /v1/users` path are untouched (Round C-open-followup closed). Closes UAT regression where 运营部 Ops saw empty Designer dropdown. Repo gains one minimal method `UserRepo.ListActiveByRole(role)`; handler rewires to the new service method and returns `{data, pagination:{page:1, page_size:<len>, total:<len>}}`. `/v1/users/designers` explicitly rejects keyword/department/team/pagination parameters in docs (it already ignored them at the service path). Smoke verified on single-server pre-production: admin probe returned `admin` + `设计超级管理员` with `total:2`. The 运营超级管理员 cross-department probe is deferred to manual UAT (credentials not obtainable without a password reset, which is out of scope for this round).
- Design pattern note: future assignment-candidate-pool endpoints (e.g. assignable warehouse operators, assignable auditors) should follow the same dedicated-service-method pattern rather than extending `authorizeUserListFilter` branches. The canonical `/v1/users*` management surface must preserve strict department/team scoping; any cross-scope enumeration for assignment dropdowns belongs on its own narrow route + service method guarded exclusively by the route layer.

## Conflict Rule

If documents conflict:

1. `transport/http.go` decides what is mounted now.
2. `docs/api/openapi.yaml` decides the current request/response contract.
3. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` decides route classification and compatibility policy.
4. Everything else is secondary or historical.
