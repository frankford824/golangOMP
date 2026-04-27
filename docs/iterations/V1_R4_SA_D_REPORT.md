# V1 R4-SA-D Report

## Scope

Implemented the four `x-owner-round: R4-SA-D` paths:

| Path | Handler | Success | Deny/error matrix |
| --- | --- | --- | --- |
| `GET /v1/search` | `transport/handler/search.go` | `200 {query, results}` | `400 invalid_query`, `401` by `withAuthenticated` |
| `GET /v1/reports/l1/cards` | `transport/handler/report_l1.go` | `200 {data: L1Card[]}` | `403 PERMISSION_DENIED deny_code=reports_super_admin_only`, `401` |
| `GET /v1/reports/l1/throughput` | `transport/handler/report_l1.go` | `200 {data: L1ThroughputPoint[]}` | `400 invalid_date_range`, `403 reports_super_admin_only`, `401` |
| `GET /v1/reports/l1/module-dwell` | `transport/handler/report_l1.go` | `200 {data: L1ModuleDwellPoint[]}` | `400 invalid_date_range`, `403 reports_super_admin_only`, `401` |

## §3 Search 实装

Domain contract: `domain/search_result.go`.

SQL and source strategy:

| Source | SQL strategy |
| --- | --- |
| tasks | `tasks.task_no/product_name_snapshot/sku_code/primary_sku_code/CAST(id AS CHAR) LIKE CONCAT('%', ?, '%')`, `ORDER BY id DESC LIMIT ?`; code: `repo/mysql/search_repo.go:17` |
| assets | `task_assets.file_name LIKE CONCAT('%', ?, '%')`, excludes rows with `deleted_at` or `cleaned_at`, `ORDER BY COALESCE(asset_id,id) DESC LIMIT ?`; code: `repo/mysql/search_repo.go:50` |
| products | prefer `erp_product_snapshots` if present; otherwise fallback to distinct `tasks.sku_code/product_name_snapshot`; code: `repo/mysql/search_repo.go:79` |
| users | `users.username/display_name/email LIKE`, `status='active'`, `ORDER BY id DESC LIMIT ?`; code: `repo/mysql/search_repo.go:108` |

`users[]` low-privilege empty-array gate is in `service/search/service.go:85`: only `SuperAdmin` and `HRAdmin` reach `SearchUsers`; all others return `[]` for both `scope=all` and `scope=users`.

`highlight` is explicitly `nil` in v1 MySQL LIKE mode (`repo/mysql/search_repo.go:44`).

## §4 Report L1 实装

Domain contract: `domain/report_l1.go`.

| Handler | SQL |
| --- | --- |
| Cards | `COUNT(*) FROM tasks` for in-progress and archived totals; completed-today uses `task_module_events JOIN task_modules JOIN tasks`; code: `repo/mysql/report_l1_repo.go:17` |
| Throughput | `task_module_events JOIN task_modules JOIN tasks`, filters `[from, to+1d)`, excludes `backfill_placeholder`, groups by `DATE_FORMAT(e.created_at, '%Y-%m-%d')`; code: `repo/mysql/report_l1_repo.go:35` |
| Module dwell | SQL CTE over `task_module_events`, pairs enter-like events with later exit-like events, computes `AVG` and P95 via window rank; fixed 5 module rows returned; code: `repo/mysql/report_l1_repo.go:67` |

Throughput semantics in v1:

- `created`: `event_type='created'`.
- `completed`: distinct task count where event is `closed/archived/approved` or task is already `closed/archived`.
- `archived`: same as `completed` for v1 simplification.
- `state_exit_at IS NULL` equivalent: open modules without a later exit-like event are not counted in dwell samples.

## §5 SuperAdmin 门控证据

Report service gates all three methods before querying:

- `Cards`: `service/report_l1/service.go:32`
- `Throughput`: `service/report_l1/service.go:46`
- `ModuleDwell`: `service/report_l1/service.go:63`
- Role check: `service/report_l1/service.go:80`
- Audit write: `permission_logs.action_type='report_access_denied'` at `service/report_l1/service.go:92`

Integration evidence: `service/report_l1/sa_d_i6_report_cards_rbac_integration_test.go` asserts 403 `reports_super_admin_only` and verifies `report_access_denied` exists during the test.

## 11 Integration Assertions

