# API Usage Guide

Authoritative v0.9 contract: `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`

## ERP product search

### Official endpoints

- `GET /v1/erp/products`
- `GET /v1/erp/products/{id}`
- `GET /v1/erp/categories`

### Compatibility endpoints

- `GET /v1/products/search`
- `GET /v1/products/{id}`

New code should use `/v1/erp/products*`. The `/v1/products*` routes remain local-cache compatibility reads only and now emit compatibility headers.

### Current capability boundary

- Keyword search: supported via `q` on `/v1/erp/products`
- Fuzzy or semantic search: not supported beyond current bridge/backend matching
- Exact SKU filter: supported when exposed by the bridge contract
- Category browse/filter: supported through `/v1/erp/categories` plus `/v1/erp/products`
- Sorting and suggestion engines: not supported as separate contracts

### Result contract

- `product_id`: stable lookup key returned by list and accepted by detail
- `sku_code`: exact SKU when available; otherwise upstream/code-like fallback or empty string
- `product_name`: display name
- `category_name`: normalized category label when resolvable; otherwise empty string
- `category_code`: normalized category code when resolvable; otherwise empty string
- `product_short_name`: optional
- `image_url`: optional

Backend returns empty strings for unavailable string fields. Frontend should not infer hidden semantics from an empty string.

## Task binding

### Existing-product flow

1. Call `GET /v1/erp/products`.
2. Pick a result row.
3. Optionally call `GET /v1/erp/products/{product_id}`.
4. Submit the selected ERP row through `product_selection.erp_product` when creating or rebinding an `existing_product` task.

### Compatibility note

- `product_id` on `POST /v1/tasks` still accepts:
  - local numeric `products.id`
  - string ERP facade `product_id`
- For new clients, `product_selection.erp_product` is the clearer contract.

## Task-create references

### Official flow

1. `POST /v1/tasks/reference-upload`
2. receive one normalized `reference_file_ref` object
3. append that object into `POST /v1/tasks.reference_file_refs`

### Compatibility flow

- `POST /v1/task-create/asset-center/upload-sessions`
- `POST /v1/task-create/asset-center/upload-sessions/{session_id}/complete`

This older flow remains callable, but it is no longer the official task-create upload entry.

### Rejected create input

- `reference_images` is no longer accepted on `POST /v1/tasks`
- backend returns `400 INVALID_REQUEST`

## Task ownership

- Canonical task org fields: `owner_department`, `owner_org_team`
- Compatibility task field: `owner_team`

Current create behavior is intentionally conservative:

- backend still accepts `owner_team`
- backend may normalize configured org-team aliases into legacy `owner_team`
- backend persists canonical fields when it can determine them safely

Current read behavior still returns all three fields. New frontend logic should not treat `owner_team` as the only ownership truth.

## Product code generation

- Official preview path: `POST /v1/tasks/prepare-product-codes`
- Official create behavior: backend default allocator inside `POST /v1/tasks`
- Deprecated for task product-code configuration:
  - `GET /v1/rule-templates/product-code`
  - `PUT /v1/rule-templates/product-code`

`/v1/code-rules*` remains a separate utility module. It is not the task-create authority for default task product-code generation.
