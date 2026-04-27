# V1 · R3.5 · 集成验证轮(jst_ecs 测试库)

> 版本:**v1**(2026-04-17)
> 状态:**待 Codex 执行**
> 依赖:R3 v1 代码已落地(见 `docs/iterations/V1_R3_REPORT.md`);R2 v3 生产落地完成;`jst_ecs` 上 `/root/ecommerce_ai/backups/20260424T024501Z_r2_pre_backfill.sql.gz` 可用
> 触发原因:R3 报告里 CAS 100 线程测试使用 `fakeModuleRepo.ClaimCAS` = 原子 Bool 桩,**未经过 MySQL 真实 CAS 验证**;6 条集成断言因 `MYSQL_DSN` 缺失全部 skip。需要用一次真 MySQL 运行把 R3 的红灯转绿。
> 执行路径:**不在本地起 Docker**;在 `jst_ecs` 上新建**测试库 `jst_erp_r3_test`**,与生产 `jst_erp` 共实例但物理分库,靠连接串硬白名单护住。

---

## 0. 本轮目标(一句话)

> **在 `jst_ecs` 建一个独立测试库 `jst_erp_r3_test`,灌入 R2 pre-backfill 时刻的 `jst_erp` 快照,跑 R2 forward + backfill,然后用 R3 集成测试证明 CAS 原子性 + 6 断言 + OpenAPI 契约在真 MySQL 上成立;期间任何指向非 `*_r3_test` 库名的连接尝试必须 abort**。

## 1. 必读输入

1. `docs/iterations/V1_R3_REPORT.md` v1(本轮触发原因)
2. `prompts/V1_R3_ENGINE.md` v1 §6.3(6 条集成断言的精确定义)
3. `docs/V1_MODULE_ARCHITECTURE.md` v1.2 §7.2(CAS SQL 权威原文)+ §13 A5(验收项)
4. `docs/iterations/V1_R2_REPORT.md` v1 Backup Evidence(备份文件路径 + sha256)
5. `docs/iterations/V1_R1_6_PROD_ALIGN.md` v1 §2(Y1 + Z1 + P1 决策,本轮必须对齐)
6. `cmd/tools/migrate_v1_forward` / `migrate_v1_backfill`(在测试库上重跑一次)
7. `service/task_pool/claim_cas_concurrent_test.go`(现有桩版,本轮需加 integration 版)

## 2. 安全硬约束(全部必须实现,任一违反 abort)

### 2.1 DSN 白名单守卫

**所有 Go 工具 + 所有 integration test 在 open connection 之前必须跑**:

```go
// 伪码 · 放到 cmd/tools/internal/v1migrate/dsn_guard.go 或等价位置
// 所有 R3.5 相关 test / tool 必须调用此函数
func guardR35DSN(dsn string) error {
    cfg, err := mysql.ParseDSN(dsn)
    if err != nil { return err }
    if !strings.HasSuffix(cfg.DBName, "_r3_test") {
        return fmt.Errorf(
            "R3.5 safety violation: DSN points to %q, database name must end with '_r3_test'",
            cfg.DBName)
    }
    return nil
}
```

- 入口:`migrate_v1_forward` / `migrate_v1_backfill` / `migrate_v1_rollback` 在 `--r35-mode` 参数开启时启用守卫;integration test **全部**硬引用守卫
- 违反 → `os.Exit(4)` + 明确错误消息
- 守卫单测:传 `...jst_erp?...` 必须返回 error;传 `...jst_erp_r3_test?...` 必须返回 nil

### 2.2 所有写入端点在 R3.5 期间强制启用守卫

- R3.5 的 integration test 必须通过 env 变量(如 `R35_MODE=1`)开启守卫
- 守卫开启时,任何走 `repo.OpenDB(dsn)` 的路径都必须预校验 dsn;预校验未通过 **panic**,不允许降级

### 2.3 备份前置

- 在测试库上跑 forward/backfill 之前,先对 `jst_erp_r3_test` 本身跑一次 dump(证明即使是测试库也有回滚手段):
  `mysqldump --single-transaction --databases jst_erp_r3_test | gzip > /root/ecommerce_ai/backups/<ts>_r35_pre_test.sql.gz`
- 预期 size 较小(仅测试库);不限制下限

### 2.4 严禁触碰生产

- 禁止任何 `DSN` / `mysql` 命令包含 `jst_erp`(不带后缀)
- 禁止跨库 SELECT / INSERT / UPDATE(例如 `FROM jst_erp.tasks`)
- R3.5 跑完**不 DROP** 测试库(留作 R4 subagent 参考);但必须在报告里留下清理命令

