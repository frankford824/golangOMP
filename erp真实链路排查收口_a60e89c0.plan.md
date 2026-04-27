# ERP真实链路排查收口（a60e89c0）

## 并列修复项（新增）

### 修复项：创建任务 `product_selection` 误触发 existing_product 专属校验

- **问题现象**
  - 在 `new_product_development` / `purchase_task` 创建时，未真实传入 `product_selection`（或仅传空对象）也可能触发：
  - `product_selection is only supported when source_mode is existing_product`
- **影响范围**
  - 创建入口 `transport/handler/task.go`
  - 归一化逻辑 `service/task_product_selection.go::normalizeTaskProductSelection`
- **根因定位**
  - 历史判定链路对“字段存在”与“有效值存在”区分不充分；
  - 非 existing 场景中，对 `product_selection` 的拒绝条件过宽，空对象/无效对象也可能被当作有效输入；
  - 缺少按 `task_type` 的入口白名单防护，导致 original 相关路径对 new/purchase 产生误伤风险。

## 执行说明

1. **创建请求字段存在性判定增强**
   - 在 `createTaskReq` 中引入 `UnmarshalJSON`，记录：
     - `product_selection` 字段是否出现；
     - `product_selection` 是否为非 `null`。
   - 基于解析结果提供：
     - 原始字段存在判定（raw）；
     - 有效输入判定（非空对象且有实质字段）。

2. **入口校验收敛（最小可用）**
   - 新增按 `task_type` 白名单的创建入口校验：
     - `new_product_development` / `purchase_task`：拒绝“有效 product_selection”；
     - `original_product_development`：允许 product_selection 进入 existing_product 路径。
   - 非 `existing_product` 场景仅在“真实有效传入 product_selection”时拒绝，空对象/`null`/未传不拒绝。

3. **长期保护：按 task_type 限定跨类型字段**
   - 在 `service/task_service.go` 新增 `validateTaskTypeFieldWhitelist`：
     - `new_product_development` 拒绝 original/purchase 专属字段（如 `change_request`、`purchase_sku`、`product_channel`）；
     - `purchase_task` 拒绝 original/new 专属字段（如 `change_request`、`category_code`、`material_mode`、`design_requirement` 等）；
     - `original_product_development` 拒绝 new/purchase 专属字段（如 `category_code`、`material_mode`、`purchase_sku`、`product_channel` 等）。
   - 以 machine-readable `violations` 返回，防止“original 校验误伤 new / existing 路径误伤 purchase”的同类问题反复出现。

4. **服务层归一化防线**
   - 在 `normalizeTaskProductSelection` 中显式使用“有效 selection”判定变量，保证仅对有效输入触发 existing_product 专属错误。

5. **最小调试日志（脱敏）**
   - 创建入口新增/保留日志字段：
     - `trace_id`、`task_type`、`source_mode`、`product_id`、`sku_code`
     - `raw_has_product_selection`
     - `parsed_selection_nil_or_empty`
     - `branch`（命中校验分支名）

## 验证与验收口径

- Case1：`new_product_development` + 不传 `product_selection` + `product_id=null` + `sku_code` 有值，不触发 existing_product 报错。
- Case2：`purchase_task` + 不传 `product_selection`，不触发 existing_product 报错。
- Case3：`original_product_development` + 真实传入 `product_selection`，existing_product 路径仍可用。

建议命令：

- `go test ./transport/handler ./service`
- `go test ./service -run TestTaskServiceCreateRejectsOriginalOnlyFieldForNewProductDevelopment|TestTaskServiceCreateRejectsNewProductOnlyFieldForPurchaseTask`

通过标准：

- 上述 Case1/2/3 测试全部通过；
- `service` 与 `transport/handler` 相关回归测试通过；
- 日志可观测到分支命中（不包含敏感原文）。
