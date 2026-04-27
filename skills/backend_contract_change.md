# Skill: backend_contract_change

## 1. 适用场景

- 新增/修改后端 API 路径、请求字段、响应字段、错误码。
- 调整 canonical 与 compatibility 路由分类。
- 前后端契约对齐、字段语义收敛。

## 2. 非适用场景

- 仅内部实现优化、无对外契约变化（应使用 `runtime_feature_change.md`）。
- 仅做发布编排（应使用 `release_prep.md`）。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. `AGENT_ENTRYPOINT.md`

## 4. 禁止事项

- 禁止只改代码不改 OpenAPI。
- 禁止将 compatibility route 宣布为 canonical。
- 禁止引入与 authority 冲突的新口径。
- 禁止默认发布。

## 5. 最小实施原则

- 契约改动以“最小可用增量”提交，避免大规模重命名。
- 优先保持前向兼容；若需破坏性变更，明确迁移窗口与替代路径。
- 挂载路径、OpenAPI、策略文档三者必须一致。

## 6. 必跑测试命令

至少执行：

- `go build ./cmd/server`

建议执行：

- `go test ./transport/...`
- `go test ./service/...`

## 7. 文档更新要求

- 必须更新：`docs/api/openapi.yaml`
- 视影响更新：`docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`、`docs/API_USAGE_GUIDE.md`、相关领域指南
- 在输出中写明 canonical/compatibility 分类是否变化

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并额外补充：

- 契约变更清单（path/field/semantic）
- 兼容性影响说明
- 前端对接动作

## 9. 是否允许进入 release prep

- 默认不允许。
- 仅在用户明确授权后，按 `skills/release_prep.md` 执行。

## 10. 常见误区 / 历史包袱提醒

- 过去常见“路由挂了但 OpenAPI 没改”的漂移必须杜绝。
- 旧版 alignment 文档不可作为新契约依据。
- `transport/http.go` 决定实际挂载，冲突时以 runtime 为准并立即修文档。