## 3. 交付范围

### 3.1 建测试库脚本(shell)

新增 `scripts/r35/setup_test_db.sh`(在 `jst_ecs` 上执行):

```bash
#!/usr/bin/env bash
set -euo pipefail
cd /root/ecommerce_ai
. ./shared/main.env
export MYSQL_PWD="$DB_PASS"

TEST_DB="jst_erp_r3_test"
PROD_DUMP="/root/ecommerce_ai/backups/20260424T024501Z_r2_pre_backfill.sql.gz"

if [[ ! -f "$PROD_DUMP" ]]; then
  echo "missing $PROD_DUMP"; exit 2
fi

echo "== 1. (Re)create $TEST_DB =="
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e "DROP DATABASE IF EXISTS \`$TEST_DB\`;"
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e "CREATE DATABASE \`$TEST_DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

echo "== 2. Restore dump, rewriting DB name =="
# 关键:sed 把 `jst_erp` 所有反引号引用改写为 `jst_erp_r3_test`
# mysqldump 出的文件里 DB 引用形式是反引号包裹
gunzip -c "$PROD_DUMP" | \
  sed 's/`jst_erp`/`jst_erp_r3_test`/g' | \
  mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER"

echo "== 3. Sanity checks =="
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" "$TEST_DB" -N -B -e "
  SELECT 'tasks' tbl, COUNT(*) FROM tasks
  UNION ALL SELECT 'task_details', COUNT(*) FROM task_details
  UNION ALL SELECT 'task_assets', COUNT(*) FROM task_assets
  UNION ALL SELECT 'asset_storage_refs', COUNT(*) FROM asset_storage_refs
  UNION ALL SELECT 'users', COUNT(*) FROM users
  UNION ALL SELECT 'customization_jobs', COUNT(*) FROM customization_jobs;"

echo "== 4. Pre-test backup =="
TS=$(date -u +%Y%m%dT%H%M%SZ)
mysqldump -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" --single-transaction --databases "$TEST_DB" \
  | gzip > "/root/ecommerce_ai/backups/${TS}_r35_pre_test.sql.gz"
echo "backup: /root/ecommerce_ai/backups/${TS}_r35_pre_test.sql.gz"

echo "DONE: $TEST_DB ready"
```

### 3.2 测试库 DSN 注入脚本

新增 `scripts/r35/build_test_dsn.sh`(由本地机器 ssh 调用,**不持久化凭证**):

```bash
#!/usr/bin/env bash
# 在 jst_ecs 上运行,输出 DSN 字符串到 stdout
set -euo pipefail
cd /root/ecommerce_ai
. ./shared/main.env
echo "${DB_USER}:${DB_PASS}@tcp(${DB_HOST}:${DB_PORT})/jst_erp_r3_test?parseTime=true&multiStatements=true"
```

### 3.3 DSN 守卫代码

新增 `cmd/tools/internal/v1migrate/dsn_guard.go`:

```go
package v1migrate

import (
    "fmt"
    "strings"
    "github.com/go-sql-driver/mysql"
)

// GuardR35DSN 拒绝任何非 *_r3_test 数据库连接。
// R3.5 的 forward / backfill / integration test 必须先过这道闸。
func GuardR35DSN(dsn string) error {
    cfg, err := mysql.ParseDSN(dsn)
    if err != nil {
        return fmt.Errorf("parse DSN: %w", err)
    }
    if !strings.HasSuffix(cfg.DBName, "_r3_test") {
        return fmt.Errorf(
            "R3.5 safety violation: DSN database %q must end with '_r3_test'",
            cfg.DBName)
    }
    return nil
}
```

附带单测 `dsn_guard_test.go`:覆盖生产库名 → error;测试库名 → nil;畸形 DSN → error。

### 3.4 迁移工具加 `--r35-mode` 参数

`cmd/tools/migrate_v1_forward/main.go` 和 `cmd/tools/migrate_v1_backfill/main.go` 解析参数时,**若 `--r35-mode` 为 true**,在 `OpenDB` 前调用 `GuardR35DSN(dsn)`。守卫失败直接 `os.Exit(4)`。

### 3.5 集成测试(真 MySQL)

#### 3.5.1 公共测试辅助

新增 `testsupport/r35/setup.go`:

