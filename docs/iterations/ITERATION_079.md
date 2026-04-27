# ITERATION_079 — JST 用户同步预埋能力

**Date**: 2026-03-17  
**Version**: v0.4 (no version bump)

## Goal

为「聚水潭商家用户（getcompanyusers）-> 8080 MAIN 用户体系」做后端预埋开发，仅提供查询与导入能力，**不改变当前主业务用户/权限/登录逻辑**。

## Confirmed Facts

- Bridge(8081) 增加 JST getcompanyusers 适配：`/open/webapi/userapi/company/getcompanyusers`
- MAIN 通过 Bridge `GET /v1/erp/users` 获取 JST 用户，MAIN 不直接调 OpenWeb
- 本地 users 表扩展：`jst_u_id`、`jst_raw_snapshot_json`（migration 038）
- Admin 接口：`GET /v1/admin/jst-users`、`POST /v1/admin/jst-users/import-preview`、`POST /v1/admin/jst-users/import`
- 导入策略：匹配优先级 jst_u_id > loginId(若存在) > username；新建用户 status=disabled，密码为随机 hash（不可登录）
- 角色映射：默认 `write_roles=false`，不写入 user_roles；可选开启
- JST 仅作数据源，不接管鉴权、不自动同步、不覆盖超管

## Inferences

- 真实 JST 环境若未返回 `loginId`，则使用 `jst_u_id` 或生成 `jst_{u_id}` 作为 username
- 组织映射（ug_names -> department/team）需配置 OrgMapping，当前为空则留空
- 真实验收需有 JST 凭证与真实环境

## Verification Steps

1. 构建：`go build ./...` 通过
2. 运行 migration 038 后启动服务
3. Bridge remote/hybrid 模式下调用 `GET /v1/erp/users` 验证 JST 拉取
4. Admin 调用 `POST /v1/admin/jst-users/import-preview` 验证预览
5. Admin 调用 `POST /v1/admin/jst-users/import`（dry_run=true）验证导入逻辑
6. 验证现有登录/权限不受影响

## Auth Boundary

- JST 仅作数据源，MAIN 鉴权主链不变
- 新建导入用户默认 disabled，需管理员重置密码后启用
- 角色写入默认关闭，需显式 `write_roles=true`

## Files Changed

- `domain/jst_user.go` (new)
- `domain/auth_identity.go` (User.JstUID, JstRawSnapshotJSON)
- `config/config.go` (GetCompanyUsersPath)
- `service/erp_bridge_client.go` (GetCompanyUsers, decodeJSTUserListResponse)
- `service/erp_bridge_remote_client.go` (GetCompanyUsers, buildERPRemoteOpenWebBiz getcompanyusers)
- `service/erp_bridge_local_client.go` (GetCompanyUsers stub)
- `service/erp_bridge_remote_client.go` hybrid (GetCompanyUsers)
- `service/erp_bridge_service.go` (ListJSTUsers)
- `service/jst_user_import.go` (new)
- `transport/handler/erp_bridge.go` (ListJSTUsers)
- `transport/handler/jst_user_admin.go` (new)
- `transport/http.go` (GET /v1/erp/users, admin routes)
- `repo/interfaces.go` (GetByJstUID, UpdateJstFields)
- `repo/mysql/identity.go` (jst columns, GetByJstUID, UpdateJstFields)
- `db/migrations/038_v7_jst_user_prewire.sql` (new)
- `cmd/server/main.go`, `cmd/api/main.go` (wire jstUserAdminH)
