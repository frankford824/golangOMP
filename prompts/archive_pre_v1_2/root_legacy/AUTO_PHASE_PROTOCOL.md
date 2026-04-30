# AUTO_PHASE_PROTOCOL.md

Version: 1.0

Purpose: 让模型不再依赖人工编写 STEP_XX，而是根据仓库当前状态自动生成并执行下一阶段计划。

## 1. 目标
模型在每一轮都应：
1. 读取当前仓库状态
2. 自动判断“下一阶段最合理做什么”
3. 先生成阶段计划文件
4. 再执行本轮开发
5. 自动纠错
6. 自动更新项目状态、API 文档、迭代记录

本协议建立在以下文件之上：
- `AGENT_PROTOCOL.md`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/最新两份`

## 2. 自动阶段生成规则
模型必须按以下优先级判断下一阶段：

### Priority 1：阻塞主业务闭环的缺口
例如：
- 创建后无法分配
- 分配后无法提交设计
- 审核后无法入库
- 任务详情无法支撑前端主页面

### Priority 2：阻塞前端联调的缺口
例如：
- 聚合详情缺字段
- 列表筛选能力不足
- 返回结构不一致
- OpenAPI 与实现不一致

### Priority 3：契约失真 / 文档债
例如：
- `CURRENT_STATE.md` 过期
- `openapi.yaml` 与实现不一致
- iteration 记录失真
- ready/placeholder 标记错误

### Priority 4：基础设施占位
例如：
- ERP worker placeholder
- RBAC placeholder
- 文档收口
- handover appendix

### Priority 5：真正的基础设施接入
例如：
- NAS
- 真实上传
- whole_hash 严格校验
- WebSocket
- Verify worker

## 3. 自动阶段生成边界
模型自动生成的阶段，必须满足：

- 一轮只解决一个聚焦主题
- 不允许跨多个大主题发散
- 不允许重做已稳定阶段
- 不允许脱离 `CURRENT_STATE.md`
- 不允许绕过 `AGENT_PROTOCOL.md`

建议一轮范围控制在以下之一：
- 查询增强
- 单个业务子模块补齐
- 文档/契约收口
- 基础设施占位
- 单个前端阻塞问题修复

## 4. 自动阶段文件
模型在正式改代码前，必须先自动生成一个阶段计划文件，推荐路径：

- `docs/phases/PHASE_AUTO_XXX.md`

命名规则：
- 若最新 iteration 为 `ITERATION_005.md`
- 则下一轮可生成 `PHASE_AUTO_006.md`

## 5. 自动阶段文件必须包含
- Phase Name
- Why This Phase Now
- Current Context
- Goals
- Allowed Scope
- Forbidden Scope
- Expected File Changes
- Required API / DB Changes
- Success Criteria
- Required Document Updates
- Risks
- Completion Output Format

## 6. 自动执行流程
每轮必须按下面顺序执行：

1. Bootstrap：读取状态文件
2. 判断当前最合理下一阶段
3. 生成 `PHASE_AUTO_XXX.md`
4. 输出阶段计划摘要，供用户确认或直接执行
5. 按阶段计划改代码
6. 执行测试 / 编译
7. 自动纠错
8. 更新：
   - `CURRENT_STATE.md`
   - `docs/iterations/ITERATION_XXX.md`
   - `docs/api/openapi.yaml`
9. 输出本轮摘要与下一轮建议

## 7. 自动纠错与自动阶段的关系
如果模型在生成阶段计划或执行过程中发现：
- 当前状态文件与代码不一致
- OpenAPI 与实现不一致
- 旧阶段记录与当前实现冲突

则本轮阶段优先级可临时切换为：
**Correction / Reconciliation Phase**

即：
- 先修正文档和契约
- 再决定是否继续推进功能

## 8. 特殊规则：无显式 STEP 文件时
若仓库不存在当前轮次的 `STEP_XX.md`：
- 不应中断
- 不应要求用户手写补充
- 必须由模型依据本协议自动生成 phase 文件并继续

## 9. 输出要求
每轮完成时必须输出：
1. 自动生成的 phase plan 路径
2. 本轮改动文件列表
3. 数据表 / 迁移变更
4. API 变更
5. 自动纠错说明
6. 风险与未完成项
7. 下一轮最合理建议
