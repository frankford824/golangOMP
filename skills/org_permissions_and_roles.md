# Skill: org_permissions_and_roles

## 1. 适用场景

- 组织树、部门、团队、角色、访问规则、权限范围变更。
- `/v1/auth/*`、`/v1/users*`、`/v1/access-rules`、`/v1/org/*` 相关改造。

## 2. 非适用场景

- 仅用户资料管理而不涉及权限模型（可用 `user_admin_and_identity.md`）。
- 仅任务业务逻辑改动（可用 `task_flow_and_status.md`）。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `docs/API_USAGE_GUIDE.md`

## 4. 禁止事项

- 禁止直接在业务流程中硬编码权限分支，绕开统一规则入口。
- 禁止在未定义迁移策略时删除现有角色能力。
- 禁止把历史文档中的旧组织字段重提为新规范。

## 5. 最小实施原则

- 先确认现有权限语义，再做最小增改。
- 避免新建权限系统；尽量在现有规则模型扩展。
- 保持审计可读性：谁在何条件下拥有什么权限。

## 6. 必跑测试命令

- `go build ./cmd/server`
- `go test ./service/...`
- `go test ./transport/...`

若存在鉴权中间件测试，优先补充其回归用例。

## 7. 文档更新要求

- 涉及权限契约时更新 `docs/api/openapi.yaml`。
- 变更范围说明必须包含：角色、资源、动作、约束。
- 如果影响前端菜单/访问控制，同步补充对接说明。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并补充：

- 权限模型变化摘要
- 风险角色清单
- 回滚关注点

## 9. 是否允许进入 release prep

- 默认不允许。
- 必须显式授权后再进入发布准备。

## 10. 常见误区 / 历史包袱提醒

- “先在前端挡一下”不能替代后端权限约束。
- 权限表面扩张快于治理速度会导致不可控，需保持收敛。
