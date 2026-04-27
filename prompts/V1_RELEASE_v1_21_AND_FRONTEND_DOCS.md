## V1.1-A1 Release v1.21 · 切换正式库 + 删除测试库 + 前端联调完整接口文档

> 起草人:架构师(Claude Opus 4.7)
> 起草时间:2026-04-25
> 性质:**codex TUI / codex exec autopilot 单 prompt** · 5 阶段串行 · 强 ABORT 守卫
> 前置签字:`V1_1_A1_DONE_ARCHITECT_VERIFIED`
> 终止符(成功):`V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY`
> 用户决策:**"忽略数据风险"** — 仍强制保留 mysqldump 5 秒留底(不可绕过)

---

## §0 你是谁 · 你接的是什么

你是**接手 V1.1-A1 release 轮**的 codex。本轮你需要完成:

1. 把 V1.0(R1~R6.A.4) + V1.1-A1 后端代码打包为 **v1.21** 并部署到 jst_ecs 生产(替换当前线上 v1.20 老 V0.9 backend)
2. 在生产 `jst_erp` 库上跑 detail P99 复测 · 门 warm < 80ms
3. 删除测试库 `jst_erp_r3_test`(R3.5 起的镜像库,V1.1-A1 之后不再需要)
4. 从 `docs/api/openapi.yaml` 203 条 v1 path 生成**前端联调完整接口文档** 13 family 拆分 + 1 INDEX + 1 cheatsheet
5. 同步 ROADMAP / handoff manifest / onboarding 三件套

**必读先决条件**:接手前必须读完:

- `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`(authority)
- `docs/api/openapi.yaml`(15147 行 · 203 path · sha `b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f`)
- `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`(V1.1-A1 状态)
- `docs/iterations/V1_RETRO_REPORT.md`(R6.A.4 retro · §14 架构师裁决)
- `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md`(V1.1-A1 P99 收口报告)
- `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md`(前端联调入口)
- `prompts/V1_NEXT_MODEL_ONBOARDING.md`(接手包)
- `prompts/V1_ROADMAP.md`(§32 总览表 v1~v29 changelog)
- `deploy/deploy.sh` + `deploy/lib.sh` + `deploy/remote-deploy.sh`(release 流程)
- `scripts/r35/setup_test_db.sh` + `scripts/r35/build_test_dsn.sh`(测试库历史 · 删除前理解)
- `transport/http.go`(实际挂载的 v1 路由)

---

## §0.1 当前 baseline 锚(任何步骤启动前 step-1 必校)

### §0.1.1 当前线上版本

| 项 | 值 |
|---|---|
| 当前线上版本 | **v1.20** |
| 部署时间 | 2026-04-22T09:54:00Z |
| Artifact sha256 | `125a5d711cf7ed4f2cccccb642e1d0ae29c02c53677ba423522a38f5ad035279` |
| 部署目录 | `jst_ecs:/root/ecommerce_ai/releases/v1.20` |
| 线上 backend | **老 V0.9** (`Round W-3 detail read scope parity with list`) |
| 线上数据库 | `jst_erp` (通过 `/root/ecommerce_ai/shared/main.env` 5 变量拼 DSN) |

### §0.1.2 SHA 锚(7 业务 + OpenAPI · 任何 P 阶段都不可改 · 改了立即 ABORT)

```text
b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f  docs/api/openapi.yaml
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

V1.1-A1 4 个新改动文件 sha 也作为 baseline 不可再改:

```text
service/task_aggregator/detail_aggregator.go  (V1.1-A1)
repo/mysql/task_detail_bundle.go               (V1.1-A1)
service/identity_service.go                    (V1.1-A1)
repo/mysql/identity_actor_bundle.go            (V1.1-A1)
```

step-1 必校命令:

```bash
sha256sum \
  docs/api/openapi.yaml \
  service/asset_lifecycle/cleanup_job.go \
  service/task_draft/service.go \
  service/task_lifecycle/auto_archive_job.go \
  repo/mysql/task_auto_archive_repo.go \
  domain/task.go \
  cmd/server/main.go \
  service/task_aggregator/detail_aggregator.go \
  repo/mysql/task_detail_bundle.go \
  service/identity_service.go \
  repo/mysql/identity_actor_bundle.go \
  | tee tmp/v1_21_baseline_sha.log
