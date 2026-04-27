# V1.3-A1.1 · `tasks.priority` 五方契约漂移修复（执行 prompt）

- **版本**: v1.0
- **日期**: 2026-04-27 PT
- **被修复对象**: V1.3-A1 Issue 3 `POST /v1/tasks 500 INTERNAL_ERROR`
- **真实 root cause**（用户提供 trace `83ba7d26-385b-4bea-99b7-db0925be2975` 后已锁死）:`tasks.priority` 字段在 5 个权威源之间漂移 — DB CHECK 4 值 vs Go enum/OpenAPI/frontend docs 的 5 值（多 `urgent`）
- **修复方案**: **方案 B** · 代码 / OpenAPI / frontend docs 删 `urgent` · DB 不动（已签字 + v1.21 部署 + 真生产 0 行 `urgent`）
- **执行模型**: codex（你）

---

## §0 不可越线的硬门

1. **DB / migration 严禁改动** — `db/migrations/**` 完全只读。`067_v1_0_tasks_priority_constraint.sql` 是 SoT。
2. **架构 SoT 严禁改动** — `docs/V1_MODULE_ARCHITECTURE.md` / `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 完全只读。
3. **`transport/http.go` 不应改动**（路由层不涉及 enum 校验，纯 routing）— SHA 锚 `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396` 必须不漂移。
4. **每阶段独立 commit**（V1.2-D-2 教训登记）— 不允许把 P1~P5 合并成单 commit。
5. **测试必跑**: `go vet ./...`、`go build ./...`、`go test ./... -count=1`。`go test -race` 因 Windows Go in WSL CGO_ENABLED=0 已知不可跑，跳过。
6. 任何 ABORT 触发条件未明示满足，立刻停下、落 ABORT 报告、等架构师裁决。**禁止自签 PASS**。

---

## §1 baseline SHA 校验（P0 必跑）

P0 第一步必须校：

| 文件 | 期望 SHA | 校验方式 |
|---|---|---|
| `transport/http.go` | `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396` | sha256sum |
| `docs/V1_MODULE_ARCHITECTURE.md` | 由你跑 sha256sum 记录到 baseline log，结束时确认不漂移 | sha256sum |
| `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` | 同上 | sha256sum |
| `db/migrations/067_v1_0_tasks_priority_constraint.sql` | 同上 | sha256sum |
| `cmd/tools/migrate_v1_forward/main.go` | 同上 | sha256sum |
| `cmd/tools/migrate_v1_backfill/phases.go` | 同上 | sha256sum |

baseline log 落 `tmp/v1_3_a1_1_baseline_sha.log`。任意一项漂移立刻 ABORT。

---

## §2 修复点清单（5 处 · 5 commit）

### P1 · 删 Go enum 漂移源 → commit 1

文件: `domain/enums_v7.go`

```go
// 当前（违规）
const (
    TaskPriorityLow      TaskPriority = "low"
    TaskPriorityNormal   TaskPriority = "normal"
    TaskPriorityHigh     TaskPriority = "high"
    TaskPriorityCritical TaskPriority = "critical"
    TaskPriorityUrgent   TaskPriority = "urgent"   // ← 删除这一行
)

// 目标（合规，与 DB CHECK 一致）
const (
    TaskPriorityLow      TaskPriority = "low"
    TaskPriorityNormal   TaskPriority = "normal"
    TaskPriorityHigh     TaskPriority = "high"
    TaskPriorityCritical TaskPriority = "critical"
)
```

**关联检查**: 全仓 grep `TaskPriorityUrgent` 必须返 0 引用。如果有任何 service/handler/repo 还在使用 `TaskPriorityUrgent` 常量，必须同步删除（不要替换为 `TaskPriorityCritical`，让编译器报错暴露所有引用点，再逐个评估正确语义；正常情况下应该是 0 引用，因为 R1.6 已签字 4 值）。

**commit message**: `fix(domain): remove urgent from TaskPriority enum to align with DB SoT (4-value)`

### P2 · 后端硬化:MySQL 3819 转 400 + Domain validate → commit 2

文件: `service/task_service.go`

`mapTaskCreateTxError` 当前只识别 `1062 (duplicate key)`，遇到 `3819 (CHECK constraint violated)` 退化为 INTERNAL_ERROR。改为:

```go
// pseudocode - exact import path follow existing
import "github.com/go-sql-driver/mysql"

