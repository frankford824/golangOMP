# Skill: doc_cleanup_and_archive

## 1. 适用场景

- 文档层级治理、历史文档归档、obsolete 标记清理。
- 收敛 authority/current guide/index/archive 边界。

## 2. 非适用场景

- 运行时代码改造（改用相应代码 skill）。
- 发布执行（改用 `release_prep.md`）。

## 3. 必读 Authority Docs

1. `AGENT_ENTRYPOINT.md`
2. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
3. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
4. `docs/api/openapi.yaml`
5. `docs/archive/README.md`

## 4. 禁止事项

- 禁止把 archive 内容回灌为 current spec。
- 禁止删除有历史价值文档且不留归档路径。
- 禁止在文档中宣称未验证的 runtime 状态。

## 5. 最小实施原则

- 少量高价值治理动作优先，避免大规模重写。
- 以“降低误导风险”为第一目标，而不是“文档漂亮”。
- 归档时保留可追溯性（原文件名与归档原因）。

## 6. 必跑测试命令

文档治理默认无需大规模测试；若同时涉及 runtime 变更，至少执行：

- `go build ./cmd/server`

## 7. 文档更新要求

- 归档后更新 `docs/archive/README.md`。
- 明确 Layer 1/2/3/4 定义和示例。
- 对无法立即搬迁文档，添加强降级 banner。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并补充：

- 文档层级变更清单
- 已归档文件清单
- 仍保留但降级的文件清单

## 9. 是否允许进入 release prep

- 不允许。该 skill 仅用于治理与整理。

## 10. 常见误区 / 历史包袱提醒

- “保留在根目录但写一句 obsolete”通常不够，优先迁移到 archive。
- 只改索引不改内容层级，会导致下游继续误用旧文档。