```

任意 sha 与 §0.1.2 不一致 → **ABORT** · 不允许"顺手改一行"。

### §0.1.3 OpenAPI 路径数(203 · v1 family 统计)

```bash
grep -c '^  /v1' docs/api/openapi.yaml      # 期望 203(P1 step-1 必跑)
grep -c '^paths:' docs/api/openapi.yaml     # 期望 1
```

任意值偏离 → ABORT(说明 OpenAPI 被错改 · 违反 §0.1.2 sha 锚)。

### §0.1.4 测试库 / 正式库映射

| 库 | host | DSN 模板 | 段映射 |
|---|---|---|---|
| **正式 jst_erp** | jst_ecs(SSH tunnel from WSL `127.0.0.1:3306`)| `${DB_USER}:${DB_PASS}@tcp(${DB_HOST}:${DB_PORT})/jst_erp?charset=utf8mb4&parseTime=True&loc=Local` | 真实业务数据 · ID 范围 1 ~ 19999(用户/任务)· `[20000~89999)` 段在生产**应该全部为空** |
| **测试 jst_erp_r3_test** | jst_ecs(R3.5 起从 jst_erp dump 复制改名)| `root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true` | SA-A `[20000,30000)` / SA-B `[30000,40000)` / SA-C `[40000,50000)` / SA-D `[40000,50000)` / R5 `[50000,60000)` / R6.A.1 `[50000,60000)` / R6.A.3 `[60000,70000)` / R6.A.4 `[70000,80000)` / V1.1-A1 `[80000,90000)` |

V1.1-A1 之后所有 integration test 都已签字闭环 · 测试库没有继续测试需求 · 删除安全。

---

## §1 阶段 P1 · 本地预飞(打包前 must-pass)

### §1.1 baseline 校验

```bash
# step-1.1 sha 校验(见 §0.1.2)
# step-1.2 OpenAPI 路径数校验(见 §0.1.3)
# step-1.3 transport/http.go v1 mount 数(参考值 66 处 "/v1/" 字符串):
grep -c '"/v1/' transport/http.go
```

### §1.2 全栈 build / vet / unit / openapi-validate

```bash
go build ./...
go vet ./...
go test -count=1 ./...
go run ./cmd/tools/openapi-validate -file docs/api/openapi.yaml | tee tmp/v1_21_openapi.log
# 期望 "0 error 0 warning"
```

### §1.3 全栈 integration `-p 1` 复测

> **教训**:R6.A.4 v1.0 因默认包级并行 ABORT;v1.1 因 R2 backfill 60s outlier ABORT。本轮**强制** `-p 1 + set -o pipefail + timeout 60m`,与 R6.A.4 v1.2 + V1.1-A1 同基线。

```bash
set -o pipefail
go test -tags=integration -p 1 -count=1 -timeout 60m ./... 2>&1 \
  | tee tmp/v1_21_integration.log
# 任意 FAIL → ABORT
```

### §1.4 段隔离 step-1(测试库)

```bash
# 跑 [80000,90000) audit (V1.1-A1 段)· BEFORE/AFTER 9 表全 0
bash tmp/v1_1_a1_isolation_run.sh BEFORE
# (V1.1-A1 verify 已确认全 0 · 本轮再核 1 次以防新增运行污染)
```

### §1.5 dangling 501 检查

```bash
grep -n '"501"' docs/api/openapi.yaml | wc -l
# 期望 0
```

P1 任意失败 → ABORT 写 `docs/iterations/V1_RELEASE_v1_21_ABORT.md` 终止符 `V1_RELEASE_v1_21_ABORT_AT_P1`。

---

## §2 阶段 P2 · 打包 + 部署 v1.21

### §2.1 备份生产库(用户声明忽略风险 · 但本轮**仍强制**)

```bash
# 通过 ssh 在 jst_ecs 备份 jst_erp · 不阻塞 release · 5 分钟内可拿到 sql.gz
ssh jst_ecs '
  cd /root/ecommerce_ai
  . ./shared/main.env
  export MYSQL_PWD="$DB_PASS"
  TS=$(date -u +%Y%m%dT%H%M%SZ)
  mysqldump -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" --single-transaction --routines --events \
    --databases jst_erp \
    | gzip > "/root/ecommerce_ai/backups/${TS}_pre_v1_21_jst_erp.sql.gz"
  ls -lh "/root/ecommerce_ai/backups/${TS}_pre_v1_21_jst_erp.sql.gz"
