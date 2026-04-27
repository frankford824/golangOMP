# PHASE_PLAN_AUTO_TEMPLATE.md

## Phase Name
例如：PHASE_AUTO_006

## Why This Phase Now
说明为什么当前最合理先做这一轮。

## Current Context
- 当前 CURRENT_STATE:
- 当前 OpenAPI 版本:
- 最近 iteration:
- 当前已完成主干:
- 当前主要缺口:

## Goals
- 目标 1
- 目标 2
- 目标 3

## Allowed Scope
列出本轮允许修改的文件、模块、接口、表。

## Forbidden Scope
列出本轮禁止碰的稳定模块和暂不做内容。

## Expected File Changes
- 预计新增文件
- 预计修改文件
- 预计新增文档

## Required API / DB Changes
- 需要新增/修改的 API
- 需要新增/修改的 migration / 表
- 若本轮不改 DB，也要写明

## Success Criteria
- 编译/测试通过
- 本轮目标完成
- 文档同步完成
- API 契约同步完成

## Required Document Updates
必须更新：
- CURRENT_STATE.md
- docs/iterations/ITERATION_XXX.md
- docs/api/openapi.yaml

可选更新：
- CHANGELOG.md
- MODEL_HANDOVER.md
- docs/V7_API_READY.md
- docs/V7_FRONTEND_INTEGRATION_ORDER.md

## Risks
- 本轮风险 1
- 本轮风险 2

## Completion Output Format
1. 改动文件列表
2. 数据表/迁移变更
3. API 变更
4. 自动纠错说明
5. 风险与未完成项
6. 下一轮建议
