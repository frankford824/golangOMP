# 分类模块（Category / `src/services/api/categoriesApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **3 个在用接口**，无未使用接口。
>
> 统一说明：前端在 `useMaterialOptions` 中对分类接口做了**自动翻页累积**（`page_size=100`，最大翻 `MAX_PAGES`，到不足一页停止）。后端需保证稳定分页顺序。

---

## 3.K.1 分类列表

- **请求路径**：`/v1/categories`
- **请求方法**：`GET`
- **定义**：`categoriesApi.list`
- **调用**：`src/composables/useMaterialOptions.ts:115`（请求 `category_type='material' & is_active=true`）
- **Query (`CategoriesListParams`)**：`{ keyword?, category_type?, parent_id?, level?, is_active?, page?, page_size? }`

**期望返回**：

```json
{
  "data": [
    {
      "id": "number",
      "category_id": "number",
      "category_code": "string",
      "category_name": "string",
      "display_name": "string",
      "category_type": "string",
      "search_entry_code": "string",
      "is_search_entry": "boolean",
      "parent_id": "number",
      "level": "number",
      "is_active": "boolean",
      "sort_order": "number"
    }
  ],
  "pagination": { "total": "number", "page": "number", "page_size": "number" }
}
```

---

## 3.K.2 分类搜索（轻量下拉）

- **请求路径**：`/v1/categories/search`
- **请求方法**：`GET`
- **定义**：`categoriesApi.search`
- **调用**：`src/composables/useCategoryOptions.ts:24`，固定传 `{ is_active: true, limit: 100 }`
- **Query (`CategoriesSearchParams`)**：`{ keyword?, category_type?, is_active?, limit? }`
- **期望返回**：`{ data: Category[] }`，结构同 3.K.1 的 `data` item。

---

## 3.K.3 品类映射列表（材质选项）

- **请求路径**：`/v1/category-mappings`
- **请求方法**：`GET`
- **定义**：`categoriesApi.listMappings`
- **调用**：`useMaterialOptions.ts:134`，固定传 `{ secondary_condition_key: 'material', is_active: true }` + 翻页
- **Query (`CategoryMappingsListParams`)**：`{ keyword?, category_id?, category_code?, search_entry_code?, secondary_condition_key?, is_active?, is_primary?, page?, page_size? }`

**期望返回**：

```json
{
  "data": [
    {
      "id": "number",
      "category_id": "number",
      "category_code": "string",
      "search_entry_code": "string",
      "secondary_condition_key": "string (e.g. 'material')",
      "secondary_condition_value": "string (材质名称)",
      "tertiary_condition_key": "string",
      "tertiary_condition_value": "string",
      "is_active": "boolean",
      "is_primary": "boolean"
    }
  ],
  "pagination": { "total": "number" }
}
```

前端只消费 `secondary_condition_value` 作为材质下拉 label/value。
