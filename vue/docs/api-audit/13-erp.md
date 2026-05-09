# ERP 数据（`src/services/api/erpApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **1 个在用接口**；2 个未使用接口（`getProduct`、`getCategories`）见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

---

## 3.M.1 ERP 商品列表（选品）

- **请求路径**：`/v1/erp/products`
- **请求方法**：`GET`
- **定义**：`erpApi.getProducts`
- **调用**：`src/stores/products.ts:56` (`loadProducts`)
- **Query (`ErpProductsParams`)**：`{ keyword?, sku_code?, category?, page?, page_size? }`

**期望返回**（反推自 `normalizeErpProduct` 及 `stores/products.ts:56-67`）：

```json
{
  "data": {
    "items": [
      {
        "product_id": "string | number",
        "sku_code": "string",
        "product_name": "string",
        "category": "string",
        "category_code": "string",
        "price": "number",
        "brand": "string"
      }
    ],
    "total": "number",
    "page": "number",
    "page_size": "number"
  }
}
```

前端兼容：`body.items` / `body.products` / 裸数组；`total` / `page` / `page_size` 缺省时以数组长度兜底。
