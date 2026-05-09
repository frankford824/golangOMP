# 规则模板（Rule / `src/services/api/rulesApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **2 个在用接口**；1 个未使用接口（`getByType`）见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

---

## 3.L.1 获取所有规则模板

- **请求路径**：`/v1/rule-templates`
- **请求方法**：`GET`
- **定义**：`rulesApi.list`
- **调用**：`src/stores/rules.ts:90` (`loadRules`)
- **Query**：前端会在保存后调用 `list(signal, true)`，此时追加 `?_t=<Date.now()>` 作为缓存穿透参数（非业务字段，后端忽略即可）。

**期望返回**：前端同时兼容数组 / 对象两种形态：

形态 A（推荐，数组）：

```json
{
  "data": [
    { "type": "cost-pricing", "template_type": "cost-pricing", "...其余字段": "..." },
    { "type": "product-code", "prefix": "string", "location_code": "string", "biz_type_code": "string", "sequence_digits": "number", "date_format": "yyyyMMdd", "enabled": "boolean" },
    { "type": "short-name", "...": "..." }
  ]
}
```

形态 B（对象 map）：

```json
{
  "data": {
    "cost_pricing": { "...": "..." },
    "product_code": { "...": "..." },
    "short_name": { "...": "..." }
  }
}
```

前端将 `cost_pricing` / `cost-pricing`、`product_code` / `product-code`、`short_name` / `short-name` 双形态 key 兼容。

---

## 3.L.2 按 type 更新模板

- **请求路径**：`/v1/rule-templates/{type}`（`type ∈ cost-pricing | product-code | short-name`）
- **请求方法**：`PUT`
- **定义**：`rulesApi.updateByType`
- **调用**：`src/stores/rules.ts:156` (`saveRule`)
- **Body**：`Record<string, unknown>`，由 `codeRuleToTemplatePayload` 产出；典型字段：`prefix, location_code, biz_type_code, sequence_digits, date_format, enabled, name, ...`。
- **期望返回**：`{ data: RuleTemplate }` 或 2xx；前端保存后做 `optimisticUpdateRule`，**不再重拉 list**（规避缓存覆盖），因此后端应允许该 PUT 返回 204 或回显最新对象均可。