'
```

备份产物路径:**记录到** `tmp/v1_21_pre_release_backup.log`。

### §2.2 打包 v1.21

```bash
bash deploy/deploy.sh \
  --version v1.21 \
  --release-note "V1.1-A1 detail P99 fast path: multi-result detail bundle + identity actor bundle + async route_access (cold p99=47.525ms / warm p99=47.513ms / final warm n=500 p99=47.126ms · architect-verified 2026-04-25)" \
  --runtime-env-file deploy/main.env \
  --bridge-env-file deploy/bridge.env
# deploy.sh 自动:package_release → scp → remote-deploy → verify-runtime
# 失败任一步 → release-history.log 落 status=failed → ABORT
```

> **如果本地未配 `~/.deploy.env`**(`DEPLOY_HOST/USER/PORT/BASE_DIR`):**ABORT** · 让架构师手补 env 后再跑。不要绕过。

### §2.3 部署后产物校验

```bash
# release-history.log 必须有 v1.21 status=deployed
tail -5 deploy/release-history.log | grep '|v1.21|.*|deployed|'
# artifact sha256 落盘
sha256sum dist/ecommerce-ai-v1.21-linux-amd64.tar.gz | tee tmp/v1_21_artifact_sha.log
```

### §2.4 远端 verify-runtime 自动跑(deploy.sh 自带 6 条 smoke)

deploy.sh 内置 `verify-runtime.sh --base-dir /root/ecommerce_ai --base-url http://127.0.0.1:8080 --bridge-url http://127.0.0.1:8081 --sync-url http://127.0.0.1:8082 --auto-recover-8082`,任一失败已落 `status=failed` 终止 release。

P2 失败 → ABORT 写 `docs/iterations/V1_RELEASE_v1_21_ABORT.md` · 终止符 `V1_RELEASE_v1_21_ABORT_AT_P2`。

---

## §3 阶段 P3 · 生产 detail P99 复测

### §3.1 选 200 条生产任务 ID

```bash
# SSH tunnel jst_erp → 选 200 条非 backfill_placeholder 的活跃任务
ssh jst_ecs '
  cd /root/ecommerce_ai
  . ./shared/main.env
  export MYSQL_PWD="$DB_PASS"
  mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" jst_erp -N -B -e "
    SELECT id FROM tasks
    WHERE id < 20000
      AND task_status NOT IN (\"Archived\")
    ORDER BY updated_at DESC
    LIMIT 200;
  "
' > tmp/v1_21_prod_task_ids.txt
wc -l tmp/v1_21_prod_task_ids.txt    # 期望 200
```

### §3.2 取生产 super_admin token

> 生产已存在 super admin · 通过 `/v1/auth/login` 获取 bearer token。账号见 jst_ecs `/root/ecommerce_ai/shared/main.env` 的 `BOOTSTRAP_SUPERADMIN_USERNAME` / `_PASSWORD`(架构师交付时已设置)。

```bash
# 在 WSL 经 jst_ecs 反向代理 8080 · 或直接 ssh tunnel 18080 → 8080
ssh -fNL 18080:127.0.0.1:8080 jst_ecs
TOKEN=$(curl -sS -X POST http://127.0.0.1:18080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$BOOTSTRAP_SUPERADMIN_USERNAME\",\"password\":\"$BOOTSTRAP_SUPERADMIN_PASSWORD\"}" \
  | jq -r '.token')
echo "$TOKEN" > tmp/v1_21_prod_token.txt
```

### §3.3 P99 跑 N=200 (warmup=50)

```bash
TASK_IDS_PATH=tmp/v1_21_prod_task_ids.txt \
SUPER_ADMIN_TOKEN=$(cat tmp/v1_21_prod_token.txt) \
BASE_URL=http://127.0.0.1:18080 \
WARMUP=50 N=200 \
go run tmp/v1_1_a1_p99_runner.go | tee tmp/v1_21_prod_p99.log
# 期望 warm p99 < 80ms · cold p99 < 150ms
```

### §3.4 ABORT 守卫

- warm p99 >= 80ms → ABORT(检查 DSN 是否启 `multiStatements=true` · 若未启 fast path 不生效退回 fallback 旧路径)
- 任意 non-200 → ABORT(检查 super admin / 路由挂载)

P3 失败 → ABORT 写 `docs/iterations/V1_RELEASE_v1_21_ABORT.md` · 终止符 `V1_RELEASE_v1_21_ABORT_AT_P3`。

