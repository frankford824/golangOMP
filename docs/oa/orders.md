# OA 订单查询接口

认证后 `GET /oa-api/orders/summary`、`/products`、`/rankings`、`/personal` 和 `/sku`。统一 `{data:{rows, freshness, stale}}`。日期参数 `from`、`to`（北京时间，日期皆包含，默认今天），可选 `shop_id`，`sku` 接受 `sku` 参数，`rankings` 可选 `group=shop|platform|product|person`。经营查询权限 `orders.view`，超级管理员查看全部；其他人员按工号连接已确认的人店名册。个人查询只看本人的子单归属。接口金额用十进制字符串，未知成本/毛利用 null。`freshness` 为 `{source_committed_at, lag_seconds}`。

- `summary.rows` 每日店铺指标 `{day,shop_id,platform,order_count,payable_amount,item_pay_total,cost_verified_total,item_pay_verified_total,verified_gross,verified_coverage}`。
- `products.rows` 按宝贝 SKU `{shop_id,shop_item_id,sku,product_name,item_pay_total,quantity,cost_verified_total,item_pay_verified_total,verified_gross,verified_coverage}`。
- `rankings.rows` `{key,item_pay_total}`。
- `personal.rows` `{day,item_pay_total}`。
- `sku.rows` 同商品汇总，另有 `tasks`、`feedback` 字段。

## OA 沙箱接线

`orders.EnsureSchema(ctx, db)` 在 OA 专用 MySQL 初始化 `oa_order_feedback`。主站只读副本所需表和列如下。

- `omp_sku_records`：`sku_code,cost_price`
- `tasks`：`id,task_no,task_status,sku_code,creator_id,designer_id,current_handler_id`
- `task_sku_items`：`task_id,sku_code`

`POST /oa-api/orders/feedback` 接收 `{shop_id,sku,note,adjusted_on,task_id?}`。日期格式 `YYYY-MM-DD`。任务可选且必须通过沙箱副本的本人关系或超级管理员检查。`GET /oa-api/orders/sku` 另外返回 `current_cost`、`cost_context_available`。`GET /oa-api/orders/cost-history` 返回每条历史成本。`GET /oa-api/orders/shops` 返回 `{shop_id,shop_name,platform}`。

`OA_CLICKHOUSE_URL`、`OPS_READER_PASSWORD` 由运行环境配置。固定账号 `ops_reader`，只读开放视图。复用连接，默认并发 8，建连失败重试一次。连接、超限、超时和拒绝访问各有明确提示。同账号、同范围和同筛选的查询缓存 30 秒，范围每次重新确认。范围不能确认时直接失败。已确认范围的查询遇到连接或超时失败时最多使用 5 分钟内的同范围快照，返回 `stale=true`，旧水位保留。拒绝访问清除缓存。

## 查询核验

从 OA 环境中运行。先用不同参数执行一次，再以同一参数执行一次核对热缓存。每批包含八个并发客户端，每个客户端一次请求。`OA_BEARER` 需为本次验证用账号令牌。不要把令牌写到日志。

```bash
base="${OA_BASE_URL%/}/oa-api/orders/summary"
for mode in cold hot; do
  seq 1 8 | xargs -P8 -I{} bash -c '
    curl -sS --max-time 15 -o /dev/null -w "%{http_code} %{time_total}\n" \
      -H "Authorization: Bearer $OA_BEARER" "$0?from=$OA_FROM&to=$OA_TO"
  ' "$base" | sort -k2,2n
 done
```

8 个样本时最近秩 P95 取最大值。保留每批 HTTP 状态与耗时。`summary` 关联 `v_shop_day` 与已核实成本非空的 `v_orders` 计数以区分零值和缺失。`products` 和 `cost-history` 使用 `v_order_lines`。个人和人员排行先从 `v_line_attribution` 按店铺和期间筛选并按人员及子单去重，再匹配真实有效订单商品行。店铺和平台排行只读 `v_shop_day`。完整 SQL 和使用列见 `queries.go`。ECS 实测 7 天全店查询，summary 0.3323 秒；未命中缓存的 8 并发同范围和不同起始日期 P95 分别为 2.1557 秒及 1.6823 秒。


## 明细、退款及任务

- `GET /oa-api/orders/detail` 使用稳定下单时间和订单 ID 倒序。分页采用 `offset`，每页 100 条，响应 `has_more`。
- `GET /oa-api/orders/refunds` 按确认售后申请日汇总，分开返回退款总额、有效订单退款、找不到原单的退款和异常退款。
- `GET /oa-api/orders/ledger` 为超级管理员列出镜像核算月份、关账状态以及明细金额。金额未知时为空。按月查询并用 `offset` 分页。
- `GET /oa-api/orders/rankings?group=growth` 比较紧邻的等长度基期，应付金额零基期以 `growth_status=zero_baseline` 表示。
- `POST /oa-api/orders/tasks` 接收 `{request_id,shop_id,sku,from,to,task}`，其中 `task` 采用现有 `CreateTaskRequest`。只转发到本机沙箱网关 `127.0.0.1:18087`。任务创建前保存来源、期间及指标快照。响应包含 `task_id`，遇到不确定结果时重用原始请求编号。
- `GET /oa-api/orders/compare` 接收 `shop_id,sku,adjusted_on,days`，以实际上线日为调整后的第一天，对比两段等长度完整自然日。结果展示观察到的销量变化。

ECS 复测优化后的店日查询，单次 7 天全店为 0.0929 秒。C8 同期 P95 0.2185 秒，变化起始日期 P95 0.2144 秒。每次实际读取 7,994 行，属于非缓存命中样本。最多对 32 个双零店日批量确认核实成本，更多双零时返回 `verified_status=zero_or_unverified`，明确说明汇总无法判定。数值型成本仍为空。此测量涵盖订单开放视图 SQL，OA 全栈业务接口耗时另外测量。


### QA 单价维护

`GET /oa-api/orders/tasks/{task_id}/skus/{sku}/cost` 回读沙箱原生 SKU 单价。使用返回的 `version` 发起 `PATCH`，提交 `{version,cost_price,reason}`。单价为固定两位小数字符串。超过范围的数字、缺失原因或过期版本会被拒绝。每次成功和 SKU 更新同事务记录人员、原价、新价及维护原因。ERP 状态始终返回 `not_checked`，历史订单成本由只读视图独立提供。

维护由超管或 `catalog.manage` 授权，后者仅可维护本人创建且属于当前组织范围的任务。ERP 权限单独使用原生接口。
