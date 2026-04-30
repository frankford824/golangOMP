# AGENT_PROTOCOL.md
Version: 1.0
Purpose: 将当前仓库从“对话驱动开发”升级为“基于仓库状态的持续代理执行（Agentic Repo Loop）”。

## 1. 适用范围
本协议适用于所有参与该仓库开发的模型/智能体，包括但不限于：
- Claude
- Codex
- Cursor
- 其他具备代码执行与文档更新能力的模型

本协议必须兼容既有文档体系：
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_XXX.md`
- `CHANGELOG.md`
- 既有 `STEP_XX.md` / 迁移执行包 / 规格文档

## 2. 核心目标
每个模型在每轮执行中都必须做到：
1. 先读取当前项目状态，而不是按默认习惯重做。
2. 只在允许范围内推进当前阶段。
3. 代码、接口文档、项目状态、迭代记录必须同步更新。
4. 每轮执行必须自动留痕。
5. 若发现偏差、契约错误、文档失真，必须先纠错再继续。
6. 换模型时，后续模型能仅靠仓库文档接上当前进度。

## 3. Repository Truth Hierarchy
仓库内信息优先级如下：
1. **CURRENT_STATE.md**
2. **docs/api/openapi.yaml**
3. **最新两份 docs/iterations/ITERATION_XXX.md**
4. **MODEL_HANDOVER.md**
5. **当前阶段任务文件**
6. **历史规格文档**

若上述文件之间冲突，优先按 1 > 2 > 3 > 4 > 5 > 6 判定。
若冲突无法自动解决，必须在本轮迭代记录中写明。

## 4. 每轮执行前的固定动作（Bootstrap Protocol）
任何模型开始执行前，必须先读取并总结：
1. `CURRENT_STATE.md`
2. `MODEL_HANDOVER.md`
3. `docs/api/openapi.yaml`
4. `docs/iterations/` 最新两份
5. 当前阶段任务文件（如 `STEP_05.md` 或 `PHASE_PLAN.md`）

开始前必须先输出：
- 当前项目目标
- 当前开发进度
- 已完成模块
- 未完成模块
- 当前 API 状态
- 本轮计划范围
- 预计影响文件

在这些信息确认前，不得直接改代码。

## 5. 阶段执行合同（Phase Contract）
每一轮必须有明确阶段合同，至少包含：
- Phase Name
- 本轮目标
- 可修改文件范围
- 不可触碰范围
- 需要新增/修改的表
- 需要新增/修改的接口
- 成功标准
- 必须更新的文档
- 输出格式

## 6. Completion Protocol（本轮结束必须做的事）
每轮代码改动完成后，必须自动完成以下动作：

### 6.1 代码侧
- 编译 / 测试通过（至少 `go test ./...` 或 `go build ./...`）
- 输出改动文件列表
- 输出数据表 / 迁移变更
- 输出 API 变更
- 输出已知风险与未完成项

### 6.2 文档侧
必须同步更新：
1. `CURRENT_STATE.md`
2. `docs/iterations/ITERATION_XXX.md`
3. `docs/api/openapi.yaml`

如有必要，还应更新：
- `CHANGELOG.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

### 6.3 输出侧
每轮输出必须包含：
1. 改动文件列表
2. 数据表 / 迁移变更
3. API 变更
4. 风险与未完成项
5. 下一轮建议

## 7. 自动纠错协议（Auto-Correction Protocol）
模型必须具备“发现问题后先纠错再推进”的行为，而不是机械往下写。

### 7.1 需要自动纠错的情况
- OpenAPI 文档与代码实现不一致
- CURRENT_STATE.md 仍写着已完成但代码未实现
- iteration 记录与实际改动不一致
- 返回结构与文档描述不一致
- 新阶段任务与旧阶段已实现内容冲突
- 误把占位接口写入 ready-for-frontend
- 重复新增已存在能力
- 状态流转与当前枚举/服务逻辑冲突

### 7.2 自动纠错顺序
1. 停止新增业务
2. 识别冲突点
3. 修正代码或文档
4. 在当前 `ITERATION_XXX.md` 中新增“Correction Notes”
5. 更新 `CURRENT_STATE.md`
6. 如影响前端契约，更新 `docs/api/openapi.yaml`

### 7.3 纠错原则
- 优先修正文档失真
- 若代码和 OpenAPI 不一致，以“已通过测试的实际实现”为准更新 OpenAPI
- 若代码实现明显错误但文档和阶段合同一致，则修代码
- 不允许带着已知契约漂移继续新增功能

## 8. 自动更新 API 文档规则（API Doc Sync Protocol）
凡是新增、删除、重命名、修改以下任一内容，都必须同步更新 `docs/api/openapi.yaml`：
- 路径
- query 参数
- request body
- response schema
- error schema
- 分页结构
- 版本号
- ready/placeholder 标记

### 版本号规则
- 新增接口：minor 递增，如 `0.5.0 -> 0.6.0`
- 仅修正文档 / 小修补：patch 递增，如 `0.6.0 -> 0.6.1`
- 重大兼容性变更：需在 `MODEL_HANDOVER.md` 和 iteration 中说明

### API Source of Truth
前端对接只认：
- `docs/api/openapi.yaml`
任何聊天描述都不能替代此文件。

## 9. 自动更新项目状态文档规则（Project State Sync Protocol）
每轮结束必须更新 `CURRENT_STATE.md`，最少包括：
- Current Phase
- Completed
- In Progress（若有）
- Next Step
- Latest Iteration
- API Source of Truth
- Ready for Frontend 的接口范围
- Known Gaps

不允许出现：
- 当前轮已完成内容仍被写成 pending
- OpenAPI 版本号与 CURRENT_STATE 不一致
- Latest Iteration 未指向最新文件

## 10. 迭代记录规则（Iteration Memory Protocol）
每轮必须新增：
- `docs/iterations/ITERATION_XXX.md`

文件内容至少包括：
- Phase / Round
- 本轮目标
- 输入前提
- 改动文件
- 数据表 / 迁移
- API 变更
- 设计决策
- Correction Notes（如有）
- 风险与未完成项
- 下一轮建议

## 11. 跨模型移交协议（Model Handover Protocol）
当开发从一个模型切换到另一个模型时，新模型第一轮必须：
1. 阅读 Bootstrap Protocol 指定文件
2. 总结当前进度
3. 列出已完成/未完成
4. 指出当前 API 状态
5. 仅在确认无误后继续开发

推荐开场语：
“这是一个持续迭代中的代码库。先阅读 CURRENT_STATE.md、MODEL_HANDOVER.md、docs/api/openapi.yaml、docs/iterations/最新两份，以及当前阶段任务文件，再继续开发。不要按你的默认习惯重做，必须沿用现有 V7 Task 化语义和当前仓库进度。”

## 12. 禁止事项
模型不得：
- 脱离 `CURRENT_STATE.md` 自行重做
- 只改代码不改 OpenAPI
- 只改文档不改代码
- 省略 iteration 记录
- 将占位接口标记为 ready-for-frontend
- 在存在契约失真时继续扩需求
- 破坏 V6 兼容性
- 擅自修改已稳定阶段的业务语义

## 13. Success Definition
当一个阶段被视为完成时，必须同时满足：
- 代码通过编译/测试
- OpenAPI 已同步
- CURRENT_STATE 已同步
- ITERATION 已落地
- 输出摘要完整
- 下一轮建议明确
- 可被其他模型直接接手
