# Skill: review_first_change

## 1. 适用场景

- 几乎所有工程变更的默认入口（代码、契约、文档、配置）。
- 需求明确要求“先改、先测、先文档、先评审，再决定是否发布”。

## 2. 非适用场景

- 只做已授权发布动作（应使用 `skills/release_prep.md`）。
- 纯资料检索且不涉及任何改动。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`（需要确认 runtime 挂载时）
5. `AGENT_ENTRYPOINT.md`

## 4. 禁止事项

- 禁止跳过 authority 直接改动。
- 禁止把 archive/iterations/phases 当规范。
- 禁止默认进入发布流程。
- 禁止把 compatibility surface 扩大为新主链。

## 5. 最小实施原则

- 只改任务必需文件；优先复用现有服务与模式。
- 保持接口语义稳定；兼容改动应可被清楚追踪。
- 文档与运行时行为保持一致，不写“理想状态文档”。

## 6. 必跑测试命令

至少执行：

- `go build ./cmd/server`

按变更范围补充：

- `go test ./service/...`
- `go test ./transport/...`
- `go test ./repo/...`（若环境允许）

## 7. 文档更新要求

- 运行时或契约变更必须同步更新相关文档。
- 若涉及接口契约，优先更新 `docs/api/openapi.yaml` 与 Layer 2 指南。
- 说明本轮是否发布；默认“未发布”。

## 8. 标准输出格式要求

输出必须遵循 `templates/agent_output.md`，至少含：

- 根因/结论
- 代码修改
- 本地测试结果
- 文档更新结果
- 风险与未完成项

## 9. 是否允许进入 release prep

- 默认不允许。
- 仅在用户明确授权“允许发布/进入上线准备”时切到 `skills/release_prep.md`。

## 10. 常见误区 / 历史包袱提醒

- “代码改完就发布”是旧习惯，现流程不允许。
- `CURRENT_STATE.md` / `MODEL_HANDOVER.md` 仅索引，不是契约。
- archive 文档可看历史，不可指导新集成。
