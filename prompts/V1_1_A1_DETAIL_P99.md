# V1.1-A1 · `/v1/tasks/{id}/detail` P99 收口轮

> Last updated: 2026-04-25
> 性质:V1.1 mandatory performance round prompt
> 前置状态:V1.0 SUBSTANTIAL-COMPLETE · R6.A.4 architect-cleared
> 红门来源:`docs/iterations/V1_RETRO_REPORT.md §14` · detail P99 `334.721237ms`
> 本轮目标:只把 `GET /v1/tasks/{id}/detail` P99 拉回验收门,不改变 HTTP 契约。

---

## §0 终止符

成功完成自跑 verify 后输出:

```text
V1_1_A1_DONE_PENDING_ARCHITECT_VERIFY
```

架构师独立 verify 通过后改签:

```text
V1_1_A1_DONE_ARCHITECT_VERIFIED
```

任一硬门触发:

```text
V1_1_A1_ABORT · reason=<short>
```

禁止自签 PASS。

---

## §1 Authority / 必读

先按顺序读完:

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`
3. `docs/iterations/V1_RETRO_REPORT.md §14`
4. `prompts/V1_ROADMAP.md §32 + 变更记录 v25~v28`
5. 4 份 V1 权威文档只读引用:
   - `docs/V1_MODULE_ARCHITECTURE.md`
   - `docs/V1_INFORMATION_ARCHITECTURE.md`
   - `docs/V1_CUSTOMIZATION_WORKFLOW.md`
   - `docs/V1_ASSET_OWNERSHIP.md`
6. `docs/api/openapi.yaml`
7. `transport/http.go`

冲突时仍按工作铁律:

1. `transport/http.go` 决定实际挂载。
2. `docs/api/openapi.yaml` 决定 HTTP 契约。
3. 4 份 V1 权威文档决定架构语义。
4. retro / ROADMAP 只作证据和路线索引。

---

## §2 Scope

### §2.1 本轮允许改

只允许围绕 `GET /v1/tasks/{id}/detail` 的性能路径改动:

- `service/task_aggregator/*`
- `repo/mysql/task.go`
- `repo/mysql/task_module_repo.go`
- `repo/mysql/task_module_event_repo.go`
- `repo/mysql/reference_file_ref_repo.go`
- 必要时新增窄接口到 `repo/interfaces.go`
- 必要时新增 migration / SQL 脚本用于索引或只读聚合表/投影表
- 必要时新增 focused `_test.go`
- `tmp/v1_1_a1_*` 验证脚本
- 完成后新增 `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md`

### §2.2 本轮禁止改

- 0 OpenAPI 契约改动。
- 0 `transport/http.go` 路由改动。
- 0 4 份权威文档改动。
- 0 兼容路由下线。
- 0 notification type / module key / deny code 语义改动。
- 0 cron gate 行为改动。
- 不碰 `/v1/tasks` 列表、资产中心、报表、Excel 业务语义,除非只是复用 helper 且有回归证明。

---

## §3 Baseline

### §3.1 段

本轮使用测试库 `jst_erp_r3_test`,段 `[80000,90000)`。

开工前必须跑:

```bash
chmod +x tmp/v1_1_a1_isolation_run.sh
tmp/v1_1_a1_isolation_run.sh
```

本 prompt 起草时已落盘模板:

- `tmp/v1_1_a1_segment_audit.sql`
- `tmp/v1_1_a1_segment_clean.sql`
- `tmp/v1_1_a1_isolation_run.sh`

起草实测:

```text
BEFORE: users/tasks/task_modules/task_module_events/task_assets/org_move_requests/notifications/task_drafts = 0; permission_logs = 2288
AFTER:  9 表全 0
integration 后 step-N audit: 9 表全 0
```

### §3.2 sha 锚

开工 step-1 必须校验:

```text
b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f  docs/api/openapi.yaml
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

若本轮确需改 `domain/task.go` 或 `cmd/server/main.go`,必须先在报告里解释为什么性能路径必须触碰该锚;默认不应触碰。

### §3.3 接手 baseline 实测(2026-04-25)

环境:

```bash
export PATH=/usr/bin:/usr/local/bin:/home/wsfwk/go/bin:$PATH
export MYSQL_DSN='root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local'
export R35_MODE=1
```

起草轮已验证:

```text
sha 锚:7/7 一致 + OpenAPI 一致
DB runner: tmp/r6_a_4_select_task_ids.go wrote 100 ids (98 distinct)
go build ./... PASS
go vet ./... PASS
go test -count=1 ./... PASS
openapi-validate: 0 error 0 warning
integration -tags=integration -p 1 -count=1 -timeout 60m ./... PASS
```

integration 关键包计时:

```text
workflow/cmd/tools/migrate_v1_backfill  38.053s
workflow/cmd/tools/run-cleanup          185.888s
workflow/service/search                 17.445s
workflow/service/task_lifecycle         8.932s
workflow/transport/handler              7.552s
```

完整日志:

```text
tmp/v1_1_a1_integration_baseline.log
tmp/v1_1_a1_segment_audit_after_integration.log
```

---

## §4 当前 detail 热路径事实

运行时挂载:

```text
transport/http.go: GET /v1/tasks/:id/detail -> handler.TaskDetailHandler.GetByTaskID
cmd/server/main.go: TaskDetailHandler.SetR3DetailService(task_aggregator.NewDetailService(...))
```

当前 R3 detail 聚合是 5 次独立 DB query:

1. `tasks.GetByID(taskID)` -> `tasks WHERE id=?`
2. `tasks.GetDetailByTaskID(taskID)` -> `task_details WHERE task_id=?`
3. `modules.ListByTask(taskID)` -> `task_modules WHERE task_id=? ... ORDER BY FIELD(...)`
4. `events.ListRecentByTask(taskID, 50)` -> `task_module_events JOIN task_modules WHERE tm.task_id=? ORDER BY created_at DESC,id DESC LIMIT 50`
5. `refs.ListByTask(taskID)` -> `reference_file_refs WHERE task_id=? ORDER BY owner_module_key,attached_at,id`

起草时 `SHOW INDEX` 观察:

- `tasks`:有 `PRIMARY(id)`, `idx_tasks_task_status(task_status)`, `idx_tasks_priority_created(priority,created_at)`, `idx_tasks_creator_id`, `idx_tasks_designer_id`;没有 `(task_status,updated_at)`。
- `task_details`:有 `uq_task_details_task_id(task_id)`。
- `task_modules`:有 `uq_task_modules_task_module(task_id,module_key)`, `idx_task_modules_task(task_id)`, `idx_task_modules_claim(claimed_by,state,updated_at)`。
- `task_module_events`:有 `idx_task_module_events_module_created(task_module_id,created_at)`, `idx_task_module_events_type_created(event_type,created_at)`。
- `reference_file_refs`:有 unique `(task_id,ref_id,sku_item_id)` 和 `idx_reference_file_refs_owner_task(owner_module_key,task_id)`。

初步判断:单 task_id 基础索引并不空缺,本轮要先用 `EXPLAIN ANALYZE` / `performance_schema` / per-query timing 证明瓶颈,再选择改法。不要凭想象上物化表。

---

## §5 实施候选与决策顺序

按低风险到高风险评估:

1. **Per-query latency instrumentation**
   - 在 tmp runner 或 focused test 中拆分 detail 五段耗时。
   - 必须输出 cold/warm 分桶和最慢 query。

2. **SQL 合并 / round-trip 收敛**
   - 优先考虑新增 repo 方法一次查询 task + task_detail。
   - 对 events 可先取 `task_modules` ids 后用 `WHERE task_module_id IN (...) ORDER BY ... LIMIT 50`,避免 join/order 误选计划。
   - 保持 JSON response shape 不变。

3. **索引补强**
   - 只有 EXPLAIN 证明需要时才加。
   - 候选:
     - `tasks(task_status, updated_at)` 用于状态/更新时间类后续 detail/list/probe,但若本轮 detail by id 用不上,不得为 P99 硬加。
     - `task_module_events(task_module_id, created_at, id)` 覆盖 `ORDER BY created_at DESC,id DESC LIMIT 50`。
     - `reference_file_refs(task_id, owner_module_key, attached_at, id)` 覆盖当前排序。

4. **只读聚合投影**
   - MySQL 8.0 没有真 materialized view。
   - 若要做 `v_task_detail_aggregate` 视图或投影表,必须说明刷新时机、写入放大、回滚策略、与 task/module 事件一致性。
   - 不得引入 Redis / 外部 cache 作为 V1.1 必需依赖。

5. **进程内冷暖 cache**
   - 仅作为可选优化。
   - 必须有失效策略或明确只用于 read-only benchmark。
   - 若不能证明一致性,不得进入主实现。

推荐起步方案:先做 instrumentation + EXPLAIN,若瓶颈是 round-trip 或 event join,优先做 repo 层 query 收敛和窄索引;物化/缓存作为第二阶段。

---

## §6 验收门

### §6.1 功能回归

必须 PASS:

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go vet ./...
/home/wsfwk/go/bin/go test -count=1 ./...
set -o pipefail && /home/wsfwk/go/bin/go test -tags=integration -p 1 -count=1 -timeout 60m ./... 2>&1 | tee tmp/v1_1_a1_integration.log
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

### §6.2 P99

必须使用 `cmd/server` live HTTP 路径,不允许只测 service 层:

```bash
SERVER_PORT=18086 /home/wsfwk/go/bin/go run ./cmd/server > tmp/v1_1_a1_server.log 2>&1 &
curl -sS http://127.0.0.1:18086/healthz
SUPER_ADMIN_TOKEN=<token> /home/wsfwk/go/bin/go run tmp/v1_1_a1_p99_runner.go
```

验收门:

```text
cold n=100 p99 < 150ms
warm n=100 p99 < 80ms
non-200 = 0
```

建议额外跑:

```text
n=500 warm P95/P99 记录到报告,不作为硬门
```

### §6.3 A1~A12 zero regression

必须重跑 R6.A.4 A-matrix:

```bash
set -o pipefail && /home/wsfwk/go/bin/go test -tags=integration -run '^TestRetro_A' ./tmp/... 2>&1 | tee tmp/v1_1_a1_a_matrix.log
```

### §6.4 段隔离

开工前和完成后均需:

```bash
ssh jst_ecs '... mysql ... < /tmp/v1_1_a1_segment_audit.sql'
```

完成后 9 表必须全 0。

---

## §7 ABORT

命中任一立即停止:

1. sha 锚在开工时漂移且用户未确认。
2. 4 份权威文档有写入。
3. OpenAPI 有任何改动或 validate 非 0/0。
4. `transport/http.go` 有任何路由改动。
5. 全栈 integration 未使用 `-p 1` 或无 `set -o pipefail`。
6. 全栈 integration `-p 1` 任一包 FAIL。
7. P99 cold 仍 `>=150ms`,且已做一轮低风险优化后无明确下一步证据。
8. warm P99 仍 `>=80ms` 且无可解释 outlier。
9. `[80000,90000)` step-N audit 任一表非 0。
10. 实现需要新增/删除 HTTP path 或改变 response shape。
11. 实现需要改 notification type / module key / deny code。
12. 实现需要引入外部缓存、队列或 cron side effect。

---

## §8 交付报告

完成后新增:

```text
docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md
```

报告至少包含:

- baseline sha 和新 sha。
- 改动文件清单。
- 瓶颈定位证据:per-query latency + EXPLAIN。
- 决策 trace:为什么选 SQL/索引/投影/缓存中的哪一种。
- cold/warm P99 数字。
- A1~A12、build/vet/unit/integration/openapi 结果。
- `[80000,90000)` step-1 / step-N audit。
- 未解决风险。
- 终止符。

---

## §9 开工咒语

```text
我已读完 prompts/V1_1_A1_DETAIL_P99.md。
我接手的轮:V1.1-A1 detail P99。
我接手的段:[80000,90000)。
我已校 §3.2 sha 锚:7/7 一致。
我承诺不改 OpenAPI / transport route / 4 份权威文档。
我承诺 integration 只用 -p 1 + set -o pipefail。

现在开始 step-1。
```