---

## §4 阶段 P4 · 删除测试库 jst_erp_r3_test

### §4.1 备份测试库(用户声明忽略风险 · 但**仍强制 5 秒 mysqldump**)

```bash
ssh jst_ecs '
  cd /root/ecommerce_ai
  . ./shared/main.env
  export MYSQL_PWD="$DB_PASS"
  TS=$(date -u +%Y%m%dT%H%M%SZ)
  mysqldump -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" --single-transaction \
    --databases jst_erp_r3_test \
    | gzip > "/root/ecommerce_ai/backups/${TS}_pre_drop_jst_erp_r3_test.sql.gz"
  ls -lh "/root/ecommerce_ai/backups/${TS}_pre_drop_jst_erp_r3_test.sql.gz"
'
```

记录路径到 `tmp/v1_21_test_db_drop_backup.log`。

### §4.2 DROP DATABASE

```bash
ssh jst_ecs '
  cd /root/ecommerce_ai
  . ./shared/main.env
  export MYSQL_PWD="$DB_PASS"
  mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e "DROP DATABASE IF EXISTS \`jst_erp_r3_test\`;"
  mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e "SHOW DATABASES LIKE \"jst_erp_r3_test\";"
  # 第二个 SHOW 应返回空 (Empty set)
' | tee tmp/v1_21_test_db_drop.log
```

### §4.3 关停测试服务进程

```bash
# 18087 测试服务 V1.1-A1 时已停 · 复核没残留
pgrep -af 'cmd/server' | grep -v ':8080' || echo "no test server running"
# 18086 旧测试端口也清掉(若存在)
ssh jst_ecs 'ss -lntp | grep -E ":1808[6-7]" || echo "no test ports listening"'
```

### §4.4 不删的东西(保留重建能力)

- ✅ 保留 `scripts/r35/setup_test_db.sh`(架构师未来若要恢复测试库,15 分钟可重建)
- ✅ 保留 `scripts/r35/build_test_dsn.sh`
- ✅ 保留 `testsupport/r35/setup.go` + `cmd/tools/internal/v1migrate/dsn_guard_test.go`
- ✅ 保留 `tmp/v1_1_a1_isolation_run.sh` 等段隔离 verify 脚本
- ✅ 保留 4 份产线 backup `sql.gz`(`/root/ecommerce_ai/backups/*r2_pre_backfill*` 等)
- ✅ 保留 `R35_MODE` env 跳板路径(`identity_service.go` 未来若需再次创建测试 DSN)

理由:删测试库是**释放硬盘 + 解除段隔离心智负担**,不是"销毁能力"。

P4 失败 → ABORT 写 `docs/iterations/V1_RELEASE_v1_21_ABORT.md` · 终止符 `V1_RELEASE_v1_21_ABORT_AT_P4`。

---

## §5 阶段 P5 · 前端联调完整接口文档(13 family + INDEX + cheatsheet)

### §5.1 总产物目录

新建 `docs/frontend/` 目录,落 15 个 markdown:

