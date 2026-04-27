# Skill: frontend_handoff

## 1. 适用场景

- 前端对接新接口、迁移旧接口、联调交付说明编写。
- 需要明确 canonical 与 compatibility 边界的交接任务。

## 2. 非适用场景

- 纯后端运行时逻辑重构（改用 `runtime_feature_change.md`）。
- 仅发布动作（改用 `release_prep.md`）。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
5. `docs/FRONTEND_MAIN_FLOW_CHECKLIST.md`
6. `docs/API_USAGE_GUIDE.md`

## 4. 禁止事项

- 禁止使用 compatibility-only 路由做新接入。
- 禁止引用 archive/obsolete 前端对齐文档作为当前规范。
- 禁止只交“接口名”不交字段语义和错误处理策略。

## 5. 最小实施原则

- 只交付当前需求必需的最小契约说明。
- canonical path、必填字段、错误语义必须明确。
- 兼容字段存在时必须标注“不可用于新逻辑”。

## 6. 必跑测试命令

- `go build ./cmd/server`
- `go test ./transport/...`

如含业务变更，补 `go test ./service/...`。

## 7. 文档更新要求

- 变更后更新前端对接所需 guide。
- 若契约变化，必须同步 `docs/api/openapi.yaml`。
- 交付文本必须可直接给前端执行，不依赖口头背景。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并补充：

- 前端接入入口顺序
- 字段映射要点
- 兼容字段禁用说明
- 联调验收建议

## 9. 是否允许进入 release prep

- 默认不允许。
- 需要明确授权。

## 10. 常见误区 / 历史包袱提醒

- 老版 frontend alignment 文档常混入旧路由，必须拒绝复用。
- 只给“示例请求”但不给“错误语义”会导致联调反复。
