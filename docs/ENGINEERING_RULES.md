# ENGINEERING_RULES

Last updated: 2026-04-13

本文件定义全工程统一工程规则，适用于所有 agent 与开发者。

## 1. 简洁性原则

- prefer reuse over abstraction
- 不在“未来可能需要”时提前建设平台化能力
- 非必要不新建 package；优先在现有 service 内承载逻辑
- 变更以最小可用增量为先，避免一次性大重构

## 2. 文档原则

- runtime truth first
- source-of-truth first
- archive is not spec
- docs must match runtime

层级规则：

- Layer 1 authority：`docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`、`docs/api/openapi.yaml`、`transport/http.go`
- Layer 1 index：`docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
- Layer 2 current guides：仅做执行指导，不覆盖 Layer 1
- Layer 3 index-only：`CURRENT_STATE.md`、`MODEL_HANDOVER.md`
- Layer 4 archive：`docs/archive/**`、`docs/iterations/**`、`docs/phases/**`

## 3. 开发原则

- review-first
- local validation before release prep
- no release unless explicitly requested
- no overdesign
- keep compatibility surface shrinking, not growing

默认流程：

1. 读取 `AGENT_ENTRYPOINT.md`
2. 读取 authority 链
3. 选择 skill
4. 最小改动实现
5. 本地验证
6. 文档同步
7. 模板化输出
8. 人工评审
9. 明确授权后才可进入 release prep

## 4. Agent 原则

- read manifest first
- follow skill file
- use output template
- do not invent new process every time

发布控制：

- 未出现“允许发布/进入上线准备”明确指令时，一律不得执行发布动作。
- 发布准备必须显式使用 `skills/release_prep.md`。
