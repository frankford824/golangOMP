# V1.1-A2 · Contract Drift Purge Retro Report

> 日期: 2026-04-27
> 终止符: `V1_1_A2_CONTRACT_DRIFT_PURGED`
> 裁决: 已完成,等待架构师独立 verify;本文不自签 PASS。

## §1 启动证据

用户在 v1.21 上线后实测 `GET /v1/tasks/{id}/detail` 返回 `data.{task, task_detail, modules, events, reference_file_refs}` 5 段,而 OpenAPI 仍声明旧 `TaskDetail` 富 schema。V1.1-A1 fast path 实现在 `service/task_aggregator/detail_aggregator.go` 的 `Detail` struct,本轮按 code-wins 修订契约与前端文档。

## §2 全量扫描结论

- 总 path: 203
- clean: 192
- P0: 1
- P1: 5
- P2: 5
- defer-v12: 5

详见 `docs/iterations/V1_1_A2_DRIFT_INVENTORY.md`。

## §3 修订摘要

- OpenAPI 行数: 15147 → 15297,净增 150 行。
- schema 新增: 4(`TaskAggregateDetailV2`, `TaskAggregateModule`, `TaskModuleEvent`, `ReferenceFileRefFlat`)。
- schema 修改: 2(`Task`, `TaskDetail`)。
- schema 删除: 0。
- path schema 修改: 1(`/v1/tasks/{id}/detail` 200 response data ref)。
- frontend docs: 16 份均追加 V1.1-A2 revision marker;`V1_API_TASKS.md` detail 段重写为 5 段结构。

## §4 验证矩阵

| # | 检查项 | 结果 |
|---|---|---|
| 1 | 业务 sha 锚组 A(10 文件) | PASS · final 与 baseline 一致 |
| 2 | OpenAPI path 数 | PASS · 203 |
| 3 | OpenAPI validator | PASS · 0 error 0 warning |
| 4 | dangling 501 | PASS · 0 |
| 5 | inventory 覆盖率 | PASS · 203/203 path |
| 6 | P0/P1 全部 `[done]` | PASS · 6/6 |
| 7 | `go vet ./...` | PASS |
| 8 | `go build ./...` | PASS |
| 9 | frontend doc 文件数 | PASS · 16 |
| 10 | git status `*.go` 改动 | PASS · 0 文件 |
| 11 | 抽样 5 条 P0/P1 path 手工 diff | PASS · detail/product-info/cost-info/business-info/assign 对齐本轮 schema |

## §5 SHA baseline / final

Baseline 记录: `tmp/v1_1_a2/baseline_sha.log`。

业务锚组 A final 与 baseline 一致:

```text
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e  service/task_aggregator/detail_aggregator.go
c6518daef3db588525c6cada3f366118c21483643c9241e81b1e6a13a81b70ba  repo/mysql/task_detail_bundle.go
00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644  service/identity_service.go
d8c135221fc8c6745b6863521230a0a39ba43cc6420c713cc836e474fc1e8a6a  repo/mysql/identity_actor_bundle.go
```

OpenAPI post V1.1-A2 sha: `0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3`。

## §6 已知遗留(defer-v12)

- prompt 示例 regex 漏掉 dotted path `/v1/tasks/batch-create/template.xlsx`;本轮用 YAML parser 保证 203 path 覆盖。
- `identity_service.go` 线上 v1.21 编译历史 sha 与 git HEAD sha 不一致的 micro-drift 已按架构师裁决接受 git HEAD 为本轮 baseline;V1.2 应 rebuild/deploy v1.22 对齐线上二进制。
- 旧 `TaskModule` R1 schema 本轮未做 orphan cleanup,避免无 inventory 背书扩散;如需清理转 V1.2。

## §7 待架构师独立 verify

Codex 已完成 V1.1-A2,但不自签 PASS。请架构师按 prompt §6 11 项矩阵独立复核后签字 `V1_1_A2_DONE_ARCHITECT_VERIFIED`。
