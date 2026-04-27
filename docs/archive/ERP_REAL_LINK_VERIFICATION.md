# ERP 真实链路验收（OpenWeb + products 副本 + 品类）

在 **Bridge(8081)** 配置 `ERP_REMOTE_MODE=remote` 或 `hybrid`、`ERP_REMOTE_AUTH_MODE=openweb`、`ERP_REMOTE_BASE_URL`、`ERP_REMOTE_SKU_QUERY_PATH`（默认 `/open/sku/query`）、AppKey/AppSecret/AccessToken 后执行。

**live 真相源收口**：使用 `scripts/live_truth_source_verify.sh` 在服务器上执行完整验收，报告模板见 `docs/LIVE_TRUTH_SOURCE_VERIFICATION.md`。

## v0.8 验收通过（2026-03-18）

A/B 组已通过 live 实证：
- **A 组 8081 OpenWeb**：`erp_bridge_product_search result=remote_ok fallback_used=false`；`remote_erp_openweb_request_started/completed` 日志可见。
- **B 组 JST sync**：`erp_sync_run_finish status=success source_mode=jst total_received=7451 total_upserted=7451`；products 从 20 增至 7470；HQT21413 样本 `spec_json` 含 `sync_role=8080_products_replica_from_openweb`。

## A — 8081 OpenWeb 商品查询
```bash
# 搜索（MAIN 转发时换 8080 + Token）
curl -sS "http://127.0.0.1:8081/v1/erp/products?q=HQT21413&page=1&page_size=20" -H "Authorization: Bearer $TOKEN"

# 详情
curl -sS "http://127.0.0.1:8081/v1/erp/products/HQT21413" -H "Authorization: Bearer $TOKEN"
```
**日志**：`erp_bridge_product_search` / `erp_bridge_product_by_id`  
- `result=remote_ok`，`fallback_used=false`：主链命中 OpenWeb。  
- `result=fallback_local_products`：仅 hybrid 且上游超时/5xx 等可回退错误时出现。

## B — 8080 products 同步（JST Provider）
```bash
# MAIN
curl -sS -X POST "http://127.0.0.1:8080/v1/products/sync/run" -H "Authorization: Bearer $TOKEN"
```
前置：`ERP_SYNC_SOURCE_MODE=jst`、与 Bridge 一致的 OpenWeb 凭据。  
抽样比对：`products.spec_json` 中含 `sync_role=8080_products_replica_from_openweb`；与 8082 `jst_inventory` 为**不同表职责**（见 CURRENT_STATE 四层定义）。

## C — 原品开发 defer_local_product_binding
- `defer_local_product_binding=false`：应完成 `EnsureLocalProduct`，`tasks.product_id` 非空（在本地 products 可解析时）。  
- `defer_local_product_binding=true`：`product_id` 可为 null，需 `erp_product` 含 `product_id`/`sku_id`/`sku_code` 之一 + 可展示名称；`GET /v1/tasks/{id}/detail` 中 `product.status=erp_snapshot` 且 `spec_json.deferred_local_product_binding=true`。

## D — 全局品类（禁止扫 jst_inventory）
- **业务分类主语义**：款式编码（`i_id`）；聚水潭 `category` 为 ERP 原始字段，不等于业务分类。
- 主推荐：`GET /v1/categories` 或 `GET /v1/categories/search`（本地可配置映射层，当前含 31 行样例，非生产真实分类库）。  
- 辅助：`GET /v1/erp/categories`（Bridge 侧来自本地映射，非库存大表）。  
- **不要**让前端以「拉全表 SKU/库存」接口充当品类树数据源。
