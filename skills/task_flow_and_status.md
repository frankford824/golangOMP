# Skill: task_flow_and_status

## 1. 适用场景

- 任务创建、状态流转、处理人字段、批量 SKU 规则调整。
- 任务明细、事件、提交设计、审核流相关变更。

## 2. 非适用场景

- 纯资产传输能力（改用 `storage_and_assets.md`）。
- 纯组织权限模型设计（改用 `org_permissions_and_roles.md`）。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `docs/TASK_CREATE_RULES.md`
5. `docs/API_USAGE_GUIDE.md`

## 4. 禁止事项

- 禁止引入与 canonical actor 字段冲突的新命名。
- 禁止把 mother-task 的兼容摘要字段当作 per-SKU 真相源。
- 禁止绕开主链改走历史兼容路由。

## 5. 最小实施原则

- 对任务流转改动保持可追踪：输入条件、状态变更、输出副作用。
- 优先复用现有 service/handler，不新建流程平台。
- 批量任务场景下，先保证 SKU 级真相，再考虑汇总展示。

## 6. 必跑测试命令

- `go build ./cmd/server`
- `go test ./service/...`
- `go test ./transport/...`

按影响补充任务相关包测试。

## 7. 文档更新要求

- 任务创建规则变化需更新 `docs/TASK_CREATE_RULES.md`。
- 可观察接口变化同步更新 `docs/api/openapi.yaml`。
- 使用指南变化同步更新 `docs/API_USAGE_GUIDE.md`。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并补充：

- 任务阶段/状态变化图（可文字化）
- 处理人/归属字段变化
- 对批量模式影响

## 9. 是否允许进入 release prep

- 默认不允许。
- 需明确授权。

## 10. 常见误区 / 历史包袱提醒

- 旧文档中可能混用 `owner_team` 与 canonical 所有者字段，必须以 authority 为准。
- 为短期需求新增“旁路状态”会显著提高长期维护成本，应避免。
