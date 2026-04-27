# V1.2-D-1 · `task_detail.go` fallback 出口干净切除 · CRITICAL drift 修复

> 范围:**最小修改面** · 只切除 `TaskDetailHandler` 自身的 fallback 出口 · 让 `/v1/tasks/:id/detail` 唯一返回 V1.1-A1 fast-path 5 段 schema。
> 不动 `domain.TaskDetailAggregate` / `service.TaskDetailAggregateService`(其他 handler `TaskHandler` 在用)。
> 不动 OpenAPI(已经是 5 段)· 不动 frontend doc(已经是 5 段)。
> 验收:`tools/contract_audit` 上 `GET /v1/tasks/:id/detail` 由 `both_diff` → `clean` · drift 由 72 → 71。

## §0 背景

**架构师在 V1.2-C 工具落地后抽样发现的 CRITICAL 漂移**:

```text
GET /v1/tasks/:id/detail
verdict       = both_diff          ← 应该 clean(V1.1-A2 已修过)
response_type = TaskDetailAggregate ← !!! 工具反推到老 36 字段富 schema
code_fields   = 36 个老富字段
openapi_fields= 5 段(events/modules/reference_file_refs/task/task_detail)
only_in_code  = 33 个老富字段 ← 真实 fallback 出口残留
```

**证据**(`transport/handler/task_detail.go` 当前 50 行):

```text
L31-43:  if h.r3Svc != nil { aggregate := h.r3Svc.Get(...); respondOK(c, aggregate); return }   ← fast-path 走 task_aggregator.AggregateDetail(5 段)
L44-49:  aggregate, _ := h.svc.GetByTaskID(...); respondOK(c, aggregate)                          ← fallback 走 domain.TaskDetailAggregate(36 字段)
```

生产 `r3Svc != nil` · 实际只走 fast-path 给前端 5 段 · 用户实测对齐 OpenAPI。
但 fallback 出口仍存在 · 一旦 `cmd/server/main.go` 配置/运行错误使 r3Svc 为 nil(或某天有人改回退路径)· 生产将返回 36 字段而 OpenAPI 只声明 5 字段 → 前端崩。

V1.2-C audit 工具的 `lastRespondExpr` 取**最后一个** `respondOK` 调用,因此反推到 fallback 路径的 `domain.TaskDetailAggregate`,把这条 critical 风险显式化暴露。

## §1 baseline 锚 SHA(P0 校验 · 不一致 ABORT)

```
transport/handler/task_detail.go               b8636965bda71004143bb968263080c0d737047db84f953a2a721c3d77a1d603
docs/api/openapi.yaml                          80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f
domain/task_detail_aggregate.go                315aef20dc7e34ad3233bf8f3e6bf8ae8e7477103586856d494a8c9e62bb82f0
service/task_aggregator/detail_aggregator.go   6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e
tools/contract_audit/main.go                   fc86c550622c3fcdbcd59beca8fe08e7a44b1fecd33c3c9f42dc116ac9f6455d
transport/http.go                              9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396
domain/task.go                                 658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b
service/identity_service.go                    00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644
```

## §2 ABORT 触发(任意一项立即 ABORT)

| # | 触发条件 | 行为 |
|---|---|---|
| 1 | §1 任一 SHA 漂移 | ABORT |
| 2 | `domain/task_detail_aggregate.go` 内容修改 | ABORT(其他 handler 在用) |
| 3 | `service.TaskDetailAggregateService` interface 在 `service/` 内被改/删 | ABORT(`TaskHandler` 在用) |
| 4 | `docs/api/openapi.yaml` 修改 | ABORT |
| 5 | `docs/frontend/*.md` 修改 | ABORT |
| 6 | `transport/http.go` 修改 | ABORT(路由表不动 · NewRouter 签名不变) |
| 7 | 修改 `service/task_aggregator/detail_aggregator.go` | ABORT |
| 8 | `cmd/server/main.go` 中 `task_aggregator.NewDetailService` 构造逻辑被删/改 | ABORT(只允许构造时若 r3 != nil 在 NewTaskDetailHandler 之前传入) |
| 9 | 修复后 `tools/contract_audit` 跑出的 `GET /v1/tasks/:id/detail` verdict ≠ `clean` | ABORT |
| 10 | 修复后 `summary.drift` ≥ 72(应该减少至少 1) | ABORT |
| 11 | 修复后 `go test ./...` 任一包 FAIL | ABORT |
| 12 | 任何 `git push` 命令 | ABORT |

