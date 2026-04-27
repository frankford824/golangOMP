# Skill: runtime_feature_change

## 1. 适用场景

- 业务逻辑、流程判断、校验、默认值、聚合逻辑变更。
- 不显式新增外部 API，但可能影响运行时行为。

## 2. 非适用场景

- 对外契约（path/schema）变更（应使用 `backend_contract_change.md`）。
- 仅文档归档治理（应使用 `doc_cleanup_and_archive.md`）。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `AGENT_ENTRYPOINT.md`

## 4. 禁止事项

- 禁止在无必要时新增抽象层或新包。
- 禁止扩大兼容面，保持兼容面收敛。
- 禁止只改实现不更新行为文档。
- 禁止默认发布。

## 5. 最小实施原则

- Prefer reuse over abstraction。
- 优先在已有 service/repo/handler 内做局部改动。
- 写清“为什么改”，避免只堆实现细节。

## 6. 必跑测试命令

至少执行：

- `go build ./cmd/server`

按改动位置执行：

- `go test ./service/...`
- `go test ./transport/...`
- `go test ./domain/...`

## 7. 文档更新要求

- 若行为变化影响对接方，更新相应 Layer 2 指南。
- 若行为变化可观察到响应语义，补充 OpenAPI 描述或示例。
- 输出中注明“行为变化但契约不变”或“行为与契约同时变化”。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并明确：

- 触发条件
- 关键分支行为
- 回归风险点

## 9. 是否允许进入 release prep

- 默认不允许。
- 需要用户显式授权。

## 10. 常见误区 / 历史包袱提醒

- “只改逻辑不用改文档”会导致后续排障成本极高。
- 旧迭代文档中的临时策略不得复活为当前默认行为。
