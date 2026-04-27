# ITERATION_083 — 文档纠偏 + 真相源口径统一（2026-03-18）

## v0.8 验收通过（2026-03-18 后续）

A/B 组 live 实证已完成，文档口径可升级为「已实证」：
- 8081 `remote_ok`、`fallback_used=false` 已实证
- 8080 `ERP_SYNC_SOURCE_MODE=jst`、`JSTOpenWebProductProvider` 已实证
- products 从 20 增至 7470、HQT21413 样本刷新与副本标记已实证

## 目标
将《架构法医 / 真相源总审计报告》结论正式写回项目文档体系，纠正历史误导口径，统一商品主数据、分类语义、products、jst_inventory、8081 OpenWeb、original_product_development 的真相源叙事。

## 核心口径（权威，写入文档时必须以此为准）

1. **categories 表**：当前 31 行来自 migration + seed 的开发样例数据；是可配置映射骨架，**不是**生产真实分类中心。
2. **业务分类主语义**：**款式编码（i_id）**；聚水潭 `category` 字段只是 ERP 原始字段，不等于业务分类。
3. **商品搜索真相源**：8081 OpenWeb 主链；local/fallback 只是兜底；未经服务器实证不得写成“已完全切主链”。
4. **products**：副本 / 映射缓存 / 业务承接表；**不是**商品搜索唯一真相源，**不是**原品创建唯一硬前置。
5. **jst_inventory**：同步驻留原始层；**不是**前台商品搜索主表，**不是**原品开发创建主表。
6. **original_product_development**：允许 `defer_local_product_binding=true` 时 `product_id` 为空，基于 ERP snapshot 创建；本地 products 绑定可后置。
7. **状态分层**：Design Target / Code Implemented / Server Verified / Live Effective 必须严格区分。

## 修改文件列表
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`
- `docs/FRONTEND_ALIGNMENT_v0.5.md`
- `docs/ERP_REAL_LINK_VERIFICATION.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_082.md`
- `MODEL_v0.4_memory.md`
- `ITERATION_INDEX.md`
- 新增 `docs/TRUTH_SOURCE_ALIGNMENT.md`（真相源统一说明专章）

## 参考
- `docs/ARCHITECTURE_FORENSIC_AUDIT_REPORT.md`