| ID | File | Evidence |
| --- | --- | --- |
| SA-D-I1 | `service/search/sa_d_i1_search_all_integration_test.go` | `scope=all` finds `TASK50001`; operator sees `users=[]`. |
| SA-D-I2 | `service/search/sa_d_i2_search_users_low_priv_integration_test.go` | SuperAdmin/HRAdmin see matching user; Member sees `users=[]`. |
| SA-D-I3 | `service/search/sa_d_i3_search_limit_integration_test.go` | `limit=3` caps all arrays. |
| SA-D-I4 | `service/search/sa_d_i4_search_scope_integration_test.go` | `scope=tasks` isolates tasks; low-priv `scope=users` returns empty. |
| SA-D-I5 | `service/search/sa_d_i5_search_invalid_query_integration_test.go` | blank `q` returns `invalid_query`; one-character query is accepted. |
| SA-D-I6 | `service/report_l1/sa_d_i6_report_cards_rbac_integration_test.go` | non-SuperAdmin cards call returns deny and audit row. |
| SA-D-I7 | `service/report_l1/sa_d_i7_report_cards_super_admin_integration_test.go` | SuperAdmin cards return at least `tasks_in_progress`, `tasks_completed_today`, `archived_total`. |
| SA-D-I8 | `service/report_l1/sa_d_i8_report_throughput_integration_test.go` | seeded events on 2026-04-20..22 return points in range. |
| SA-D-I9 | `service/report_l1/sa_d_i9_report_module_dwell_integration_test.go` | module dwell returns 5 module keys and non-negative metrics. |
| SA-D-I10 | `service/report_l1/sa_d_i10_report_date_range_invalid_integration_test.go` | `from > to` returns `invalid_date_range` for throughput and module dwell. |
| SA-D-I11 | `service/search/sa_d_i11_search_p95_performance_integration_test.go` | 100 search calls measured and asserted P95 < 1s. |

## Test DB Touch

Test range guards:

- Search helper enforces `user_id` and `task_id` in `[50000, 60000)`: `service/search/sa_d_test_helper_test.go:20`.
- Report helper enforces same range: `service/report_l1/sa_d_test_helper_test.go:20`.
- Cleanup is registered with `t.Cleanup`, and DB close is also registered after cleanup-safe usage: `service/search/sa_d_test_helper_test.go:24`, `service/report_l1/sa_d_test_helper_test.go:24`.

Final test DB audit after cleanup:

```text
SELECT MIN(id), MAX(id) FROM users WHERE id BETWEEN 50000 AND 59999;
NULL    NULL

SELECT COUNT(*) FROM tasks WHERE id BETWEEN 50000 AND 59999;
0

SELECT COUNT(*) FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id BETWEEN 50000 AND 59999;
0

SELECT COUNT(*) FROM permission_logs WHERE action_type='report_access_denied' AND created_at >= DATE_SUB(UTC_TIMESTAMP(), INTERVAL 6 HOUR);
0 after cleanup; SA-D-I6 observed a non-zero audit count before cleanup.
```

## Dangling 501 Cleanup

`transport/http.go` now mounts live handlers:

- `/v1/search`: `transport/http.go:127`
- `/v1/reports/l1/cards`: `transport/http.go:131`
- `/v1/reports/l1/throughput`: `transport/http.go:133`
- `/v1/reports/l1/module-dwell`: `transport/http.go:135`

The four paths were removed from `v1R1ContractRouteSpecs`; `grep -n "/search\\|reports/l1" transport/http.go` shows no R4-SA-D reserved entries under the contract specs section.

## OpenAPI Conformance

```text
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
openapi validate: 0 error 0 warning
```

## Performance

`/v1/search` 100-call integration run:

```text
SA-D search perf avg=93.09161ms median=92.392577ms p95=96.768514ms
```

IA-A3 test-gate target for this test run was P95 < 1s; observed P95 is below that.

## §4 数据隔离审计

Executed against `jst_erp_r3_test` after cleanup:

```sql
SELECT MIN(id), MAX(id) FROM users WHERE id BETWEEN 50000 AND 59999;
-- NULL / NULL

SELECT COUNT(*) FROM tasks WHERE id BETWEEN 50000 AND 59999;
-- 0

SELECT COUNT(*) FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id BETWEEN 50000 AND 59999;
-- 0

SELECT COUNT(*) FROM permission_logs WHERE action_type='report_access_denied' AND created_at >= DATE_SUB(UTC_TIMESTAMP(), INTERVAL 6 HOUR);
-- 0 after cleanup; SA-D-I6 verifies non-zero during the denial test itself.
```

