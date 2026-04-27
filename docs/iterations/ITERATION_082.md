# ITERATION_082 — ERP 商品主数据四层职责收口（2026-03-18）

## 目标
- 8081：OpenWeb 真实查询为主链；hybrid 仅在**可恢复上游故障**时回退本地 `products`。
- 8080：`products` 定位为同步副本 / 映射缓存 / 业务承接表；`ERP_SYNC_SOURCE_MODE=jst` 经 `JSTOpenWebProductProvider` upsert。
- 8082：`jst_inventory` 在本仓库无实现体；职责在文档与架构中固定为同步驻留/证据/对账层（非前端搜索主链）。
- 原品开发：`defer_local_product_binding` 允许 `product_id=null`；详情聚合补 `product` 读模型（ERP snapshot）。
- 全局品类：业务分类主语义 = **款式编码（i_id）**；走 `/v1/categories`（本地可配置映射层，当前含样例数据）或经 Bridge 的 `/v1/erp/categories`（8081 hybrid 下同样来自本地映射表，**不扫** `jst_inventory`）。

## 代码变更摘要
- `service/erp_bridge_remote_client.go`：hybrid 回退策略收紧（OpenWeb 业务错误、响应解析错误、鉴权模式错误、远程明确未命中 SKU **不回退**）；结构化日志 `erp_bridge_product_search` / `erp_bridge_product_by_id`（含 `trace_id`、`fallback_used`、`fallback_reason`）。
- `service/erp_bridge_service.go`：远程「未命中」映射 404；鉴权错误映射 400。
- `cmd/server/main.go`：8081 在 `remote`/`hybrid` 下强制 `ERP_REMOTE_BASE_URL` + `ERP_REMOTE_AUTH_MODE=openweb`。
- `domain/context_trace.go` + `transport/http.go`：请求上下文注入 `trace_id` 供 ERP 日志关联。
- `service/erp_sync_jst_provider.go`：`spec_json` 增强（`erp_product_id`/`sku_id`/sync_role）。
- `service/task_detail_service.go`：`product_id` 为空且 defer 绑定时，合成 `aggregate.product`（`status=erp_snapshot`）。

## 服务器验收
本轮在**无 OpenWeb 凭证的本地环境**仅执行 `go test ./...` 通过；对 `223.4.249.11` 的 A/B/C/D 四项需在部署本迭代二进制后按 `docs/ERP_REAL_LINK_VERIFICATION.md` 重跑并贴日志。

## 文档
- `docs/ERP_REAL_LINK_VERIFICATION.md`
- `CURRENT_STATE.md`、`MODEL_HANDOVER.md`、`ITERATION_INDEX.md`
- `docs/FRONTEND_ALIGNMENT_v0.5.md`、`docs/api/openapi.yaml`（描述层）
