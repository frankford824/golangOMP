# PHASE_EXECUTION_TEMPLATE.md

## Phase Name
填写阶段名称，例如：STEP_05 / Query Enhancement

## Goal
简述本轮目标。

## Inputs
开始前必须读取：
- CURRENT_STATE.md
- MODEL_HANDOVER.md
- docs/api/openapi.yaml
- docs/iterations/最新两份
- 当前阶段任务文件

## Allowed Scope
列出本轮允许修改的模块、目录、接口、表。

## Forbidden Scope
列出本轮禁止触碰的模块、已稳定能力、暂不做内容。

## Required Code Changes
- 新增/修改的领域对象
- 新增/修改的 repo / service / handler
- 新增/修改的 migration
- 新增/修改的接口

## Required Doc Changes
必须更新：
- CURRENT_STATE.md
- docs/iterations/ITERATION_XXX.md
- docs/api/openapi.yaml

可选更新：
- CHANGELOG.md
- MODEL_HANDOVER.md
- docs/V7_API_READY.md
- docs/V7_FRONTEND_INTEGRATION_ORDER.md

## Success Criteria
- 编译或测试通过
- API 契约与实现一致
- 本轮目标完成
- 输出摘要完整

## Completion Output Format
1. 改动文件列表
2. 数据表/迁移变更
3. API 变更
4. 风险与未完成项
5. 下一轮建议

## Auto-Correction Checklist
- [ ] OpenAPI 是否与代码一致
- [ ] CURRENT_STATE 是否指向最新阶段
- [ ] ITERATION 是否准确记录本轮
- [ ] 是否存在重复实现已有能力
- [ ] 是否把占位接口误写为 ready
- [ ] 是否出现阶段目标外改动