```go
// 伪码
package r35

import (
    "database/sql"
    "os"
    "strings"
    "testing"

    "workflow/cmd/tools/internal/v1migrate"
)

// MustOpenTestDB 打开测试库,若 DSN 不是 *_r3_test 则 t.Fatalf。
func MustOpenTestDB(t *testing.T) *sql.DB {
    t.Helper()
    dsn := os.Getenv("MYSQL_DSN")
    if dsn == "" {
        t.Skip("MYSQL_DSN not set")
    }
    if err := v1migrate.GuardR35DSN(dsn); err != nil {
        t.Fatalf("R35 guard failed: %v", err)
    }
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        t.Fatalf("open DB: %v", err)
    }
    if err := db.Ping(); err != nil {
        t.Fatalf("ping: %v", err)
    }
    return db
}

// TruncateR2Tables 清空 R2 的 7 张表,便于每个测试独立重跑 backfill
func TruncateR2Tables(t *testing.T, db *sql.DB) { ... }
```

#### 3.5.2 100 线程 CAS 真 MySQL 测试

新增 `service/task_pool/claim_cas_mysql_integration_test.go`(build tag `integration`):

```go
//go:build integration

package task_pool

import (
    "context"
    "sync"
    "sync/atomic"
    "testing"

    "workflow/testsupport/r35"
    // 真 MySQL repo
)

func TestClaimCAS_100Concurrent_MySQL(t *testing.T) {
    db := r35.MustOpenTestDB(t)
    defer db.Close()

    // 1. 准备一条 pending_claim 的 module(INSERT 或 UPDATE 某个 backfill 产生的 module 到 pending_claim)
    taskID, moduleKey := setupPendingClaimModule(t, db)
    defer restoreModuleState(t, db, taskID, moduleKey)

    // 2. 真 CAS 服务实例
    svc := newRealClaimService(db)

    // 3. 100 goroutine 并发抢
    actors := prepareActors(t, db, 100) // 真实 user 行,都属 design_standard 组
    var successCount int64
    var conflictCount int64
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(actor domain.RequestActor) {
            defer wg.Done()
            dec := svc.Claim(context.Background(), actor, taskID, moduleKey, "design_standard")
            if dec.OK {
                atomic.AddInt64(&successCount, 1)
            } else if dec.DenyCode == "module_claim_conflict" {
                atomic.AddInt64(&conflictCount, 1)
            }
        }(actors[i])
    }
    wg.Wait()

    if successCount != 1 || conflictCount != 99 {
        t.Fatalf("real MySQL CAS: success=%d conflict=%d, want 1/99", successCount, conflictCount)
    }

    // 4. 真实数据库状态断言
    var finalState string
    var claimedBy int64
    db.QueryRow(`SELECT state, claimed_by FROM task_modules WHERE task_id=? AND module_key=?`,
        taskID, moduleKey).Scan(&finalState, &claimedBy)
    if finalState != "in_progress" { t.Fatalf("state=%s, want in_progress", finalState) }
    if claimedBy < 1 || claimedBy > 100 { t.Fatalf("claimed_by=%d out of range", claimedBy) }

    // 5. 真实事件表断言:恰好 1 条 claimed 事件
    var claimEvents int
    db.QueryRow(`SELECT COUNT(*) FROM task_module_events tme
                   JOIN task_modules tm ON tme.task_module_id = tm.id
                  WHERE tm.task_id=? AND tm.module_key=? AND tme.event_type='claimed'`,
        taskID, moduleKey).Scan(&claimEvents)
    if claimEvents != 1 { t.Fatalf("claimed events=%d, want 1", claimEvents) }
}
```

#### 3.5.3 6 条集成断言(严格对齐 R3 prompt §6.3)

新增 `service/task_pool/pool_query_integration_test.go`、`service/module_action/action_integration_test.go`、`service/task_cancel/cancel_integration_test.go`、`service/task_aggregator/detail_integration_test.go`、`service/task_aggregator/list_integration_test.go`(都 build tag `integration`):

