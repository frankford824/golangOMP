# Skill: user_admin_and_identity

## 1. 适用场景

- 用户管理、身份字段、认证会话、用户-组织关系调整。
- 用户 CRUD、登录态、身份映射与展示语义的改造。

## 2. 非适用场景

- 权限模型整体改造（改用 `org_permissions_and_roles.md`）。
- 与用户无关的任务/资产流程改造。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `docs/API_USAGE_GUIDE.md`

## 4. 禁止事项

- 禁止在无迁移方案下直接替换身份主键语义。
- 禁止在 handler 层散布临时身份映射逻辑。
- 禁止只改返回字段不更新文档与前端对接说明。

## 5. 最小实施原则

- 用户身份字段变更遵循“可回溯、可兼容、可验证”。
- 优先复用现有认证链路与用户模型。
- 避免为短期需求新增平行身份体系。

## 6. 必跑测试命令

- `go build ./cmd/server`
- `go test ./service/...`
- `go test ./transport/...`

根据改动补充认证与用户管理相关测试。

## 7. 文档更新要求

- 对外字段变化更新 `docs/api/openapi.yaml`。
- 行为规则变化更新 `docs/API_USAGE_GUIDE.md` 或相关 guide。
- 输出中明确前端是否需要字段迁移。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并补充：

- 身份字段变更点
- 兼容输出策略
- 对接方影响清单

## 9. 是否允许进入 release prep

- 默认不允许。
- 仅用户明确授权后允许。

## 10. 常见误区 / 历史包袱提醒

- 将显示名、登录名、唯一身份标识混用会引起权限与审计错误。
- 忽略历史兼容字段会导致旧客户端回归问题。