## §3 实现计划(单 P · 单 commit)

### P1 改 `transport/handler/task_detail.go`

**目标**:删除 fallback 出口 · 删除 `svc` 字段 · 删除 `SetR3DetailService` setter · 改 `NewTaskDetailHandler` 单参数构造。

修改后**完整文件**目标(50 → ~30 行):

```go
package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service/task_aggregator"
)

type TaskDetailHandler struct {
	r3Svc *task_aggregator.DetailService
}

func NewTaskDetailHandler(r3Svc *task_aggregator.DetailService) *TaskDetailHandler {
	return &TaskDetailHandler{r3Svc: r3Svc}
}

// GetByTaskID handles GET /v1/tasks/:id/detail
// 返回 V1.1-A1 fast-path 5 段 schema(task / task_detail / modules / events / reference_file_refs).
func (h *TaskDetailHandler) GetByTaskID(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	if h.r3Svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task detail aggregate service not configured", nil))
		return
	}
	aggregate, err := h.r3Svc.Get(c.Request.Context(), taskID)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil))
		return
	}
	if aggregate == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	respondOK(c, aggregate)
}
```

### P2 改调用方

#### P2.1 `cmd/server/main.go`

定位 `NewTaskDetailHandler(taskDetailSvc)` 调用(预计在 v1.21 baseline 行号附近),改为传 `r3DetailSvc`:

- 调用前必须保证 `r3DetailSvc` 已构造(查 `task_aggregator.NewDetailService(...)` 调用,确保它在 `NewTaskDetailHandler` 之前完成)
- 把 `taskDetailHandler := handler.NewTaskDetailHandler(taskDetailSvc)` 改为 `taskDetailHandler := handler.NewTaskDetailHandler(r3DetailSvc)`
- **删除**紧随其后的 `taskDetailHandler.SetR3DetailService(r3DetailSvc)` 行(已在构造时传入)
- **不删** `taskDetailSvc := service.NewTaskDetailAggregateService(...)`(`TaskHandler` 仍依赖)

如果 cmd/server/main.go 中 `r3DetailSvc` 在 `NewTaskDetailHandler` 之后才构造,则将 `r3DetailSvc` 构造前移,使其在 `NewTaskDetailHandler(r3DetailSvc)` 之前完成。

#### P2.2 `cmd/api/main.go`

同上(line ~229 附近)。如果 `cmd/api/main.go` 没有构造 `r3DetailSvc`(即 cmd/api 是个 v0.9 老 entrypoint),则有 2 种合规做法:

- (a) 镜像 cmd/server 的 r3DetailSvc 构造,加一份到 cmd/api。
- (b) 如果 cmd/api 已废弃(查 `prompts/V1_NEXT_MODEL_ONBOARDING.md` 与 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 中是否提到 cmd/api 是 deprecated),则 cmd/api 中也走 NewTaskDetailHandler(r3DetailSvc),前提是构造逻辑齐全。
- 如果 cmd/api 同时使用老 fallback,**ABORT** 并报告 cmd/api 现状,等架构师决断是否删除 cmd/api。

### P3 改 handler 测试(如有)

- `transport/handler/task_detail_*_test.go`(如存在)`NewTaskDetailHandler` 调用相应改成单参数。
- 任何调用 `SetR3DetailService` 的测试,删除该行,改在构造时传入。

## §4 verify 矩阵(13 项)