func mapTaskCreateTxError(err error) *domain.AppError {
    var mErr *mysql.MySQLError
    if errors.As(err, &mErr) {
        switch mErr.Number {
        case 1062:
            return domain.NewAppError(domain.ErrCodeInvalidRequest, "task already exists", nil)
        case 3819:
            // CHECK constraint violated. Most likely an enum drift between code/contract
            // and DB CHECK. Surface as 400 with a stable error code so frontend can react.
            return domain.NewAppError(domain.ErrCodeInvalidRequest,
                "task field violates DB constraint: "+mErr.Message, nil)
        }
    }
    // existing fallback path unchanged
    return nil // or whatever existing default branch returns
}
```

**约束**:
- 不得引入新的 ErrCode；用现有 `ErrCodeInvalidRequest`
- error message 可以含 `mErr.Message`（含 constraint 名 `chk_tasks_priority_v1`），但**不得**含完整 SQL 或 stack
- 同时在 `transport/handler/task.go::Create` 的 entry 处加 `priority` 字段 domain validate（白名单 4 值）— 这是更早一层的拦截，DB-level 拦截是兜底
  - 在已有 `validateCreateTaskProductSelectionWhitelist` 同模式新加 `validateCreateTaskPriority(p string) (string, *domain.AppError)`，仅允许 `{"", "low", "normal", "high", "critical"}`（空字符串视为默认 normal）
  - 在 `taskH.Create` 的 validation 段调用，违规返 400 `task_priority_invalid`

**commit message**: `fix(service): map MySQL 3819 CHECK violation to 400 + domain-level priority validate`

### P3 · OpenAPI 5 值 → 4 值 → commit 3

文件: `docs/api/openapi.yaml`

L3915 当前:
```yaml
enum: [low, normal, high, urgent, critical]
```

目标:
```yaml
enum: [low, normal, high, critical]
```

**全文搜索** `urgent` 必须确认只在 L3915 一处出现（如果有其它地方再出现，全部去掉，但不要误删跟 priority 无关的 `urgent` 字符串）。

校验:
```bash
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
# 必须 0 error 0 warning
```

**commit message**: `fix(openapi): drop urgent from priority enum to match DB CHECK 4-value`

### P4 · frontend docs 重生 → commit 4

不要手改 `docs/frontend/V1_API_TASKS.md`。**重跑** `scripts/docs/generate_frontend_docs.py`（V1.2-D-2 已统一生成器路径），让它从更新后的 OpenAPI 重生 16 份 frontend docs。

```bash
python scripts/docs/generate_frontend_docs.py
# 或 python3，按你环境
```

校验:
- `docs/frontend/V1_API_TASKS.md` 全文 grep `urgent` 必须 0 hit
- `docs/frontend/V1_API_TASKS.md:184` priority 行应改成 `enum(low/normal/high/critical)` 4 值
- 全 16 份 frontend docs 内 grep `urgent.*priority|priority.*urgent` 必须 0 hit
- INDEX.md 修订历史追加一行（如果生成器自动加，不必手改）

**commit message**: `docs(frontend): regenerate API docs after priority enum 4-value alignment`

### P5 · 治理 + 验证 → commit 5

落:
- `docs/iterations/V1_3_A1_1_PRIORITY_DRIFT_FIX_REPORT.md` 报告（含 root cause 链路、5 处修复点、测试结果、audit 回归、SHA 锚 verify、未自签 PASS）
- `prompts/V1_ROADMAP.md` 加 v57 行
- `docs/iterations/V1_RETRO_REPORT.md` 把 V1.3-A1 状态升级为 PARTIALLY-CLOSED-PENDING-A1.1-VERIFY → CLOSED-A1.1-PENDING-ARCHITECT-VERIFY；同时新登记一项 V1.3 工具补强债 **Q-V1.3-T4 audit 工具不识别 enum value drift**:
  - 现状:`tools/contract_audit/main.go` 只比 struct field 名，不比 enum value
  - 影响:这次 priority 漂移五方源都对得上 field 名（都叫 `priority`），但值集不同，audit `verdict=clean` 假阴性
  - V1.3 工具升级:对 OpenAPI `enum: [...]` 与 Go `const` block 做交叉对账，加 `verdict=enum_value_drift`，同时把 DB CHECK 解析进 audit 三方变四方
- 跑独立 audit 重新生成 `tmp/v1_3_a1_1_audit.json` 确认 `summary.drift == 0` 不退化
- `--fail-on-drift true` exit=0

**commit message**: `docs(governance): V1.3-A1.1 priority drift fixed - report + ROADMAP v57 + RETRO updates`

---

## §3 测试必跑（P5 内）

```bash
go vet ./...                                                    # 必 PASS
go build ./...                                                   # 必 PASS
go test ./domain/... -count=1                                    # 必 PASS
go test ./service/... -count=1                                   # 必 PASS（task_service mapTaskCreateTxError 单测必须有覆盖 3819 case，没有就加 1 个）
go test ./transport/handler/... -count=1                         # 必 PASS（task.go validate priority 单测必须新加 1 个）
go test ./tools/contract_audit/... -count=1                      # 必 PASS
go test ./... -count=1                                           # 必 PASS（耗时长可拆分子目录跑）