| # | 断言 | 落点文件 |
| --- | --- | --- |
| 1 | `GET /v1/tasks/pool?pool_team_code=design_standard` 返回行**不含** `data.backfill_placeholder=true` 的模块 | pool_query_integration_test.go |
| 2 | 同一 module 两次 claim:第一次 OK,第二次 409 `module_claim_conflict` | claim_cas_mysql_integration_test.go(独立 test 函数) |
| 3 | `audit.approve` 执行后:audit state=`closed`,warehouse state=`pending_claim`,事件表出现 `approved` + `entered` 共 2 条 | action_integration_test.go |
| 4 | `POST /v1/tasks/{id}/cancel` with `reason=user_cancel`:`tasks.task_status='Cancelled'`,事件表出现 `task_cancelled` | cancel_integration_test.go |
| 5 | `GET /v1/tasks/{id}/detail` 返回 `modules[]` 数量 = blueprint 期望,每个 module 的 `visibility='visible'` | detail_integration_test.go |
| 6 | `GET /v1/tasks` 插入两条 priority=`low` / `high` 的任务,排序后 `high` 在 `low` 之前 | list_integration_test.go |

每条测试必须:
- 调用 `r35.MustOpenTestDB(t)`,自动 skip or abort
- 在开始前 snapshot 相关行,在结束后恢复(defer 清理),保证重跑幂等
- 若涉及对 R2 backfill 产出的 `migrated_from_v0_9` 事件冲突,测试数据用**新建 task**(INSERT 到 `task_id >= 10000` 段),避免污染 backfill 产出的历史任务

### 3.6 执行脚本

新增 `scripts/r35/run_verification.sh`(本地机器运行,ssh 到 jst_ecs):

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "== 1. Setup test DB on jst_ecs =="
ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/setup_test_db.sh'

echo "== 2. Build linux binaries =="
GOOS=linux GOARCH=amd64 /home/wsfwk/go/bin/go build -o /tmp/r2_forward  ./cmd/tools/migrate_v1_forward
GOOS=linux GOARCH=amd64 /home/wsfwk/go/bin/go build -o /tmp/r2_backfill ./cmd/tools/migrate_v1_backfill
scp /tmp/r2_forward /tmp/r2_backfill jst_ecs:/root/ecommerce_ai/r3_5/bin/
scp db/migrations/059_*.sql db/migrations/060_*.sql db/migrations/061_*.sql \
    db/migrations/062_*.sql db/migrations/063_*.sql db/migrations/064_*.sql \
    db/migrations/065_*.sql db/migrations/066_*.sql db/migrations/067_*.sql \
    db/migrations/068_*.sql jst_ecs:/root/ecommerce_ai/r3_5/sql/

echo "== 3. R2 forward + backfill on test DB =="
ssh jst_ecs 'cd /root/ecommerce_ai && . ./shared/main.env && \
  DSN="${DB_USER}:${DB_PASS}@tcp(${DB_HOST}:${DB_PORT})/jst_erp_r3_test?parseTime=true&multiStatements=true" && \
  /root/ecommerce_ai/r3_5/bin/r2_forward  --dsn="$DSN" --sql-dir=/root/ecommerce_ai/r3_5/sql --r35-mode=true && \
  /root/ecommerce_ai/r3_5/bin/r2_backfill --dsn="$DSN" --r35-mode=true'

echo "== 4. Run integration tests from local machine via SSH tunnel =="
# 方案 A:在 jst_ecs 上跑 go test(需要把源码 rsync 过去)
# 方案 B:本地跑 go test,连接串指向 jst_ecs 的 MySQL(需 SSH 隧道或 MySQL 放行 3306)
# 取 Codex 判断;若 jst_ecs 防火墙不放 3306,走方案 A

# 方案 A 示例:
# rsync -a --exclude='.git' ./ jst_ecs:/root/ecommerce_ai/r3_5/src/
# ssh jst_ecs 'cd /root/ecommerce_ai/r3_5/src && \
#   DSN=... R35_MODE=1 MYSQL_DSN="$DSN" /path/to/go test ./... -tags=integration -count=1 -v'

echo "DONE"
```

Codex 自行决定方案 A 还是方案 B,以**实际可行 + 不暴露生产凭证到本地日志**为准。

### 3.7 报告

`docs/iterations/V1_R3_5_INTEGRATION_VERIFICATION.md`,强制章节:

- `## DSN Guard Evidence`:守卫单测 PASS + 故意用生产 DSN 跑一次 → exit code 4 证明守卫生效
- `## Test DB Setup`:`jst_erp_r3_test` 各表行数(应与 R1.6 §1 快照 ± 自然漂移一致)
- `## R2 Forward on Test DB`:SHOW TABLES 证明 R2 七张表已建
- `## R2 Backfill on Test DB`:Phase A~E stats + source_module_key 分布(应与 V1_R2_REPORT.md §Backfill Stats 的 basic_info=9 / customization=5 / design=251 一致,允许 ± 自然漂移)
- `## CAS 100-Thread Real MySQL`:success/conflict = 1/99 + 终态 state = in_progress + claimed 事件恰好 1 条
- `## 6 Integration Assertions`:每条断言 PASS/FAIL 的实际数据证据
- `## OpenAPI Conformance`:与 R3 报告一致,保持 0 error 0 warning
- `## Cleanup Instructions`:给出 DROP DATABASE 命令(不执行),留给未来回收
- `## Production Touch Statement`:明确声明本轮所有写入均落 `jst_erp_r3_test`,未触碰 `jst_erp`

