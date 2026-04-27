# v0.8 后下一阶段路线图

> 基线：**v0.8 = 商品主数据 live 真相源切换完成版**（2026-03-18 已实证）
>
> 本文档定义 v0.8 之后的开发优先级与验收检查项。

---

## 第一优先级：原品开发 / 商品 / 成本联调闭环

围绕已打通的主链，收真实联调闭环。

### 关注点

| 领域 | 检查项 |
|------|--------|
| **original_product_development** | defer / 非 defer 两条路径的创建与详情；`product_id=null` 归一；`product.status=erp_snapshot` |
| **product-info / cost-info** | `GET/PATCH /v1/tasks/{id}/product-info`、`GET/PATCH /v1/tasks/{id}/cost-info`、`POST /v1/tasks/{id}/cost-quote/preview` 读写与校验 |
| **filing / upsert** | 业务信息填报后到 Bridge `POST /v1/erp/products/upsert` 的链路；`filed_at` 触发 |
| **前端详情与读模型** | detail 与 `product_selection`、`product`、`cost` 字段一致性 |

### 参考文档

- `docs/FRONTEND_ALIGNMENT_v0.5.md`（原品开发创建、ERP 绑定归一）
- `docs/TRUTH_SOURCE_ALIGNMENT.md`（defer 语义）
- `docs/ERP_REAL_LINK_VERIFICATION.md`（C 组 defer_local_product_binding）

---

## 第二优先级：设计资产中心闭环

### 关注点

| 领域 | 检查项 |
|------|--------|
| **download / version-download** | `GET /v1/tasks/{id}/assets/{asset_id}/download`、`GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download`；权限与受控访问 |
| **delivery 推审** | delivery complete 后自动推进 `PendingAuditA`（已恢复，需持续验证） |
| **真实上传下载联调** | 小文件、multipart、NAS 全链路；reference / source / delivery 三类资产 |

### 参考文档

- `docs/ASSET_ACCESS_POLICY.md`
- `docs/FRONTEND_ALIGNMENT_v0.5.md`（资产中心上传主链）

---

## 第三优先级：版本口径统一

将仍挂着 `v0.5` 命名的对齐文档和描述逐步收成 v0.8 或统一版本号。

### 待收口项

| 文件/位置 | 当前 | 目标 |
|-----------|------|------|
| `docs/FRONTEND_ALIGNMENT_v0.5.md` | 文件名 v0.5 | 可重命名为 `FRONTEND_ALIGNMENT.md` 或注明「v0.8 联调基线」 |
| CURRENT_STATE / MODEL_HANDOVER | 多处 v0.5/v0.6 历史 | 明确「当前基线 = v0.8」 |
| 其他引用 v0.5 的文档 | 散落 | 统一为 v0.8 或「当前版本」 |

---

## 验收标准（DoD）

- **第一优先级完成**：原品 defer/非 defer 创建 + product-info/cost-info 5 接口 + filing 到 Bridge 全链路，前端联调通过
- **第二优先级完成**：download/version-download 可用，delivery 推审稳定，上传下载全链路联调通过
- **第三优先级完成**：文档版本口径统一，无歧义引用
