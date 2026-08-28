# ERP 成本接口快速接入

这套接口供同公司内部系统只读调用。调用方只需要：

1. 一个固定服务地址；
2. 一个固定 Token；
3. 在每次请求中带同一个请求头。

不需要登录用户、OAuth、动态签名或权限申请。

## 连接信息

公司 Tailscale 网络内推荐地址：

```text
http://100.125.196.22:8081
```

固定请求头：

```text
X-ERP-Bridge-Cost-Token: <双方约定的固定Token>
```

也可以使用标准 Bearer 写法：

```text
Authorization: Bearer <双方约定的固定Token>
```

以下示例统一使用：

```bash
BASE_URL="http://100.125.196.22:8081"
TOKEN="由接口提供方单独发送的固定Token"
```

## 1. 批量查询当前成本

日常补漏和抽查优先使用这个接口，最多一次查询 2000 个 SKU。

```bash
curl -sS "$BASE_URL/api/cost/batch-query" \
  -H "X-ERP-Bridge-Cost-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sku_ids":["SKU-001","SKU-002","组合装编码"]}'
```

返回：

```json
{
  "data": [
    {
      "sku_id": "SKU-001",
      "sku_type": "Normal",
      "cost_price": "5.3619",
      "sale_price": "12.00",
      "modified_at": "2026-08-28T13:38:38+08:00"
    }
  ],
  "missing_sku_ids": ["SKU-002"],
  "watermark": "2026-08-28T13:38:38+08:00",
  "snapshot_version": "..."
}
```

`cost_price` 固定保留四位小数。`sku_type` 可能为空，调用方不要自行猜测类型。

## 2. 增量同步当前成本

首次同步可以不传 `updated_since`：

```bash
curl -sS "$BASE_URL/api/cost/skus?limit=1000" \
  -H "X-ERP-Bridge-Cost-Token: $TOKEN"
```

如果返回 `next_cursor`，原样带回即可：

```bash
curl -sS --get "$BASE_URL/api/cost/skus" \
  -H "X-ERP-Bridge-Cost-Token: $TOKEN" \
  --data-urlencode "cursor=<上次返回的next_cursor>" \
  --data-urlencode "limit=1000"
```

循环到没有 `next_cursor` 为止，然后保存最后一页的 `watermark`。下次同步把它作为 `updated_since`：

```bash
curl -sS --get "$BASE_URL/api/cost/skus" \
  -H "X-ERP-Bridge-Cost-Token: $TOKEN" \
  --data-urlencode "updated_since=2026-08-28T13:38:38+08:00" \
  --data-urlencode "limit=1000"
```

调用方只需原样保存 `cursor`、`watermark` 和 `snapshot_version`，不需要解析或生成 cursor。

## 3. 查询指定日期的历史成本

```bash
curl -sS --get "$BASE_URL/api/cost/history" \
  -H "X-ERP-Bridge-Cost-Token: $TOKEN" \
  --data-urlencode "sku_ids=SKU-001,SKU-002" \
  --data-urlencode "as_of=2026-08-01"
```

如果公司存在多个仓储方，可额外传：

```text
wms_co_ids=1001,1002
```

返回结果会保留 `wms_co_id`，不会在多个仓储方之间静默选一条。

## 4. 查询成本变动

```bash
curl -sS --get "$BASE_URL/api/cost/changes" \
  -H "X-ERP-Bridge-Cost-Token: $TOKEN" \
  --data-urlencode "since=2026-08-28T00:00:00+08:00" \
  --data-urlencode "limit=1000"
```

返回每次真实成本变化的旧值和新值：

```json
{
  "data": [
    {
      "id": 18,
      "sku_id": "SKU-001",
      "sku_type": "Normal",
      "old_cost_price": "5.0000",
      "new_cost_price": "5.3619",
      "modified_at": "2026-08-28T13:38:38+08:00",
      "changed_at": "2026-08-28T13:39:02.123+08:00"
    }
  ],
  "watermark": 20,
  "snapshot_version": "..."
}
```

若返回 `next_cursor`，处理方式与增量成本接口相同：原样回传，直到没有 `next_cursor`。

注意：成本变动流水从服务上线并应用迁移后开始记录，不伪造上线前的旧值。

## 错误处理

- `400`：参数格式错误或 SKU 数量超过限制；修正请求后重试。
- `401`：Token 缺失或不正确。
- `502`：聚水潭历史成本接口暂时不可用；稍后重试。
- 批查中部分 SKU 不存在时仍返回 `200`，不存在的编码放在 `missing_sku_ids`。

## 最简单的接入建议

- 每天或每小时调用一次 `/api/cost/skus` 做增量同步；
- 对缺失 SKU 使用 `/api/cost/batch-query` 补查；
- 计算历史订单时调用 `/api/cost/history`；
- 定时调用 `/api/cost/changes` 判断哪些历史订单需要重新核对。