## 4. 严禁触碰(DO NOT TOUCH)

- `jst_erp` 生产库(读也不推荐,所有 SELECT 应指向 `jst_erp_r3_test`)
- R3 v1 已提交的生产代码(`service/` / `repo/` / `transport/` / `domain/`)**结构不动**;只允许加 `_integration_test.go` 和 `testsupport/` 新文件
- R2 migration 文件 059~068 不改
- OpenAPI 不改
- 任何写入指向生产的 DSN / 命令(守卫应确保这不可能)

## 5. 验收脚本(10 步)

```bash
# 1. DSN 守卫单测
/home/wsfwk/go/bin/go test ./cmd/tools/internal/v1migrate/... -run TestGuardR35DSN -v
#   断言:生产 DSN 返回 error;测试 DSN 返回 nil

# 2. 故意攻击:用 jst_erp 跑 forward(必须 abort)
ssh jst_ecs '/root/ecommerce_ai/r3_5/bin/r2_forward \
  --dsn="user:pass@tcp(host:3306)/jst_erp?parseTime=true" \
  --r35-mode=true; echo "exit=$?"'
#   断言:exit=4 + 日志含 "R3.5 safety violation"

# 3. 测试库 setup
ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/setup_test_db.sh'
#   断言:tasks ≈ 95~100(自然漂移)

# 4. R2 forward on test DB (with --r35-mode)

# 5. R2 backfill on test DB
#   断言:Phase A~E 全绿 + errors=0 + 分布与生产一致

# 6. 100 线程 CAS 真 MySQL 测试
MYSQL_DSN="..." R35_MODE=1 /home/wsfwk/go/bin/go test \
  ./service/task_pool/... -tags=integration -run TestClaimCAS_100Concurrent_MySQL -v -count=1
#   断言:success=1, conflict=99, state=in_progress, claimed events=1

# 7. 6 条集成断言
MYSQL_DSN="..." R35_MODE=1 /home/wsfwk/go/bin/go test \
  ./... -tags=integration -run "Integration" -v -count=1
#   断言:6 条全 PASS

# 8. 全量 go test(不带 integration tag,本机 / WSL 都可)
/home/wsfwk/go/bin/go test ./... -count=1
#   断言:仍 PASS

# 9. OpenAPI validate
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
#   断言:0/0

# 10. 生产健康探测(只读,确认未污染)
ssh jst_ecs 'bash /tmp/r2_probe_readonly.sh'
#   断言:与 R2 报告 post probe 相比,仅自然漂移(tasks/asset 增加),R2 目标表行数与 R2 post 一致 ± 0 (R3.5 没写 jst_erp)
```

## 6. 失败终止条件(Codex 必须 abort)

- DSN 守卫单测不通过
- 故意攻击测试未返回 exit=4
- `setup_test_db.sh` 后 `SHOW DATABASES` 看不到 `jst_erp_r3_test` 或行数与生产差距 > 20%
- 100 线程 CAS 出现 `success > 1` 或 `conflict + success != 100`
- 6 断言任一 FAIL
- 生产探测发现 R2 目标表行数变化(说明守卫漏了)

---

## 7. 给 Codex 的最后一句话

> R3 v1 的代码读起来对;缺的只是一次在真 MySQL 上的并发 + 行为压测。
> **守卫 + 测试库 + 6 断言**是唯一三件事,30~60 分钟做完。
> 任何一次想把 DSN 指到 `jst_erp`(不带后缀)的操作,**立刻 abort**,不要偷懒。
> 生产库零改动是本轮红线,R3.5 报告里必须**以生产 probe diff 作证据收尾**。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-17 | 初版 · 起因 R3 v1 集成断言与 100 线程 CAS 未经 MySQL 验证;方案 D2:jst_ecs 建 `jst_erp_r3_test` 测试库,DSN 白名单守卫 + `--r35-mode` 参数 + 7 个 integration test 文件 + 报告 |