| # | 文件 | 范围 | 路径数(估) |
|---|---|---|---|
| 0 | `docs/frontend/INDEX.md` | 入口索引 + base URL + 鉴权方式 + 错误码总表 + 联调起步 6 步 | — |
| 1 | `docs/frontend/V1_API_AUTH.md` | `/v1/auth/*`(register / login / logout / register-options / change-password / refresh)| ~6 |
| 2 | `docs/frontend/V1_API_ME.md` | `/v1/me*`(me / org / notifications / preferences / change-password)| ~10 |
| 3 | `docs/frontend/V1_API_USERS.md` | `/v1/users*`(list / create / patch / activate / deactivate / delete / designers)| ~15 |
| 4 | `docs/frontend/V1_API_ORG.md` | `/v1/org/*`(options / departments / move-requests)| ~6 |
| 5 | `docs/frontend/V1_API_TASKS.md` | `/v1/tasks*`(list / create / detail / cancel / 模块 claim/action/reassign/pool-reassign)| ~30 |
| 6 | `docs/frontend/V1_API_TASK_ASSETS.md` | `/v1/tasks/{id}/asset-center/*`(upload-sessions / list / detail / archive / restore)| ~12 |
| 7 | `docs/frontend/V1_API_ASSETS.md` | `/v1/assets*`(search / detail / preview / files / reference-upload)| ~10 |
| 8 | `docs/frontend/V1_API_DRAFTS.md` | `/v1/task-drafts*`(list / create / get / delete) + 7 天/20 条规则 | ~5 |
| 9 | `docs/frontend/V1_API_NOTIFICATIONS.md` | `/v1/me/notifications*`(list / mark-read / mark-all-read) + 5 类 NotificationType | ~5 |
| 10 | `docs/frontend/V1_API_BATCH.md` | `/v1/tasks/batch-create/*`(template.xlsx / parse-excel)+ Excel 字段约定 | ~2 |
| 11 | `docs/frontend/V1_API_ERP.md` | `/v1/erp/products*` + by-code | ~8 |
| 12 | `docs/frontend/V1_API_SEARCH.md` | `/v1/search`(全局)+ `/v1/assets/search` + `/v1/design-sources/search` | ~3 |
| 13 | `docs/frontend/V1_API_REPORTS.md` | `/v1/reports/l1/*`(throughput / module-dwell / cards · super_admin only)| ~4 |
| 14 | `docs/frontend/V1_API_WS.md` | `/v1/ws/v1` WebSocket(3 类 event · gws 协议) | ~1 |
| 15 | `docs/frontend/V1_API_CHEATSHEET.md` | 一页全 203 路径速查表(method + path + summary + RBAC · 一行一条) | 203 |

family 数总和:**203 ± 5(以 openapi.yaml 实际为准)**。

### §5.2 每条 path 必含字段(单条模板)

每条 path 在 family doc 里都必须有以下结构:

```markdown
## POST /v1/tasks

### 简介
中文简介,1~2 句话说明用途。

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)
- 允许角色:`super_admin`, `customer_service`, ...
- 字段级授权:无 / 详见 §5.4 (主权威文档)
- deny_code 候选:`task_create_field_denied_by_scope` / ...

### 请求体 schema
| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `task_type` | enum(`new_product_development`/`old_product_redo`) | 是 | 任务类型 |
| `priority` | int | 否 | 优先级 1~4(默认 2)|
| `reference_file_refs` | array<ReferenceFileRef> | 否 | 参考文件(走 `/v1/tasks/reference-upload` 拿 ref) |
| ... | ... | ... | ... |

### 响应体 schema(成功 200)
```json
{
  "id": 12345,
  "task_status": "InProgress",
  "modules": [...]
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int64 | 任务 ID |
| `task_status` | enum | 主状态 7 值:`InProgress/Completed/Cancelled/Archived/...` |
| ... | ... | ... |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | — | 未登录 / token 过期 |
| 403 | PERMISSION_DENIED | `task_create_field_denied_by_scope` | 字段越权 |
| 422 | VALIDATION_ERROR | — | 字段验证失败 |
| ... | ... | ... | ... |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"task_type":"new_product_development","priority":2}'
```

### 前端最佳实践
- 优先用 canonical 路径,不要走 `withCompatibilityRoute` 别名。
- 失败必须展示 `error.code` 或 `deny_code`(后端口径)。
- 详见 `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md` §5 联调硬门。
```

### §5.3 INDEX.md 必含节

- §0 base URL(生产 + 测试占位)+ 鉴权方式(Bearer token)
- §1 联调起步 6 步(login → me → tasks → detail → upload → batch)
- §2 错误码总表(401/403/404/409/422/500 + deny_code 列表 · 每条对应 family)
- §3 RBAC 角色矩阵(super_admin / hr_admin / customer_service / designer / retoucher / procurement / auditor / warehouse_keeper / member · 9 角色 × 主要权限点)
- §4 路由分类(canonical / compatibility / deprecated · 各列出)
- §5 详细 family 索引链接(15 个 doc)
- §6 联调硬门(引用 V1_1_FRONTEND_INTEGRATION_HANDOFF.md §5)
- §7 已知 deprecated 与 compatibility 路径清单(让前端绕开)

### §5.4 CHEATSHEET.md 模板

```markdown
# V1 API 速查表(203 路径 · 一行一条)

| Method | Path | Summary | RBAC | family doc |
|---|---|---|---|---|
| POST | /v1/auth/login | 登录 | 公开 | V1_API_AUTH.md |
| GET | /v1/me | 当前用户 | 已登录 | V1_API_ME.md |
| GET | /v1/tasks | 任务列表 | scope-aware | V1_API_TASKS.md |
| GET | /v1/tasks/{id}/detail | 任务详情(P99 47ms · 一屏聚合) | scope-aware | V1_API_TASKS.md |
| ... 共 203 行 ... |
```

