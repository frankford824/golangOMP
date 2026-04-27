# ITERATION_080 — v0.5 发布收口（已完成）

**Date**: 2026-03-17  
**Version**: v0.5  
**状态**: 已完成，v0.5 已发布

## 本轮发布目标

v0.5 部署收口与真实验收：迁移、构建、部署与 API 验收，不继续大改功能。

## 实际执行的 Migration

- **038**：`users` 表新增 `jst_u_id`、`jst_raw_snapshot_json`（**v0.5 启动前置条件**，必须先于 039/040 执行）
- **039**：`rule_templates` 表 + 3 条种子数据
- **040**：`server_logs` 表

部署时必须按 **038 → 039 → 040** 顺序执行；若只执行 039/040 而漏掉 038，MAIN 启动后会因查询不存在的 users 字段而立即退出。

## 部署结果

- **发布目标机**：`223.4.249.11`
- **发布目录**：`/root/ecommerce_ai/releases/v0.5`
- **当前线上版本**：v0.5

## 三服务运行状态（已验收）

| 服务 | 端口 | PID | 二进制路径/名称 | health |
|------|------|-----|------------------|--------|
| MAIN | 8080 | 3589336 | `/root/ecommerce_ai/releases/v0.5/ecommerce-api` | 200 |
| Bridge | 8081 | 3589373 | `/root/ecommerce_ai/releases/v0.5/erp_bridge` | 200 |
| Sync | 8082 | 3589421 | `erp_bridge_sync` | 200 |

## API 验收范围（已完成）

以下接口已在 v0.5 环境中完成验证：

- 登录
- `GET /v1/auth/me`（/me）
- `GET /v1/rule-templates`（rule-templates）
- `GET /v1/server-logs`、`POST /v1/server-logs/clean`（server-logs）
- `GET /v1/tasks`、`POST /v1/tasks`、`GET /v1/tasks/{id}`（tasks）

## 部署中遇到的问题与修复

- **现象**：初次部署时 MAIN 启动后立即退出。
- **原因**：v0.5 代码会查询 `users.jst_u_id`、`users.jst_raw_snapshot_json`，但 migration 038 尚未执行，表中无此两列。
- **处理**：在目标环境补齐执行 migration 038 后，MAIN 恢复正常启动。

结论：**migration 038 是 v0.5 启动前置条件**，不能只执行 039/040 而漏掉 038。

## 本轮已交付能力（代码与文档）

1. **任务创建与详情**  
   创建时若传 `designer_id`，任务直接进入 `InProgress`，并设置 `current_handler_id`。创建请求支持 `assignee_id`、`reference_file_refs`、`note`、`need_outsource`、`requester_id`。创建响应返回完整 `TaskReadModel`。`GET /v1/tasks/{id}` 返回 `assignee_id`、`assignee_name`、`design_requirement`、`note`、`reference_file_refs`、`creator_name`。新增 `GET /v1/users/designers`。`creator_name`/`assignee_name` 通过 `UserDisplayNameResolver` 解析。

2. **审核流程**  
   asset-center 完成 delivery 上传后，若任务状态在 `InProgress`/`RejectedByAuditA`/`RejectedByAuditB`，自动推进到 `PendingAuditA`。submit-design 后也应进入 `PendingAuditA`。审核链支持 claim、reject、approve。

3. **参考图上传限制**  
   单张 base64 最大 200KB，总计 512KB，最多 5 张。超限返回 400，并提示改走 asset-center upload-sessions。

4. **服务器日志管理**  
   `GET /v1/server-logs`、`POST /v1/server-logs/clean`；5xx 自动写入 `server_logs`；返回前脱敏；Admin 权限。

5. **规则及模板**  
   rule_templates 表 + GET/PUT API（cost-pricing, product-code, short-name）；主菜单「规则及模板」。

6. **文档收口**  
   `CURRENT_STATE.md`、`MODEL_HANDOVER.md`、`ITERATION_INDEX.md`、`docs/FRONTEND_ALIGNMENT_v0.5.md` 已更新为 v0.5 已发布状态；前端联调以 `FRONTEND_ALIGNMENT_v0.5.md` 与 openapi 为准。

## 最终结论

**v0.5 已发布**。当前线上版本为 v0.5，三服务运行正常，migrations 038/039/040 已执行，API 验收已完成，前端接口收口对齐文档已更新为 v0.5 联调基准。

## 相关文件

- `deploy/run-migrations-v05.sh`（需包含 038/039/040，且 038 必须先执行）
- `deploy/verify-v05-acceptance.sh`
- `deploy/lib.sh`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/FRONTEND_ALIGNMENT_v0.5.md`