go run ./cmd/tools/openapi-validate docs/api/openapi.yaml        # 必 0 error 0 warning

go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_3_a1_1_audit.json \
  --markdown tmp/v1_3_a1_1_audit.md \
  --fail-on-drift true                                           # 必 exit=0
```

测试新增清单（必须新加，否则修复无回归保护）:
1. `service/task_service_priority_test.go`（或既有 `*_test.go` 文件加 case）— 包含一个 3819 MySQLError 的 fake，断言 `mapTaskCreateTxError` 返 `ErrCodeInvalidRequest`
2. `transport/handler/task_priority_validate_test.go`（或既有 `task_*_test.go` 加 case）— 包含 5 case:`""` / `"low"` / `"normal"` / `"high"` / `"critical"` 全 PASS，`"urgent"` / `"random"` / `"LOW"` 全返 400

---

## §4 ABORT 触发条件

任意一项触发立刻停下、落 `docs/iterations/V1_3_A1_1_ABORT_REPORT.md`、等架构师裁决，禁止继续:

1. P0 SHA baseline 任意一项漂移
2. P1 grep `TaskPriorityUrgent` 在删除后仍有引用，且引用点不是 `urgent` 字面值（说明语义复杂超出本轮 scope）
3. P2 已存在的 `mapTaskCreateTxError` 函数体与 prompt 描述差异显著（说明真实代码与诊断时不一致，需要架构师重新评估）
4. P3 OpenAPI grep `urgent` 出现在 priority enum 之外的地方（说明有其它字段也用 `urgent` 字面值，需要架构师评估是否独立处置）
5. P4 frontend docs 生成器报错或生成结果与 OpenAPI 不一致
6. P5 audit `summary.drift > 0` 或 `--fail-on-drift true exit != 0`
7. 任何测试失败
8. `transport/http.go` SHA 漂移
9. DB migration / 架构 SoT 任何文件被 stage 或修改

---

## §5 commit 顺序与终止符

**5 个独立 commit**（不允许合并）:

```text
1. fix(domain): remove urgent from TaskPriority enum to align with DB SoT (4-value)
2. fix(service): map MySQL 3819 CHECK violation to 400 + domain-level priority validate
3. fix(openapi): drop urgent from priority enum to match DB CHECK 4-value
4. docs(frontend): regenerate API docs after priority enum 4-value alignment
5. docs(governance): V1.3-A1.1 priority drift fixed - report + ROADMAP v57 + RETRO updates
```

完成后输出**唯一终止符**:

```text
V1_3_A1_1_PRIORITY_DRIFT_FIXED_PENDING_ARCHITECT_VERIFY
```

不自签 PASS。等架构师独立 verify。

---

## §6 verify 提交清单（给架构师参考）

完成后，架构师 verify 关注:
- 5 个 commit 顺序与 message 是否符合
- `domain/enums_v7.go` `TaskPriorityUrgent` 已删 + 全仓 0 引用
- `service/task_service.go` 3819 → 400 链路 + handler 层 domain validate
- OpenAPI L3915 改成 4 值
- 16 份 frontend docs 0 `urgent` 命中
- audit `summary.drift == 0` 没退化
- 测试套件全 PASS（含新增 priority 测试覆盖）
- 6 锚 SHA 全部不漂移
- 治理三件套（report / ROADMAP / RETRO）已落 + Q-V1.3-T4 已登记
- 终止符正确

---

立即开始 P0。
