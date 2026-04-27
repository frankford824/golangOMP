# AGENT_ENTRYPOINT

Last updated: 2026-04-13

本文件是全工程统一的 agent 工作入口。适用于 Codex、Claude Code、以及其他模型协作。

## 0) Commander Prompt

使用 `COMMANDER_PROMPT.md` 作为面向 Codex/Claude 的系统提示入口；本文件是其 Step 1 的技术启动协议文档。

## 1) 启动顺序（强制）

任何任务开始前，必须按以下顺序阅读：

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`（当需要确认运行时挂载时）
5. 对应 `skills/*.md`

未完成以上步骤，不得开始改代码或改契约。

## 2) Authority Hierarchy

### Layer 1: Authority

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

### Layer 1 Entry Index

- `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`

### Layer 2: Current-use guides（不覆盖 Layer 1）

- `docs/TASK_CREATE_RULES.md`
- `docs/API_USAGE_GUIDE.md`
- `docs/ASSET_UPLOAD_INTEGRATION.md`
- `docs/ASSET_ACCESS_POLICY.md`
- `docs/ASSET_STORAGE_AND_FLOW_RULES.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/FRONTEND_MAIN_FLOW_CHECKLIST.md`
- `docs/ERP_SEARCH_CAPABILITY.md`
- `docs/TRUTH_SOURCE_ALIGNMENT.md`
- `docs/FRONTEND_DIST_PUBLISH_SOP.md`
- `docs/THREE_ENDPOINT_CONTROL_PLANE.md`
- `docs/ops/NAS_SSH_ACCESS.md`

### Layer 3: Index-only

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`

### Layer 4: Archive / Obsolete

- `docs/archive/**`
- `docs/iterations/**`
- `docs/phases/**`

## 3) 禁止事项

- 不允许把 archive 文档当作当前 spec。
- 不允许把旧 iteration / phase 文档当作当前合同。
- 不允许把 compatibility route 当 canonical mainline。
- 不允许跳过 source-of-truth 直接改代码。
- 不允许默认进入发布流程。

## 4) 默认工作方式

- review-first
- minimal change
- reuse existing code
- avoid overdesign
- runtime truth first
- docs must match runtime changes

默认流程：

1. 读取入口与 authority
2. 选择并读取 skill
3. 实施最小变更
4. 本地验证
5. 更新文档
6. 按模板输出
7. 进入人工 review
8. 仅在明确授权时进入 release prep

## 5) Skill 使用方式

技能目录：`skills/`

最低用法：

- 指定入口：`AGENT_ENTRYPOINT.md`
- 指定 skill：例如 `skills/backend_contract_change.md`
- 指定任务范围：例如修改 `/v1/tasks` 某契约
- 指定发布策略：默认 `review-first, no release`

## 6) 标准输出模板

所有任务结项输出优先遵循：

- `templates/agent_output.md`

如任务较小，可简化字段；但必须覆盖：

- 结论
- 代码修改
- 本地验证
- 文档更新
- 风险/未完成项

## 7) Release 触发条件

只有在明确指令出现以下语义时，才允许进入发布准备：

- “允许发布”
- “进入上线准备”
- “开始 release prep”

否则一律停留在 review-first 阶段。
