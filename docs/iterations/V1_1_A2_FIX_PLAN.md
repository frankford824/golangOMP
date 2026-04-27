# V1.1-A2 · Fix Plan

> 生成时间:2026-04-27T04:02:17Z
> 来源: docs/iterations/V1_1_A2_DRIFT_INVENTORY.md
> 原则: code wins · 不改 Go · 不改 path/method/parameters/RBAC/status code

## §1 修订裁决

默认裁决:OpenAPI 跟当前 git HEAD 实现走。`transport/http.go` 决定挂载,handler `respondOK`/`c.JSON` 实参和 Go struct `json` tag 决定字段名。

例外裁决:无。本轮没有发现应改 Go struct 而不改 OpenAPI 的 P0/P1 项。

## §2 P0/P1 修订动作

| 状态 | inventory | path | method | schema 动作 | 具体修改 | frontend family |
|---|---|---|---|---|---|---|
| [done] | P0-1 | `/v1/tasks/{id}/detail` | GET | 新增 `TaskAggregateDetailV2` 并切换 200 data ref | 修改该 path summary/description/200 description;保留 tags/x-api-readiness/x-rbac-placeholder/parameters/status code;`data` 改 `$ref: '#/components/schemas/TaskAggregateDetailV2'` | `docs/frontend/V1_API_TASKS.md`, `V1_API_CHEATSHEET.md` |
| [done] | P1-1 | `/v1/tasks/{id}/product-info` | PATCH | 重写 `TaskDetail` component | `TaskDetail` 从旧 R1 富详情/模块视图改为 `domain.TaskDetail` json tag 字段集;保留被 PATCH product-info/cost-info/business-info 成功响应复用 | `docs/frontend/V1_API_TASKS.md` |
| [done] | P1-2 | `/v1/tasks/{id}/cost-info` | PATCH | 复用上条 `TaskDetail` component | 同 P1-1 | `docs/frontend/V1_API_TASKS.md` |
| [done] | P1-3 | `/v1/tasks/{id}/business-info` | PATCH | 复用上条 `TaskDetail` component | 同 P1-1 | `docs/frontend/V1_API_TASKS.md` |
| [done] | P1-4 | `/v1/tasks/{id}/assign` | POST | 重写 `Task` component | `Task` 对齐 `domain.Task`:删除 phantom `workflow_lane`,新增 `is_outsource` | `docs/frontend/V1_API_TASKS.md`, `V1_API_CHEATSHEET.md` |
| [done] | P1-5 | `/v1/tasks/{id}/warehouse/prepare` | POST | 复用上条 `Task` component | 同 P1-4 | `docs/frontend/V1_API_TASKS.md` |

## §3 新增/修改 components.schemas

新增:

- `TaskAggregateDetailV2`: V1.1-A1 fast-path 5-section response body.
- `TaskAggregateModule`: `domain.TaskModule` + `task_aggregator.ModuleDetail` extra fields.
- `TaskModuleEvent`: `domain.TaskModuleEvent` json tag 字段集.
- `ReferenceFileRefFlat`: `domain.ReferenceFileRefFlat` json tag 字段集.

修改:

- `Task`: 对齐 `domain.Task`.
- `TaskDetail`: 对齐 `domain.TaskDetail`.

删除:

- 无。旧 `TaskModule`/相关 R1 module schema 本轮不删除,避免无 inventory 的 orphan cleanup 扩散;它们如果后续确认为孤儿,转 V1.2 单独清理。

## §4 修订顺序

1. 修改 `Task` component。
2. 修改 `TaskDetail` component。
3. 新增 `TaskAggregateModule` / `TaskModuleEvent` / `ReferenceFileRefFlat` / `TaskAggregateDetailV2`。
4. 修改 `/v1/tasks/{id}/detail` 200 schema ref 与描述。
5. 跑 OpenAPI validate 和 203 path/501/Go status 守卫。
6. 基于新 OpenAPI 更新 frontend docs。
7. 同步治理文档与 retro。

## §5 P2/defer-v12

| inventory | 项 | 处理 |
|---|---|---|
| P2-1 | prompt regex 漏 `/v1/tasks/batch-create/template.xlsx` | 本轮不改 path;记录给 V1.2 工具脚本修正 |
| P2-2~P2-5 | identity_service.go post-v1.21 micro-drift potential | 本轮不部署不改 Go;V1.2 重新构建 v1.22 对齐线上与 git HEAD |

## §6 预估 OpenAPI 行数变化

- 新增 schema:4 个,约 +110 行。
- 修改 schema:2 个,约 -10~+20 行。
- 修改 path 描述:1 处,约 +8 行。
- 预计总变化:+90~+140 行。

## §7 受影响 frontend doc family

- `docs/frontend/V1_API_TASKS.md`
- `docs/frontend/V1_API_CHEATSHEET.md`
- `docs/frontend/INDEX.md`
- 其他 13 份 family doc 仅追加 V1.1-A2 revision marker,文件名集合不变。