### §5.5 数据来源 + 校验

- 来源:**仅** `docs/api/openapi.yaml`(15147 行 · 不读其他 markdown · 不复制示例值 · 一切以 openapi 为准)
- 中文字段说明:从 `docs/V1_MODULE_ARCHITECTURE.md` / `docs/V1_INFORMATION_ARCHITECTURE.md` / `docs/V1_ASSET_OWNERSHIP.md` / `docs/V1_CUSTOMIZATION_WORKFLOW.md` 4 份权威文档摘录(标注引用源)
- 错误码:从 `domain/errors.go` + handler 各处的 deny_code 字符串收集
- RBAC:从 `policy/*.go` + handler `requireRole` 调用收集

### §5.6 校验工具(P5 step-N)

```bash
# 5.6.1 family 路径数 sum 应 = 203 ± 5
grep -E '^## (GET|POST|PUT|DELETE|PATCH) /v1/' docs/frontend/V1_API_*.md | wc -l

# 5.6.2 INDEX 必含 §0~§7 八节
grep -c '^## §[0-7]' docs/frontend/INDEX.md  # 期望 >= 8

# 5.6.3 CHEATSHEET 行数应接近 203 + 表头 2 = 205
wc -l docs/frontend/V1_API_CHEATSHEET.md  # 期望 >= 210

# 5.6.4 每条 path 必含 4 节(简介/鉴权/请求体/响应体/错误码/curl/最佳实践)
# codex 自查:每个 V1_API_*.md 用 awk 统计 "### " 子节数,每条 path 至少 5 个子节
```

P5 任意失败 → ABORT 写 `docs/iterations/V1_RELEASE_v1_21_ABORT.md` · 终止符 `V1_RELEASE_v1_21_ABORT_AT_P5`。

---

## §6 阶段 P6 · 文档同步 + 终止符

### §6.1 ROADMAP changelog 追加 v30 / v31 / v32

`prompts/V1_ROADMAP.md` 状态行(行 3)更新到:

```text
> 状态:**... · V1.1-A1 + Release v1.21 + 测试库已删除 + 前端 API 文档 15 份产物 · architect-verified · 前端联调起步**
```

§32 表新增 1 行 `Release-v1.21`,changelog 追加 3 条:

```text
v30 (2026-04-25) · Release v1.21 部署:V1.0(R1~R6.A.4) + V1.1-A1 后端首次上线 · jst_ecs:/root/ecommerce_ai/releases/v1.21 · 替换 v1.20 老 V0.9 backend · artifact sha256=<填> · 生产 detail P99 warm <填> ms · architect-verified
v31 (2026-04-25) · 测试库 jst_erp_r3_test DROP · 备份在 jst_ecs:/root/ecommerce_ai/backups/<TS>_pre_drop_jst_erp_r3_test.sql.gz · 段隔离心智负担解除 · 重建路径保留 (scripts/r35/setup_test_db.sh)
v32 (2026-04-25) · 前端联调完整接口文档 15 份产物落盘 docs/frontend/ · 203 path 全覆盖 · 13 family + INDEX + CHEATSHEET · 与 openapi.yaml 1:1 对齐 · 前端联调首轮门已开
```

### §6.2 V1_TO_V2_MODEL_HANDOFF_MANIFEST.md 同步

新增 §release evidence 节:

```markdown
## §<n> Release v1.21 · 2026-04-25
- 当前线上版本:**v1.21**(替换 v1.20 老 V0.9)
- 部署目录:jst_ecs:/root/ecommerce_ai/releases/v1.21
- artifact sha256:<填>
- 生产 jst_erp 首次跑 V1 backend
- 生产 detail P99 warm:<填> ms(门 < 80ms)· cold:<填> ms(门 < 150ms)
- 测试库 jst_erp_r3_test 已 DROP(备份 sql.gz 路径见 ROADMAP v31)
- 前端联调入口:docs/frontend/INDEX.md(15 份 family doc + cheatsheet)
```

### §6.3 V1_NEXT_MODEL_ONBOARDING.md 同步

§3.2 段隔离 Map 顶部追加:

