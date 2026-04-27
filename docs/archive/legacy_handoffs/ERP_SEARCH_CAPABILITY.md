# ERP Search Capability

## Ready level
- Main flow usable: yes
- Frontend result-page integratable: yes
- Task binding ready: yes

## What the backend guarantees
- `GET /v1/erp/products` returns a stable result-page contract centered on `product_id`, `product_name`, and best-effort `sku_code/category_name/category_code/image_url/product_short_name`.
- `GET /v1/erp/products/{id}` accepts the `product_id` emitted by list and performs fallback lookup when upstream detail is inconsistent.
- `GET /v1/erp/categories` is the backend validation source for exact category filtering.

## What remains intentionally unsupported
- Global search
- Search suggestions / association
- Arbitrary multi-dimensional filter platform
- Sorting

## Frontend integration rule
- Treat `product_id` as an opaque lookup key, not as a raw ERP numeric id assumption.
- Treat empty `image_url` as "render placeholder".
- Treat empty `sku_code/category_name/category_code/product_short_name` as "field unavailable from upstream".