## Known Non-Goals

L2/L3 reports (R7+), Elasticsearch/Meilisearch (R6+/R7+), materialized report tables (R6+), CSV/Excel export, `task_timeout` metrics, and cross-instance aggregation were not implemented.

## SA-D sign-off candidate

SA-D is a sign-off candidate: the four R4-SA-D routes are live, dangling 501 stubs are removed, SuperAdmin report gating and `report_access_denied` audit are implemented, search user visibility follows the low-privilege empty-array rule, and validation A-F passed with no ABORT condition observed.

---

## Architect Adjudication(SA-D v1.0 签字)

**签字日期**:2026-04-25
**裁决人**:主对话架构师
**结论**:**通过签字**(sign-off)

### 5 大硬门复核

| 门 | 指标 | 实测 | 结论 |
| --- | --- | --- | --- |
| 1. 生产零污染(post-probe D1~D4) | 4 条聚合全 0 | D1=0 · D2=0 · D3=0 · D4=0(7 项跨域字段) | PASS |
| 2. 架构师独立 verify_sa_d.sh | 7/7 二级检查 PASS | A build 双 tag · B DSN · C SADI batch(search 19.7s · report_l1 6.5s)· D SA-A/B/C/R3 回归 PASS · E openapi-validate 0/0 · G 501 清除 · H owner=4;F 本地 socket 连不上但 ssh 直查补证 8 项全 0 | PASS |
| 3. SA-D-I1~I11 integration | 11/11 断言 PASS | `service/search` 5 条 · `service/report_l1` 5 条 · search performance 1 条 | PASS |
| 4. 测试数据隔离(`jst_erp_r3_test` t.Cleanup 后) | 8 项全 0 | users/tasks/task_modules/task_module_events(JOIN)/notifications/task_drafts/permission_logs.report_access_denied(6h+all) = 0 | PASS |
| 5. 性能 p95(IA-A3 门 < 1s;SA-D 自规 < 500ms) | p95 = 96.77 ms | < 500 ms · 远低 1 s | PASS |

### 架构偏差与裁决

本轮暴露 3 处 prompt / 架构假设与真实 schema 的偏差 · Codex 均自行合理补救:

1. **prompt §4.88 "直查 task_module_events 按 module_key 分组"**
   - 生产 `task_module_events` 实际无 `module_key`/`task_id`/`state_enter_at`/`state_exit_at` 列;这些属于 `task_modules`。
   - Codex 实装走 `task_module_events e JOIN task_modules m ON m.id=e.task_module_id`(code: `repo/mysql/report_l1_repo.go:17/35/67`)· module-dwell 用 CTE 配对 enter-like/exit-like event + window rank 求 P95。
   - 架构师裁决:**接受**为 "直查 task_module_events" 的等价实装 · 语义符合 prompt 意图与 IA §4 报表定义;该偏差在下面 §12 / IA 同步记录校正。

2. **prompt §4 "throughput.archived 用 task.task_status='archived' 去重"**
   - Codex 实装把 `archived` 视作与 `completed` 同集合(基于 event_type `closed/archived/approved`)· v1 简化。
   - 架构师裁决:**接受**为 v1 简化 · v2+ 视需要再拆分;该简化在下面 IA 同步记录为显式决议。

3. **R1.7-D Q1=A1 "users[] 低权返空数组"未列明可见角色名单**
   - Codex 实装把 SuperAdmin + HRAdmin 视为"高权"可见 users[] · 其余角色全空。
   - 架构师裁决:**接受** · 与 V1_MODULE_ARCHITECTURE §17 Q6.6(HR 管理员已存在)+ Q6.1(三级授权)对齐;在 V1_R4_SA_D_REPORT §3 行 27 显式固化为规则。

### 同步落到架构文档的 2 处补白

- **V1_MODULE_ARCHITECTURE §17 决策清单**追加 `报表数据源实装校正(SA-D)` 行。
- **V1_INFORMATION_ARCHITECTURE §2 菜单可见性矩阵**下方追加 §2.1 报表 L1 实装规则快照(含 v1 简化 `archived==completed` + `users[]` 可见角色表)。

### 签字

R4-SA-D v1.0 **正式签字生效**。R4 P3 顺序 4 轮(SA-A / SA-B / SA-C / SA-D)全部闭环 · 合计签字 11 个(4 主轮 + R1.7/R1.7-B/R1.7-C/R1.7-D + SA-B.1/SA-B.2/SA-C.1)。