```markdown
> 注:测试库 `jst_erp_r3_test` 已于 2026-04-25 DROP(详见 ROADMAP v31)。本表保留作为历史段映射记录;若未来需重建测试库,跑 `scripts/r35/setup_test_db.sh` 即可恢复。
```

§6 候选下一轮顶部追加:

```markdown
> ✅ 已完成:Release v1.21(2026-04-25)+ 前端 API 文档(`docs/frontend/`)。前端联调首轮门已开。下一轮候选见下表。
```

### §6.4 V1_1_FRONTEND_INTEGRATION_HANDOFF.md 同步

§1 Backend Status 块追加:

```text
released_v1_21=2026-04-25T<填>Z
prod_detail_p99_warm_ms=<填>
prod_detail_p99_cold_ms=<填>
prod_db=jst_erp
test_db_jst_erp_r3_test=DROPPED
frontend_docs=docs/frontend/INDEX.md (15 files / 203 paths)
```

### §6.5 终止符

`docs/iterations/V1_RELEASE_v1_21_REPORT.md`(新建 · 完整记录 5 阶段 + 13 项 verify + 架构师 self-check):

```markdown
# Release v1.21 Report · 2026-04-25

## §0 Verdict
V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY

## §1 Baseline (P1 step-1)
... sha 锚 11 文件 / OpenAPI 203 path / integration -p 1 PASS ...

## §2 Release v1.21 (P2)
... artifact sha256 / deploy-history line / verify-runtime PASS ...

## §3 Production P99 (P3)
... N=200 warmup=50 / cold p99 / warm p99 / max ...

## §4 Test DB Drop (P4)
... backup path / DROP confirm / SHOW DATABASES empty ...

## §5 Frontend Docs (P5)
... 15 files / 203 path / 校验工具实测 ...

## §6 Sync (P6)
... ROADMAP v30/v31/v32 / handoff manifest / onboarding / frontend-handoff ...

## §7 终止符
V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY
```

成功条件 = 13 项 verify 全 PASS + 5 个文档全同步 + 终止符落盘。

---

## §7 验证矩阵(13 项 · 一项不绿即 ABORT)

| # | 项 | 命令 / 期望 |
|---|---|---|
| V1 | sha 锚 11 文件 | §0.1.2 校验 → 与 baseline 一致 |
| V2 | OpenAPI 路径数 = 203 | §0.1.3 校验 |
| V3 | go build/vet PASS | §1.2 |
| V4 | go test -count=1 PASS | §1.2 |
| V5 | openapi-validate `0 error 0 warning` | §1.2 |
| V6 | full integration `-p 1 -timeout 60m` 0 FAIL | §1.3 |
| V7 | 段 [80000,90000) BEFORE/AFTER 9 表全 0 | §1.4 |
| V8 | dangling 501 = 0 | §1.5 |
| V9 | release-history.log v1.21 status=deployed | §2.3 |
| V10 | 生产 detail P99 warm < 80ms · cold < 150ms | §3.3 |
| V11 | jst_erp_r3_test SHOW 为空 | §4.2 |
| V12 | docs/frontend/ 15 文件齐 + 203 path 覆盖 | §5.6 |
| V13 | ROADMAP v30/v31/v32 + handoff + onboarding + frontend-handoff 全同步 | §6.1~6.4 |

每项产物落 `tmp/v1_21_verify_<#>.log`,最后归集到 `tmp/v1_21_arch_verify.log`。

---

## §8 ABORT 触发器(8 条 · 任意命中立即终止 · 不允许尝试自修)

1. **§0.1.2 sha 锚**任意文件被改 → ABORT_AT_BASELINE
2. **§0.1.3 OpenAPI 路径数**偏离 203 → ABORT_AT_BASELINE
3. **§1.3 integration `-p 1`**任意 FAIL → ABORT_AT_P1
4. **§2.2 deploy.sh** `release-history.log status=failed` 或 `verify-runtime` 任一冒烟红 → ABORT_AT_P2
5. **§3.3 生产 P99** warm >= 80ms 或 任意 non-200 → ABORT_AT_P3
6. **§4.1 mysqldump 失败**(disk 满 / 权限错)→ ABORT_AT_P4(不允许跳过备份直接 DROP · 即使用户说"忽略数据风险")
7. **§5.6 校验**任一项 FAIL → ABORT_AT_P5
8. **§6 任意文档同步缺失**或 sha 锚被错改 → ABORT_AT_P6