| # | check | 期望 |
|---|---|---|
| 1 | §1 baseline SHA 校验(除允许的 task_detail.go + 2 cmd 之外) | 0 漂移 |
| 2 | `go vet ./...` | exit 0 |
| 3 | `go build ./...` | exit 0 |
| 4 | `go test ./tools/contract_audit/... -count=1` | PASS |
| 5 | `go test ./transport/handler/... -count=1` | PASS |
| 6 | `go test ./service/task_aggregator/... -count=1` | PASS |
| 7 | `go test ./... -count=1` | PASS(忽略需要 DSN 的 integration 测试 skip) |
| 8 | `go run ./tools/contract_audit --transport transport/http.go --handlers transport/handler --domain domain --openapi docs/api/openapi.yaml --output tmp/v1_2_d_1_audit.json --markdown tmp/v1_2_d_1_audit.md` exit 0 | PASS |
| 9 | `tmp/v1_2_d_1_audit.json` 中 `GET /v1/tasks/:id/detail` verdict | `clean` |
| 10 | 同上 `code_fields` | 5 段(events/modules/reference_file_refs/task/task_detail · 顺序按 sort) |
| 11 | 同上 `only_in_code + only_in_openapi` | 0 + 0 |
| 12 | 整体 `summary.drift` | 71(原 72 - 1) |
| 13 | 整体 `summary.clean` | 85(原 84 + 1) |
| 14 | `docs/api/openapi.yaml` SHA | 不变 `80730ec3...` |
| 15 | `docs/frontend/V1_API_TASKS.md` SHA | 不变 |
| 16 | `transport/http.go` SHA | 不变 `9a6d194b...` |
| 17 | `cmd/server/main.go` git diff 净影响 | 仅 NewTaskDetailHandler 1 行 + 删 SetR3DetailService 1 行 + 可能的 r3DetailSvc 构造行序调整(无新逻辑) |

## §5 落盘报告

新建 `docs/iterations/V1_2_D_1_REPORT.md` 含 9 段:

1. date / terminator / scope
2. baseline SHA(§1)+ 改后 SHA
3. handler 修改对照(P1 之前/之后,行数 50→~30)
4. cmd/server/main.go 修改对照(P2.1 行号 + diff 摘要)
5. cmd/api/main.go 修改对照(P2.2 决定:同步/废弃/ABORT)
6. handler tests 修改清单(P3)
7. tools/contract_audit before-after summary diff(drift 72→71 / clean 84→85 / `/v1/tasks/:id/detail` both_diff→clean)
8. verify 矩阵 17 项结果
9. terminator `V1_2_D_1_FALLBACK_REMOVED`

并同步更新:

- `docs/iterations/V1_RETRO_REPORT.md §18 V1.2-D 候选`:把 CRITICAL 行改为 CLOSED + 引用 V1_2_D_1_REPORT.md
- `prompts/V1_ROADMAP.md`:追加 v44 行(V1.2-D-1 fallback removal)

## §6 严禁

- ❌ 修改 `domain/task_detail_aggregate.go`
- ❌ 修改 `domain.TaskDetailAggregate` 字段或 `service.TaskDetailAggregateService` interface
- ❌ 修改 `service/task_aggregator/detail_aggregator.go`
- ❌ 修改 OpenAPI / 16 frontend doc
- ❌ 修改 `transport/http.go`
- ❌ 删除 `service.NewTaskDetailAggregateService(...)` 构造(`TaskHandler` 还在用)
- ❌ `git push`
- ❌ 写入 `[contract-skip-justified]` 跳 CI guard(本轮**应该**让 contract-guard 通过 — code 改了,OpenAPI 没改,但 contract_audit 真三向 diff 应判 `/v1/tasks/:id/detail` clean,因此 hook 应放行;若 hook 阻断,改 hook 是范围外,先改代码让 audit 真 clean,再让 hook 自然放行)

## §7 终止符

完成后输出 `V1_2_D_1_FALLBACK_REMOVED` 加 §4 verify 矩阵 17 项结果摘要(每项一行),不自签 PASS,等待架构师独立 verify。

架构师 verify 通过后改签 `V1_2_D_1_DONE_ARCHITECT_VERIFIED`,并由架构师把 V1_RETRO §18 CRITICAL 行 close。
