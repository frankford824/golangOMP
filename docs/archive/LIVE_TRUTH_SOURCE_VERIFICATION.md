# live 真相源收口 — 验收报告模板

本专项目标：拿到 8081 命中 OpenWeb、8080 `products` 由 JST/OpenWeb 驱动增长的**服务器硬证据**。

## 执行方式

在服务器 `root@223.4.249.11` 上：

```bash
cd /root/ecommerce_ai
bash scripts/live_truth_source_verify.sh
```

将输出保存后，按以下结构整理为最终报告。

---

## 1. 当前 live 运行态

| 项目 | 值 |
|------|-----|
| 8080 PID | |
| 8081 PID | |
| 8082 PID | |
| `/proc/<pid>/exe` | |
| 8080 health | |
| 8081 health | |
| 8082 health | |
| **ERP_REMOTE_MODE** | local / remote / hybrid |
| **ERP_SYNC_SOURCE_MODE** | stub / jst |
| **ERP_SYNC_ENABLED** | true / false |
| **ERP_REMOTE_BASE_URL** | |
| **ERP_REMOTE_AUTH_MODE** | |

**结论**：8081 是否具备 OpenWeb 查询条件？8080 是否具备 JST sync 到 products 的条件？

---

## 2. A 组：8081 OpenWeb 主链验收

| 项目 | 内容 |
|------|------|
| 查询请求 | `curl .../v1/erp/products?q=HQT21413` |
| 响应摘要 | 状态码、条数、耗时 |
| 日志证据 | `grep erp_bridge_product_search` 输出 |
| 是否真实命中 OpenWeb | 是 / 否 |
| 是否 fallback | 是 / 否 |
| fallback 原因 | |
| **结论** | 通过 / 未通过 |

**通过标准**：日志出现 `result=remote_ok`、`fallback_used=false`。

---

## 3. B 组：JST/OpenWeb -> products 副本验收

| 项目 | 内容 |
|------|------|
| sync 入口 | `POST /v1/products/sync/run` |
| provider/source 证据 | `grep erp_sync_run` 输出 |
| sync 日志摘要 | erp_sync_run_start/finish |
| products 更新前后对比 | COUNT 变化 |
| 样本 SKU 对比 | HQT21413 等 |
| **结论** | 通过 / 未通过 |

**通过标准**：`source_mode=jst`、`provider=JSTOpenWebProductProvider`、`total_upserted>0`、products 有 `sync_role=8080_products_replica_from_openweb`。

---

## 4. 本轮做的最小修复

| 文件/配置项 | 修改原因 | 修改内容 | 是否重启 | 复验结果 |
|-------------|----------|----------|----------|----------|
| service/erp_sync_service.go | sync 可观测性补强 | 增加 erp_sync_run_start/finish 日志 | 需重启 8080 | |
| scripts/live_truth_source_verify.sh | 验收脚本 | 新增 | 无 | |

---

## 5. 最终结论

**一句话**：当前 live 是否已有硬证据表明 8081 命中 OpenWeb、8080 products 由 JST/OpenWeb 驱动增长？

- [ ] 是
- [ ] 否

---

## 6. 若仍未通过，最小阻塞清单

- [ ] 配置：ERP_REMOTE_MODE / ERP_SYNC_SOURCE_MODE / 凭证
- [ ] 日志可观测性
- [ ] provider 命中
- [ ] sync 入口
- [ ] 数据表关系
- [ ] 部署状态

---

## 日志关键字速查

| 目标 | grep 关键字 |
|------|-------------|
| 8081 OpenWeb 主链成功 | `erp_bridge_product_search` + `result=remote_ok` |
| 8081 fallback | `fallback_local_products` |
| 8081 仅本地 | `8081_local_only` |
| sync JST 命中 | `erp_sync_run_start` + `provider=JSTOpenWebProductProvider` |
| sync 完成 | `erp_sync_run_finish` |