ABORT 时:

- 写 `docs/iterations/V1_RELEASE_v1_21_ABORT.md`(列出 ABORT 阶段、原因、已做的步、下一步建议)
- 终止符 `V1_RELEASE_v1_21_ABORT_AT_<P>`
- **不要**编辑 ROADMAP / handoff manifest / onboarding / frontend-handoff(等架构师裁决再决定如何 backup / proceed / abandon)
- **不要**自签 PASS

---

## §9 工作时间估算

- P1 baseline + integration:30~40 min(integration 318s · 加 sha + openapi-validate)
- P2 打包 + 部署:15~20 min(deploy.sh 自带 verify-runtime)
- P3 生产 P99:5~10 min(N=200 + warmup=50)
- P4 备份 + DROP:5 min
- P5 前端文档:**最长** · 80~120 min(203 path × 单条 5 字段需要细致中文化 · 不要赶)
- P6 文档同步:15~20 min

**总计**:2.5~3.5 小时(单 codex TUI / exec autopilot 一气呵成)。

---

## §10 工作模式建议

- **codex TUI**:适合 P1~P4(短反馈循环 · 容易调 ssh / mysqldump / deploy.sh 出错)
- **codex exec autopilot**:适合 P5(大量 markdown 写作 · 一次性产出 15 文件 · TUI 反而频繁 confirm)
- **二选一**:用户决定 · 默认推荐 TUI(P1~P4)+ autopilot(P5)分两轮

---

## §11 红线(铁律)

- **不要**在 P1~P4 完成前开始写前端文档(P5 依赖 P3 实测 P99 数据)
- **不要**在生产 jst_erp 上跑 integration 测试(测试库已删 · 没有 fallback)
- **不要**绕过 §0.1.2 sha 锚("反正 V1.1-A1 已签字了顺手优化下"= 立即 ABORT)
- **不要**在 frontend doc 里编造 openapi.yaml 没有的字段(从 §5.5 数据源摘 · 不补脑)
- **不要**改 4 份 V1 权威文档(主 / IA / 资产 / 定制)
- **不要**改 `transport/http.go` 路由挂载(本轮零 routing 改动)
- **不要**移除 `scripts/r35/*` 测试库重建脚本(保留重建能力)
- **不要**删 `R35_MODE` env 跳板(identity_service.go 历史路径)

---

## §12 起手 SOP

1. **step-1**:跑 §0.1.2 sha + §0.1.3 path count → 全绿才进 P1
2. **step-2**:P1 §1.1~1.5 全绿 → 进 P2
3. **step-3**:P2 deploy.sh → 5 分钟 watch release-history.log
4. **step-4**:P3 生产 P99 → 写 `tmp/v1_21_prod_p99.log`
5. **step-5**:P4 备份 + DROP → 等 mysqldump 完成再 DROP
6. **step-6**:P5 写 15 文件 → 自校 §5.6
7. **step-7**:P6 同步 4 文档 + 写 release report
8. **step-8**:终止符 + 13 项 verify log 归集 + 报告 closing

---

## §13 终止符

成功:`V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY`

ABORT(任意阶段):`V1_RELEASE_v1_21_ABORT_AT_<P1|P2|P3|P4|P5|P6>`

---

## §14 用户原始诉求(留底)

> "查询现在线上版本号以及现在开始切换正式数据库并删除测试库的方案,忽略数据风险,并最后给出 prompt 让 codex 执行后给出前端同事具体的联调文档,我需要完整的接口文档给到前端"

架构师拆解:

1. ✅ 查询当前线上版本号 → §0.1.1 已答(v1.20)
2. ✅ 切换正式数据库 → 解释为"把 V1.1-A1 后端发布为 v1.21 部署到生产 jst_erp"(P1~P3)
3. ✅ 删除测试库 → P4 (mysqldump 5 秒留底 + DROP)
4. ✅ 忽略数据风险 → 接受 · 但**强制 mysqldump**(5 秒 · 防误操作)
5. ✅ codex prompt → 本文件
6. ✅ 完整接口文档 → P5 · 15 份 markdown · 203 path 全覆盖 · 单条 7 字段(简介/鉴权/请求/响应/错误/curl/最佳实践)

---

终止符提醒:本 prompt 落盘后,等待用户决定是 codex TUI 还是 autopilot 跑。架构师不替 codex 执行 P1~P6。
