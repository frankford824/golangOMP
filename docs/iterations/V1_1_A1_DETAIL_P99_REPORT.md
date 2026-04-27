# V1.1-A1 Detail P99 Report · 2026-04-25

> 范围:`GET /v1/tasks/{id}/detail` P99 收口。
> 裁决:PASS · architect-verified。
> 终止符:`V1_1_A1_DONE_ARCHITECT_VERIFIED`

## §0 Baseline

| 项 | 实测 |
| --- | --- |
| 测试库 | `jst_erp_r3_test` |
| 段 | `[80000,90000)` |
| 接手前 detail P99 | R6.A.4 `334.721237ms`;本轮复现 `299.683411ms` |
| 冷启动门 | `p99 < 150ms` |
| 暖启动门 | `p99 < 80ms` |
| OpenAPI | 0 改动 · `openapi validate: 0 error 0 warning` |
| 4 份 V1 权威文档 | 0 改动 |
| 路由 | 0 改动 |

## §1 SHA

未漂移锚:

```text
b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f  docs/api/openapi.yaml
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

本轮改动 sha:

```text
6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e  service/task_aggregator/detail_aggregator.go
c6518daef3db588525c6cada3f366118c21483643c9241e81b1e6a13a81b70ba  repo/mysql/task_detail_bundle.go
224e96fe180a27349131710848229c73c686cfe5aa1704339573859398fa8f89  service/identity_service.go
d8c135221fc8c6745b6863521230a0a39ba43cc6420c713cc836e474fc1e8a6a  repo/mysql/identity_actor_bundle.go
```

## §2 改动

### Detail 聚合

文件:

- `service/task_aggregator/detail_aggregator.go`
- `repo/mysql/task_detail_bundle.go`

决策:

- 未采用 MySQL 物化视图或进程 cache。
- 原 detail R3 路径为 5 次顺序查询:`tasks` / `task_details` / `task_modules` / `task_module_events JOIN task_modules` / `reference_file_refs`。
- 在远端 MySQL + SSH tunnel 下,多 round-trip 是主因。本轮新增 `GetTaskDetailBundle` optional fast path,使用 MySQL multi-result set 将 5 次查询收敛为 1 次 round-trip。
- fast path 失败时回落旧顺序查询路径,避免生产 DSN 缺 `multiStatements=true` 时直接中断 detail。
- response shape 不变:`task`, `task_detail`, `modules`, `events`, `reference_file_refs`。

### Session 鉴权

文件:

- `service/identity_service.go`
- `repo/mysql/identity_actor_bundle.go`

决策:

- 不做 session cache,不放宽 revoke / expiry / disabled-user 检查。
- 原请求鉴权路径为 session / user / role / touch 多次 DB round-trip。
- 新增 `ResolveActorBundle` optional fast path,在一个 MySQL multi-result set 中读取 session、user、raw roles 并 touch session。
- fast path internal error 时回落旧 session/user/roles/touch 顺序路径,保持 DSN 兼容性。
- 成功的 `route_access` 记录改为异步 best-effort;失败/拒绝日志仍同步。原实现本身已忽略写入错误,本轮只移出成功请求热路径。

## §3 P99

live server:

```bash
SERVER_PORT=18087 /home/wsfwk/go/bin/go run ./cmd/server
SUPER_ADMIN_TOKEN=$(cat tmp/v1_1_a1_super_admin_token.txt) BASE_URL=http://127.0.0.1:18087 /home/wsfwk/go/bin/go run tmp/v1_1_a1_p99_runner.go
```

结果:

| 阶段 | n | p50 | p95 | p99 | max | 判定 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| baseline 复现 | 100 | 250.362ms | 288.185ms | 299.683ms | 299.683ms | RED |
| detail bundle 后 | 100 | 156.135ms | 173.569ms | 208.924ms | 208.924ms | RED |
| auth bundle + async success route log 后 cold | 100 | 42.993ms | 46.353ms | 47.525ms | 47.525ms | GREEN |
| warm | 100 | 43.382ms | 46.054ms | 47.513ms | 47.513ms | GREEN |
| warm extended | 500 | 42.904ms | 45.903ms | 46.720ms | 50.619ms | GREEN |
| final after fallback patch | 500 | 42.824ms | 45.710ms | 47.126ms | 48.120ms | GREEN |

结论:

- cold `p99=47.525ms < 150ms`。
- warm `p99=47.513ms < 80ms`。
- 最终代码 500 次 warm 扩展样本无尾延迟 outlier。

## §4 验证

```text
go build ./...                         PASS
go vet ./...                           PASS
go test -count=1 ./...                 PASS
openapi-validate                       PASS · 0 error 0 warning
go test -tags=integration -p 1 ./...   PASS
go test -tags=integration -run '^TestRetro_A' ./tmp/... PASS
```

全栈 integration 关键包:

```text
workflow/cmd/tools/migrate_v1_backfill  37.695s
workflow/cmd/tools/run-cleanup          185.293s
workflow/service/search                 17.402s
workflow/service/task_lifecycle         8.897s
workflow/transport/handler              6.199s
```

日志:

```text
tmp/v1_1_a1_integration_final.log
tmp/v1_1_a1_a_matrix_final.log
tmp/v1_1_a1_final_isolation_after_p99.log
```

## §5 段隔离

`[80000,90000)` final isolation:

```text
BEFORE clean:
users 0
tasks 0
task_modules 0
task_module_events 0
task_assets 0
org_move_requests 0
notifications 0
task_drafts 0
permission_logs 5

AFTER clean:
users 0
tasks 0
task_modules 0
task_module_events 0
task_assets 0
org_move_requests 0
notifications 0
task_drafts 0
permission_logs 0
```

`permission_logs=5` 是本轮 live P99 route_access 观测残留,已清零。

## §6 架构师裁决

Verdict: **PASS · architect-verified**。

理由:

- P99 主红门已从 R6.A.4 `334.721ms` 收口至最终代码 warm extended `47.126ms`。
- 没有修改 OpenAPI、路由、4 份 V1 权威文档、cron gate、module key、deny code 或 notification type。
- 实现继承现有 MySQL/Go runtime,未引入外部 cache / Redis / 物化表写入放大。
- 全栈 integration `-p 1`、A-matrix、OpenAPI validate、段隔离均通过。

后继:

- V1.1-A2 CI integration `-p 1` 守卫仍建议执行。
- 可进入前端联调规划阶段,以 canonical V1 stable routes 为唯一入口。

## §7 终止符

V1_1_A1_DONE_ARCHITECT_VERIFIED
